// Package workspace suggests, validates, and persists the workspace root — the
// directory under which materialized Works live (FR-006, FR-007, R8). It is
// kept separate from the source clones and from the Work home state directory.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/workhome"
)

// Subdirectories created under a workspace root.
const (
	InProgressDir = "in-progress"
	ArchivedDir   = "archived"
)

// SuggestDefault returns the suggested workspace root for a first run: ~/work
// (%USERPROFILE%\work on Windows).
func SuggestDefault() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, "work"), nil
}

// Validate resolves raw and checks it can serve as a workspace root: it is (or
// can become) a writable directory, it is not inside a git work tree, and it is
// not inside any configured repository root. It returns the absolute path.
func Validate(raw string, repositoryRoots []string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", diag.New(diag.Usage, "the workspace root is empty")
	}
	abs, err := resolve(trimmed)
	if err != nil {
		return "", diag.Newf(diag.Usage, "cannot resolve workspace root %q", raw)
	}

	info, statErr := os.Stat(abs)
	switch {
	case statErr == nil && !info.IsDir():
		return "", diag.Newf(diag.Usage, "workspace root %s is a file", raw)
	case statErr == nil:
		if !writable(abs) {
			return "", diag.Newf(diag.Usage, "workspace root %s is not writable", raw)
		}
	case errors.Is(statErr, fs.ErrNotExist):
		parent := firstExistingAncestor(abs)
		if !writable(parent) {
			return "", diag.Newf(diag.Usage, "cannot create workspace root %s: %s is not writable", raw, parent)
		}
	default:
		return "", diag.Newf(diag.Usage, "cannot inspect workspace root %s", raw)
	}

	if inside := firstExistingAncestor(abs); inside != "" {
		if wt, _ := gitx.Open(inside).IsWorkTree(); wt {
			return "", diag.Newf(diag.Usage, "workspace root %s is inside a git repository", raw)
		}
	}

	for _, root := range repositoryRoots {
		rr, err := resolve(root)
		if err != nil {
			continue
		}
		if abs == rr || isSubpath(rr, abs) {
			return "", diag.Newf(diag.Usage, "workspace root %s is inside repository root %s", raw, root)
		}
	}

	return abs, nil
}

// Persist writes absRoot to config.workspace and creates the root's
// in-progress/ and archived/ subdirectories. absRoot must already be validated.
func Persist(h workhome.Home, absRoot string) error {
	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		return err
	}
	cfg.Workspace = absRoot
	if err := config.Save(h.ConfigFile(), cfg); err != nil {
		return diag.Wrap(diag.MaterializationFailed, err, "cannot save the workspace root")
	}
	return EnsureLayout(absRoot)
}

// EnsureLayout creates the in-progress/ and archived/ subdirectories of root.
func EnsureLayout(root string) error {
	for _, sub := range []string{InProgressDir, ArchivedDir} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return diag.Wrapf(diag.MaterializationFailed, err, "cannot create %s under the workspace root", sub)
		}
	}
	return nil
}

func resolve(raw string) (string, error) {
	p := raw
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	// Resolve symlinks on the longest existing prefix and re-append the rest,
	// so a not-yet-created root and an existing repository_root are compared on
	// the same (resolved) basis — matters where TMPDIR itself is a symlink
	// (macOS /var -> /private/var).
	existing := abs
	var tail []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		tail = append([]string{filepath.Base(existing)}, tail...)
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		resolved = existing
	}
	return filepath.Join(append([]string{resolved}, tail...)...), nil
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".work-writetest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func firstExistingAncestor(p string) string {
	for {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		p = parent
	}
}

func isSubpath(ancestor, p string) bool {
	rel, err := filepath.Rel(ancestor, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "."
}
