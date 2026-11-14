// Package installfx publishes fixture plugin directories into a disposable
// Work home for tests that need an installed provider. Production has exactly
// one publisher (the resolved marketplace entry); this helper stands in for
// "a plugin was installed from some earlier marketplace add" so unrelated
// suites (extensions, starters, policy, administration) need not drive a real
// marketplace. It records ordinary local provenance and uses the same
// registration and journal path as production, never a shortcut format.
package installfx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// T is the part of testing.TB the helpers need, so they also work from a
// testscript command.
type T interface {
	Helper()
	Fatalf(format string, args ...any)
}

// RegistrationID is the deterministic source-instance token a fixture
// marketplace gets, so repeated installs agree and different namespaces differ.
func RegistrationID(marketplaceName string) string {
	sum := sha256.Sum256([]byte("fixture-source:" + marketplaceName))
	return hex.EncodeToString(sum[:16])
}

// Provenance builds valid local provenance for a fixture provider.
func Provenance(p pluginid.Provider, dir, digest string) registry.Provenance {
	return registry.Provenance{
		RegistrationID: RegistrationID(p.Marketplace), Kind: pluginid.KindLocal, Source: dir,
		MarketplaceName: p.Marketplace, PluginName: p.Plugin, Path: "plugins/" + p.Plugin,
		Revision: "sha256:" + digest, CatalogDigest: "sha256:" + digest,
	}
}

// Package returns a valid dummy package record for registry-level tests.
func Package(p pluginid.Provider, enabled bool) registry.Package {
	return registry.Package{
		Provider: p, InstallMode: registry.InstallModeCopy, Enabled: enabled, Version: "1",
		ContentDigest: "sha256:fixture", DigestAlgorithm: registry.DigestPackageV1,
		Provenance: Provenance(p, "/fixture", "fixture"),
	}
}

// EnsurePackage upserts a dummy package for p when none is recorded, so a
// directly registered component is part of a valid registry.
func EnsurePackage(reg *registry.Registry, p pluginid.Provider) {
	if _, ok := reg.PackageByProvider(p); !ok {
		reg.UpsertPackage(Package(p, true))
	}
}

// Install publishes the plugin directory dir (plugin.json plus entrypoints) as
// provider p through an administrative transaction, enabled or disabled.
func Install(t T, h workhome.Home, p pluginid.Provider, dir string, enabled bool) registry.Package {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatalf("fixture %s: %v", dir, err)
	}
	manifest, err := plugin.Parse(raw)
	if err != nil {
		t.Fatalf("fixture %s: %v", dir, err)
	}
	tree, err := marketplace.ReadTree(dir)
	if err != nil {
		t.Fatalf("fixture %s: %v", dir, err)
	}
	digest, err := tree.Digest()
	if err != nil {
		t.Fatalf("%v", err)
	}
	spec := plugininstall.Spec{Provider: p, Manifest: manifest, Enabled: enabled, ContentDigest: digest, Provenance: Provenance(p, dir, digest[len("sha256:"):])}
	var pkg registry.Package
	err = adminstate.WithWrite(context.Background(), h, "work plugin add "+p.String(), func(s *adminstate.Session) error {
		stage, err := s.NewStage()
		if err != nil {
			return err
		}
		if err = tree.CopyTo(filepath.Join(stage, "source")); err == nil {
			err = plugininstall.WriteMeta(stage, spec)
		}
		if err == nil {
			err = s.Replace(p, stage)
		}
		if err != nil {
			_ = os.RemoveAll(stage)
			return err
		}
		res, err := plugininstall.Register(s.Registry, spec)
		pkg = res.Package
		return err
	})
	if err != nil {
		t.Fatalf("installing fixture %s: %v", p, err)
	}
	return pkg
}

// SetEnabled flips a recorded package's enablement without an administration
// command.
func SetEnabled(path string, p pluginid.Provider, enabled bool) error {
	r, err := registry.Load(path)
	if err != nil {
		return err
	}
	pkg, ok := r.PackageByProvider(p)
	if !ok {
		return os.ErrNotExist
	}
	pkg.Enabled = enabled
	r.UpsertPackage(pkg)
	return registry.Save(path, r)
}

// P is the provider of plugin in the shared test marketplace "m".
func P(plugin string) pluginid.Provider {
	return pluginid.Provider{Marketplace: "m", Plugin: plugin}
}

// Reg builds a valid registry holding the components, with one package per
// provider that is enabled unless its plugin name appears in off.
func Reg(off []string, comps ...registry.Component) *registry.Registry {
	r := registry.New()
	for _, c := range comps {
		if _, ok := r.PackageByProvider(c.Provider); !ok {
			r.UpsertPackage(Package(c.Provider, !contains(off, c.Provider.Plugin)))
		}
		r.UpsertComponent(c)
	}
	return r
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Disable marks the plugin's package disabled in reg, adding it when absent.
func SetPackageEnabled(reg *registry.Registry, plugin string, enabled bool) {
	pkg, ok := reg.PackageByProvider(P(plugin))
	if !ok {
		pkg = Package(P(plugin), enabled)
	}
	pkg.Enabled = enabled
	reg.UpsertPackage(pkg)
}

// InstallAs publishes dir under the marketplace, taking the plugin name from
// its manifest, and returns the provider it installed as.
func InstallAs(t T, h workhome.Home, marketplaceName, dir string, enabled bool) pluginid.Provider {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	m, err := plugin.Parse(raw)
	if err != nil {
		t.Fatalf("fixture %s: %v", dir, err)
	}
	p := pluginid.Provider{Marketplace: marketplaceName, Plugin: m.Name}
	Install(t, h, p, dir, enabled)
	return p
}
