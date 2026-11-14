package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

type simulatedCrash struct{ point string }

// crashAt runs op and aborts at point with a panic that skips the compensation
// code after the checkpoint the way a killed process does. Only the deferred
// lock release runs, as the operating system would release a dead holder's lock.
func crashAt(t *testing.T, point string, op func(adminstate.WriteOptions)) {
	t.Helper()
	defer func() {
		switch r := recover().(type) {
		case nil:
			t.Fatalf("the operation finished without reaching %q", point)
		case simulatedCrash:
		default:
			panic(r)
		}
	}()
	op(adminstate.WriteOptions{Checkpoint: func(p string) error {
		if p == point {
			panic(simulatedCrash{p})
		}
		return nil
	}})
}

func newRecoveryHome(t *testing.T) workhome.Home {
	return workhome.At(filepath.Join(t.TempDir(), "home"))
}

func registerLocal(t *testing.T, h workhome.Home, src string) {
	t.Helper()
	spec, err := marketplaceadmin.Classify(src, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = marketplaceadmin.Add(context.Background(), h, spec); err != nil {
		t.Fatal(err)
	}
}

func installNamed(t *testing.T, h workhome.Home, plugin, mkt string) {
	t.Helper()
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, plugin, mkt)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if _, err = plan.Apply(context.Background(), false); err != nil {
		t.Fatal(err)
	}
}

func adminRead(t *testing.T, h workhome.Home) *adminstate.Snapshot {
	t.Helper()
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// stateImages are the bytes of every journaled state file.
func stateImages(t *testing.T, h workhome.Home) map[string]string {
	t.Helper()
	out := map[string]string{}
	for name, path := range map[string]string{"config": h.ConfigFile(), "registry": h.RegistryFile(), "catalogs": h.CatalogsFile(), "lifecycle": h.LifecycleFile()} {
		b, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[name] = string(b)
	}
	return out
}

func requireClean(t *testing.T, h workhome.Home) {
	t.Helper()
	if entries, _ := os.ReadDir(h.AdministrationDir()); len(entries) != 0 {
		t.Errorf("transaction evidence remains: %v", entries)
	}
	entries, _ := os.ReadDir(h.PluginsDir())
	for _, e := range entries {
		if workhome.IsStageName(e.Name()) {
			t.Errorf("stage remains: %s", e.Name())
		}
	}
}

func recoverInterrupted(t *testing.T, h workhome.Home) {
	t.Helper()
	// Before the pending epoch and after the stable one nothing needs recovery;
	// in between, a pure read must say so instead of repairing anything.
	if _, err := adminstate.Read(context.Background(), h); err != nil && diag.ExitCode(err) != diag.AdministrationRecoveryRequired.Code {
		t.Fatalf("a pure read of interrupted state = %v (exit %d)", err, diag.ExitCode(err))
	}
	if _, err := adminstate.Preflight(context.Background(), h); err != nil {
		t.Fatalf("recovery: %v", err)
	}
}

// Everything up to the commit marker is rolled back; from the marker onward the
// reviewed removal is completed. Both leave no evidence behind.
func TestWholeSourceRemovalRecoversAtEveryCheckpoint(t *testing.T) {
	points := []string{
		"prepared", "pending",
		"move-backup:alpha@demo", "move-publish:alpha@demo", "move-backup:beta@demo", "move-publish:beta@demo",
		"json:config", "json:registry", "json:catalogs", "before-commit",
		"committed", "stable", "cleanup",
	}
	committed := map[string]bool{"committed": true, "stable": true, "cleanup": true}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			h := newRecoveryHome(t)
			registerLocal(t, h, marketplaces.Copy(t, "multiple"))
			installNamed(t, h, "alpha", "demo")
			installNamed(t, h, "beta", "demo")
			before := stateImages(t, h)
			alphaDigest, _ := adminstate.OwnedPackageDigest(h, pluginid.Provider{Marketplace: "demo", Plugin: "alpha"})

			plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "demo")
			if err != nil {
				t.Fatal(err)
			}
			crashAt(t, point, func(o adminstate.WriteOptions) { _, _ = plan.ApplyWithOptions(context.Background(), false, o) })
			recoverInterrupted(t, h)

			s := adminRead(t, h)
			_, registered := s.Config.Marketplace("demo")
			if committed[point] {
				if registered || len(s.Registry.Packages) != 0 || len(s.Catalogs.Catalogs) != 0 {
					t.Fatalf("a committed removal was not completed: %+v", s.Registry.Packages)
				}
				if _, err := os.Stat(filepath.Join(h.PluginsDir(), "demo")); !os.IsNotExist(err) {
					t.Fatalf("namespace storage remains: %v", err)
				}
			} else {
				if !registered || len(s.Registry.Packages) != 2 {
					t.Fatalf("a prepared removal was not rolled back: registered=%v packages=%d", registered, len(s.Registry.Packages))
				}
				if got := stateImages(t, h); !equalImages(before, got) {
					t.Fatalf("state images differ after rollback:\nbefore %v\nafter  %v", before, got)
				}
				if d, _ := adminstate.OwnedPackageDigest(h, pluginid.Provider{Marketplace: "demo", Plugin: "alpha"}); d != alphaDigest {
					t.Fatal("owned package bytes changed by rollback")
				}
			}
			requireClean(t, h)
		})
	}
}

