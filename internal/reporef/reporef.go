// Package reporef validates a user-supplied path to a local Git repository.
// In F1 the seed Starter returns only repository.path, so the core validates it
// directly here (ADD §7.1): the path must resolve to a readable directory that
// is a usable, non-bare Git repository with at least one commit. Each rejection
// maps to a distinct diag category, and diagnostics quote only the path the
// user supplied.
package reporef

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
)

// RepositoryReference is the validated result of resolving a local source path.
type RepositoryReference struct {
	// Raw is the string the user supplied, unchanged.
	Raw string
	// Path is the absolute, symlink-resolved repository path.
	Path string
}

// ValidatePath resolves raw and checks it is a usable local Git repository.
// It returns the absolute, symlink-resolved path on success.
func ValidatePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", diag.New(diag.InvalidPath, "no repository path was given")
	}

	expanded, err := expandHome(trimmed)
	if err != nil {
		return "", diag.Newf(diag.InvalidPath, "cannot resolve path %q", raw)
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", diag.Newf(diag.InvalidPath, "cannot resolve path %q", raw)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", diag.Newf(diag.InvalidPath, "path does not exist: %s", raw)
		}
		return "", diag.Newf(diag.InvalidPath, "cannot resolve path %q", raw)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", diag.Newf(diag.InvalidPath, "path does not exist: %s", raw)
		}
		return "", diag.Newf(diag.InvalidPath, "cannot read path: %s", raw)
	}
	if !info.IsDir() {
		return "", diag.Newf(diag.InvalidPath, "path is not a directory: %s", raw)
	}
	if _, err := os.ReadDir(resolved); err != nil {
		return "", diag.Newf(diag.InvalidPath, "path is not readable: %s", raw)
	}

	repo := gitx.Open(resolved)
	if _, err := repo.RevParse("--git-dir"); err != nil {
		return "", diag.Newf(diag.UnusableRepo, "not a Git repository: %s", raw)
	}
	bare, err := repo.IsBare()
	if err != nil {
		return "", diag.Newf(diag.UnusableRepo, "not a Git repository: %s", raw)
	}
	if bare {
		return "", diag.Newf(diag.UnusableRepo, "repository is bare: %s", raw)
	}
	hasCommit, err := repo.HasCommit()
	if err != nil {
		return "", diag.Wrapf(diag.UnusableRepo, err, "cannot inspect repository at %s", raw)
	}
	if !hasCommit {
		return "", diag.Newf(diag.UnusableRepo, "repository has no commits: %s", raw)
	}

	return resolved, nil
}

// expandHome replaces a leading ~ or ~/ with the user's home directory.
func expandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// Format renders a reference for the confirm summary.
func (r RepositoryReference) Format() string {
	return fmt.Sprintf("%s (%s)", filepath.Base(r.Path), r.Path)
}
