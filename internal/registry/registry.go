// Package registry reads and writes registry.json, the generated component
// index built from installed plugin manifests. It is never hand-edited. Writes
// are atomic, entry upserts are idempotent, and the one supported shape is
// strictly validated on load: every package, component and convention belongs
// to a composed (marketplace, plugin) provider.
package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/jsonx"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
)

// Schema is the only supported registry.json schema.
const Schema = 1

// InstallModeCopy is the only installation mode: every package executes from an
// independent managed copy of one selected marketplace entry.
const InstallModeCopy = "copy"

// DigestPackageV1 names the package digest algorithm recorded with a package.
const DigestPackageV1 = "work-package-v1"

// Component is one registered component. Identity is (Provider, Name). The
// activation fields (On, Manual, Inputs, Key, Discover) are recorded for
// Importers and Linkers.
type Component struct {
	Revision     string            `json:"-"` // transient package baseline captured by adminstate.Read
	Provider     pluginid.Provider `json:"provider"`
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	Entrypoint   string            `json:"entrypoint"`
	Runtime      string            `json:"runtime,omitempty"`
	Pattern      string            `json:"pattern,omitempty"`
	StarterLayer string            `json:"starter_layer,omitempty"`
	Accepts      []string          `json:"accepts,omitempty"`
	DisplayName  string            `json:"display_name,omitempty"`
	Description  string            `json:"description,omitempty"`

	On       []plugin.Subscription `json:"on,omitempty"`
	Manual   *plugin.Manual        `json:"manual,omitempty"`
	Inputs   []string              `json:"inputs,omitempty"` // importer inputs, or a linker's discover.inputs
	Key      string                `json:"key,omitempty"`
	Discover *plugin.Discover      `json:"discover,omitempty"`
}

// Ref is the component's fully qualified contribution identity.
func (c Component) Ref() pluginid.Ref { return pluginid.Ref{Provider: c.Provider, Name: c.Name} }

// EntrypointPath resolves the component's entrypoint inside a plugins
// directory: <pluginsDir>/<marketplace>/<plugin>/source/<entrypoint>. On
// Windows a native executable gets a ".exe" suffix; a script run by a declared
// runtime keeps its name, since it is not itself executable.
func (c Component) EntrypointPath(pluginsDir string) string {
	p := filepath.Join(pluginsDir, c.Provider.Marketplace, c.Provider.Plugin, "source", c.Entrypoint)
	if runtime.GOOS == "windows" && c.Runtime == "" {
		p += ".exe"
	}
	return p
}

// Target says how to start the component: through its declared runtime when
// it has one, directly otherwise.
func (c Component) Target(pluginsDir string) ipc.Target {
	return ipc.Target{Runtime: c.Runtime, Path: c.EntrypointPath(pluginsDir)}
}

// QualifiedName is "<plugin>@<marketplace>/<name>", the component's
// unambiguous identity in ordering, provenance and warnings.
func (c Component) QualifiedName() string { return c.Ref().String() }

// Convention is one registered branch convention. Identity is (Provider, Name):
// equal names from different providers are independent records.
type Convention struct {
	Provider pluginid.Provider `json:"provider"`
	Name     string            `json:"name"`
	Prefixes []string          `json:"prefixes"`
}

// Ref is the convention's fully qualified identity.
func (c Convention) Ref() pluginid.Ref { return pluginid.Ref{Provider: c.Provider, Name: c.Name} }

// Provenance is the actual fixed source fact an installed copy came from. Kind
// says where the offer was published (git, local or builtin), never how the
// copy was installed.
type Provenance struct {
	RegistrationID  string `json:"registration_id"`
	Kind            string `json:"kind"`
	Source          string `json:"source"`
	Branch          string `json:"branch,omitempty"`
	MarketplaceName string `json:"marketplace_name"`
	PluginName      string `json:"plugin_name"`
	Path            string `json:"path"`
	Revision        string `json:"revision"`
	CatalogDigest   string `json:"catalog_digest"`
}

// Package is one installed plugin package's record. Identity is Provider.
type Package struct {
	Provider        pluginid.Provider `json:"provider"`
	InstallMode     string            `json:"install_mode"`
	Enabled         bool              `json:"enabled"`
	Version         string            `json:"version"` // opaque; compared only for equality
	ContentDigest   string            `json:"content_digest"`
	DigestAlgorithm string            `json:"digest_algorithm"`
	Provenance      Provenance        `json:"provenance"`
}

// ID is the public plugin@marketplace identity.
func (p Package) ID() string { return p.Provider.String() }

// Registry is the whole registry.json document.
type Registry struct {
	Schema      int          `json:"schema"`
	Packages    []Package    `json:"packages"`
	Components  []Component  `json:"components"`
	Conventions []Convention `json:"conventions"`
}

// New returns an empty current-schema registry.
func New() *Registry {
	return &Registry{Schema: Schema, Packages: []Package{}, Components: []Component{}, Conventions: []Convention{}}
}

