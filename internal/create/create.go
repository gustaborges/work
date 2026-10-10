// Package create is the transactional orchestrator for `work start`. It runs
// an ordered pipeline of side-effecting steps, each pushing a compensating
// action onto a LIFO stack; the commit point is the projection upsert (step 5).
// Any error or cancellation before commit unwinds the stack top-down so no
// orphan branch, worktree, directory, snapshot, or row survives (R10, SC-003).
package create

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/id"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

// Params is the fully-resolved input to a creation. Every value has already
// been validated by the pipeline in internal/cli.
type Params struct {
	Home            workhome.Home
	SourceRepo      string // absolute, validated repository path
	RepoName        string // base name of the source repository directory
	WorkspaceRoot   string // absolute workspace root
	Slug            string
	Branch          string // derived branch name (real, may contain '/')
	BaseRefname     string // fully-qualified ref used as the worktree base
	BaseBranchShort string // short name stored in work.base_branch
	Convention      string // "freeform" in F1
	Starter         string // "local-path-starter" in F1
	// StartMode selects the materialization path (F4): "new"/"fork" (or empty,
	// F1's implicit default) create a new branch; "contribution" checks out an
	// already-existing one without creating it, and its rollback never deletes
	// that branch (research R12).
	StartMode string
	// Meta and Links are the context the Starter published, already validated;
	// they land in the first snapshot. StarterComponent is the Starter's
	// "<alias>/<name>", recorded as the source of each entry's provenance.
	Meta             map[string]any
	Links            map[string]string
	StarterComponent string
	// Now, when zero, defaults to time.Now().UTC().
	Now time.Time
	// ID, when empty, is generated.
	ID string
}

// Result identifies the materialized Work.
type Result struct {
	WorkID       string
	DirPath      string
	WorktreePath string
	SnapshotPath string
	BaseObject   string // resolved base tip (short) for the success summary
}

// failPoint reports whether WORK_FAIL_AT names step.
func failPoint(step string) bool {
	return os.Getenv("WORK_FAIL_AT") == step
}

type compensator struct {
	name string
	undo func() error
}

