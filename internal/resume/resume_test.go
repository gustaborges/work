package resume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// fixture materializes a Work directory with a snapshot and a projection row and
// returns the pieces Run needs.
func fixture(t *testing.T, status string) (home workhome.Home, db *projection.DB, id, snapPath, wtPath string) {
	t.Helper()
	root := t.TempDir()
	home = workhome.At(root)
	if err := home.EnsureLayout(); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "ws", "in-progress", "demo_alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath = filepath.Join(dir, "worktree")
	snapPath = filepath.Join(dir, "work-state.json")
	id = "01ABCDEFGHJKMNPQRSTVWXYZ01"
	created := "2026-01-01T00:00:00Z"

	s := &work.State{
		Schema: work.Schema,
		Work: work.WorkSection{
			ID: id, Slug: "alpha", Status: status, StartMode: "new",
			Starter: "local-path-starter", Branch: "alpha", BaseBranch: "main",
			BranchConvention: "freeform", CreatedAt: created, LastAccessedAt: created,
		},
		Meta: map[string]any{}, Links: map[string]string{},
	}
	if status == work.StatusArchived {
		s.Work.ArchivedAt = created
	}
	if err := work.Write(snapPath, s); err != nil {
		t.Fatal(err)
	}

	var err error
	db, err = projection.Open(home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(projection.Work{
		ID: id, Slug: "alpha", Status: status, StartMode: "new",
		Starter: "local-path-starter", Branch: "alpha", BaseBranch: "main",
		BranchConvention: "freeform", RepoName: "demo", DirPath: dir,
		WorktreePath: wtPath, SnapshotPath: snapPath, CreatedAt: created,
		LastAccessedAt: created, ArchivedAt: s.Work.ArchivedAt,
	}); err != nil {
		t.Fatal(err)
	}
	return home, db, id, snapPath, wtPath
}

func TestRunBumpsSnapshotAndProjectionInStep(t *testing.T) {
	home, db, id, snapPath, wtPath := fixture(t, work.StatusInProgress)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	res, err := Run(context.Background(), Params{
		Home: home, DB: db, ID: id, SnapshotPath: snapPath, WorktreePath: wtPath, Now: now,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.WorktreePath != wtPath {
		t.Errorf("worktree = %q, want %q", res.WorktreePath, wtPath)
	}
	if res.IndexStale {
		t.Error("IndexStale = true, want false")
	}

	snap, err := work.Read(snapPath)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Format(time.RFC3339)
	if snap.Work.LastAccessedAt != want {
		t.Errorf("snapshot last_accessed_at = %q, want %q", snap.Work.LastAccessedAt, want)
	}
	if snap.Schema != 2 {
		t.Errorf("snapshot schema = %d, want 2", snap.Schema)
	}
	row, _, err := db.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if row.LastAccessedAt != snap.Work.LastAccessedAt {
		t.Errorf("projection last_accessed_at = %q, snapshot = %q", row.LastAccessedAt, snap.Work.LastAccessedAt)
	}
}

func TestRunRefusesArchived(t *testing.T) {
	home, db, id, snapPath, wtPath := fixture(t, work.StatusArchived)
	before, _ := work.Read(snapPath)

	_, err := Run(context.Background(), Params{
		Home: home, DB: db, ID: id, SnapshotPath: snapPath, WorktreePath: wtPath,
	})
	var d *diag.Error
	if !errors.As(err, &d) || d.Category != diag.TargetArchived {
		t.Fatalf("err = %v, want diag TargetArchived", err)
	}
	after, _ := work.Read(snapPath)
	if before.Work.LastAccessedAt != after.Work.LastAccessedAt {
		t.Error("archived refusal still wrote the snapshot")
	}
}

func TestRunSucceedsWhenProjectionTrailerFails(t *testing.T) {
	home, db, id, snapPath, wtPath := fixture(t, work.StatusInProgress)
	db.Close() // force SetAccessed to fail

	res, err := Run(context.Background(), Params{
		Home: home, DB: db, ID: id, SnapshotPath: snapPath, WorktreePath: wtPath,
	})
	if err != nil {
		t.Fatalf("Run: %v (a post-commit trailer failure must not fail the resume)", err)
	}
	if !res.IndexStale {
		t.Error("IndexStale = false, want true after the projection write failed")
	}
	// The canonical snapshot was still committed.
	snap, err := work.Read(snapPath)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Work.LastAccessedAt == "2026-01-01T00:00:00Z" {
		t.Error("snapshot was not bumped")
	}
}

func TestRunFailsCleanlyUnderContention(t *testing.T) {
	home, db, id, snapPath, wtPath := fixture(t, work.StatusInProgress)

	// Hold the Work's advisory lock from another "operation".
	release, err := lockfile.Acquire(home.LockPath(lockKey(id)))
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	old := lockTimeout
	lockTimeout = 150 * time.Millisecond
	defer func() { lockTimeout = old }()

	before, _ := work.Read(snapPath)
	_, err = Run(context.Background(), Params{
		Home: home, DB: db, ID: id, SnapshotPath: snapPath, WorktreePath: wtPath,
	})
	var d *diag.Error
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want a diag error", err)
	}
	if !strings.Contains(d.Msg, "another work operation is using this Work") {
		t.Errorf("message = %q, want it to mention the contending operation", d.Msg)
	}
	after, _ := work.Read(snapPath)
	if before.Work.LastAccessedAt != after.Work.LastAccessedAt {
		t.Error("a contended resume still wrote the snapshot")
	}
}