func equalImages(a, b map[string]string) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestPackageUpdateRecoversAtEveryCheckpoint(t *testing.T) {
	points := []string{
		"prepared", "pending", "move-backup:sample@solo", "move-publish:sample@solo", "json:registry", "before-commit",
		"committed", "stable", "cleanup",
	}
	committed := map[string]bool{"committed": true, "stable": true, "cleanup": true}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			h := newRecoveryHome(t)
			src := marketplaces.Copy(t, "single")
			registerLocal(t, h, src)
			installNamed(t, h, "sample", "solo")
			bump(t, src)
			before := stateImages(t, h)

			plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{{Marketplace: "solo", Plugin: "sample"}}, true)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			crashAt(t, point, func(o adminstate.WriteOptions) {
				_, _ = pluginadmin.ApplyUpdatesWithOptions(context.Background(), plan, false, o)
			})
			recoverInterrupted(t, h)

			pkg, _ := adminRead(t, h).Registry.PackageByProvider(pluginid.Provider{Marketplace: "solo", Plugin: "sample"})
			if committed[point] != (pkg.Version == "9.9.9") {
				t.Fatalf("version after %s recovery = %q", point, pkg.Version)
			}
			if !committed[point] {
				if got := stateImages(t, h); !equalImages(before, got) {
					t.Fatalf("state images differ after rollback")
				}
			}
			requireClean(t, h)
		})
	}
}

