package integration

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

var (
	alphaDemo  = pluginid.Provider{Marketplace: "demo", Plugin: "alpha"}
	betaDemo   = pluginid.Provider{Marketplace: "demo", Plugin: "beta"}
	sampleSolo = pluginid.Provider{Marketplace: "solo", Plugin: "sample"}
)

// Refresh, installs and a removal of another source race for the one
// administration lock. Whatever order they take, the result is a coherent
// state: every installed copy belongs to a registered source and nothing is
// left staged, leased or half-journaled.
func TestConcurrentRefreshInstallAndRemoveSerialize(t *testing.T) {
	h := newRecoveryHome(t)
	registerLocal(t, h, marketplaces.Copy(t, "multiple"))
	registerLocal(t, h, marketplaces.Copy(t, "single"))
	installNamed(t, h, "sample", "solo")

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- fn()
		}()
	}
	run(func() error {
		_, err := marketplaceadmin.Upgrade(context.Background(), h, []string{"demo"})
		return err
	})
	for _, name := range []string{"alpha", "beta"} {
		run(func() error {
			plan, err := pluginadmin.PrepareInstall(context.Background(), h, name, "demo")
			if err != nil {
				return err
			}
			defer plan.Close()
			_, err = plan.Apply(context.Background(), false)
			return err
		})
	}
	run(func() error {
		plan, err := marketplaceadmin.PrepareRemove(context.Background(), h, "solo")
		if err != nil {
			return err
		}
		_, err = plan.Apply(context.Background(), true)
		return err
	})
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		// A lost race may conflict on exactly the facts it reviewed; nothing else is acceptable.
		if err != nil && diag.ExitCode(err) != diag.AdministrationConflict.Code {
			t.Errorf("unexpected failure: %v (exit %d)", err, diag.ExitCode(err))
		}
	}

	s := adminRead(t, h)
	if _, ok := s.Config.Marketplace("solo"); ok {
		t.Fatal("the removed source is still registered")
	}
	for _, pkg := range s.Registry.Packages {
		if pkg.Provider == sampleSolo {
			t.Fatal("a removed source kept an installed copy")
		}
		if _, ok := s.Config.Marketplace(pkg.Provider.Marketplace); !ok {
			t.Fatalf("%s outlived its source", pkg.Provider)
		}
		if s.Inspection[pkg.ID()] != "ready" {
			t.Fatalf("%s inspection = %q", pkg.ID(), s.Inspection[pkg.ID()])
		}
	}
	requireClean(t, h)
	if leases, _ := os.ReadDir(h.LeasesDir()); len(leases) != 0 {
		t.Fatalf("leases remain: %v", leases)
	}
}

// A force-pushed source no longer has the registered commit. Installing from
// the stored offer fails rather than falling forward to the new tip; an
// explicit refresh then adopts the new history.
func TestForcePushedSourceNeverFallsForwardUntilRefreshed(t *testing.T) {
	h := newRecoveryHome(t)
	dir := marketplaces.Copy(t, "single")
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "one")
	registerLocal(t, h, "file://"+filepath.ToSlash(dir))

	tool := filepath.Join(dir, "plugins", "sample", "bin", "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho rewritten\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "--amend", "-m", "one, rewritten")
	gittest.Git(t, dir, "reflog", "expire", "--expire=now", "--all")
	gittest.Git(t, dir, "gc", "-q", "--prune=now")

	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "sample", "solo")
	if plan != nil {
		plan.Close()
	}
	if diag.ExitCode(err) != diag.MarketplaceSourceUnavailable.Code {
		t.Fatalf("install from a vanished commit = %v (exit %d)", err, diag.ExitCode(err))
	}
	if n := len(adminRead(t, h).Registry.Packages); n != 0 {
		t.Fatalf("a failed install registered %d packages", n)
	}
	requireClean(t, h)

	if _, err = marketplaceadmin.Upgrade(context.Background(), h, []string{"solo"}); err != nil {
		t.Fatal(err)
	}
	installNamed(t, h, "sample", "solo")
	root, _ := h.PackageRoot(sampleSolo)
	if b, _ := os.ReadFile(filepath.Join(root, "source", "bin", "tool")); string(b) != "#!/bin/sh\necho rewritten\n" {
		t.Fatalf("installed %q", b)
	}
}

