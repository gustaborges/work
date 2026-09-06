package branchname

import (
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/gitx"
)

func TestValidate(t *testing.T) {
	good := []string{"add-retry", "feature/x", "user/fix-123"}
	for _, n := range good {
		if err := Validate(n); err != nil {
			t.Errorf("Validate(%q) = %v", n, err)
		}
	}
	// git's own rules are authoritative here; the leading-dash rule is enforced
	// only at the slug prompt (tui.ValidateSlug), not by check-ref-format.
	bad := []string{"", "has space", "..", "trailing.lock", "back\\slash", "end/", "a~b", "a^b", "a:b"}
	for _, n := range bad {
		if err := Validate(n); diag.Token(err) != diag.InvalidBranchName.Token {
			t.Errorf("Validate(%q) token = %v, want invalid-branch-name", n, diag.Token(err))
		}
	}
}

func TestDetectCollisionLocal(t *testing.T) {
	dir := gittest.Repo(t)
	gittest.Git(t, dir, "branch", "taken")
	err := DetectCollision(gitx.Open(dir), "taken")
	if diag.Token(err) != diag.BranchCollision.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectCollisionRemoteTracking(t *testing.T) {
	origin := gittest.Repo(t)
	gittest.Git(t, origin, "branch", "upstream-only")
	clone := t.TempDir()
	gittest.Git(t, clone, "clone", "-q", origin, ".")
	err := DetectCollision(gitx.Open(clone), "upstream-only")
	if diag.Token(err) != diag.BranchCollision.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectCollisionWorktree(t *testing.T) {
	dir := gittest.Repo(t)
	wt := t.TempDir() + "/wt"
	gittest.Git(t, dir, "worktree", "add", "-b", "wt-branch", wt)
	err := DetectCollision(gitx.Open(dir), "wt-branch")
	if diag.Token(err) != diag.BranchCollision.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectCollisionFree(t *testing.T) {
	dir := gittest.Repo(t)
	if err := DetectCollision(gitx.Open(dir), "brand-new"); err != nil {
		t.Fatalf("err = %v", err)
	}
}
