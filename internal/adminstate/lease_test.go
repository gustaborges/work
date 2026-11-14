package adminstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

func hexOf(c byte) string { return strings.Repeat(string(c), 64) }

func mkdir(t *testing.T, path string, files ...string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(path, f), []byte(f), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func gone(path string) bool { _, err := os.Lstat(path); return os.IsNotExist(err) }

func TestLiveStageLeaseProtectsFromCollection(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	lease, stage, err := NewPackageStage(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(h.PluginsDir(), workhome.StagePrefix+"orphan")
	mkdir(t, orphan, "x")
	if err := os.WriteFile(filepath.Join(stage, "prepared"), []byte("outside the journal"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	if gone(filepath.Join(stage, "prepared")) {
		t.Fatal("a live stage prepared outside the journal was collected")
	}
	if !gone(orphan) {
		t.Fatal("an unleased orphan stage survived collection")
	}

	lease.Release()
	lease.Release() // idempotent
	if rec, _ := leasePaths(h, lease.ID()); !gone(rec) {
		t.Fatal("record not cleaned on release")
	}
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	if !gone(stage) {
		t.Fatal("a released, unreferenced stage must be collected")
	}
}

// A crashed process releases its OS lock, so liveness never depends on age or PID.
func TestCrashedHolderLeaseBecomesCollectable(t *testing.T) {
	if root := os.Getenv("WORK_TEST_LEASE_HOME"); root != "" {
		h := workhome.At(root)
		_, stage, err := NewPackageStage(context.Background(), h)
		if err != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(filepath.Join(stage, "partial"), []byte("x"), 0o600)
		_, cstage, err := NewCaptureStage(context.Background(), h)
		if err != nil {
			os.Exit(3)
		}
		_ = os.MkdirAll(cstage, 0o700)
		os.Exit(77)
	}
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashedHolderLeaseBecomesCollectable$")
	cmd.Env = append(os.Environ(), "WORK_TEST_LEASE_HOME="+h.Root())
	if err := cmd.Run(); err == nil {
		t.Fatal("helper survived")
	}
	stages, _ := filepath.Glob(filepath.Join(h.PluginsDir(), workhome.StagePrefix+"*"))
	captures, _ := filepath.Glob(filepath.Join(h.SnapshotsDir(), ".stage-*"))
	records, _ := filepath.Glob(filepath.Join(h.LeasesDir(), "*.json"))
	if len(stages) != 1 || len(captures) != 1 || len(records) != 2 {
		t.Fatalf("helper left %v %v %v", stages, captures, records)
	}
	if _, err := Preflight(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(append(stages, captures...), records...) {
		if !gone(p) {
			t.Errorf("abandoned object %s survived", p)
		}
	}
	if locks, _ := filepath.Glob(filepath.Join(h.LeasesDir(), "*.lock")); len(locks) != 0 {
		t.Errorf("dead lock files remain: %v", locks)
	}
}

func TestCaptureCollectionRespectsReferencesAndLeases(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	held, orphan := hexOf('b'), hexOf('c')
	st := storedLocal(t)
	referenced, _ := marketplace.HexPayload(*st.CaptureDigest)

	if err := WithWrite(ctx, h, "work plugin marketplace add", func(s *Session) error {
		s.Catalogs.Upsert(st)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{referenced, held, orphan} {
		d, _ := h.SnapshotDir(name)
		mkdir(t, d, "marketplace.json")
	}
	keep := filepath.Join(h.SnapshotsDir(), "keep.txt")
	if err := os.WriteFile(keep, []byte("not ours"), 0o600); err != nil {
		t.Fatal(err)
	}
	lease, err := HoldCapture(ctx, h, held)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	for name, wantKept := range map[string]bool{referenced: true, held: true, orphan: false} {
		d, _ := h.SnapshotDir(name)
		if gone(d) == wantKept {
			t.Errorf("capture %s kept=%v, want %v", name[:1], !gone(d), wantKept)
		}
	}
	if gone(keep) {
		t.Error("collection removed a path it does not own")
	}
	lease.Release()
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	if d, _ := h.SnapshotDir(held); !gone(d) {
		t.Error("an unreferenced capture survived after its lease ended")
	}
	if d, _ := h.SnapshotDir(referenced); gone(d) {
		t.Error("a referenced capture was collected")
	}
}

func TestHoldCaptureRequiresAnExistingCapture(t *testing.T) {
	h := workhome.At(t.TempDir())
	if _, err := HoldCapture(context.Background(), h, hexOf('d')); err == nil {
		t.Fatal("leased a capture that does not exist")
	}
	if _, err := HoldCapture(context.Background(), h, "../../etc"); err == nil {
		t.Fatal("leased a path-shaped digest")
	}
	if recs, _ := filepath.Glob(filepath.Join(h.LeasesDir(), "*")); len(recs) != 0 {
		t.Fatalf("failed acquisition left lease artifacts: %v", recs)
	}
}

func TestPublishCaptureIsHexOnlyAndIdempotent(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	st := storedLocal(t)
	d1 := *st.CaptureDigest
	hexE, _ := marketplace.HexPayload(d1)
	lease, stage, err := NewCaptureStage(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	mkdir(t, stage, "marketplace.json")
	if err := WithWrite(ctx, h, "work plugin marketplace add", func(s *Session) error {
		if err := s.PublishCapture(filepath.Join(h.PluginsDir(), "x"), d1); err == nil {
			t.Error("a foreign stage was published")
		}
		if err := s.PublishCapture(stage, "sha256:../../x"); err == nil {
			t.Error("a path-shaped digest was published")
		}
		if err := s.PublishCapture(stage, "md5:abc"); err == nil {
			t.Error("a foreign algorithm was published")
		}
		if err := s.PublishCapture(stage, d1); err != nil {
			return err
		}
		// Publication and the reference that retains it share one lock hold;
		// an unreferenced capture is collectable by the next mutation.
		s.Catalogs.Upsert(st)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir, _ := h.SnapshotDir(hexE)
	if _, err := os.Stat(filepath.Join(dir, "marketplace.json")); err != nil || !gone(stage) {
		t.Fatalf("publication = %v, stage gone=%v", err, gone(stage))
	}
	// Publishing the same digest again keeps the published bytes and drops the stage.
	lease2, stage2, _ := NewCaptureStage(ctx, h)
	defer lease2.Release()
	mkdir(t, stage2, "different")
	if err := WithWrite(ctx, h, "again", func(s *Session) error { return s.PublishCapture(stage2, d1) }); err != nil {
		t.Fatal(err)
	}
	if gone(filepath.Join(dir, "marketplace.json")) || !gone(stage2) {
		t.Error("idempotent publication replaced the published capture")
	}
}

func TestLeaseCancellationAndNoopCollectionLeaveStateUnchanged(t *testing.T) {
	h := workhome.At(t.TempDir())
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := NewPackageStage(cancelled, h); diag.ExitCode(err) != 20 {
		t.Fatalf("cancelled acquisition = %v", err)
	}
	if _, err := os.Stat(h.PluginsDir()); !os.IsNotExist(err) {
		t.Fatal("cancelled acquisition created state")
	}
	ctx := context.Background()
	if err := WithWrite(ctx, h, "setup", func(s *Session) error { s.Config.Workspace = "w"; return nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.AdministrationFile())
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(h.AdministrationFile())
	if string(before) != string(after) {
		t.Fatal("a collection pass with nothing to collect advanced the epoch")
	}
}

// A closing plan must never stall behind a long administration holder.
func TestReleaseDoesNotWaitForAdministration(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	lease, _, err := NewPackageStage(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	hold, err := acquire(ctx, h, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	lease.Release()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Release blocked for %v", elapsed)
	}
	// The lease lock itself is already free even though the record is not.
	_, lockPath := leasePaths(h, lease.ID())
	free, err := lockfile.AcquireContext(ctx, lockPath, 0)
	if err != nil {
		t.Fatalf("lease lock still held after Release: %v", err)
	}
	free()
	hold()
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	if rec, _ := leasePaths(h, lease.ID()); !gone(rec) {
		t.Fatal("a dead record left by a busy release was not collected")
	}
}

func TestMalformedLeaseArtifactsAreNotTrustedAsLive(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	mkdir(t, h.LeasesDir())
	id := strings.Repeat("a", 32)
	rec, _ := leasePaths(h, id)
	// A forged record naming a stage it does not own protects nothing without a held lock.
	if err := os.WriteFile(rec, []byte(`{"schema":1,"id":"`+id+`","kind":"package-stage","object":"../escape"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(h.LeasesDir(), "notes.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	if !gone(rec) || gone(stray) {
		t.Errorf("record gone=%v stray gone=%v; only lease artifacts may be collected", gone(rec), gone(stray))
	}
}

// storedLocal builds a valid empty local catalog whose capture digest is the
// one its (empty) entry list determines.
func storedLocal(t *testing.T) marketplace.Stored {
	t.Helper()
	cat := marketplace.CatalogDigest("team", "", "", nil)
	capture := marketplace.CaptureDigest(cat, nil)
	return marketplace.Stored{
		RegistrationID: "0123456789abcdef0123456789abcdef", Namespace: "team",
		Source:   marketplace.Source{Kind: pluginid.KindLocal, Source: "/src"},
		Revision: capture, CatalogDigest: cat, DigestAlgorithm: marketplace.AlgorithmCatalog,
		CaptureDigest: &capture, Entries: []marketplace.Entry{},
	}
}