// Roles / layers used by queries.
const (
	RoleStarter           = "starter"
	RoleRepositoryLocator = "repository-locator"
	LayerFallback         = "fallback"
)

// Validate applies the strict current-schema rules, including referential
// integrity between packages, components and conventions.
func (r *Registry) Validate() error {
	if r.Schema != Schema {
		return fmt.Errorf("unsupported registry schema %d", r.Schema)
	}
	pkgs := map[pluginid.Provider]bool{}
	for _, p := range r.Packages {
		if err := p.Provider.Validate(); err != nil {
			return fmt.Errorf("package %s: %w", p.Provider, err)
		}
		if pkgs[p.Provider] {
			return fmt.Errorf("package %s is recorded twice", p.ID())
		}
		pkgs[p.Provider] = true
		if err := p.validate(); err != nil {
			return fmt.Errorf("package %s: %w", p.ID(), err)
		}
	}
	comps := map[pluginid.Ref]bool{}
	for _, c := range r.Components {
		if !pkgs[c.Provider] {
			return fmt.Errorf("component %s has no installed package", c.QualifiedName())
		}
		if _, err := pluginid.NewRef(c.Provider, c.Name); err != nil {
			return fmt.Errorf("component %s: %w", c.QualifiedName(), err)
		}
		if comps[c.Ref()] {
			return fmt.Errorf("component %s is recorded twice", c.QualifiedName())
		}
		comps[c.Ref()] = true
		if c.Role == "" || c.Entrypoint == "" {
			return fmt.Errorf("component %s lacks role or entrypoint", c.QualifiedName())
		}
	}
	convs := map[pluginid.Ref]bool{}
	for _, c := range r.Conventions {
		if !pkgs[c.Provider] {
			return fmt.Errorf("convention %s has no installed package", c.Ref())
		}
		if _, err := pluginid.NewRef(c.Provider, c.Name); err != nil {
			return fmt.Errorf("convention %s: %w", c.Ref(), err)
		}
		if convs[c.Ref()] {
			return fmt.Errorf("convention %s is recorded twice", c.Ref())
		}
		convs[c.Ref()] = true
	}
	return nil
}

func (p Package) validate() error {
	if p.InstallMode != InstallModeCopy {
		return fmt.Errorf("unsupported install mode %q", p.InstallMode)
	}
	if p.Version == "" || p.ContentDigest == "" || p.DigestAlgorithm != DigestPackageV1 {
		return errors.New("version, content digest and digest algorithm are required")
	}
	pv := p.Provenance
	if err := pluginid.ValidateRegistrationID(pv.RegistrationID); err != nil {
		return err
	}
	switch pv.Kind {
	case pluginid.KindGit:
		if pv.Branch == "" {
			return errors.New("git provenance records its branch")
		}
	case pluginid.KindLocal:
		if pv.Branch != "" {
			return errors.New("local provenance has no branch")
		}
	case pluginid.KindBuiltin:
		if pv.RegistrationID != pluginid.BuiltinRegistrationID || p.Provider.Marketplace != pluginid.Builtin {
			return errors.New("builtin provenance belongs to the builtin provider")
		}
	default:
		return fmt.Errorf("unsupported provenance kind %q", pv.Kind)
	}
	if (pv.Kind == pluginid.KindBuiltin) != (pv.RegistrationID == pluginid.BuiltinRegistrationID) {
		return errors.New("registration id does not match provenance kind")
	}
	if pv.MarketplaceName != p.Provider.Marketplace || pv.PluginName != p.Provider.Plugin {
		return errors.New("provenance names do not match the provider")
	}
	if pv.Source == "" || pv.Path == "" || pv.Revision == "" || pv.CatalogDigest == "" {
		return errors.New("incomplete provenance")
	}
	return nil
}

// Load reads registry.json. A missing file yields an empty Registry; anything
// that is not the current strict shape is an error.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: reading %s: %w", path, err)
	}
	r, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("registry: %s is invalid: %w", path, err)
	}
	return r, nil
}

// Decode parses and validates registry.json bytes.
func Decode(data []byte) (*Registry, error) {
	var r Registry
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	if jsonx.TrailingData(dec) {
		return nil, errors.New("trailing data")
	}
	if r.Packages == nil {
		r.Packages = []Package{}
	}
	if r.Components == nil {
		r.Components = []Component{}
	}
	if r.Conventions == nil {
		r.Conventions = []Convention{}
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// Save writes r to path atomically.
func Save(path string, r *Registry) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: creating state dir: %w", err)
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return fmt.Errorf("registry: writing %s: %w", path, err)
	}
	return nil
}

// UpsertComponent inserts or replaces the entry keyed by (Provider, Name).
func (r *Registry) UpsertComponent(c Component) {
	for i := range r.Components {
		if r.Components[i].Provider == c.Provider && r.Components[i].Name == c.Name {
			r.Components[i] = c
			return
		}
	}
	r.Components = append(r.Components, c)
}

