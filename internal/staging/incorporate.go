package staging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Applied owns newly created entries until Commit or a complete Rollback.
// Failed removals remain owned and may be retried independently of cancellation.
type Applied struct {
	Created  int
	undo     []string
	terminal string
	remove   func(string) error
	owned    map[string]os.FileInfo
}

// Unresolved returns paths still owned by this application.
func (a *Applied) Unresolved() []string { return append([]string(nil), a.undo...) }

// Commit relinquishes undo ownership. Repeated commits are harmless.
func (a *Applied) Commit() error {
	if a.terminal == "rollback" {
		return fmt.Errorf("staging: cannot commit rolled back output")
	}
	a.terminal = "commit"
	a.undo = nil
	return nil
}

// Rollback removes only this application's entries, children before parents.
func (a *Applied) Rollback() error {
	if a.terminal == "commit" {
		return fmt.Errorf("staging: cannot roll back committed output")
	}
	remove := a.remove
	if remove == nil {
		remove = os.Remove
	}
	var failures []error
	var remaining []string
	for i := len(a.undo) - 1; i >= 0; i-- {
		path := a.undo[i]
		if owned := a.owned[path]; owned != nil {
			current, err := os.Lstat(path)
			if err == nil && !os.SameFile(owned, current) {
				failures = append(failures, fmt.Errorf("staging: owned path changed: %s", path))
				remaining = append([]string{path}, remaining...)
				continue
			}
		}
		if err := remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failures = append(failures, fmt.Errorf("staging: removing %s: %w", path, err))
			remaining = append([]string{path}, remaining...)
		}
	}
	a.undo = remaining
	if len(remaining) == 0 {
		a.terminal = "rollback"
	}
	return errors.Join(failures...)
}

// ApplyContext copies in bounded chunks, retaining undo even on failure.
// The caller must roll back a failed application or commit a successful one.
func (p Plan) ApplyContext(ctx context.Context) (*Applied, error) {
	a := &Applied{owned: map[string]os.FileInfo{}}
	dirs := make(map[string]os.FileInfo, len(p.dirs))
	for path, info := range p.dirs {
		dirs[path] = info
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		for path, info := range dirs {
			latest, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if !latest.IsDir() || !os.SameFile(info, latest) {
				return &Refusal{Rel: filepath.Base(path), Reason: Exists}
			}
		}
		return nil
	}
	for i, it := range p.Items {
		if err := check(); err != nil {
			return a, err
		}
		if i == injectedFailure() {
			return a, fmt.Errorf("staging: injected failure at item %d", i)
		}
		if it.IsDir {
			err := os.Mkdir(it.Dest, 0755)
			if errors.Is(err, fs.ErrExist) {
				if _, known := dirs[it.Dest]; !known {
					return a, &Refusal{Rel: it.Rel, Reason: Exists}
				}
				continue
			}
			if err != nil {
				return a, err
			}
			a.undo = append(a.undo, it.Dest)
			a.Created++
			info, err := lstatIdentity(it.Dest)
			if err != nil {
				return a, err
			}
			// Native aliases of a previously created directory must never merge.
			for path, previous := range dirs {
				if path != it.Dest && os.SameFile(previous, info) {
					return a, &Refusal{Rel: it.Rel, Reason: Exists}
				}
			}
			dirs[it.Dest] = info
			a.owned[it.Dest] = info
		} else {
			made, err := copyExclusiveContext(ctx, it)
			if made {
				a.undo = append(a.undo, it.Dest)
				a.Created++
				owned, identityErr := lstatIdentity(it.Dest)
				if identityErr != nil {
					return a, errors.Join(err, identityErr)
				}
				a.owned[it.Dest] = owned
			}
			if err != nil {
				if errors.Is(err, fs.ErrExist) {
					return a, &Refusal{Rel: it.Rel, Reason: Exists}
				}
				return a, err
			}
		}
	}
	return a, check()
}

// Incorporate preserves the automatic caller's apply/commit interface.
func (p Plan) Incorporate() (int, error) {
	a, err := p.ApplyContext(context.Background())
	if err != nil {
		return 0, errors.Join(err, a.Rollback())
	}
	return a.Created, a.Commit()
}

func copyExclusiveContext(ctx context.Context, it Item) (bool, error) {
	source, err := lstatIdentity(it.Src)
	if err != nil {
		return false, err
	}
	if !source.Mode().IsRegular() {
		return false, &Refusal{Rel: it.Rel, Reason: NotRegular}
	}
	in, err := os.Open(it.Src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return false, err
	}
	if !os.SameFile(source, info) {
		return false, &Refusal{Rel: it.Rel, Reason: NotRegular}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	out, err := os.OpenFile(it.Dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return false, err
	}
	buf := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return true, errors.Join(err, out.Close())
		}
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return true, errors.Join(err, out.Close())
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return true, errors.Join(readErr, out.Close())
		}
	}
	return true, out.Close()
}

func injectedFailure() int {
	v, ok := strings.CutPrefix(os.Getenv("WORK_FAIL_AT"), "incorporate:")
	if !ok {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return -1
	}
	return n
}
