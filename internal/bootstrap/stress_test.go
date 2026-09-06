package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// installPhases are every checkpoint install() passes through, in order. The
// stress test injects a failure at each in turn.
var installPhases = []string{
	"staged", "staged-complete", "dest-backed-up", "renamed", "registered",
}

func assertConverged(t *testing.T, h workhome.Home) {
	t.Helper()

	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		t.Fatalf("registry.Load: %v", err)
	}
	if got := reg.ByRole(registry.RoleStarter); len(got) != 1 || got[0].Name != componentStarter {
		t.Errorf("starter entries = %+v, want exactly one %q", got, componentStarter)
	}
	if got := reg.ByRole(registry.RoleRepositoryLocator); len(got) != 1 || got[0].Name != componentLocator {
		t.Errorf("locator entries = %+v, want exactly one %q", got, componentLocator)
	}
	if _, ok := reg.ConventionByName(conventionName); !ok {
		t.Errorf("convention %q not registered", conventionName)
	}

	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	policy := 0
	for _, l := range cfg.RepositoryResolution.Locators {
		if l == LocatorPolicyEntry {
			policy++
		}
	}
	if policy != 1 {
		t.Errorf("resolution policy has %d seed-locator entries, want 1: %v", policy, cfg.RepositoryResolution.Locators)
	}

	// Exactly one installed package directory, its binaries present, and no
	// staging directory left behind.
	entries, err := os.ReadDir(h.PluginsDir())
	if err != nil {
		t.Fatalf("read plugins dir: %v", err)
	}
	var dirs []string
	for _, e := range entries {
		dirs = append(dirs, e.Name())
		if strings.HasPrefix(e.Name(), stagePrefix) {
			t.Errorf("stale staging directory left behind: %s", e.Name())
		}
	}
	if len(dirs) != 1 || dirs[0] != Alias {
		t.Errorf("plugins dir = %v, want exactly [%s]", dirs, Alias)
	}
	src := filepath.Join(h.PluginsDir(), Alias, "source")
	for _, name := range binaryNames() {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Errorf("missing binary %s: %v", name, err)
		}
	}
}

// TestEnsureSeedStressWithInjectedInterruptions loops EnsureSeed many times,
// failing install() at every checkpoint in rotation, and asserts the install
// always converges to exactly one usable record of each component with no
// partial directory (SC-007, FR-005; quickstart S11).
func TestEnsureSeedStressWithInjectedInterruptions(t *testing.T) {
	h := testHome(t)
	t.Cleanup(func() { installCheckpoint = nil })

	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	staleMeta := func() {
		if err := os.WriteFile(metaPath,
			[]byte(`{"origin":"embedded-seed","content_digest":"stale","installed_at":"2020-01-01T00:00:00Z"}`),
			0o644); err != nil {
			t.Fatalf("stale meta: %v", err)
		}
	}

	const iterations = 100
	injected := 0
	for i := range iterations {
		// Every fifth run is interrupted at a rotating checkpoint (20 total).
		// Staling the digest first guarantees install() actually runs and so
		// reaches the checkpoint rather than short-circuiting as a no-op.
		if i%5 == 0 {
			if i > 0 {
				staleMeta()
			}
			phase := installPhases[(i/5)%len(installPhases)]
			installCheckpoint = func(p string) error {
				if p == phase {
					return fmt.Errorf("injected interruption at %s", p)
				}
				return nil
			}
			injected++
			if err := EnsureSeed(h); err == nil {
				t.Fatalf("iter %d: EnsureSeed succeeded despite injected failure at %q", i, phase)
			}
			installCheckpoint = nil
		}

		if err := EnsureSeed(h); err != nil {
			t.Fatalf("iter %d: recovery EnsureSeed: %v", i, err)
		}
	}

	if injected != 20 {
		t.Fatalf("injected %d interruptions, want 20", injected)
	}
	assertConverged(t, h)
}

func TestEnsureSeedRecoversAfterDestinationIsBackedUp(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	if err := os.WriteFile(metaPath,
		[]byte(`{"origin":"embedded-seed","content_digest":"stale","installed_at":"2020-01-01T00:00:00Z"}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { installCheckpoint = nil })
	installCheckpoint = func(phase string) error {
		if phase == "dest-backed-up" {
			return errors.New("injected interruption")
		}
		return nil
	}
	if err := EnsureSeed(h); err == nil {
		t.Fatal("EnsureSeed succeeded despite injected interruption")
	}
	installCheckpoint = nil

	dest := filepath.Join(h.PluginsDir(), Alias)
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination after interruption: want absent, got %v", err)
	}
	if _, err := os.Stat(dest + backupSuffix); err != nil {
		t.Fatalf("backup after interruption: %v", err)
	}

	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed recovery: %v", err)
	}
	if _, err := os.Stat(dest + backupSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup remains after recovery: %v", err)
	}
	assertConverged(t, h)
}

func TestEnsureSeedRepairsMissingLocatorPolicyAfterInterruption(t *testing.T) {
	h := testHome(t)
	t.Cleanup(func() { installCheckpoint = nil })
	installCheckpoint = func(phase string) error {
		if phase == "registered" {
			return errors.New("injected interruption")
		}
		return nil
	}
	if err := EnsureSeed(h); err == nil {
		t.Fatal("EnsureSeed succeeded despite injected interruption")
	}
	installCheckpoint = nil

	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed recovery: %v", err)
	}
	assertConverged(t, h)
}

// TestEnsureSeedConcurrent runs many EnsureSeed calls in parallel against one
// home; the bootstrap lock must serialize them into a single coherent install.
func TestEnsureSeedConcurrent(t *testing.T) {
	h := testHome(t)

	const goroutines = 12
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := range goroutines {
		wg.Go(func() { errs[i] = EnsureSeed(h) })
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	assertConverged(t, h)
}

// TestEnsureSeedSweepsOrphanStagingDir plants a staging directory as a killed
// install would have left it, then confirms the next EnsureSeed removes it.
func TestEnsureSeedSweepsOrphanStagingDir(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}

	orphan := filepath.Join(h.PluginsDir(), stagePrefix+"deadbeef")
	if err := os.MkdirAll(filepath.Join(orphan, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Force a re-install so install() (which sweeps) runs again.
	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"origin":"embedded-seed","content_digest":"stale","installed_at":"2020-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed after planting orphan: %v", err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("orphan staging dir not swept: stat err = %v", err)
	}
	assertConverged(t, h)
}
