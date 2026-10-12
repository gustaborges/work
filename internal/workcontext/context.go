// Package workcontext reads canonical Work snapshots without creating or
// repairing any application state. It does not require a checkout or index.
package workcontext

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/work"
)

// Context captures the canonical identity and real directory for one Work.
type Context struct {
	State        *work.State
	WorkDir      string
	SnapshotPath string
	WorktreePath string
}

// Read validates a snapshot at an actual lifecycle-child directory. Snapshot
// symlinks are refused; directory symlinks are normalized before reading.
func Read(dir string) (Context, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Context{}, unreadable(err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Context{}, unreadable(err)
	}
	area := filepath.Base(filepath.Dir(real))
	if area != work.StatusInProgress && area != work.StatusArchived {
		return Context{}, unreadable(fmt.Errorf("not a lifecycle child"))
	}
	path := filepath.Join(real, "work-state.json")
	info, err := os.Lstat(path)
	if err != nil {
		return Context{}, unreadable(err)
	}
	if !info.Mode().IsRegular() {
		return Context{}, unreadable(fmt.Errorf("snapshot is not a regular file"))
	}
	s, err := work.Read(path)
	if err != nil {
		return Context{}, unreadable(err)
	}
	if err := s.Validate(); err != nil {
		return Context{}, unreadable(err)
	}
	c := Context{State: s, WorkDir: real, SnapshotPath: path}
	if s.Work.Status == work.StatusInProgress {
		c.WorktreePath = filepath.Join(real, "worktree")
	}
	return c, nil
}

// Current resolves the nearest enclosing lifecycle child with a snapshot.
// An invalid enclosing snapshot is an error, never permission to fall back.
func Current(cwd string) (Context, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Context{}, unreadable(err)
	}
	dir, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Context{}, unreadable(err)
	}
	for {
		area := filepath.Base(filepath.Dir(dir))
		if area == work.StatusInProgress || area == work.StatusArchived {
			_, err := os.Lstat(filepath.Join(dir, "work-state.json"))
			if err == nil || !os.IsNotExist(err) {
				return Read(dir)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return Context{}, notFound()
}
func unreadable(err error) error {
	return diag.Wrap(diag.SnapshotUnreadable, err, "cannot read a valid canonical Work snapshot").WithHint("Check the Work's work-state.json; status never repairs saved state.")
}
func notFound() error {
	return diag.New(diag.TargetNotFound, "no Work found; enter a Work directory or pass its complete id to `work status`").WithHint("For a Work in an old workspace, enter its directory and run `work status`.")
}
