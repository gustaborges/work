package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

type env struct {
	home    workhome.Home
	db      *projection.DB
	ws      string
	srcRepo string
}

// setup builds a Work home, a projection, a workspace, and a source repo, then
// materializes one real in-progress Work per slug (a git worktree + snapshot +
// projection row). Works are created oldest-first so slice order == recency.
func setup(t *testing.T, slugs ...string) (env, map[string]projection.Work) {
	t.Helper()
	root := t.TempDir()
	h := workhome.At(filepath.Join(root, "dotwork"))
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	db, err := projection.Open(h.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	src := gittest.Repo(t)
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(filepath.Join(ws, "in-progress"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := env{home: h, db: db, ws: ws, srcRepo: src}
	rows := make(map[string]projection.Work, len(slugs))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, slug := range slugs {
		dir := filepath.Join(ws, "in-progress", "demo_"+slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		wt := filepath.Join(dir, "worktree")
		if err := gitx.Open(src).WorktreeAdd(wt, slug, "refs/heads/main"); err != nil {
			t.Fatalf("worktree add %s: %v", slug, err)
		}
		snap := filepath.Join(dir, "work-state.json")
		ts := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
		st := &work.State{
			Schema: work.Schema,
			Work: work.WorkSection{
				ID: "01" + slug + pad(slug), Slug: slug, Status: work.StatusInProgress,
				StartMode: "new", Starter: "local-path-starter", Branch: slug,
				BaseBranch: "main", BranchConvention: "freeform",
				CreatedAt: ts, LastAccessedAt: ts,
			},
			Meta: map[string]any{}, Links: map[string]string{},
		}
		if err := work.Write(snap, st); err != nil {
			t.Fatal(err)
		}
		row := projection.Work{
			ID: st.Work.ID, Slug: slug, Status: work.StatusInProgress, StartMode: "new",
			Starter: "local-path-starter", Branch: slug, BaseBranch: "main",
			BranchConvention: "freeform", RepoName: "demo", DirPath: dir,
			WorktreePath: wt, SnapshotPath: snap, CreatedAt: ts, LastAccessedAt: ts,
		}
		if err := db.Upsert(row); err != nil {
			t.Fatal(err)
		}
		rows[slug] = row
	}
	return e, rows
}

// pad makes a 26-char ULID-shaped id out of a short slug.
func pad(s string) string {
	const fill = "0000000000000000000000000000"
	return fill[:26-2-len(s)]
}

func TestRunArchivesCleanBatch(t *testing.T) {
	e, rows := setup(t, "alpha", "bravo")
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"], rows["bravo"]},
		Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Archived() != 2 {
		t.Fatalf("archived %d of 2", rep.Archived())
	}
	for _, slug := range []string{"alpha", "bravo"} {
		old := rows[slug].DirPath
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("%s: in-progress dir still present: %v", slug, err)
		}
		dst := filepath.Join(e.ws, "archived", "20260906-demo_"+slug)
		snap, err := work.Read(filepath.Join(dst, "work-state.json"))
		if err != nil {
			t.Fatalf("%s: archived snapshot: %v", slug, err)
		}
		if snap.Work.Status != work.StatusArchived || snap.Work.ArchivedAt == "" {
			t.Errorf("%s: snapshot not archived: %+v", slug, snap.Work)
		}
		if _, err := os.Stat(filepath.Join(dst, "worktree")); !os.IsNotExist(err) {
			t.Errorf("%s: worktree survived the archive", slug)
		}
		row, ok, _ := e.db.Get(rows[slug].ID)
		if !ok || row.Status != work.StatusArchived || row.WorktreePath != "" {
			t.Errorf("%s: row not archived: %+v", slug, row)
		}
		// The branch the Work created is left intact (FR-014).
		if exists, _ := gitx.Open(e.srcRepo).ShowRefVerify("refs/heads/" + slug); !exists {
			t.Errorf("%s: archiving deleted the branch", slug)
		}
	}
}

func TestRunLeavesDirtyWorkActive(t *testing.T) {
	e, rows := setup(t, "alpha")
	// Make the worktree dirty.
	if err := os.WriteFile(filepath.Join(rows["alpha"].WorktreePath, "UNTRACKED"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Outcomes[0].State != StateLeftActive {
		t.Fatalf("state = %d, want StateLeftActive", rep.Outcomes[0].State)
	}
	snap, _ := work.Read(rows["alpha"].SnapshotPath)
	if snap.Work.Status != work.StatusInProgress {
		t.Errorf("dirty Work snapshot was changed: %+v", snap.Work)
	}
	if _, err := os.Stat(rows["alpha"].WorktreePath); err != nil {
		t.Errorf("dirty Work worktree was removed: %v", err)
	}
}

func TestRunForceDirtyArchivesDirty(t *testing.T) {
	e, rows := setup(t, "alpha")
	os.WriteFile(filepath.Join(rows["alpha"].WorktreePath, "UNTRACKED"), []byte("x"), 0o644)
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]}, ForceDirty: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Archived() != 1 {
		t.Fatalf("--force-dirty did not archive the dirty Work")
	}
}

func TestRunFaultInjectionLeavesWorkFullyInOneState(t *testing.T) {
	for _, step := range []string{"snapshot", "worktree", "move"} {
		t.Run(step, func(t *testing.T) {
			e, rows := setup(t, "alpha", "bravo")
			t.Setenv("WORK_FAIL_AT", step)
			rep, err := Run(context.Background(), Params{
				Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
				Rows: []projection.Work{rows["alpha"], rows["bravo"]},
				Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			// alpha (not last) is fully archived.
			if rep.Outcomes[0].State != StateArchived {
				t.Errorf("alpha state = %d, want StateArchived", rep.Outcomes[0].State)
			}
			if _, err := os.Stat(rows["alpha"].DirPath); !os.IsNotExist(err) {
				t.Errorf("alpha in-progress dir survived")
			}
			// bravo (last, fault injected pre-commit) is fully active.
			if rep.Outcomes[1].State != StateLeftActive {
				t.Errorf("bravo state = %d, want StateLeftActive", rep.Outcomes[1].State)
			}
			snap, _ := work.Read(rows["bravo"].SnapshotPath)
			if snap.Work.Status != work.StatusInProgress {
				t.Errorf("bravo snapshot = %q, want in-progress", snap.Work.Status)
			}
			if head, err := gitx.Open(rows["bravo"].WorktreePath).CurrentBranch(); err != nil || head != "bravo" {
				t.Errorf("bravo worktree not intact: head=%q err=%v", head, err)
			}
			row, _, _ := e.db.Get(rows["bravo"].ID)
			if row.Status != work.StatusInProgress {
				t.Errorf("bravo row = %q, want in-progress", row.Status)
			}
		})
	}
}

func TestRunProjectionFaultSelfHeals(t *testing.T) {
	e, rows := setup(t, "alpha")
	t.Setenv("WORK_FAIL_AT", "projection")
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]},
		Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v (Upsert heal should have recovered)", err)
	}
	if rep.Outcomes[0].State != StateArchived {
		t.Fatalf("state = %d, want StateArchived (healed)", rep.Outcomes[0].State)
	}
	row, _, _ := e.db.Get(rows["alpha"].ID)
	if row.Status != work.StatusArchived {
		t.Errorf("row not healed: %+v", row)
	}
}

func TestRunProjectionFaultFailsWhenHealFails(t *testing.T) {
	e, rows := setup(t, "alpha")
	t.Setenv("WORK_FAIL_AT", "projection")
	e.db.Close() // both MarkArchived and the Upsert heal now fail
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]},
		Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
	})
	if diag.ExitCode(err) != 24 {
		t.Fatalf("exit = %d, want 24 (%v)", diag.ExitCode(err), err)
	}
	if rep.Outcomes[0].State != StateFailed {
		t.Fatalf("state = %d, want StateFailed", rep.Outcomes[0].State)
	}
	// The Work is still canonically archived on disk (not rolled back).
	dst := filepath.Join(e.ws, "archived", "20260906-demo_alpha")
	snap, rerr := work.Read(filepath.Join(dst, "work-state.json"))
	if rerr != nil || snap.Work.Status != work.StatusArchived {
		t.Errorf("canonically-archived Work was rolled back: %v %+v", rerr, snap)
	}
}

