package resume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/archive"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// workspaceRootOf recovers <root>/ws from a snapshot path the fixture built at
// <root>/ws/in-progress/demo_alpha/work-state.json.
func workspaceRootOf(snapPath string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(snapPath)))
}

// Two `work` operations racing on the SAME Work: only one mutation can be in
// flight at a time, and the operation that does not hold the advisory lock fails
// cleanly on the timeout ("another work operation is using this Work") without
// touching the snapshot or the index. Once the lock is free the same operation
// succeeds. resume+resume and resume+archive are both covered.
// (research R17; spec edge case.)
func TestConcurrentMutationsOnOneWorkFailClean(t *testing.T) {
	restore := archive.SetLockTimeoutForTest(150 * time.Millisecond)
	defer restore()
	oldR := lockTimeout
	lockTimeout = 150 * time.Millisecond
	defer func() { lockTimeout = oldR }()

	type op struct {
		name string
		call func(home workhome.Home, db *projection.DB, id, snap, wt string) error
	}
	resumeOp := op{"resume", func(home workhome.Home, db *projection.DB, id, snap, wt string) error {
		_, err := Run(context.Background(), Params{Home: home, DB: db, ID: id, SnapshotPath: snap, WorktreePath: wt})
		return err
	}}
	archiveOp := op{"archive", func(home workhome.Home, db *projection.DB, id, snap, wt string) error {
		row, ok, err := db.Get(id)
		if err != nil || !ok {
			return errors.New("archive: row missing")
		}
		row.WorktreePath = "" // keep archive off the git path; rename + projection still run
		rep, rerr := archive.Run(context.Background(), archive.Params{
			Home: home, DB: db, WorkspaceRoot: workspaceRootOf(snap),
			Rows: []projection.Work{row},
		})
		if rerr != nil {
			return rerr
		}
		if o := rep.Outcomes[0]; o.State != archive.StateArchived {
			return errors.New(o.Note) // a lock timeout surfaces here as StateLeftActive + note
		}
		return nil
	}}

	for _, tc := range []struct{ loser op }{{resumeOp}, {archiveOp}} {
		t.Run("winner holds lock, "+tc.loser.name+" loses", func(t *testing.T) {
			home, db, id, snap, wt := fixture(t, work.StatusInProgress)
			before, _ := work.Read(snap)

			// Stand in for the winning operation holding the Work's lock while it
			// works, longer than the loser will wait.
			release, err := lockfile.Acquire(home.LockPath(lockKey(id)))
			if err != nil {
				t.Fatal(err)
			}
			freed := make(chan struct{})
			go func() {
				time.Sleep(400 * time.Millisecond)
				release()
				close(freed)
			}()

			err = tc.loser.call(home, db, id, snap, wt)
			if err == nil || !strings.Contains(err.Error(), "another work operation is using this Work") {
				t.Fatalf("loser err = %v, want a clean lock-timeout failure", err)
			}
			if after, _ := work.Read(snap); after.Work.Status != work.StatusInProgress ||
				after.Work.LastAccessedAt != before.Work.LastAccessedAt {
				t.Fatalf("loser mutated the snapshot: %+v", after.Work)
			}
			if row, _, _ := db.Get(id); row.LastAccessedAt != before.Work.LastAccessedAt {
				t.Fatalf("loser mutated the index: %+v", row)
			}

			<-freed
			if err := tc.loser.call(home, db, id, snap, wt); err != nil {
				t.Fatalf("%s still failing after the lock was released: %v", tc.loser.name, err)
			}
		})
	}
}

// Operations on DIFFERENT Works take different lock keys and never block each
// other.
func TestConcurrentOperationsOnDifferentWorksDoNotContend(t *testing.T) {
	oldR := lockTimeout
	lockTimeout = 200 * time.Millisecond
	defer func() { lockTimeout = oldR }()

	home, db, idA, snapA, wtA := fixture(t, work.StatusInProgress)

	// Materialize a second Work sharing the same home/projection.
	idB := "01BBBBBBBBBBBBBBBBBBBBBBBBB"
	dirB := filepath.Join(workspaceRootOf(snapA), "in-progress", "demo_bravo")
	if err := writeWorkFixture(dirB, idB, "bravo"); err != nil {
		t.Fatal(err)
	}
	snapB := filepath.Join(dirB, "work-state.json")
	if err := db.Upsert(projection.Work{
		ID: idB, Slug: "bravo", Status: work.StatusInProgress, StartMode: "new",
		Starter: "local-path-starter", Branch: "bravo", BaseBranch: "main",
		BranchConvention: "freeform", RepoName: "demo", DirPath: dirB,
		WorktreePath: filepath.Join(dirB, "worktree"), SnapshotPath: snapB,
		CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	// Hold work A's lock for the whole test.
	releaseA, err := lockfile.Acquire(home.LockPath(lockKey(idA)))
	if err != nil {
		t.Fatal(err)
	}
	defer releaseA()

	done := make(chan error, 1)
	go func() {
		_, e := Run(context.Background(), Params{
			Home: home, DB: db, ID: idB, SnapshotPath: snapB, WorktreePath: filepath.Join(dirB, "worktree"),
		})
		done <- e
	}()

	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("resume of an unrelated Work failed: %v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resume of an unrelated Work blocked on another Work's lock")
	}

	if s, _ := work.Read(snapA); s.Work.LastAccessedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("work A snapshot changed: %q", s.Work.LastAccessedAt)
	}
	_, _ = wtA, snapA
}

// writeWorkFixture materializes a minimal in-progress Work snapshot at
// dir/work-state.json.
func writeWorkFixture(dir, id, slug string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return work.Write(filepath.Join(dir, "work-state.json"), &work.State{
		Schema: work.Schema,
		Work: work.WorkSection{
			ID: id, Slug: slug, Status: work.StatusInProgress, StartMode: "new",
			Starter: "local-path-starter", Branch: slug, BaseBranch: "main",
			BranchConvention: "freeform", CreatedAt: "2026-01-01T00:00:00Z",
			LastAccessedAt: "2026-01-01T00:00:00Z",
		},
		Meta: map[string]any{}, Links: map[string]string{},
	})
}
