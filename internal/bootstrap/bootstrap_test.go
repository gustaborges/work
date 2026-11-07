package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
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

func read(t *testing.T, h workhome.Home) *adminstate.Snapshot {
	t.Helper()
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// assertInstalled checks the one converged end state: an enabled, independent
// copy with builtin provenance, its declarations, wrapper metadata and policy.
func assertInstalled(t *testing.T, h workhome.Home, wantPolicy int) {
	t.Helper()
	s := read(t, h)
	pkg, ok := s.Registry.PackageByProvider(Provider)
	if !ok || !pkg.Enabled || pkg.ID() != "work-reference@builtin" {
		t.Fatalf("package = %+v, %v", pkg, ok)
	}
	pv := pkg.Provenance
	if pv.Kind != pluginid.KindBuiltin || pv.RegistrationID != pluginid.BuiltinRegistrationID || pv.Path != seed.ReferencePath {
		t.Errorf("provenance = %+v", pv)
	}
	snap, err := marketplace.BuiltinSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := snap.Offer.Entry(Provider.Plugin)
	if pkg.ContentDigest != entry.PackageDigest || pv.CatalogDigest != snap.Offer.CatalogDigest || pv.Revision != snap.Revision {
		t.Errorf("package facts do not match the offer: %+v", pkg)
	}
	starters, locators := s.Registry.ByRole(registry.RoleStarter), s.Registry.ByRole(registry.RoleRepositoryLocator)
	if len(starters) != 1 || starters[0].Name != "local-path-starter" || starters[0].StarterLayer != registry.LayerFallback {
		t.Errorf("starters = %+v", starters)
	}
	if len(locators) != 1 || locators[0].Name != "filesystem-repository-locator" || locators[0].Provider != Provider {
		t.Errorf("locators = %+v", locators)
	}
	if _, ok := s.Registry.ConventionByRef(pluginid.Ref{Provider: Provider, Name: "freeform"}); !ok {
		t.Error("freeform convention not registered under the provider")
	}
	if s.Inspection[pkg.ID()] != "ready" {
		t.Errorf("inspection = %v", s.Inspection)
	}
	count := 0
	for _, l := range s.Config.RepositoryResolution.Locators {
		if l == LocatorPolicyEntry {
			count++
		}
	}
	if count != wantPolicy {
		t.Errorf("policy has %d seed entries, want %d: %v", count, wantPolicy, s.Config.RepositoryResolution.Locators)
	}
	if !s.Lifecycle.Initialized || !s.Lifecycle.PolicyHandled || s.Lifecycle.Removed {
		t.Errorf("lifecycle = %+v", s.Lifecycle)
	}
	root, _ := h.PackageRoot(Provider)
	for _, f := range []string{".install-meta.json", filepath.Join("source", "plugin.json")} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	info, err := os.Stat(filepath.Join(root, "source", "starter"+ext))
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
		t.Errorf("starter entrypoint = %v, %v", info, err)
	}
}

func stray(t *testing.T, h workhome.Home) {
	t.Helper()
	entries, _ := os.ReadDir(h.PluginsDir())
	for _, e := range entries {
		if e.Name() != pluginid.Builtin {
			t.Errorf("stray entry %q under plugins/", e.Name())
		}
	}
	if left, _ := os.ReadDir(h.AdministrationDir()); len(left) != 0 {
		t.Errorf("journal evidence left behind: %v", left)
	}
}

func TestFreshSetupInstallsThroughTheCommonEntryAndSeedsPolicyOnce(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	assertInstalled(t, h, 1)
	stray(t, h)
}

func TestSetupRunsOnceAndLaterRunsAreByteIdenticalNoOps(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	files := []string{h.ConfigFile(), h.RegistryFile(), h.LifecycleFile(), h.AdministrationFile()}
	before := map[string]string{}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		before[f] = string(b)
	}
	for range 3 {
		if err := EnsureSeed(h); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if b, _ := os.ReadFile(f); string(b) != before[f] {
			t.Errorf("%s changed on a repeated setup", filepath.Base(f))
		}
	}
	assertInstalled(t, h, 1)
}

func TestNeverRepairsOrResurrectsAnInstalledCopy(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	root, _ := h.PackageRoot(Provider)
	if err := os.RemoveAll(filepath.Join(root, "source")); err != nil {
		t.Fatal(err)
	}
	regBefore, _ := os.ReadFile(h.RegistryFile())
	for range 2 {
		if err := EnsureSeed(h); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "source")); !os.IsNotExist(err) {
		t.Error("setup silently repaired missing content")
	}
	if regAfter, _ := os.ReadFile(h.RegistryFile()); string(regAfter) != string(regBefore) {
		t.Error("setup rewrote the registry")
	}
	if s := read(t, h); s.Inspection["work-reference@builtin"] != "content-unavailable" {
		t.Errorf("inspection = %v", s.Inspection)
	}
}