func TestRunStepsOutOfTheCurrentDirectoryWorktree(t *testing.T) {
	e, rows := setup(t, "alpha")
	wt := rows["alpha"].WorktreePath
	// Put the process cwd inside the worktree being archived; the orchestrator
	// must step out (to the workspace root) before removing it — Windows cannot
	// delete a live process's cwd.
	t.Chdir(wt)

	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]},
		Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Archived() != 1 {
		t.Fatalf("cwd-worktree archive failed: %+v", rep.Outcomes[0])
	}
	if !rep.Outcomes[0].WasCWD {
		t.Error("WasCWD not flagged for the current-directory Work")
	}
	if got, _ := os.Getwd(); got != mustEval(t, e.ws) && got != e.ws {
		t.Errorf("cwd = %q after archiving its own worktree, want the workspace root %q", got, e.ws)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}

func TestRunMissingWorktreeDegrades(t *testing.T) {
	e, rows := setup(t, "alpha")
	// Manually delete the worktree directory (git still has the admin entry).
	if err := os.RemoveAll(rows["alpha"].WorktreePath); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), Params{
		Home: e.home, DB: e.db, WorkspaceRoot: e.ws,
		Rows: []projection.Work{rows["alpha"]},
		Now:  time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Archived() != 1 {
		t.Fatalf("a missing worktree should not block the archive: %+v", rep.Outcomes[0])
	}
}
