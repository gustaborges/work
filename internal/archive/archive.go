package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// lockTimeout bounds the wait for a Work's advisory lock; a second concurrent
// operation on the same Work fails fast rather than hanging (research R17). It
// is a var only so tests can shorten it.
var lockTimeout = 3 * time.Second

// OutcomeState is the terminal state of one Work after the batch runs.
type OutcomeState int

const (
	// StateArchived: the Work is fully archived — snapshot flipped, worktree
	// destroyed, directory moved under archived/, projection row updated.
	StateArchived OutcomeState = iota
	// StateLeftActive: the Work was left fully in-progress — its worktree is
	// dirty and the archival was not acknowledged.
	StateLeftActive
	// StateFailed: a post-commit step failed and the index could not self-heal;
	// the Work is canonically archived on disk but needs the next command's
	// reconcile to bring its row into agreement (exit 24).
	StateFailed
)

// Outcome is the per-Work result the CLI turns into stdout/stderr lines.
type Outcome struct {
	ID          string
	State       OutcomeState
	ArchivedDir string // absolute archived directory, set when State == StateArchived
	FailedStep  string // the step name, set when State == StateFailed
	// Note carries a non-fatal degradation message (e.g. the source repository
	// was unreachable so the worktree could not be removed via git).
	Note string
	// WasCWD is true when this Work's worktree was (or contained) the caller's
	// current directory and the session must be repositioned out.
	WasCWD bool
}

// Params is the fully-resolved input to a batch archive. internal/cli has
// already resolved the targets to in-progress projection rows and decided the
// confirmation.
type Params struct {
	Home          workhome.Home
	DB            *projection.DB
	WorkspaceRoot string
	// Rows are the Works to archive, in the order they should be processed.
	Rows []projection.Work
	// ForceDirty skips the dirty-worktree guard for every Work (--force-dirty).
	ForceDirty bool
	// AckDirty, when non-nil, is called for a dirty Work that was not
	// force-archived; returning true archives it anyway. A nil AckDirty (a
	// non-interactive run) leaves a dirty Work active.
	AckDirty func(projection.Work) (bool, error)
	// CallerCWD is the caller's resolved working directory, for reposition-out
	// detection. Empty disables the check.
	CallerCWD string
	// Now, when zero, defaults to time.Now().UTC(); read once per Work.
	Now time.Time
}

// Report is the batch result.
type Report struct {
	Outcomes []Outcome
}

// Archived counts the Works that ended fully archived.
func (r Report) Archived() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.State == StateArchived {
			n++
		}
	}
	return n
}

// Failed reports whether any Work ended in StateFailed.
func (r Report) Failed() bool {
	return slices.ContainsFunc(r.Outcomes, func(o Outcome) bool { return o.State == StateFailed })
}

// Run archives every Work in p.Rows, one at a time, each under its own advisory
// lock and its own LIFO compensation stack. A failure on one Work never rolls
// back a Work already archived earlier in the batch (FR-017). The returned error
// is a diag.ArchiveFailed naming the Works that need the index to self-heal;
// Report is always populated, error or not.
func Run(ctx context.Context, p Params) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var rep Report
	for i, row := range p.Rows {
		last := i == len(p.Rows)-1
		o := archiveOne(ctx, p, row, last)
		rep.Outcomes = append(rep.Outcomes, o)
	}
	if rep.Failed() {
		var ids []string
		for _, o := range rep.Outcomes {
			if o.State == StateFailed {
				ids = append(ids, o.ID+" (at "+o.FailedStep+")")
			}
		}
		return rep, diag.New(diag.ArchiveFailed,
			"the lookup index is behind for "+strings.Join(ids, ", ")+"; it will self-heal on the next `work` command")
	}
	return rep, nil
}

type compensator struct {
	name string
	undo func() error
}

// failPoint reports whether WORK_FAIL_AT names step. In a batch it fires only on
// the last Work, so the transactionality suite can assert the earlier members
// stay consistently archived (research R6, T024).
func failPoint(step string, last bool) bool {
	return last && os.Getenv("WORK_FAIL_AT") == step
}

