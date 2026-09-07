package verify

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// coherentWork builds a real Work and returns its id and the projection DB.
func coherentWork(t *testing.T) (*projection.DB, string) {
	t.Helper()

	src := t.TempDir()
	git(t, src, "init", "-q", "-b", "main")
	git(t, src, "commit", "-q", "--allow-empty", "-m", "init")

	dir := filepath.Join(t.TempDir(), "demo_add-retry")
	worktree := filepath.Join(dir, "worktree")
	if err := gitx.Open(src).WorktreeAdd(worktree, "add-retry", "main"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	snap := &work.State{
		Schema: work.Schema,
		Work: work.WorkSection{
			ID: "01JCOHERENT", Slug: "add-retry", Status: work.StatusInProgress,
			StartMode: work.StartModeNew, Starter: "local-path-starter",
			Branch: "add-retry", BaseBranch: "main", BranchConvention: "freeform",
			CreatedAt: "2026-09-05T14:03:11Z", LastAccessedAt: "2026-09-05T14:03:11Z",
		},
		Meta:  map[string]any{},
		Links: map[string]string{},
	}
	snapPath := filepath.Join(dir, "work-state.json")
	if err := work.Write(snapPath, snap); err != nil {
		t.Fatalf("work.Write: %v", err)
	}

	db, err := projection.Open(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	row := projection.Work{
		ID: snap.Work.ID, Slug: snap.Work.Slug, Status: snap.Work.Status,
		StartMode: snap.Work.StartMode, Starter: snap.Work.Starter, Branch: snap.Work.Branch,
		BaseBranch: snap.Work.BaseBranch, BranchConvention: snap.Work.BranchConvention,
		RepoName: "demo", DirPath: dir, WorktreePath: worktree, SnapshotPath: snapPath,
		CreatedAt: snap.Work.CreatedAt, LastAccessedAt: snap.Work.LastAccessedAt,
	}
	if err := db.Upsert(row); err != nil {
		t.Fatal(err)
	}
	return db, snap.Work.ID
}

func TestCheckCoherent(t *testing.T) {
	db, id := coherentWork(t)
	rep, err := Check(db, id)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rep.OK {
		t.Errorf("Check reported problems: %v", rep.Problems)
	}
}

func TestCheckMissingRow(t *testing.T) {
	db, _ := coherentWork(t)
	rep, err := Check(db, "nonexistent")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if rep.OK || len(rep.Problems) == 0 {
		t.Errorf("Check(missing) OK = %v", rep.OK)
	}
}

// archivedWork builds a coherent archived Work: snapshot under archived/, no
// worktree, status archived with archived_at set.
func archivedWork(t *testing.T) (*projection.DB, string) {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, "archived", "20260906-demo_add-retry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	snap := &work.State{
		Schema: work.Schema,
		Work: work.WorkSection{
			ID: "01JARCHIVED", Slug: "add-retry", Status: work.StatusArchived,
			ArchivedAt: "2026-09-06T18:22:00Z", StartMode: work.StartModeNew,
			Starter: "local-path-starter", Branch: "add-retry", BaseBranch: "main",
			BranchConvention: "freeform", CreatedAt: "2026-09-05T14:03:11Z",
			LastAccessedAt: "2026-09-06T18:22:00Z",
		},
		Meta:  map[string]any{},
		Links: map[string]string{},
	}
	snapPath := filepath.Join(dir, "work-state.json")
	if err := work.Write(snapPath, snap); err != nil {
		t.Fatalf("work.Write: %v", err)
	}
	db, err := projection.Open(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	row := projection.Work{
		ID: snap.Work.ID, Slug: snap.Work.Slug, Status: snap.Work.Status,
		StartMode: snap.Work.StartMode, Starter: snap.Work.Starter, Branch: snap.Work.Branch,
		BaseBranch: snap.Work.BaseBranch, BranchConvention: snap.Work.BranchConvention,
		RepoName: "demo", DirPath: dir, WorktreePath: "", SnapshotPath: snapPath,
		CreatedAt: snap.Work.CreatedAt, LastAccessedAt: snap.Work.LastAccessedAt,
		ArchivedAt: snap.Work.ArchivedAt,
	}
	if err := db.Upsert(row); err != nil {
		t.Fatal(err)
	}
	return db, snap.Work.ID
}

func TestCheckArchivedCoherent(t *testing.T) {
	db, id := archivedWork(t)
	rep, err := Check(db, id)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !rep.OK {
		t.Errorf("archived Work reported problems: %v", rep.Problems)
	}
}

func TestCheckArchivedRejectsWorktreePath(t *testing.T) {
	db, id := archivedWork(t)
	row, _, _ := db.Get(id)
	row.WorktreePath = "/somewhere/worktree"
	if err := db.Upsert(row); err != nil {
		t.Fatal(err)
	}
	rep, _ := Check(db, id)
	if rep.OK || !strings.Contains(strings.Join(rep.Problems, "; "), "worktree_path") {
		t.Errorf("expected a worktree_path problem, got %v", rep.Problems)
	}
}

func TestCheckArchivedRejectsStaleStatus(t *testing.T) {
	db, id := archivedWork(t)
	row, _, _ := db.Get(id)
	row.Status = "in-progress" // row disagrees with the snapshot
	row.ArchivedAt = ""
	if err := db.Upsert(row); err != nil {
		t.Fatal(err)
	}
	rep, _ := Check(db, id)
	if rep.OK {
		t.Errorf("Check should fail when the row status disagrees with the snapshot")
	}
}

func TestCheckDetectsBranchMismatch(t *testing.T) {
	db, id := coherentWork(t)
	// Corrupt the row's branch.
	row, _, _ := db.Get(id)
	row.Branch = "wrong"
	if err := db.Upsert(row); err != nil {
		t.Fatal(err)
	}
	rep, _ := Check(db, id)
	if rep.OK {
		t.Errorf("Check should have failed on branch mismatch")
	}
	joined := strings.Join(rep.Problems, "; ")
	if !strings.Contains(joined, "branch") {
		t.Errorf("problems do not mention branch: %v", rep.Problems)
	}
}
