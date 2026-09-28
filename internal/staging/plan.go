package staging

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// RefusalReason says why an Importer's output was refused.
type RefusalReason string

// The reasons a plan is refused.
const (
	// Exists: a file's destination already exists, or a directory's
	// destination exists and is not a directory. Nothing is ever overwritten.
	Exists RefusalReason = "exists"
	// ReservedPath: the destination is the worktree, something inside it, or
	// the Work's snapshot.
	ReservedPath RefusalReason = "reserved-path"
	// NotRegular: the entry is neither a regular file nor a directory
	// (symlinks included).
	NotRegular RefusalReason = "not-regular"
	// EscapesWork: the deepest existing part of the destination resolves to a
	// place outside the Work directory.
	EscapesWork RefusalReason = "escapes-work"
)

// Refusal is the error Build returns when any staged entry is unsafe. It names
// the offending relative path (slash-separated) and never any content.
type Refusal struct {
	Rel    string
	Reason RefusalReason
}

// Error names the refused relative path and why; it never includes content.
func (r *Refusal) Error() string {
	var what string
	switch r.Reason {
	case Exists:
		what = "already exists"
	case ReservedPath:
		what = "is reserved for Work"
	case NotRegular:
		what = "is not a regular file or a directory"
	case EscapesWork:
		what = "would land outside the Work directory"
	default:
		what = string(r.Reason)
	}
	return fmt.Sprintf("staging: %q %s", r.Rel, what)
}

// Item is one directory or file the plan would create.
type Item struct {
	// Rel is the slash-separated path relative to the stage and to the Work.
	Rel string
	// Src is the staged path; Dest is where it will be created.
	Src, Dest string
	IsDir     bool
}

// Plan is what incorporating a stage would create, parents before children.
type Plan struct {
	Items []Item
}

// Build walks the stage read-only and returns the plan for adding its content
// beneath workDir. If any entry is unsafe it returns a *Refusal for the first
// one in path order and the plan is unusable; nothing on disk is changed
// either way.
//
// The Work directory is <workspace>/in-progress/<repo>_<branch>, the directory
// that holds worktree/ and work-state.json.
func Build(stageDir, workDir string) (Plan, error) {
	root, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return Plan{}, fmt.Errorf("staging: resolving the Work directory: %w", err)
	}

	var items []Item
	walkErr := filepath.WalkDir(stageDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == stageDir {
			return nil
		}
		rel, err := filepath.Rel(stageDir, path)
		if err != nil {
			return err
		}
		item := Item{Rel: filepath.ToSlash(rel), Src: path, Dest: filepath.Join(workDir, rel), IsDir: d.IsDir()}
		if reason, ok := refuse(item, d, root); ok {
			return &Refusal{Rel: item.Rel, Reason: reason}
		}
		items = append(items, item)
		return nil
	})
	if walkErr != nil {
		return Plan{}, walkErr
	}

	// Directories first, so every file lands in a directory that exists.
	slices.SortStableFunc(items, func(a, b Item) int {
		switch {
		case a.IsDir && !b.IsDir:
			return -1
		case !a.IsDir && b.IsDir:
			return 1
		}
		return strings.Compare(a.Rel, b.Rel)
	})
	return Plan{Items: items}, nil
}

// refuse applies the refusal matrix to one staged entry.
func refuse(it Item, d fs.DirEntry, resolvedWork string) (RefusalReason, bool) {
	first, _, _ := strings.Cut(it.Rel, "/")
	if strings.EqualFold(first, "worktree") || strings.EqualFold(it.Rel, "work-state.json") {
		return ReservedPath, true
	}
	if t := d.Type(); !t.IsRegular() && !t.IsDir() {
		return NotRegular, true
	}
	if escapes(it.Dest, resolvedWork) {
		return EscapesWork, true
	}
	info, err := os.Lstat(it.Dest)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", false
	case err != nil:
		return Exists, true
	case it.IsDir && info.IsDir():
		return "", false // merged into
	}
	return Exists, true
}

// escapes reports whether the deepest existing part of dest resolves outside
// the (already resolved) Work directory, which is how a symlink inside the
// Work could redirect a write elsewhere.
func escapes(dest, resolvedWork string) bool {
	for p := dest; ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			resolved, err := filepath.EvalSymlinks(p)
			if err != nil {
				return true
			}
			rel, err := filepath.Rel(resolvedWork, resolved)
			return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		if p == filepath.Dir(p) {
			return true
		}
	}
}
