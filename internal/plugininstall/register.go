// Package plugininstall turns a validated plugin manifest into registry
// records. It has no publishing API of its own: new packages reach the managed
// plugins directory only through a resolved marketplace entry, and this package
// supplies the one place a manifest becomes registry declarations so every
// publisher records identical activation data.
package plugininstall

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

// Result is what a registration produced, for the caller to render.
type Result struct {
	Package    registry.Package
	Components []plugin.Component
}

// Spec is everything needed to record one installed package.
type Spec struct {
	Provider      pluginid.Provider
	Manifest      *plugin.Manifest
	Enabled       bool
	ContentDigest string
	Provenance    registry.Provenance
}

// Register replaces everything the provider registered before with the
// manifest's declarations, so nothing a previous version declared outlives it.
// It validates identity facts but not content: callers have already validated
// the package against its marketplace entry.
func Register(reg *registry.Registry, s Spec) (Result, error) {
	if err := s.Provider.Validate(); err != nil {
		return Result{}, diag.Wrap(diag.PluginInvalid, err, "invalid plugin identity")
	}
	if s.Manifest.Name != s.Provider.Plugin {
		return Result{}, diag.New(diag.PluginInvalid, "the manifest name does not match the plugin identity")
	}
	pkg := registry.Package{
		Provider: s.Provider, InstallMode: registry.InstallModeCopy, Enabled: s.Enabled,
		Version: s.Manifest.Version, ContentDigest: s.ContentDigest, DigestAlgorithm: registry.DigestPackageV1,
		Provenance: s.Provenance,
	}
	reg.RemoveProvider(s.Provider)
	reg.UpsertPackage(pkg)
	for _, c := range s.Manifest.Components {
		reg.UpsertComponent(ComponentEntry(s.Provider, c))
	}
	for _, cv := range s.Manifest.Conventions {
		reg.UpsertConvention(registry.Convention{Provider: s.Provider, Name: cv.Name, Prefixes: cv.Prefixes})
	}
	return Result{Package: pkg, Components: s.Manifest.Components}, nil
}

// ComponentEntry is the one place a manifest component becomes a registry
// entry. A Linker's discover.inputs is copied into the single Inputs slot the
// runtime reads.
func ComponentEntry(p pluginid.Provider, c plugin.Component) registry.Component {
	entry := registry.Component{
		Provider:    p,
		Name:        c.Name,
		Role:        c.Role,
		Entrypoint:  c.Entrypoint,
		Runtime:     c.Runtime,
		Pattern:     c.Pattern,
		Accepts:     c.Accepts,
		DisplayName: c.DisplayName,
		Description: c.Description,
		On:          c.On,
		Manual:      c.Manual,
		Inputs:      c.Inputs,
		Key:         c.Key,
	}
	if d := c.Discover; d != nil {
		disc := *d
		entry.Discover = &disc
		entry.Inputs = d.Inputs
	}
	if c.IsFallbackStarter() {
		entry.StarterLayer = registry.LayerFallback
	}
	return entry
}

// PreflightRuntimes fails when a declared runtime is not on PATH. Discovery
// never requires a runtime; selected installation does, so a package that
// could never start is refused up front instead of warned about forever.
func PreflightRuntimes(m *plugin.Manifest) error {
	for _, c := range m.Components {
		if c.Runtime == "" {
			continue
		}
		if _, err := exec.LookPath(c.Runtime); err != nil {
			return diag.Newf(diag.PluginInstallFailed,
				"the runtime %q declared by component %q was not found on PATH", c.Runtime, c.Name).
				WithHint(fmt.Sprintf("Install %q or fix PATH, then install again.", c.Runtime))
		}
	}
	return nil
}

// MetaFile is the wrapper metadata file inside an owned package root. It sits
// beside source/, never inside it, so it is not part of the package digest.
const MetaFile = ".install-meta.json"

// Meta records the actual facts an installed copy came from.
type Meta struct {
	Schema          int                 `json:"schema"`
	Provider        pluginid.Provider   `json:"provider"`
	Provenance      registry.Provenance `json:"provenance"`
	ContentDigest   string              `json:"content_digest"`
	DigestAlgorithm string              `json:"digest_algorithm"`
}

// WriteMeta writes the wrapper metadata into a package root (or its stage).
func WriteMeta(root string, s Spec) error {
	data, err := json.MarshalIndent(Meta{
		Schema: 1, Provider: s.Provider, Provenance: s.Provenance,
		ContentDigest: s.ContentDigest, DigestAlgorithm: registry.DigestPackageV1,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, MetaFile), append(data, '\n'), 0o644)
}
