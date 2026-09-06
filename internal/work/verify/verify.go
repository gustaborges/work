// Package verify checks that a Work is internally coherent: its canonical
// snapshot, its git worktree and branch, and its projection row all agree.
// It is a library used by the integration suite and available to a future
// `work doctor` without expanding the F1 CLI surface (FR-028).
package verify

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
)

// Report is the outcome of Check. OK is true only when Problems is empty.
type Report struct {
	WorkID   string
	OK       bool
	Problems []string
}

func (r *Report) fail(format string, args ...any) {
	r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
}

// Check verifies the Work identified by workID against its projection row.
func Check(db *projection.DB, workID string) (Report, error) {
	r := Report{WorkID: workID}

	row, ok, err := db.Get(workID)
	if err != nil {
		return r, err
	}
	if !ok {
		r.fail("no works row for id %s", workID)
		return r, nil
	}

	snap, err := work.Read(row.SnapshotPath)
	if err != nil {
		r.fail("snapshot unreadable at %s: %v", row.SnapshotPath, err)
		return r, nil
	}
	if err := snap.Validate(); err != nil {
		r.fail("snapshot invalid: %v", err)
		return r, nil
	}

	// Snapshot vs projection row.
	w := snap.Work
	checkEq(&r, "id", w.ID, row.ID)
	checkEq(&r, "slug", w.Slug, row.Slug)
	checkEq(&r, "status", w.Status, row.Status)
	checkEq(&r, "start_mode", w.StartMode, row.StartMode)
	checkEq(&r, "starter", w.Starter, row.Starter)
	checkEq(&r, "branch", w.Branch, row.Branch)
	checkEq(&r, "base_branch", w.BaseBranch, row.BaseBranch)
	checkEq(&r, "branch_convention", w.BranchConvention, row.BranchConvention)
	checkEq(&r, "created_at", w.CreatedAt, row.CreatedAt)
	checkEq(&r, "last_accessed_at", w.LastAccessedAt, row.LastAccessedAt)
	checkEq(&r, "archived_at", w.ArchivedAt, row.ArchivedAt)

	if w.Status == work.StatusArchived {
		checkArchived(&r, w, row)
		r.OK = len(r.Problems) == 0
		return r, nil
	}

	// Worktree HEAD.
	wt := gitx.Open(row.WorktreePath)
	head, err := wt.CurrentBranch()
	if err != nil {
		r.fail("cannot read worktree HEAD at %s: %v", row.WorktreePath, err)
	} else if head != w.Branch {
		r.fail("worktree HEAD is %q, snapshot branch is %q", head, w.Branch)
	}

	// Branch descends from the recorded base at its recorded point: at creation
	// the branch tip, the merge-base, and the base tip all coincide.
	baseTip, baseErr := wt.RevParse(w.BaseBranch)
	mb, mbErr := wt.MergeBase(w.Branch, w.BaseBranch)
	switch {
	case baseErr != nil:
		r.fail("cannot resolve base branch %q: %v", w.BaseBranch, baseErr)
	case mbErr != nil:
		r.fail("cannot compute merge-base of %q and %q: %v", w.Branch, w.BaseBranch, mbErr)
	case mb != baseTip:
		r.fail("branch %q did not start from %q (merge-base %s != base tip %s)", w.Branch, w.BaseBranch, mb, baseTip)
	}

	r.OK = len(r.Problems) == 0
	return r, nil
}

// checkArchived runs the coherence rules specific to an archived Work: no
// worktree, the directory sits under <workspace>/archived/, and archived_at is
// a well-formed RFC 3339 timestamp. No worktree or branch check is run — the
// worktree is gone and the branch is intentionally left in the source repo and
// not tracked (data-model.md §3.3).
func checkArchived(r *Report, w work.WorkSection, row projection.Work) {
	if row.WorktreePath != "" {
		r.fail("archived Work has a worktree_path: %q", row.WorktreePath)
	}
	if parent := filepath.Base(filepath.Dir(row.DirPath)); parent != "archived" {
		r.fail("archived Work dir_path is not under archived/: %q", row.DirPath)
	}
	if w.ArchivedAt == "" {
		r.fail("archived Work snapshot has no archived_at")
	} else if _, err := time.Parse(time.RFC3339, w.ArchivedAt); err != nil {
		r.fail("archived_at %q is not RFC 3339: %v", w.ArchivedAt, err)
	}
}

func checkEq(r *Report, field, snapVal, rowVal string) {
	if snapVal != rowVal {
		r.fail("%s: snapshot %q != row %q", field, snapVal, rowVal)
	}
}