// Stage and capture leases protect prepared-but-unpublished objects from the
// collection that every mutation performs, for exactly as long as the plan is
// open.
func TestLeasesProtectLivePreparationUntilClosed(t *testing.T) {
	h := newRecoveryHome(t)
	registerLocal(t, h, marketplaces.Copy(t, "single"))
	stored := adminRead(t, h).Catalogs.Catalogs[0]
	hexDigest, _ := marketplace.HexPayload(*stored.CaptureDigest)
	capture, _ := h.SnapshotDir(hexDigest)

	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "sample", "solo")
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(h.PluginsDir())
	if len(entries) != 1 {
		t.Fatalf("expected one live stage, found %v", entries)
	}
	stage := entries[0].Name()

	// The source is removed and several mutations (each collecting orphans) run.
	removal, err := marketplaceadmin.PrepareRemove(context.Background(), h, "solo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = removal.Apply(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := adminstate.WithWrite(context.Background(), h, "work repository policy replace", func(s *adminstate.Session) error {
			s.Persist("lifecycle")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(h.PluginsDir(), stage), capture} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("a live preparation lost %s: %v", path, err)
		}
	}
	if _, err := plan.Apply(context.Background(), false); diag.ExitCode(err) != diag.AdministrationConflict.Code {
		t.Fatalf("applying against a removed source = %v", err)
	}

	plan.Close()
	if err := adminstate.WithWrite(context.Background(), h, "work repository policy replace", func(s *adminstate.Session) error {
		s.Persist("lifecycle")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(h.PluginsDir(), stage), capture} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s outlived its closed preparation: %v", path, err)
		}
	}
}

// Removing a package whose component is running waits for the invocation (which
// holds only the administration lock) and the stale invocation that follows is
// refused instead of executing deleted bytes.
func TestRunningPackageRemovalWaitsAndNeverDeletesActiveBytes(t *testing.T) {
	h := newRecoveryHome(t)
	registerLocal(t, h, marketplaces.Copy(t, "multiple"))
	installNamed(t, h, "alpha", "demo")
	component := adminRead(t, h).Registry.ProviderComponents(alphaDemo)[0]
	entry := component.EntrypointPath(h.PluginsDir())

	running, finish := make(chan struct{}), make(chan struct{})
	invoked := make(chan error, 1)
	go func() {
		invoked <- adminstate.WithInvocation(context.Background(), h, component, func() error {
			close(running)
			<-finish
			return nil
		})
	}()
	<-running

	removed := make(chan error, 1)
	go func() {
		plan, err := pluginadmin.PlanUninstall(context.Background(), h, []pluginid.Provider{alphaDemo})
		if err != nil {
			removed <- err
			return
		}
		_, err = pluginadmin.Uninstall(context.Background(), h, plan, false)
		removed <- err
	}()
	select {
	case err := <-removed:
		t.Fatalf("removal did not wait for the running component: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := os.Stat(filepath.Dir(entry)); err != nil {
		t.Fatalf("active bytes were deleted: %v", err)
	}

	close(finish)
	if err := <-invoked; err != nil {
		t.Fatal(err)
	}
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
	err := adminstate.WithInvocation(context.Background(), h, component, func() error {
		t.Fatal("a stale invocation ran")
		return nil
	})
	if diag.ExitCode(err) != diag.AdministrationConflict.Code {
		t.Fatalf("stale invocation = %v (exit %d)", err, diag.ExitCode(err))
	}
}

// A reviewed update merges with unrelated concurrent changes, including a
// refresh of its own source, and conflicts only on the facts it depended on.
func TestUnaffectedConcurrentChangesMergeWithAnUpdate(t *testing.T) {
	h := newRecoveryHome(t)
	src := marketplaces.Copy(t, "multiple")
	registerLocal(t, h, src)
	installNamed(t, h, "alpha", "demo")
	installNamed(t, h, "beta", "demo")
	publishVersion(t, src, "alpha", "1.0.0", "2.0.0", "")

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{alphaDemo}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if _, err = pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{betaDemo}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = marketplaceadmin.Upgrade(context.Background(), h, []string{"demo"}); err != nil {
		t.Fatal(err)
	}
	rows, err := pluginadmin.ApplyUpdates(context.Background(), plan, false)
	if err != nil || rows[0].Status != pluginadmin.UpdateUpdated {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	s := adminRead(t, h)
	alpha, _ := s.Registry.PackageByProvider(alphaDemo)
	if alpha.Version != "2.0.0" || s.Registry.PackageEnabled(betaDemo) {
		t.Fatalf("alpha=%+v betaEnabled=%v", alpha, s.Registry.PackageEnabled(betaDemo))
	}
}