// Run executes the pipeline. On success it returns the Result and a nil error.
func Run(ctx context.Context, p Params) (Result, error) {
	if p.Now.IsZero() {
		p.Now = time.Now().UTC()
	} else {
		p.Now = p.Now.UTC()
	}
	if p.ID == "" {
		p.ID = id.New(p.Now)
	}

	dirPath := filepath.Join(p.WorkspaceRoot, "in-progress", p.RepoName+"_"+sanitizeBranch(p.Branch))
	worktreePath := filepath.Join(dirPath, "worktree")
	snapshotPath := filepath.Join(dirPath, "work-state.json")

	// Pre-step check: the target directory must not already exist.
	if _, err := os.Lstat(dirPath); err == nil {
		return Result{}, diag.Newf(diag.DestinationUnavailable,
			"the target directory already exists: %s", dirPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, diag.Wrapf(diag.DestinationUnavailable, err,
			"cannot inspect the target directory: %s", dirPath)
	}

	var stack []compensator
	unwind := func() {
		for _, c := range slices.Backward(stack) {
			_ = c.undo()
		}
		stack = nil
	}
	fail := func(cat diag.Category, cause error, msg string) (Result, error) {
		unwind()
		if cause != nil {
			return Result{}, diag.Wrap(cat, cause, msg)
		}
		return Result{}, diag.New(cat, msg)
	}
	// cancelled unwinds every effect so far and reports exit 20. It is checked
	// at every step boundary so an interrupt before the commit point never
	// leaves an orphan branch, worktree, directory, or snapshot (R10, FR-021).
	cancelled := func() (Result, error, bool) {
		if ctx.Err() == nil {
			return Result{}, nil, false
		}
		unwind()
		return Result{}, diag.New(diag.Cancelled, "creation cancelled"), true
	}

	if r, err, done := cancelled(); done {
		return r, err
	}

	repo := gitx.Open(p.SourceRepo)

	// Step 1: lock on the computed (workspace, repo, branch) triple.
	if failPoint("lock") {
		return fail(diag.MaterializationFailed, nil, "injected failure at step: lock")
	}
	lockName := lockKey(p.WorkspaceRoot, p.SourceRepo, p.Branch)
	release, err := lockfile.Acquire(p.Home.LockPath(lockName))
	if err != nil {
		return fail(diag.MaterializationFailed, err, "another creation for this branch is in progress")
	}
	stack = append(stack, compensator{"lock", func() error { release(); return nil }})

	if r, err, done := cancelled(); done {
		return r, err
	}

	// Step 2: own the Work directory.
	if failPoint("dir") {
		return fail(diag.MaterializationFailed, nil, "injected failure at step: dir")
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fail(diag.MaterializationFailed, err, "cannot create the Work directory")
	}
	stack = append(stack, compensator{"dir", func() error { return os.RemoveAll(dirPath) }})

	if r, err, done := cancelled(); done {
		return r, err
	}

	// Step 3: create the branch and its worktree in one git call — except in
	// contribution mode, which checks out an already-existing branch instead
	// (it was never created by this Work, so rollback must never delete it,
	// research R12).
	if failPoint("worktree") {
		return fail(diag.MaterializationFailed, nil, "injected failure at step: worktree")
	}
	if p.StartMode == work.StartModeContribution {
		// A branch that already exists locally is never this run's to delete.
		// One that exists only as a remote-tracking ref gets a local tracking
		// branch created here, which rollback must therefore remove again.
		existedLocally, err := repo.ShowRefVerify("refs/heads/" + p.Branch)
		if err != nil {
			return fail(diag.MaterializationFailed, err, "cannot inspect the branch to check out")
		}
		if existedLocally {
			err = repo.WorktreeAddExisting(worktreePath, p.Branch)
		} else {
			err = repo.WorktreeAddTracking(worktreePath, p.Branch, p.BaseRefname)
		}
		if err != nil {
			return fail(diag.MaterializationFailed, err, "cannot create the git worktree")
		}
		stack = append(stack, compensator{"worktree", func() error {
			rmErr := repo.WorktreeRemove(worktreePath)
			if existedLocally {
				return rmErr
			}
			return errors.Join(rmErr, repo.BranchDelete(p.Branch))
		}})
	} else {
		if err := repo.WorktreeAdd(worktreePath, p.Branch, p.BaseRefname); err != nil {
			return fail(diag.MaterializationFailed, err, "cannot create the git worktree")
		}
		stack = append(stack, compensator{"worktree", func() error {
			_ = repo.WorktreeRemove(worktreePath)
			return repo.BranchDelete(p.Branch)
		}})
	}

	baseObject, _ := repo.Run("rev-parse", "--short", p.BaseRefname)

	if r, err, done := cancelled(); done {
		return r, err
	}

	// Step 4: write the canonical snapshot atomically.
	if failPoint("snapshot") {
		return fail(diag.MaterializationFailed, nil, "injected failure at step: snapshot")
	}
	state, row, prov := build(p, dirPath, worktreePath, snapshotPath)
	if err := work.Write(snapshotPath, state); err != nil {
		return fail(diag.MaterializationFailed, err, "cannot write work-state.json")
	}
	// Covered by the step-2 RemoveAll compensator.

	if r, err, done := cancelled(); done {
		return r, err
	}

	// Step 5 (commit): publish the projection row.
	if failPoint("projection") {
		return fail(diag.MaterializationFailed, nil, "injected failure at step: projection")
	}
	db, err := projection.Open(p.Home.DBFile())
	if err != nil {
		return fail(diag.MaterializationFailed, err, "cannot open the projection database")
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return fail(diag.MaterializationFailed, err, "cannot migrate the projection database")
	}
	if err := db.UpsertWithProvenance(row, prov); err != nil {
		return fail(diag.MaterializationFailed, err, "cannot record the Work")
	}

	// Committed: release the lock but keep every other effect.
	release()

	return Result{
		WorkID:       p.ID,
		DirPath:      dirPath,
		WorktreePath: worktreePath,
		SnapshotPath: snapshotPath,
		BaseObject:   strings.TrimSpace(baseObject),
	}, nil
}

// build produces the canonical work.State and the projection.Work row from one
// set of inputs so the two can never drift (FR-018). Every governed work.*
// field originates here and nowhere else.
func build(p Params, dirPath, worktreePath, snapshotPath string) (*work.State, projection.Work, []projection.Provenance) {
	startMode := p.StartMode
	if startMode == "" {
		startMode = work.StartModeNew
	}
	ts := p.Now.Format(time.RFC3339)
	// Contribution mode records what was checked out, by its local name,
	// rather than the remote-tracking ref it may have been resolved from.
	baseBranch := p.BaseBranchShort
	if startMode == work.StartModeContribution {
		baseBranch = p.Branch
	}
	ws := work.WorkSection{
		ID:               p.ID,
		Slug:             p.Slug,
		Status:           work.StatusInProgress,
		StartMode:        startMode,
		Starter:          p.Starter,
		Branch:           p.Branch,
		BaseBranch:       baseBranch,
		BranchConvention: p.Convention,
		CreatedAt:        ts,
		LastAccessedAt:   ts,
	}
	state := &work.State{
		Schema: work.Schema,
		Work:   ws,
		Meta:   maps.Clone(p.Meta),
		Links:  maps.Clone(p.Links),
	}
	if state.Meta == nil {
		state.Meta = map[string]any{}
	}
	if state.Links == nil {
		state.Links = map[string]string{}
	}
	row := projection.Work{
		ID:               ws.ID,
		Slug:             ws.Slug,
		Status:           ws.Status,
		StartMode:        ws.StartMode,
		Starter:          ws.Starter,
		Branch:           ws.Branch,
		BaseBranch:       ws.BaseBranch,
		BranchConvention: ws.BranchConvention,
		RepoName:         p.RepoName,
		DirPath:          dirPath,
		WorktreePath:     worktreePath,
		SnapshotPath:     snapshotPath,
		CreatedAt:        ws.CreatedAt,
		LastAccessedAt:   ws.LastAccessedAt,
	}
	return state, row, startProvenance(p, ts)
}

// startProvenance records the Starter as the source of every entry it
// published, so the index can say where each key came from. Sorted so the
// rows are written in a reproducible order.
func startProvenance(p Params, recordedAt string) []projection.Provenance {
	entry := func(section, key string) projection.Provenance {
		return projection.Provenance{
			WorkID: p.ID, Section: section, Key: key,
			SourceComponent: p.StarterComponent, SourceOperation: "start", RecordedAt: recordedAt,
		}
	}
	var out []projection.Provenance
	for _, k := range slices.Sorted(maps.Keys(p.Links)) {
		out = append(out, entry("links", k))
	}
	for _, k := range slices.Sorted(maps.Keys(p.Meta)) {
		out = append(out, entry("meta", k))
	}
	return out
}

// sanitizeBranch makes a branch name safe as a single path segment.
func sanitizeBranch(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

func lockKey(workspace, repo, branch string) string {
	sum := sha256.Sum256([]byte(workspace + "\x00" + repo + "\x00" + branch))
	return hex.EncodeToString(sum[:])
}
