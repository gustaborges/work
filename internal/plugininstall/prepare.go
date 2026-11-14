package plugininstall

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
)

// Prepared is one resolved marketplace entry that was validated for selected
// installation and copied into a private stage. It carries the actual source,
// entry, revision and digest facts the entry was resolved with, so what gets
// recorded is exactly what was reviewed.
type Prepared struct {
	Entry    *marketplace.ResolvedEntry
	Manifest *plugin.Manifest
	Stage    string
}

// ProvenanceOf is the registry provenance of an installed copy of entry.
func ProvenanceOf(e *marketplace.ResolvedEntry) registry.Provenance {
	return registry.Provenance{
		RegistrationID: e.RegistrationID, Kind: e.Source.Kind, Source: e.Source.Source, Branch: e.Source.Branch,
		MarketplaceName: e.Provider.Marketplace, PluginName: e.Provider.Plugin, Path: e.Entry.Path,
		Revision: e.Revision, CatalogDigest: e.CatalogDigest,
	}
}

// Prepare validates entry for selected installation and materializes its
// package into stage/source with the wrapper metadata beside it. It is the only
// way package content reaches a stage: there is no path, URL or manifest
// argument, and the copy comes from the entry's fixed content.
//
// Discovery tolerates a missing runtime or entrypoint; installation does not,
// because a package that could never start is refused up front instead of
// warned about forever. stage must be an empty directory the caller owns.
func Prepare(entry *marketplace.ResolvedEntry, stage string) (*Prepared, error) {
	tree, manifest, err := Validate(entry)
	if err != nil {
		return nil, err
	}
	if err = tree.CopyTo(filepath.Join(stage, "source")); err != nil {
		return nil, diag.Wrap(diag.PluginInstallFailed, err, "cannot copy the plugin package")
	}
	p := &Prepared{Entry: entry, Manifest: manifest, Stage: stage}
	if err = WriteMeta(stage, p.Spec(true)); err != nil {
		return nil, diag.Wrap(diag.PluginInstallFailed, err, "cannot record the plugin's origin")
	}
	return p, nil
}

// Validate proves that entry's fixed content is exactly what its offer
// promised and that the package could be installed, without copying anything.
// Maintenance checks use it to judge a candidate even when it would not be
// applied; Prepare builds on it so both accept precisely the same packages.
func Validate(entry *marketplace.ResolvedEntry) (*marketplace.Tree, *plugin.Manifest, error) {
	tree, err := entry.Tree()
	if err != nil {
		return nil, nil, err
	}
	raw, err := tree.ReadFile("plugin.json")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: plugin.json: %v", marketplace.ErrInvalid, err)
	}
	manifest, err := plugin.Parse(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", marketplace.ErrInvalid, err)
	}
	if manifest.Name != entry.Provider.Plugin || manifest.Version != entry.Entry.Version {
		return nil, nil, fmt.Errorf("%w: the package no longer matches its offer", marketplace.ErrInvalid)
	}
	if err = requireEntrypoints(tree, manifest); err != nil {
		return nil, nil, err
	}
	if err = PreflightRuntimes(manifest); err != nil {
		return nil, nil, err
	}
	return tree, manifest, nil
}

// requireEntrypoints fails when a declared entrypoint is not a regular file of
// the package. On Windows a native executable carries an ".exe" suffix that the
// manifest omits.
func requireEntrypoints(tree *marketplace.Tree, m *plugin.Manifest) error {
	for _, c := range m.Components {
		rel := c.Entrypoint
		if runtime.GOOS == "windows" && c.Runtime == "" {
			rel += ".exe"
		}
		if _, err := tree.ResolveFile(rel); err != nil {
			return diag.Newf(diag.PluginInstallFailed, "component %q declares the entrypoint %q, which the package does not contain", c.Name, c.Entrypoint)
		}
	}
	return nil
}

// Spec is the registration record of the prepared package.
func (p *Prepared) Spec(enabled bool) Spec {
	return Spec{
		Provider: p.Entry.Provider, Manifest: p.Manifest, Enabled: enabled,
		ContentDigest: p.Entry.Entry.PackageDigest, Provenance: ProvenanceOf(p.Entry),
	}
}

// Publish hands the stage to the open administrative session and registers the
// package's declarations, replacing everything the provider registered before.
// Publication to the managed directory and registration commit together with
// the session.
func (p *Prepared) Publish(s *adminstate.Session, enabled bool) (Result, error) {
	if err := s.Replace(p.Entry.Provider, p.Stage); err != nil {
		return Result{}, err
	}
	return Register(s.Registry, p.Spec(enabled))
}