// bump publishes a new opaque version of the single fixture's plugin.
func bump(t *testing.T, src string) {
	t.Helper()
	path := filepath.Join(src, "plugins", "sample", "plugin.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(string(raw), `"version": "1.0.0"`, `"version": "9.9.9"`, 1)
	if next == string(raw) {
		t.Fatalf("fixture version marker not found in %s", raw)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPreCommitFailureRestoresTheCompleteReviewedSet(t *testing.T) {
	for _, point := range []string{"move-backup:beta@demo", "move-publish:alpha@demo", "json:registry", "before-commit"} {
		t.Run(point, func(t *testing.T) {
			h := newRecoveryHome(t)
			registerLocal(t, h, marketplaces.Copy(t, "multiple"))
			installNamed(t, h, "alpha", "demo")
			installNamed(t, h, "beta", "demo")
			before := stateImages(t, h)
			plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "demo")
			if err != nil {
				t.Fatal(err)
			}
			boom := errors.New("injected failure")
			res, err := plan.ApplyWithOptions(context.Background(), false, adminstate.WriteOptions{Checkpoint: func(p string) error {
				if p == point {
					return boom
				}
				return nil
			}})
			if err == nil || res.Committed || len(res.Removed) != 0 {
				t.Fatalf("result = %+v, err = %v", res, err)
			}
			if got := stateImages(t, h); !equalImages(before, got) {
				t.Fatal("a failed removal did not restore the exact before-images")
			}
			if n := len(adminRead(t, h).Registry.Packages); n != 2 {
				t.Fatalf("packages = %d", n)
			}
			requireClean(t, h)
		})
	}
}

// A failure after the commit marker is committed work: the result reports the
// applied removal with exit 55, and the next mutating command finishes cleanup.
func TestCommittedCleanupFailureReportsAppliedEffectsAndRecovers(t *testing.T) {
	for _, point := range []string{"committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			h := newRecoveryHome(t)
			registerLocal(t, h, marketplaces.Copy(t, "multiple"))
			installNamed(t, h, "alpha", "demo")
			plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "demo")
			if err != nil {
				t.Fatal(err)
			}
			res, err := plan.ApplyWithOptions(context.Background(), false, adminstate.WriteOptions{Checkpoint: func(p string) error {
				if p == point {
					return errors.New("injected cleanup failure")
				}
				return nil
			}})
			if diag.ExitCode(err) != diag.AdministrationCleanupFailed.Code || !res.Committed || !res.CleanupPending ||
				!slices.Equal(res.Removed, []pluginid.Provider{{Marketplace: "demo", Plugin: "alpha"}}) {
				t.Fatalf("result = %+v, err = %v (exit %d)", res, err, diag.ExitCode(err))
			}
			if _, err := adminstate.Preflight(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			s := adminRead(t, h)
			if _, ok := s.Config.Marketplace("demo"); ok || len(s.Registry.Packages) != 0 {
				t.Fatal("recovery undid committed work")
			}
			requireClean(t, h)
		})
	}
}

// A capture is protected while a live lease names it, even once the committed
// catalog no longer references it, and collected afterwards. Collection is an
// internal mutation, never a read side effect.
func TestCaptureOfARemovedSourceSurvivesALiveLeaseThenIsCollected(t *testing.T) {
	h := newRecoveryHome(t)
	registerLocal(t, h, marketplaces.Copy(t, "single"))
	stored := adminRead(t, h).Catalogs.Catalogs[0]
	hexDigest, ok := marketplace.HexPayload(*stored.CaptureDigest)
	if !ok {
		t.Fatalf("capture digest %q", *stored.CaptureDigest)
	}
	capture, _ := h.SnapshotDir(hexDigest)

	lease, err := adminstate.HoldCapture(context.Background(), h, hexDigest)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "solo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Apply(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	touch := func() {
		if err := adminstate.WithWrite(context.Background(), h, "work repository policy replace", func(s *adminstate.Session) error {
			s.Persist("lifecycle")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	touch()
	if _, err := os.Stat(capture); err != nil {
		t.Fatalf("a leased capture was collected: %v", err)
	}
	lease.Release()
	touch()
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("an unreferenced, unleased capture remains: %v", err)
	}
}

// Reading never recovers or collects: it reports the pending transaction and
// leaves every byte alone.
func TestReadsOfInterruptedRemovalReportRecoveryWithoutChangingAnything(t *testing.T) {
	h := newRecoveryHome(t)
	registerLocal(t, h, marketplaces.Copy(t, "single"))
	installNamed(t, h, "sample", "solo")
	plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "solo")
	if err != nil {
		t.Fatal(err)
	}
	crashAt(t, "json:registry", func(o adminstate.WriteOptions) { _, _ = plan.ApplyWithOptions(context.Background(), false, o) })
	before := stateImages(t, h)
	for i := 0; i < 2; i++ {
		if _, err := adminstate.Read(context.Background(), h); diag.ExitCode(err) != diag.AdministrationRecoveryRequired.Code ||
			!strings.Contains(err.Error(), "work plugin marketplace remove") {
			t.Fatalf("read = %v", err)
		}
	}
	if got := stateImages(t, h); !equalImages(before, got) {
		t.Fatal("a read changed state")
	}
}
