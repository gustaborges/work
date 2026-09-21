package create

import (
	"context"
	"database/sql"
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
	_ "modernc.org/sqlite"
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

// contributionParams returns Params for a contribution-mode run: an
// already-existing branch in the source repo, checked out directly (no slug,
// no convention — mirrors what internal/cli/start.go is expected to pass).
func contributionParams(t *testing.T) Params {
	t.Helper()
	p := params(t)
	gittest.Git(t, p.SourceRepo, "branch", "pr-branch")
	p.Branch = "pr-branch"
	p.BaseRefname = "refs/heads/pr-branch"
	p.BaseBranchShort = "pr-branch"
	p.Slug = ""
	p.Convention = ""
	p.StartMode = work.StartModeContribution
	return p
}

func TestRunContributionModeChecksOutExistingBranch(t *testing.T) {
	p := contributionParams(t)
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	snap, err := work.Read(res.SnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("snapshot invalid: %v", err)
	}
	if snap.Work.StartMode != work.StartModeContribution {
		t.Errorf("StartMode = %q, want %q", snap.Work.StartMode, work.StartModeContribution)
	}
	if snap.Work.Slug != "" || snap.Work.BranchConvention != "" {
		t.Errorf("contribution mode must persist no slug/branch_convention: %+v", snap.Work)
	}
	if snap.Work.Branch != "pr-branch" {
		t.Errorf("Branch = %q, want pr-branch", snap.Work.Branch)
	}
	// Contribution mode reuses the one base_branch field for the checked-out
	// branch itself — there is no separate base to record (ADD §7, T047).
	if snap.Work.BaseBranch != snap.Work.Branch {
		t.Errorf("BaseBranch = %q, want it to equal Branch %q in contribution mode", snap.Work.BaseBranch, snap.Work.Branch)
	}

	head, _ := gitx.Open(res.WorktreePath).CurrentBranch()
	if head != "pr-branch" {
		t.Errorf("worktree HEAD = %q, want pr-branch", head)
	}
}

func TestRunContributionModeUsesWorktreeAddExisting(t *testing.T) {
	p := contributionParams(t)
	// WorktreeAdd would fail outright on an already-existing branch
	// (`git worktree add -b` rejects it) — contribution mode's whole premise.
	// Running successfully here proves WorktreeAddExisting (no -b) was used.
	if _, err := Run(context.Background(), p); err != nil {
		t.Fatalf("Run: %v (WorktreeAdd -b would have failed on an existing branch)", err)
	}
}

func TestRunContributionModeRollbackNeverDeletesBranch(t *testing.T) {
	p := contributionParams(t)
	t.Setenv("WORK_FAIL_AT", "snapshot")
	if _, err := Run(context.Background(), p); diag.ExitCode(err) != 17 {
		t.Fatalf("exit = %d, want 17 (%v)", diag.ExitCode(err), err)
	}

	src := gitx.Open(p.SourceRepo)
	if ok, _ := src.ShowRefVerify("refs/heads/pr-branch"); !ok {
		t.Error("contribution-mode rollback deleted the pre-existing branch")
	}
	// The worktree itself must still be cleaned up.
	wts, _ := src.WorktreeList()
	for _, w := range wts {
		if strings.Contains(w.Path, "pr-branch") {
			t.Errorf("orphan worktree: %s", w.Path)
		}
	}
}

// remoteOnlyContributionParams returns Params for a contribution-mode run
// whose branch exists in the source repository only as a remote-tracking
// branch — what a fresh clone looks like for a branch it never checked out.
func remoteOnlyContributionParams(t *testing.T) Params {
	t.Helper()
	p := params(t)
	upstream := gittest.Repo(t)
	gittest.Git(t, upstream, "branch", "pr-branch")
	clone := filepath.Join(t.TempDir(), "clone")
	gittest.Git(t, t.TempDir(), "clone", "-q", upstream, clone)
	p.SourceRepo = clone
	p.RepoName = filepath.Base(clone)
	p.Branch = "pr-branch"
	p.BaseRefname = "refs/remotes/origin/pr-branch"
	p.BaseBranchShort = "origin/pr-branch"
	p.Slug = ""
	p.Convention = ""
	p.StartMode = work.StartModeContribution
	return p
}

func TestRunContributionModeRemoteOnlyBranchChecksOutTrackingBranch(t *testing.T) {
	p := remoteOnlyContributionParams(t)
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	head, _ := gitx.Open(res.WorktreePath).CurrentBranch()
	if head != "pr-branch" {
		t.Errorf("worktree HEAD = %q, want the local branch pr-branch (not a detached HEAD)", head)
	}
	if remote := gittest.Git(t, p.SourceRepo, "config", "branch.pr-branch.remote"); remote != "origin" {
		t.Errorf("branch.pr-branch.remote = %q, want origin", remote)
	}

	snap, err := work.Read(res.SnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Work.Branch != "pr-branch" || snap.Work.BaseBranch != "pr-branch" {
		t.Errorf("branch/base_branch = %q/%q, want the local name pr-branch for both",
			snap.Work.Branch, snap.Work.BaseBranch)
	}
}

func TestRunContributionModeRollbackRemovesTrackingBranchItCreated(t *testing.T) {
	p := remoteOnlyContributionParams(t)
	t.Setenv("WORK_FAIL_AT", "snapshot")
	if _, err := Run(context.Background(), p); diag.ExitCode(err) != 17 {
		t.Fatalf("exit = %d, want 17 (%v)", diag.ExitCode(err), err)
	}

	src := gitx.Open(p.SourceRepo)
	if ok, _ := src.ShowRefVerify("refs/heads/pr-branch"); ok {
		t.Error("rollback left the local tracking branch this run created")
	}
	if ok, _ := src.ShowRefVerify("refs/remotes/origin/pr-branch"); !ok {
		t.Error("rollback must never touch the remote-tracking branch")
	}
}

func TestRunForkAndNewModesUnchangedFromF1(t *testing.T) {
	for _, mode := range []string{"", work.StartModeNew, work.StartModeFork} {
		t.Run("mode="+mode, func(t *testing.T) {
			p := params(t)
			p.StartMode = mode
			res, err := Run(context.Background(), p)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			snap, err := work.Read(res.SnapshotPath)
			if err != nil {
				t.Fatal(err)
			}
			wantMode := mode
			if wantMode == "" {
				wantMode = work.StartModeNew
			}
			if snap.Work.StartMode != wantMode {
				t.Errorf("StartMode = %q, want %q", snap.Work.StartMode, wantMode)
			}
			if snap.Work.Slug == "" || snap.Work.BranchConvention == "" {
				t.Errorf("fork/new mode must keep slug/branch_convention: %+v", snap.Work)
			}

			// A failure at the worktree step still deletes the newly created
			// branch (unlike contribution mode).
			p2 := params(t)
			p2.StartMode = mode
			t.Setenv("WORK_FAIL_AT", "snapshot")
			if _, err := Run(context.Background(), p2); diag.ExitCode(err) != 17 {
				t.Fatalf("exit = %d, want 17 (%v)", diag.ExitCode(err), err)
			}
			if ok, _ := gitx.Open(p2.SourceRepo).ShowRefVerify("refs/heads/" + p2.Branch); ok {
				t.Error("fork/new mode rollback left the branch it created")
			}
		})
	}
}

func TestBuildKeepsSnapshotAndRowInSync(t *testing.T) {
	p := params(t)
	p.ID = "01AAAAAAAAAAAAAAAAAAAAAAAA"
	state, row, _ := build(p, "/d", "/d/worktree", "/d/work-state.json")

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

func TestRunPublishesStarterContextIntoFirstSnapshot(t *testing.T) {
	for _, mode := range []string{"", work.StartModeFork} {
		t.Run("mode="+mode, func(t *testing.T) {
			p := params(t)
			p.StartMode = mode
			p.ID = "01J9TESTPROVENANCE000000000"
			p.Meta = map[string]any{"github.pull_request.number": float64(212)}
			p.Links = map[string]string{"github.pull_request": "https://example.test/pr/212"}
			p.StarterComponent = "acme/starter"

			res, err := Run(context.Background(), p)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			snap, err := work.Read(res.SnapshotPath)
			if err != nil {
				t.Fatal(err)
			}
			if snap.Links["github.pull_request"] != "https://example.test/pr/212" {
				t.Errorf("links = %v", snap.Links)
			}
			if snap.Meta["github.pull_request.number"] != float64(212) {
				t.Errorf("meta = %v", snap.Meta)
			}

			db, err := projection.Open(p.Home.DBFile())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows, err := db.Provenance(res.WorkID)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("provenance rows = %+v, want 2", rows)
			}
			for _, r := range rows {
				if r.SourceComponent != "acme/starter" || r.SourceOperation != "start" ||
					r.RecordedAt != "2026-02-03T04:05:06Z" {
					t.Errorf("row = %+v", r)
				}
			}
			if rows[0].Section != "links" || rows[1].Section != "meta" {
				t.Errorf("sections = %s, %s", rows[0].Section, rows[1].Section)
			}
		})
	}
}

func TestRunWithoutStarterContextWritesNoProvenance(t *testing.T) {
	p := params(t)
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := work.Read(res.SnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Meta) != 0 || len(snap.Links) != 0 {
		t.Errorf("meta/links = %v / %v, want empty", snap.Meta, snap.Links)
	}
	db, err := projection.Open(p.Home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if rows, _ := db.Provenance(res.WorkID); len(rows) != 0 {
		t.Errorf("provenance = %+v, want none", rows)
	}
}

func TestRunUnwindsWhenProvenanceInsertFails(t *testing.T) {
	p := params(t)
	p.Links = map[string]string{"github.pull_request": "https://x"}
	p.StarterComponent = "acme/starter"

	// Migrate is idempotent, so a trigger planted on the migrated database
	// survives create's own Migrate and makes only the provenance insert fail.
	db, err := projection.Open(p.Home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	raw, err := sql.Open("sqlite", p.Home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TRIGGER refuse BEFORE INSERT ON work_provenance BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	_, err = Run(context.Background(), p)
	if diag.Token(err) != diag.MaterializationFailed.Token {
		t.Fatalf("err = %v, want materialization-failed", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(p.WorkspaceRoot, "in-progress")); len(entries) != 0 {
		t.Errorf("leftovers after unwind: %v", entries)
	}
	db, err = projection.Open(p.Home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if rows, _ := db.List(); len(rows) != 0 {
		t.Errorf("works rows = %+v, want none", rows)
	}
}
