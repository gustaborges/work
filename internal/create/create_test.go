package create

import (
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/work/verify"
	"github.com/gustaborges/work/internal/workhome"
)

func params(t *testing.T) Params {
	t.Helper()
	h := workhome.At(filepath.Join(t.TempDir(), "dotwork"))
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	repo := gittest.Repo(t)
	ws := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(filepath.Join(ws, "in-progress"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Params{
		Home:            h,
		SourceRepo:      repo,
		RepoName:        filepath.Base(repo),
		WorkspaceRoot:   ws,
		Slug:            "add-retry",
		Branch:          "add-retry",
		BaseRefname:     "refs/heads/main",
		BaseBranchShort: "main",
		Convention:      "freeform",
		Starter:         "local-path-starter",
		Now:             time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC),
	}
}

func TestRunHappyPath(t *testing.T) {
	p := params(t)
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, _ := os.ReadDir(res.DirPath)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"work-state.json", "worktree"}) {
		t.Errorf("dir contents = %v", names)
	}

	snap, err := work.Read(res.SnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("snapshot invalid: %v", err)
	}
	if snap.Work.Branch != "add-retry" || snap.Work.BaseBranch != "main" ||
		snap.Work.Status != "in-progress" || snap.Work.StartMode != "new" ||
		snap.Work.BranchConvention != "freeform" || snap.Work.Starter != "local-path-starter" {
		t.Errorf("snapshot work = %+v", snap.Work)
	}
	if snap.Work.ID != res.WorkID {
		t.Errorf("id mismatch: snap %s res %s", snap.Work.ID, res.WorkID)
	}

	head, _ := gitx.Open(res.WorktreePath).CurrentBranch()
	if head != "add-retry" {
		t.Errorf("worktree HEAD = %q", head)
	}

	db, err := projection.Open(p.Home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rep, err := verify.Check(db, res.WorkID)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Errorf("verify.Check problems: %v", rep.Problems)
	}
}

func TestRunRollsBackAtEveryStep(t *testing.T) {
	for _, step := range []string{"lock", "dir", "worktree", "snapshot", "projection"} {
		t.Run(step, func(t *testing.T) {
			p := params(t)
			t.Setenv("WORK_FAIL_AT", step)
			_, err := Run(context.Background(), p)
			if diag.ExitCode(err) != 17 {
				t.Fatalf("exit = %d, want 17 (%v)", diag.ExitCode(err), err)
			}

			// No residue anywhere.
			src := gitx.Open(p.SourceRepo)
			if ok, _ := src.ShowRefVerify("refs/heads/add-retry"); ok {
				t.Error("orphan branch left in source repo")
			}
			wts, _ := src.WorktreeList()
			for _, w := range wts {
				if strings.Contains(w.Path, "add-retry") {
					t.Errorf("orphan worktree: %s", w.Path)
				}
			}
			dir := filepath.Join(p.WorkspaceRoot, "in-progress", p.RepoName+"_add-retry")
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("orphan work dir: %v", err)
			}
			if _, err := os.Stat(p.Home.DBFile()); err == nil {
				db, _ := projection.Open(p.Home.DBFile())
				rows, _ := db.List()
				db.Close()
				if len(rows) != 0 {
					t.Errorf("orphan db rows: %v", rows)
				}
			}
		})
	}
}

// assertNoResidue fails if any orphan branch, worktree, Work directory, or
// projection row survived a failed creation.
func assertNoResidue(t *testing.T, p Params) {
	t.Helper()
	src := gitx.Open(p.SourceRepo)
	if ok, _ := src.ShowRefVerify("refs/heads/" + p.Branch); ok {
		t.Error("orphan branch left in source repo")
	}
	wts, _ := src.WorktreeList()
	for _, w := range wts {
		if strings.Contains(w.Path, p.Branch) {
			t.Errorf("orphan worktree: %s", w.Path)
		}
	}
	dir := filepath.Join(p.WorkspaceRoot, "in-progress", p.RepoName+"_"+p.Branch)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("orphan work dir: %v", err)
	}
	if _, err := os.Stat(p.Home.DBFile()); err == nil {
		db, _ := projection.Open(p.Home.DBFile())
		rows, _ := db.List()
		db.Close()
		if len(rows) != 0 {
			t.Errorf("orphan db rows: %v", rows)
		}
	}
}

// TestRunRollbackFuzz injects a failure at a randomly chosen step many times and
// asserts every failed attempt leaves zero residue (R10, SC-003).
func TestRunRollbackFuzz(t *testing.T) {
	steps := []string{"lock", "dir", "worktree", "snapshot", "projection"}
	for i := range 20 {
		step := steps[rand.IntN(len(steps))]
		t.Run(step, func(t *testing.T) {
			p := params(t)
			p.Slug = "fuzz"
			p.Branch = "fuzz"
			t.Setenv("WORK_FAIL_AT", step)
			if _, err := Run(context.Background(), p); diag.ExitCode(err) != 17 {
				t.Fatalf("iter %d step %s: exit = %d, want 17 (%v)", i, step, diag.ExitCode(err), err)
			}
			assertNoResidue(t, p)
		})
	}
}

func TestRunDestinationUnavailable(t *testing.T) {
	p := params(t)
	dir := filepath.Join(p.WorkspaceRoot, "in-progress", p.RepoName+"_add-retry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), p)
	if diag.ExitCode(err) != 15 {
		t.Fatalf("exit = %d, want 15 (%v)", diag.ExitCode(err), err)
	}
}

func TestRunCancelledContext(t *testing.T) {
	p := params(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, p)
	if diag.ExitCode(err) != 20 {
		t.Fatalf("exit = %d, want 20 (%v)", diag.ExitCode(err), err)
	}
}

func TestBuildKeepsSnapshotAndRowInSync(t *testing.T) {
	p := params(t)
	p.ID = "01AAAAAAAAAAAAAAAAAAAAAAAA"
	state, row := build(p, "/d", "/d/worktree", "/d/work-state.json")

	if state.Work.ID != row.ID || state.Work.Slug != row.Slug ||
		state.Work.Status != row.Status || state.Work.StartMode != row.StartMode ||
		state.Work.Starter != row.Starter || state.Work.Branch != row.Branch ||
		state.Work.BaseBranch != row.BaseBranch ||
		state.Work.BranchConvention != row.BranchConvention ||
		state.Work.CreatedAt != row.CreatedAt ||
		state.Work.LastAccessedAt != row.LastAccessedAt {
		t.Errorf("snapshot/row drift:\n  %+v\n  %+v", state.Work, row)
	}
	// Governed fields come from the core alone.
	if state.Work.Status != "in-progress" || state.Work.StartMode != "new" {
		t.Errorf("governed fields wrong: %+v", state.Work)
	}
	if state.Work.CreatedAt != state.Work.LastAccessedAt {
		t.Errorf("timestamps differ at creation")
	}
}
