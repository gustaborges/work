package integration

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminSeedExplicitInstallRemovalAndUpgrade(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	ctx := context.Background()
	for _, args := range [][]string{{"--embedded", "source"}, {"--embedded", "--link"}, {"--embedded", "--link=false"}, {"--embedded", "--as", "other"}, {"--embedded", "--as="}, {"--embedded", "--json"}} {
		h.run(t, h.root, 2, append([]string{"plugin", "install"}, args...)...)
		if _, err := os.Stat(home.Root()); !os.IsNotExist(err) {
			t.Fatal("invalid embedded grammar wrote state")
		}
	}
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	s, err := adminstate.Read(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Config.RepositoryResolution.Locators) != 0 || !s.Lifecycle.Initialized || !s.Lifecycle.PolicySeeded || s.Lifecycle.Removed {
		t.Fatal(s)
	}
	h.run(t, h.root, 0, "plugin", "disable", bootstrap.Alias)
	// A changed old digest forces the same repair path used by a newer release.
	if err = adminstate.WithWrite(ctx, home, "old release", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByAlias(bootstrap.Alias)
		p.Version = "old-release"
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(home.PluginsDir(), bootstrap.Alias, ".install-meta.json"), []byte(`{"origin":"embedded-seed","content_digest":"old-release","policy_seeded":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = bootstrap.EnsureSeed(home); err != nil {
		t.Fatal(err)
	}
	s, err = adminstate.Read(ctx, home)
	if err != nil || s.Registry.PackageEnabled(bootstrap.Alias) || len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Fatal(s, err)
	}
	h.run(t, h.root, 0, "plugin", "uninstall", bootstrap.Alias, "--yes")
	before := adminTree(t, home.Root())
	for range 3 {
		if err = bootstrap.EnsureSeed(home); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("ordinary setup resurrected bundled package")
	}
	s, err = adminstate.Read(ctx, home)
	if err != nil || !s.Lifecycle.Removed || !s.Lifecycle.Initialized || !s.Lifecycle.PolicySeeded {
		t.Fatal(s, err)
	}
	// Fully specified start reaches bootstrap, retains removal and gives guidance.
	_, ui := h.run(t, h.root, 35, "start", h.root, "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "removed", "--prefix", "{slug}", "--yes")
	if !strings.Contains(ui, "install --embedded") {
		t.Fatal(ui)
	}
	if _, err = os.Lstat(filepath.Join(home.PluginsDir(), bootstrap.Alias)); !os.IsNotExist(err) {
		t.Fatal("start resurrected seed", err)
	}
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	s, err = adminstate.Read(ctx, home)
	if err != nil || s.Lifecycle.Removed || !s.Registry.PackageEnabled(bootstrap.Alias) || len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Fatal(s, err)
	}
}

func TestAdminSeedLegacyUninstallRetainsEmptyPolicy(t *testing.T) {
	for _, meta := range []bool{false, true} {
		t.Run(map[bool]string{false: "ownerless", true: "install-meta"}[meta], func(t *testing.T) {
			h := newAdminHarness(t)
			home := workhome.At(adminHome(h))
			ctx := context.Background()
			if err := bootstrap.EnsureSeed(home); err != nil {
				t.Fatal(err)
			}
			// Simulate a pre-lifecycle installation with an intentionally empty policy.
			s, err := adminstate.Read(ctx, home)
			if err != nil {
				t.Fatal(err)
			}
			s.Registry.Packages = nil
			if err := adminstate.WithWrite(ctx, home, "empty policy", func(s *adminstate.Session) error { s.Config.RepositoryResolution.Locators = []string{}; return nil }); err != nil {
				t.Fatal(err)
			}
			if err := registry.Save(home.RegistryFile(), s.Registry); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(home.ReferencePackageFile()); err != nil {
				t.Fatal(err)
			}
			if !meta {
				if err := os.Remove(filepath.Join(home.PluginsDir(), bootstrap.Alias, ".install-meta.json")); err != nil {
					t.Fatal(err)
				}
			}
			h.run(t, h.root, 0, "plugin", "uninstall", bootstrap.Alias, "--yes")
			if err := bootstrap.EnsureSeed(home); err != nil {
				t.Fatal(err)
			}
			h.run(t, h.root, 0, "plugin", "install", "--embedded")
			s, err = adminstate.Read(ctx, home)
			if err != nil || len(s.Config.RepositoryResolution.Locators) != 0 {
				t.Fatal(s, err)
			}
		})
	}
}

func TestAdminSeedReinstallRetainsDisablementAndFallbackOwnership(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	ctx := context.Background()
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	h.run(t, h.root, 0, "plugin", "disable", bootstrap.Alias)
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	s, err := adminstate.Read(ctx, home)
	if err != nil || s.Registry.PackageEnabled(bootstrap.Alias) {
		t.Fatal(s, err)
	}
	h.run(t, h.root, 0, "plugin", "uninstall", bootstrap.Alias, "--yes")
	if err := adminstate.WithWrite(ctx, home, "custom fallback", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(registry.Package{Alias: "custom", Origin: registry.OriginLocalPinned})
		s.Registry.UpsertComponent(registry.Component{Alias: "custom", Name: "fallback", Role: registry.RoleStarter, StarterLayer: registry.LayerFallback})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, home.Root())
	h.run(t, h.root, 33, "plugin", "install", "--embedded")
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("embedded install stole fallback ownership")
	}
}