func archiveOne(ctx context.Context, p Params, row projection.Work, last bool) Outcome {
	o := Outcome{ID: row.ID}
	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	var stack []compensator
	unwind := func() {
		for _, c := range slices.Backward(stack) {
			_ = c.undo()
		}
		stack = nil
	}

	// Step 1: lock the Work.
	release, err := lockfile.AcquireContext(ctx, p.Home.LockPath(lockKey(row.ID)), lockTimeout)
	if err != nil {
		o.State = StateLeftActive
		if errors.Is(err, lockfile.ErrTimeout) {
			o.Note = "another work operation is using this Work; try again in a moment"
		} else {
			o.Note = "could not lock the Work: " + err.Error()
		}
		return o
	}
	defer release()

	// Step 2: dirty-worktree guard.
	dirty := false
	if wt := row.WorktreePath; wt != "" && dirExists(wt) {
		if d, derr := gitx.Open(wt).IsDirty(); derr == nil {
			dirty = d
		}
	}
	if dirty && !p.ForceDirty {
		acked := false
		if p.AckDirty != nil {
			ok, aerr := p.AckDirty(row)
			acked = aerr == nil && ok
		}
		if !acked {
			o.State = StateLeftActive
			return o
		}
	}

	// Detect reposition-out before the worktree is destroyed. The process's
	// real cwd is the authority (the CLI-supplied CallerCWD is a fallback for
	// the check only); Windows cannot delete a directory that is a live
	// process's cwd, so we must step out of it before step 4 either way.
	cwd, _ := os.Getwd()
	o.WasCWD = underDir(cwd, row.WorktreePath) ||
		(p.CallerCWD != "" && underDir(p.CallerCWD, row.WorktreePath))

	// Step 3: flip the snapshot to archived — THE CANONICAL COMMIT POINT.
	if failPoint("snapshot", last) {
		unwind()
		o.State = StateLeftActive
		o.Note = "injected failure at step: snapshot"
		return o
	}
	state, rerr := work.Read(row.SnapshotPath)
	if rerr != nil {
		unwind()
		o.State = StateLeftActive
		o.Note = "cannot read the Work snapshot: " + rerr.Error()
		return o
	}
	origSnapshotPath := row.SnapshotPath
	// Archive() also bumps last_accessed_at; the compensator must restore the
	// prior value or a rolled-back Work drifts from its projection row.
	origLastAccessedAt := state.Work.LastAccessedAt
	state.Archive(now)
	if werr := work.Write(origSnapshotPath, state); werr != nil {
		unwind()
		o.State = StateLeftActive
		o.Note = "cannot write the Work snapshot: " + werr.Error()
		return o
	}
	stack = append(stack, compensator{"snapshot", func() error {
		back, e := work.Read(origSnapshotPath)
		if e != nil {
			return e
		}
		back.Work.Status = work.StatusInProgress
		back.Work.ArchivedAt = ""
		back.Work.LastAccessedAt = origLastAccessedAt
		return work.Write(origSnapshotPath, back)
	}})

	// Step 4: destroy the git worktree (the branch ref is left intact, FR-014).
	if failPoint("worktree", last) {
		unwind()
		o.State = StateLeftActive
		o.Note = "injected failure at step: worktree"
		return o
	}
	if wt := row.WorktreePath; wt != "" {
		// Step out of the worktree first when it holds the process cwd: some
		// platforms (Windows) refuse to remove a directory that is a live
		// process's working directory. The workspace root is the safe landing
		// spot the CLI also reports to the shell (research R10).
		if underDir(cwd, wt) {
			_ = os.Chdir(p.WorkspaceRoot)
		}
		src, srcErr := gitx.SourceRepoOf(wt)
		switch {
		case srcErr != nil && !dirExists(wt):
			// The worktree directory is already gone; nothing to remove. Prune
			// is best-effort and needs the source repo we could not resolve.
		case srcErr != nil:
			// The source repository is unreachable: skip the git call, keep
			// going, and note the leftover worktree (research R9).
			o.Note = "the source repository was unreachable; the worktree was not removed via git: " + wt
		default:
			repo := gitx.Open(src)
			if !dirExists(wt) {
				_ = repo.WorktreePrune()
			} else if rmErr := repo.WorktreeRemove(wt); rmErr != nil {
				unwind()
				o.State = StateLeftActive
				o.Note = "cannot remove the git worktree: " + rmErr.Error()
				return o
			} else {
				branch := row.Branch
				stack = append(stack, compensator{"worktree", func() error {
					return repo.WorktreeAddExisting(wt, branch)
				}})
			}
		}
	}

	// Step 5: move the Work directory under <workspace>/archived/.
	if failPoint("move", last) {
		unwind()
		o.State = StateLeftActive
		o.Note = "injected failure at step: move"
		return o
	}
	dst := ArchiveDir(p.WorkspaceRoot, row.RepoName, SanitizeBranch(row.Branch), now.Local())
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		unwind()
		o.State = StateLeftActive
		o.Note = "cannot create the archived area: " + err.Error()
		return o
	}
	if err := os.Rename(row.DirPath, dst); err != nil {
		unwind()
		o.State = StateLeftActive
		o.Note = "cannot move the Work directory: " + err.Error()
		return o
	}
	srcDir := row.DirPath
	stack = append(stack, compensator{"move", func() error { return os.Rename(dst, srcDir) }})
	o.ArchivedDir = dst
	newSnapshotPath := filepath.Join(dst, "work-state.json")

	// Step 6: update the projection — POST-COMMIT. A failure here does not roll
	// the physically-moved, canonically-archived Work back; it is healed.
	archivedRow := row
	archivedRow.Status = work.StatusArchived
	archivedRow.ArchivedAt = state.Work.ArchivedAt
	archivedRow.LastAccessedAt = state.Work.LastAccessedAt
	archivedRow.DirPath = dst
	archivedRow.SnapshotPath = newSnapshotPath
	archivedRow.WorktreePath = ""

	projErr := p.DB.MarkArchived(row.ID, archivedRow)
	if failPoint("projection", last) {
		projErr = errors.New("injected failure at step: projection")
	}
	if projErr != nil {
		if healErr := p.DB.Upsert(archivedRow); healErr != nil {
			// Keep the compensation stack intact but do NOT unwind: the Work is
			// canonically archived. The next reconcile fixes the row.
			stack = nil
			o.State = StateFailed
			o.FailedStep = "projection"
			return o
		}
	}

	// Committed and healthy: drop the compensation stack, keep every effect.
	stack = nil
	o.State = StateArchived
	return o
}

// lockKey is the advisory-lock name for a Work id: sha256(id) hex.
func lockKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// underDir reports whether path is dir or sits inside it, comparing cleaned
// absolute forms.
func underDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	p, err1 := filepath.Abs(path)
	d, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	p = filepath.Clean(p)
	d = filepath.Clean(d)
	if p == d {
		return true
	}
	return strings.HasPrefix(p, d+string(filepath.Separator))
}
