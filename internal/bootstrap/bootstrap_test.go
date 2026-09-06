package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
)

func testHome(t *testing.T) workhome.Home {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed for this platform; run `make seed` (%v)", err)
	}
	return workhome.At(filepath.Join(t.TempDir(), "dotwork"))
}

func assertRegistered(t *testing.T, h workhome.Home) {
	t.Helper()
	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		t.Fatalf("registry.Load: %v", err)
	}
	starters := reg.ByRole(registry.RoleStarter)
	locators := reg.ByRole(registry.RoleRepositoryLocator)
	if len(starters) != 1 || starters[0].Name != componentStarter {
		t.Errorf("starters = %+v", starters)
	}
	if starters[0].StarterLayer != registry.LayerFallback {
		t.Errorf("starter layer = %q, want fallback", starters[0].StarterLayer)
	}
	if len(locators) != 1 || locators[0].Name != componentLocator {
		t.Errorf("locators = %+v", locators)
	}
	if _, ok := reg.ConventionByName(conventionName); !ok {
		t.Errorf("convention %q not registered", conventionName)
	}

	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	count := 0
	for _, l := range cfg.RepositoryResolution.Locators {
		if l == LocatorPolicyEntry {
			count++
		}
	}
	if count != 1 {
		t.Errorf("policy has %d entries for the seed locator, want 1: %v", count, cfg.RepositoryResolution.Locators)
	}

	src := filepath.Join(h.PluginsDir(), Alias, "source")
	for _, name := range binaryNames() {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Errorf("missing binary %s: %v", name, err)
		}
	}
}

func TestEnsureSeedHappyPath(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed: %v", err)
	}
	assertRegistered(t, h)
}

func TestEnsureSeedIdempotent(t *testing.T) {
	h := testHome(t)
	for i := range 5 {
		if err := EnsureSeed(h); err != nil {
			t.Fatalf("EnsureSeed #%d: %v", i, err)
		}
	}
	assertRegistered(t, h)

	// The digest guard means no re-extraction once current.
	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	before, err := os.Stat(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(metaPath)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("install metadata was rewritten on a no-op EnsureSeed")
	}
}

func TestEnsureSeedRepairsStaleDigest(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"origin":"embedded-seed","content_digest":"stale","installed_at":"2020-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed (repair): %v", err)
	}
	// Digest restored.
	data, _ := os.ReadFile(metaPath)
	var m map[string]string
	_ = json.Unmarshal(data, &m)
	if m["content_digest"] == "stale" {
		t.Errorf("stale digest not repaired")
	}
	assertRegistered(t, h)
}

func TestEnsureSeedRepairsPartialPluginDir(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	// Remove a binary to simulate an interrupted extraction.
	src := filepath.Join(h.PluginsDir(), Alias, "source")
	if err := os.Remove(filepath.Join(src, binaryNames()[0])); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed (repair partial): %v", err)
	}
	assertRegistered(t, h)
}

func TestEnsureSeedDoesNotDuplicatePolicyEntry(t *testing.T) {
	h := testHome(t)
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	// Pre-seed config already has the locator entry.
	cfg := config.Default()
	cfg.RepositoryResolution.Locators = []string{LocatorPolicyEntry}
	if err := config.Save(h.ConfigFile(), cfg); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	got, _ := config.Load(h.ConfigFile())
	if len(got.RepositoryResolution.Locators) != 1 {
		t.Errorf("policy entries = %v, want exactly one", got.RepositoryResolution.Locators)
	}
}
