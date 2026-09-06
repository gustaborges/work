// Package branchname validates a derived branch name with git's own
// check-ref-format and detects collisions against local branches,
// remote-tracking branches, and branches already bound to a worktree — all
// before any repository mutation (FR-012, R16).
package branchname

import (
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
)

// Validate checks name is a syntactically valid branch name per
// `git check-ref-format refs/heads/<name>`.
func Validate(name string) error {
	if strings.TrimSpace(name) == "" {
		return diag.New(diag.InvalidBranchName, "the branch name is empty")
	}
	if err := gitx.CheckRefFormat("refs/heads/" + name); err != nil {
		return diag.Newf(diag.InvalidBranchName, "%q is not a valid branch name", name)
	}
	return nil
}

// CollisionKind identifies which namespace a colliding name was found in.
type CollisionKind string

const (
	CollisionLocal          CollisionKind = "local branch"
	CollisionRemoteTracking CollisionKind = "remote-tracking branch"
	CollisionWorktree       CollisionKind = "branch checked out in another worktree"
)

// DetectCollision reports the namespace in which name already exists. A nil
// error means the name is free. A branch that is checked out in a linked
// worktree also exists as a local head; it is reported as the worktree case
// first because that is the more actionable diagnostic.
func DetectCollision(repo gitx.Repo, name string) error {
	want := "refs/heads/" + name

	worktrees, err := repo.WorktreeList()
	if err != nil {
		return fmt.Errorf("checking worktree branch bindings: %w", err)
	}
	for _, w := range worktrees {
		if w.Branch == want {
			return collision(name, CollisionWorktree)
		}
	}

	local, err := repo.ShowRefVerify(want)
	if err != nil {
		return fmt.Errorf("checking for a local branch collision: %w", err)
	}
	if local {
		return collision(name, CollisionLocal)
	}

	remotes, err := repo.ForEachRef("refs/remotes/*/" + name)
	if err != nil {
		return fmt.Errorf("checking for a remote-tracking branch collision: %w", err)
	}
	if len(remotes) > 0 {
		return collision(name, CollisionRemoteTracking)
	}

	return nil
}

func collision(name string, kind CollisionKind) error {
	return diag.Newf(diag.BranchCollision, "branch %q already exists as a %s", name, kind)
}

// Check runs Validate then DetectCollision.
func Check(repo gitx.Repo, name string) error {
	if err := Validate(name); err != nil {
		return err
	}
	return DetectCollision(repo, name)
}
