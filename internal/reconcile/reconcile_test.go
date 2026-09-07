package reconcile

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
)

// writeSnap materializes a Work directory with a valid snapshot under
// <ws>/<area>/<dirName>/work-state.json and returns the snapshot path.
func writeSnap(t *testing.T, ws, area, dirName, id, slug, branch, status, last string) string {
	t.Helper()
	dir := filepath.Join(ws, area, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &work.State{
		Schema: work.Schema,
		Work: work.WorkSection{
			ID: id, Slug: slug, Status: status, StartMode: "new",
			Starter: "local-path-starter", Branch: branch, BaseBranch: "main",
			BranchConvention: "freeform", CreatedAt: "2026-01-01T00:00:00Z",
			LastAccessedAt: last,
		},
		Meta:  map[string]any{},
		Links: map[string]string{},
	}
	if status == work.StatusArchived {
		s.Work.ArchivedAt = last
	}
	snap := filepath.Join(dir, "work-state.json")
	if err := work.Write(snap, s); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	return snap
}

type fingerprint struct {
	mtime time.Time
	sum   [32]byte
}

func fingerprintTree(t *testing.T, ws string) map[string]fingerprint {
	t.Helper()
	out := map[string]fingerprint{}
	err := filepath.Walk(ws, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() || filepath.Base(p) != "work-state.json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[p] = fingerprint{mtime: fi.ModTime(), sum: sha256.Sum256(data)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertSnapshotsUntouched(t *testing.T, ws string, before map[string]fingerprint) {
	t.Helper()
	after := fingerprintTree(t, ws)
	if len(after) != len(before) {
		t.Fatalf("snapshot count changed: %d -> %d", len(before), len(after))
	}
	for p, b := range before {
		a, ok := after[p]
		if !ok {
			t.Errorf("snapshot vanished: %s", p)
			continue
		}
		if a.sum != b.sum {
			t.Errorf("snapshot content changed: %s", p)
		}
		if !a.mtime.Equal(b.mtime) {
			t.Errorf("snapshot mtime changed: %s (%v -> %v)", p, b.mtime, a.mtime)
		}
	}
}

func openDBAt(t *testing.T, path string) *projection.DB {
	t.Helper()
	db, err := projection.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRebuildClassifiesAndOrders(t *testing.T) {
	ws := t.TempDir()
	writeSnap(t, ws, "in-progress", "demo_alpha", "01ALPHA0000000000000000000", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z")
	writeSnap(t, ws, "in-progress", "demo_bravo", "01BRAVO0000000000000000000", "bravo", "bravo", "in-progress", "2026-06-03T00:00:00Z")
	writeSnap(t, ws, "archived", "20260605-demo_charlie", "01CHARLIE00000000000000000", "charlie", "charlie", "archived", "2026-06-05T00:00:00Z")
	// One corrupt snapshot: readable file, invalid JSON.
	badDir := filepath.Join(ws, "in-progress", "demo_delta")
	os.MkdirAll(badDir, 0o755)
	os.WriteFile(filepath.Join(badDir, "work-state.json"), []byte("not json"), 0o644)

	dbPath := filepath.Join(t.TempDir(), "work.db")
	before := fingerprintTree(t, ws)

	rep, err := Rebuild(ws, dbPath)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if rep.Indexed != 3 {
		t.Errorf("indexed = %d, want 3", rep.Indexed)
	}
	if len(rep.Skipped) != 1 || filepath.Base(filepath.Dir(rep.Skipped[0].Path)) != "demo_delta" {
		t.Errorf("skipped = %+v, want the demo_delta snapshot", rep.Skipped)
	}
	if rep.SnapshotsWritten != 0 {
		t.Errorf("SnapshotsWritten = %d, want 0", rep.SnapshotsWritten)
	}
	assertSnapshotsUntouched(t, ws, before)

	db := openDBAt(t, dbPath)
	if v, _ := db.UserVersion(); v != 2 {
		t.Errorf("user_version = %d, want 2", v)
	}
	all, _ := db.List()
	gotOrder := []string{all[0].Slug, all[1].Slug, all[2].Slug}
	want := []string{"charlie", "bravo", "alpha"}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("order = %v, want %v", gotOrder, want)
		}
	}

	active, _ := db.ListActive()
	if len(active) != 2 {
		t.Errorf("active = %d, want 2", len(active))
	}
	var charlie projection.Work
	for _, w := range all {
		if w.Slug == "charlie" {
			charlie = w
		}
	}
	if charlie.Status != "archived" || charlie.WorktreePath != "" || charlie.RepoName != "demo" {
		t.Errorf("archived row wrong: %+v", charlie)
	}
	if charlie.ArchivedAt != "2026-06-05T00:00:00Z" {
		t.Errorf("archived_at = %q", charlie.ArchivedAt)
	}
	if filepath.Base(filepath.Dir(charlie.DirPath)) != "archived" {
		t.Errorf("archived dir_path = %q", charlie.DirPath)
	}
}

func TestReconcileFixesStaleAndDropsGhosts(t *testing.T) {
	ws := t.TempDir()
	writeSnap(t, ws, "in-progress", "demo_alpha", "01ALPHA0000000000000000000", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z")
	dbPath := filepath.Join(t.TempDir(), "work.db")

	// Prime the DB with a rebuild, then hand-corrupt it.
	if _, err := Rebuild(ws, dbPath); err != nil {
		t.Fatal(err)
	}
	before := fingerprintTree(t, ws)

	db := openDBAt(t, dbPath)
	// Stale last_accessed_at on the real row.
	if err := db.SetAccessed("01ALPHA0000000000000000000", "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// A ghost row whose snapshot does not exist.
	ghost := projection.Work{
		ID: "01GHOST0000000000000000000", Slug: "ghost", Status: "in-progress", StartMode: "new",
		Starter: "local-path-starter", Branch: "ghost", BaseBranch: "main", BranchConvention: "freeform",
		RepoName: "demo", DirPath: filepath.Join(ws, "in-progress", "demo_ghost"),
		WorktreePath: filepath.Join(ws, "in-progress", "demo_ghost", "worktree"),
		SnapshotPath: filepath.Join(ws, "in-progress", "demo_ghost", "work-state.json"),
		CreatedAt:    "2026-01-01T00:00:00Z", LastAccessedAt: "2026-07-01T00:00:00Z",
	}
	if err := db.Upsert(ghost); err != nil {
		t.Fatal(err)
	}

	rep, err := Reconcile(db, ws)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if rep.Dropped != 1 {
		t.Errorf("dropped = %d, want 1", rep.Dropped)
	}
	if rep.SnapshotsWritten != 0 {
		t.Errorf("SnapshotsWritten = %d, want 0", rep.SnapshotsWritten)
	}

	got, ok, _ := db.Get("01ALPHA0000000000000000000")
	if !ok || got.LastAccessedAt != "2026-06-01T00:00:00Z" {
		t.Errorf("stale time not corrected: %+v", got)
	}
	if _, ok, _ := db.Get("01GHOST0000000000000000000"); ok {
		t.Errorf("ghost row not dropped")
	}
	assertSnapshotsUntouched(t, ws, before)
}

func TestReconcileReaddsMissingRow(t *testing.T) {
	ws := t.TempDir()
	writeSnap(t, ws, "in-progress", "demo_alpha", "01ALPHA0000000000000000000", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z")
	dbPath := filepath.Join(t.TempDir(), "work.db")
	if _, err := Rebuild(ws, dbPath); err != nil {
		t.Fatal(err)
	}
	db := openDBAt(t, dbPath)
	if err := db.Delete("01ALPHA0000000000000000000"); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(db, ws); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.Get("01ALPHA0000000000000000000"); !ok {
		t.Errorf("missing row not re-added by reconcile")
	}
}

func TestOpenRebuildsWhenAbsentAndReconcilesWhenHealthy(t *testing.T) {
	ws := t.TempDir()
	writeSnap(t, ws, "in-progress", "demo_alpha", "01ALPHA0000000000000000000", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z")
	dbPath := filepath.Join(t.TempDir(), "work.db")

	// Absent -> rebuilt at v2.
	db, _, err := Open(ws, dbPath)
	if err != nil {
		t.Fatalf("Open (absent): %v", err)
	}
	if v, _ := db.UserVersion(); v != 2 {
		t.Errorf("user_version after rebuild = %d, want 2", v)
	}
	db.Close()

	// Healthy v2 -> reconcile only (row preserved, no drop of the real Work).
	db2, rep, err := Open(ws, dbPath)
	if err != nil {
		t.Fatalf("Open (healthy): %v", err)
	}
	defer db2.Close()
	if rep.Dropped != 0 {
		t.Errorf("healthy reconcile dropped %d rows", rep.Dropped)
	}
	if _, ok, _ := db2.Get("01ALPHA0000000000000000000"); !ok {
		t.Errorf("row lost on healthy Open")
	}
}