// UpsertConvention inserts or replaces the entry keyed by (Provider, Name).
// Equal names from different providers stay distinct records.
func (r *Registry) UpsertConvention(c Convention) {
	for i := range r.Conventions {
		if r.Conventions[i].Provider == c.Provider && r.Conventions[i].Name == c.Name {
			r.Conventions[i] = c
			return
		}
	}
	r.Conventions = append(r.Conventions, c)
}

// RemoveProvider retracts everything the provider registered: its components,
// its conventions and its Package record. Called before a replacement
// registers the new manifest, so nothing the previous version declared
// outlives it.
func (r *Registry) RemoveProvider(p pluginid.Provider) {
	r.Components = slices.DeleteFunc(r.Components, func(c Component) bool { return c.Provider == p })
	r.Conventions = slices.DeleteFunc(r.Conventions, func(c Convention) bool { return c.Provider == p })
	r.Packages = slices.DeleteFunc(r.Packages, func(pk Package) bool { return pk.Provider == p })
}

// UpsertPackage inserts or replaces the entry keyed by Provider.
func (r *Registry) UpsertPackage(p Package) {
	for i := range r.Packages {
		if r.Packages[i].Provider == p.Provider {
			r.Packages[i] = p
			return
		}
	}
	r.Packages = append(r.Packages, p)
}

// PackageByProvider returns the package registered for p.
func (r *Registry) PackageByProvider(p pluginid.Provider) (Package, bool) {
	for _, pk := range r.Packages {
		if pk.Provider == p {
			return pk, true
		}
	}
	return Package{}, false
}

// ListPackages returns a copy of the registered packages, sorted by
// marketplace then plugin.
func (r *Registry) ListPackages() []Package {
	out := slices.Clone(r.Packages)
	slices.SortFunc(out, func(a, b Package) int { return compareProviders(a.Provider, b.Provider) })
	return out
}

func compareProviders(a, b pluginid.Provider) int {
	if c := strings.Compare(a.Marketplace, b.Marketplace); c != 0 {
		return c
	}
	return strings.Compare(a.Plugin, b.Plugin)
}

// ProviderComponents returns the provider's components in registration order.
func (r *Registry) ProviderComponents(p pluginid.Provider) []Component {
	var out []Component
	for _, c := range r.Components {
		if c.Provider == p {
			out = append(out, c)
		}
	}
	return out
}

// ProviderConventions returns the provider's conventions in registration order.
func (r *Registry) ProviderConventions(p pluginid.Provider) []Convention {
	var out []Convention
	for _, c := range r.Conventions {
		if c.Provider == p {
			out = append(out, c)
		}
	}
	return out
}

// ByRole returns every component with the given role.
func (r *Registry) ByRole(role string) []Component {
	var out []Component
	for _, c := range r.Components {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

// Extensions returns the Importers or Linkers registered with the given role.
// Any other role yields nothing: Starters and Locators are not extensions.
func (r *Registry) Extensions(role string) []Component {
	if role != plugin.RoleImporter && role != plugin.RoleLinker {
		return nil
	}
	return r.ByRole(role)
}

// ListConventions returns a copy of the registered conventions.
func (r *Registry) ListConventions() []Convention {
	return slices.Clone(r.Conventions)
}

// ConventionsNamed returns every provider's convention with the given bare
// name, in registration order. More than one result means the bare name is
// ambiguous.
func (r *Registry) ConventionsNamed(name string) []Convention {
	var out []Convention
	for _, c := range r.Conventions {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// ConventionByRef returns the convention with the exact owner and name.
func (r *Registry) ConventionByRef(ref pluginid.Ref) (Convention, bool) {
	for _, c := range r.Conventions {
		if c.Provider == ref.Provider && c.Name == ref.Name {
			return c, true
		}
	}
	return Convention{}, false
}

// StarterFallback returns the single fallback-layer starter, if one is
// registered.
func (r *Registry) StarterFallback() (Component, bool) {
	for _, c := range r.Components {
		if c.Role == RoleStarter && c.StarterLayer == LayerFallback {
			return c, true
		}
	}
	return Component{}, false
}

// HasComponent reports whether the exact contribution is registered.
func (r *Registry) HasComponent(ref pluginid.Ref) bool {
	for _, c := range r.Components {
		if c.Provider == ref.Provider && c.Name == ref.Name {
			return true
		}
	}
	return false
}

// PackageEnabled reports the provider's retained enablement. An unknown
// provider is not enabled.
func (r *Registry) PackageEnabled(p pluginid.Provider) bool {
	pk, ok := r.PackageByProvider(p)
	return ok && pk.Enabled
}

// Effective returns a separate catalog for activation and availability that
// contains only enabled providers' records.
func (r *Registry) Effective() *Registry {
	out := New()
	for _, p := range r.Packages {
		if p.Enabled {
			out.Packages = append(out.Packages, p)
		}
	}
	for _, c := range r.Components {
		if r.PackageEnabled(c.Provider) {
			out.Components = append(out.Components, c)
		}
	}
	for _, c := range r.Conventions {
		if r.PackageEnabled(c.Provider) {
			out.Conventions = append(out.Conventions, c)
		}
	}
	return out
}
