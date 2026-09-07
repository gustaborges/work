package resume

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// lockTimeout bounds the wait for the per-Work advisory lock. It is short so a
// second concurrent `work` operation on the same Work fails fast with a clear
// message rather than appearing to hang (research R17). It is a var only so
// tests can shorten it.
var lockTimeout = 3 * time.Second

// Params is the fully-resolved input to a resume. The caller (internal/cli) has
// already resolved the target through the reconciled projection and confirmed it
// is in-progress.
type Params struct {
	Home         workhome.Home
	DB           *projection.DB
	ID           string // work.id (ULID)
	SnapshotPath string // absolute path to the Work's work-state.json
	WorktreePath string // absolute worktree path, echoed and handed to the shell
	// Now, when zero, defaults to time.Now().UTC(); it is read once so the
	// snapshot and the projection record the identical timestamp.
	Now time.Time
}

// Result is the outcome of a successful resume.
type Result struct {
	// WorktreePath is the absolute directory the caller repositions into.
	WorktreePath string
	// IndexStale is true when the canonical snapshot was updated but the
	// post-commit projection trailer failed. The resume still succeeded; the
	// index self-heals on the next `work` command (research R3, R11).
	IndexStale bool
}

// Run performs the recent-access update for one Work under an advisory lock on
// its id: it reads the canonical snapshot, refuses a Work that is not
// in-progress (diag target-archived), bumps last_accessed_at and rewrites the
// snapshot atomically (the canonical commit point), then mirrors the timestamp
// into the projection. A projection failure after the snapshot commit is
// reported via Result.IndexStale, never rolled back.
//
// A lock held by another operation on the same Work fails cleanly after
// lockTimeout with no state change.
func Run(ctx context.Context, p Params) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	release, err := lockfile.AcquireContext(ctx, p.Home.LockPath(lockKey(p.ID)), lockTimeout)
	if err != nil {
		if errors.Is(err, lockfile.ErrTimeout) {
			return Result{}, diag.New(diag.MaterializationFailed,
				"another work operation is using this Work; try again in a moment")
		}
		if errors.Is(err, context.Canceled) {
			return Result{}, diag.New(diag.Cancelled, "cancelled")
		}
		return Result{}, diag.Wrap(diag.MaterializationFailed, err, "cannot lock the Work")
	}
	defer release()

	// (a) Read the canonical snapshot and guard against a race with archive.
	state, err := work.Read(p.SnapshotPath)
	if err != nil {
		return Result{}, diag.Wrap(diag.SnapshotUnreadable, err, "cannot read the Work snapshot")
	}
	if state.Work.Status != work.StatusInProgress {
		return Result{}, diag.New(diag.TargetArchived,
			"this Work is archived and cannot be resumed")
	}

	// (b) Canonical commit: bump last_accessed_at and rewrite atomically.
	state.Touch(now)
	if err := work.Write(p.SnapshotPath, state); err != nil {
		return Result{}, diag.Wrap(diag.MaterializationFailed, err, "cannot update the Work snapshot")
	}

	// (c) Post-commit trailer: mirror the timestamp into the projection. A
	// failure here leaves the authoritative snapshot correct; the index trails
	// and self-heals on the next command.
	res := Result{WorktreePath: p.WorktreePath}
	if err := p.DB.SetAccessed(p.ID, state.Work.LastAccessedAt); err != nil {
		res.IndexStale = true
	}
	return res, nil
}

// lockKey is the advisory-lock name for a Work id: sha256(id) hex, scoped per
// Work rather than per (workspace, repo, branch) (research R17).
func lockKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
