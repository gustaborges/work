// Package repoconfig implements the Repository Resolution Policy and
// repository search root operations over config.Config (ADR-0015, ADD §7.2):
// list/add/remove/move/replace, the availability model, and the validation
// shared by the `work repository` commands and work start's first-run setup.
// It has no internal/present import — the CLI owns all prompting and
// rendering; this package only validates and mutates the in-memory
// config.Config the caller loaded and will save.
package repoconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workspace"
)

// ListRoots returns the configured search roots in their stored order. An
// empty configuration yields an empty slice, never an error.
func ListRoots(cfg *config.Config) []string {
	return slices.Clone(cfg.RepositoryRoots)
}

// AddRoots validates and appends each path to cfg.RepositoryRoots. Nothing is
// written until every path validates: the first invalid path aborts the whole
// call leaving cfg untouched, so the caller's single config.Save is either a
// full success or a no-op (FR-020, FR-022, research R13).
func AddRoots(cfg *config.Config, paths []string) error {
	next := slices.Clone(cfg.RepositoryRoots)
	for _, p := range paths {
		abs, err := ValidateRoot(cfg, p)
		if err != nil {
			return err
		}
		if !containsCanonical(next, abs) {
			next = append(next, abs)
		}
	}
	cfg.RepositoryRoots = next
	return nil
}

// RemoveRoots drops each path from cfg.RepositoryRoots on a canonical match; a
// path that is not configured is a no-op success.
func RemoveRoots(cfg *config.Config, paths []string) error {
	next := cfg.RepositoryRoots
	for _, p := range paths {
		abs, err := absExpand(p)
		if err != nil {
			return diag.Newf(diag.Usage, "cannot resolve path %q", p)
		}
		next = removeCanonical(next, abs)
	}
	cfg.RepositoryRoots = next
	return nil
}

// ReplaceRoots validates the whole new set (including workspace overlap) and
// only then replaces cfg.RepositoryRoots — a rejected entry leaves cfg
// untouched.
func ReplaceRoots(cfg *config.Config, paths []string) error {
	next := []string{}
	for _, p := range paths {
		abs, err := ValidateRoot(cfg, p)
		if err != nil {
			return err
		}
		if !containsCanonical(next, abs) {
			next = append(next, abs)
		}
	}
	cfg.RepositoryRoots = next
	return nil
}

// ValidateRoot expands a leading ~, absolutizes path, and checks it is an
// existing readable directory that does not overlap cfg.Workspace in either
// direction (FR-020, research R13). It is exported for the CLI's `work start`
// first-run setup step as well as `root add`/`replace`, and never imports
// internal/present.
func ValidateRoot(cfg *config.Config, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", diag.New(diag.Usage, "a repository search root path is required")
	}
	abs, err := absExpand(trimmed)
	if err != nil {
		return "", diag.Newf(diag.Usage, "cannot resolve path %q", path)
	}
	info, statErr := os.Stat(abs)
	if statErr != nil {
		return "", diag.Newf(diag.Usage, "repository root %s does not exist", path)
	}
	if !info.IsDir() {
		return "", diag.Newf(diag.Usage, "repository root %s is not a directory", path)
	}
	if _, err := os.ReadDir(abs); err != nil {
		return "", diag.Newf(diag.Usage, "repository root %s is not readable", path)
	}

	if ws := strings.TrimSpace(cfg.Workspace); ws != "" {
		if overlaps, err := workspace.Overlaps(abs, ws); err == nil && overlaps {
			return "", diag.Newf(diag.Usage,
				"repository root %s overlaps the workspace root %s", path, ws)
		}
	}
	return abs, nil
}

// NeedsSetup reports whether the interactive `work start` first-run wizard
// must still ask for the workspace root and/or a repository search root
// (research R21).
func NeedsSetup(cfg *config.Config) (wantWorkspace, wantRoot bool) {
	return strings.TrimSpace(cfg.Workspace) == "", len(cfg.RepositoryRoots) == 0
}

// absExpand expands a leading ~ and makes path absolute, without resolving
// symlinks — mirrors workspace.absPath / reporef.expandHome so the stored
// value stays close to what the user typed.
func absExpand(raw string) (string, error) {
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
	return filepath.Abs(p)
}

func containsCanonical(list []string, abs string) bool {
	for _, l := range list {
		if workspace.Canonical(l) == workspace.Canonical(abs) {
			return true
		}
	}
	return false
}

func removeCanonical(list []string, abs string) []string {
	target := workspace.Canonical(abs)
	out := make([]string, 0, len(list))
	for _, l := range list {
		if workspace.Canonical(l) != target {
			out = append(out, l)
		}
	}
	return out
}
