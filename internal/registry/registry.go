// Package registry reads and writes registry.json, the generated component
// index built from installed plugin manifests. It is never hand-edited. Writes
// are atomic and entry upserts are idempotent.
package registry

import (
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
	"github.com/gustaborges/work/internal/plugin"
)

// Component is one registered component. Identity is (Alias, Name). The
// activation fields (On, Manual, Inputs, Key, Discover) are recorded for
// Importers and Linkers; a component registered before they existed has none
// and is never eligible until it is reinstalled.
type Component struct {
	Alias        string   `json:"alias"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Entrypoint   string   `json:"entrypoint"`
	Runtime      string   `json:"runtime,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	StarterLayer string   `json:"starter_layer,omitempty"`
	Accepts      []string `json:"accepts,omitempty"`
	DisplayName  string   `json:"display_name,omitempty"`
	Description  string   `json:"description,omitempty"`

	On       []plugin.Subscription `json:"on,omitempty"`
	Manual   *plugin.Manual        `json:"manual,omitempty"`
	Inputs   []string              `json:"inputs,omitempty"` // importer inputs, or a linker's discover.inputs
	Key      string                `json:"key,omitempty"`
	Discover *plugin.Discover      `json:"discover,omitempty"`
}

// EntrypointPath resolves the component's entrypoint inside a plugins
// directory: <pluginsDir>/<alias>/source/<entrypoint>. On Windows a native
// executable gets a ".exe" suffix; a script run by a declared runtime keeps
// its name, since it is not itself executable.
func (c Component) EntrypointPath(pluginsDir string) string {
	p := filepath.Join(pluginsDir, c.Alias, "source", c.Entrypoint)
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

// QualifiedName is "<alias>/<name>", the component's unambiguous identity in
// ordering, provenance and warnings.
func (c Component) QualifiedName() string {
	return c.Alias + "/" + c.Name
}

// Convention is one registered branch convention. Identity is Name.
type Convention struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

// Package is one installed plugin package's record (F4). Identity is Alias.
// Every Component registered from this package carries the same Alias —
// Package is a new parent record, not a new identity scheme. Conventions
// names the convention names this package's manifest declared, for `work
// plugin list`: registry.Convention itself carries no Alias (its identity
// stays a bare Name, unchanged — two packages declaring the same convention
// name are independent catalog data, never disambiguated the way component
// names are), so Package is where a package's own declared convention names
// are recorded.
type Package struct {
	Alias       string   `json:"alias"`
	Origin      string   `json:"origin"`                // one of the Origin* constants
	Reference   string   `json:"reference"`             // absolute source path (local kinds), or "<source>@<sha>" (remote)
	Conventions []string `json:"conventions,omitempty"` // convention names this package's manifest declared
	PluginName  string   `json:"plugin_name,omitempty"` // manifest name; the alias stands in when empty
}

// Registry is the whole registry.json document.
type Registry struct {
	Components  []Component  `json:"components"`
	Conventions []Convention `json:"conventions"`
	Packages    []Package    `json:"packages,omitempty"`
}

// Roles / layers used by queries.
const (
	RoleStarter           = "starter"
	RoleRepositoryLocator = "repository-locator"
	LayerFallback         = "fallback"
)

// Package origin kinds (F4, ADR-0002).
const (
	OriginLocalLinked  = "local-linked"
	OriginLocalPinned  = "local-pinned"
	OriginRemotePinned = "remote-pinned"
)

// Load reads registry.json. A missing file yields an empty Registry.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Registry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: reading %s: %w", path, err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("registry: %s is not valid JSON: %w", path, err)
	}
	return &r, nil
}

// Save writes r to path atomically.
func Save(path string, r *Registry) error {
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

// UpsertComponent inserts or replaces the entry keyed by (Alias, Name).
func (r *Registry) UpsertComponent(c Component) {
	for i := range r.Components {
		if r.Components[i].Alias == c.Alias && r.Components[i].Name == c.Name {
			r.Components[i] = c
			return
		}
	}
	r.Components = append(r.Components, c)
}

// UpsertConvention inserts or replaces the entry keyed by Name.
func (r *Registry) UpsertConvention(c Convention) {
	for i := range r.Conventions {
		if r.Conventions[i].Name == c.Name {
			r.Conventions[i] = c
			return
		}
	}
	r.Conventions = append(r.Conventions, c)
}

// RemoveAlias retracts everything alias registered: its components, its
// Package record, and each convention that Package declared unless another
// Package still declares it. Called before a reinstall registers the new
// manifest, so nothing the previous version declared outlives it.
func (r *Registry) RemoveAlias(alias string) {
	r.Components = slices.DeleteFunc(r.Components, func(c Component) bool { return c.Alias == alias })

	var declared []string
	r.Packages = slices.DeleteFunc(r.Packages, func(p Package) bool {
		if p.Alias != alias {
			return false
		}
		declared = p.Conventions
		return true
	})
	for _, name := range declared {
		stillDeclared := slices.ContainsFunc(r.Packages, func(p Package) bool {
			return slices.Contains(p.Conventions, name)
		})
		if !stillDeclared {
			r.Conventions = slices.DeleteFunc(r.Conventions, func(c Convention) bool { return c.Name == name })
		}
	}
}

// UpsertPackage inserts or replaces the entry keyed by Alias.
func (r *Registry) UpsertPackage(p Package) {
	for i := range r.Packages {
		if r.Packages[i].Alias == p.Alias {
			r.Packages[i] = p
			return
		}
	}
	r.Packages = append(r.Packages, p)
}

// PackageByAlias returns the package registered under alias.
func (r *Registry) PackageByAlias(alias string) (Package, bool) {
	for _, p := range r.Packages {
		if p.Alias == alias {
			return p, true
		}
	}
	return Package{}, false
}

// PluginNameOf returns the manifest name of the plugin installed under alias.
// It differs from the alias when the user installed with --as, and private
// Semantic Conventions keys are owned by the manifest name. A package recorded
// before the name was stored falls back to the alias.
func (r *Registry) PluginNameOf(alias string) string {
	if p, ok := r.PackageByAlias(alias); ok && p.PluginName != "" {
		return p.PluginName
	}
	return alias
}

// ListPackages returns a copy of the registered packages, sorted by alias.
func (r *Registry) ListPackages() []Package {
	out := slices.Clone(r.Packages)
	slices.SortFunc(out, func(a, b Package) int { return strings.Compare(a.Alias, b.Alias) })
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

// ConventionByName returns the named convention.
func (r *Registry) ConventionByName(name string) (Convention, bool) {
	for _, c := range r.Conventions {
		if c.Name == name {
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

// HasComponent reports whether an entry keyed by (alias, name) exists.
func (r *Registry) HasComponent(alias, name string) bool {
	for _, c := range r.Components {
		if c.Alias == alias && c.Name == name {
			return true
		}
	}
	return false
}