func TestDisabledAndRemovedDefaultsStayThatWay(t *testing.T) {
	h := testHome(t)
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	if err := adminstate.WithWrite(context.Background(), h, "work plugin disable work-reference@builtin", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByProvider(Provider)
		p.Enabled = false
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	if s := read(t, h); s.Registry.PackageEnabled(Provider) {
		t.Fatal("setup re-enabled a disabled default")
	}

	// A deliberate removal is recorded outside the package and is never undone.
	if err := adminstate.WithWrite(context.Background(), h, "work plugin remove", func(s *adminstate.Session) error {
		if err := s.Remove(Provider); err != nil {
			return err
		}
		s.Registry.RemoveProvider(Provider)
		s.Lifecycle.MarkRemoved()
		s.Persist("lifecycle")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := EnsureSeed(h); err != nil {
			t.Fatal(err)
		}
	}
	s := read(t, h)
	if _, ok := s.Registry.PackageByProvider(Provider); ok || !s.Lifecycle.Removed {
		t.Fatalf("removed default came back: %+v", s.Lifecycle)
	}
	if root, _ := h.PackageRoot(Provider); func() bool { _, err := os.Stat(root); return err == nil }() {
		t.Error("removed default's content came back")
	}
}

func TestRegistrationBeforeFirstSetupDoesNotHandlePolicy(t *testing.T) {
	h := testHome(t)
	if err := adminstate.WithWrite(context.Background(), h, "work plugin marketplace add", func(s *adminstate.Session) error {
		s.Config.Marketplaces = append(s.Config.Marketplaces, config.Marketplace{RegistrationID: "0123456789abcdef0123456789abcdef", Name: "team", Kind: config.KindLocal, Source: "/src"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if s := read(t, h); s.Lifecycle.PolicyHandled || s.Lifecycle.Initialized {
		t.Fatalf("registration marked lifecycle: %+v", s.Lifecycle)
	}
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	assertInstalled(t, h, 1)
}

func TestExplicitPolicyChoiceIsPreservedAcrossFirstSetup(t *testing.T) {
	for _, name := range []string{"explicit empty policy", "explicit nonempty policy"} {
		t.Run(name, func(t *testing.T) {
			h := testHome(t)
			other := "other@team/locator"
			if err := adminstate.WithWrite(context.Background(), h, "work repository policy replace", func(s *adminstate.Session) error {
				if name == "explicit nonempty policy" {
					s.Config.RepositoryResolution.Locators = []string{other}
				}
				s.Lifecycle.PolicyHandled = true
				s.Persist("lifecycle")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := EnsureSeed(h); err != nil {
				t.Fatal(err)
			}
			s := read(t, h)
			want := []string{}
			if name == "explicit nonempty policy" {
				want = []string{other}
			}
			if fmt.Sprint(s.Config.RepositoryResolution.Locators) != fmt.Sprint(want) {
				t.Fatalf("policy = %v, want %v", s.Config.RepositoryResolution.Locators, want)
			}
			if _, ok := s.Registry.PackageByProvider(Provider); !ok || !s.Lifecycle.Initialized {
				t.Fatal("the default package must still be installed")
			}
		})
	}
}

func TestInterruptedSetupRollsBackAndConverges(t *testing.T) {
	t.Cleanup(func() { installCheckpoint = nil })
	for _, phase := range []string{"staged", "staged-complete", "dest-backed-up", "renamed", "registered"} {
		t.Run(phase, func(t *testing.T) {
			h := testHome(t)
			installCheckpoint = func(p string) error {
				if p == phase {
					return fmt.Errorf("injected interruption at %s", p)
				}
				return nil
			}
			if err := EnsureSeed(h); err == nil {
				t.Skipf("phase %q is not reached on a fresh home", phase)
			}
			installCheckpoint = nil
			if s, err := adminstate.Read(context.Background(), h); err != nil || s.Lifecycle.Initialized || len(s.Registry.Packages) != 0 {
				t.Fatalf("a failed setup left partial state: %+v %v", s, err)
			}
			if err := EnsureSeed(h); err != nil {
				t.Fatalf("recovery: %v", err)
			}
			assertInstalled(t, h, 1)
			stray(t, h)
		})
	}
}

func TestConcurrentSetupInstallsOnce(t *testing.T) {
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
	assertInstalled(t, h, 1)
	stray(t, h)
}

func TestSetupCollectsAnOrphanStage(t *testing.T) {
	h := testHome(t)
	orphan := filepath.Join(h.PluginsDir(), workhome.StagePrefix+"deadbeef")
	if err := os.MkdirAll(filepath.Join(orphan, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeed(h); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("orphan stage not collected: %v", err)
	}
	assertInstalled(t, h, 1)
}

func TestMalformedLifecycleIsRejectedNotConverted(t *testing.T) {
	h := testHome(t)
	if err := os.MkdirAll(h.StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"schema":1,"initialized":true,"policy_seeded":true,"removed":false}`)
	if err := os.WriteFile(h.LifecycleFile(), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeed(h); err == nil {
		t.Fatal("an earlier-shape lifecycle was accepted")
	}
	if got, _ := os.ReadFile(h.LifecycleFile()); string(got) != string(bad) {
		t.Fatal("the unsupported file was converted or deleted")
	}
}
