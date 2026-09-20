// Package repoidentity computes the ADR-0011 repository identity key: a
// stable string that names "the same repository" across clones, used to key
// the per-repository branch convention memory (internal/repoconv). It is
// never persisted itself — only recomputed, on demand, from the repository at
// hand.
package repoidentity

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/gitx"
)

// Identify computes repo's identity key by trying three layers in fixed
// order, each used only when the previous one is absent — never because it is
// "less convenient" (ADR-0011):
//
//  1. the "origin" remote's fetch URL, if configured;
//  2. otherwise, when the repository is not shallow, every root commit
//     reachable from HEAD, sorted lexicographically and "+"-joined — sorted
//     so a repository formed by merging unrelated histories gets the same
//     key regardless of merge order;
//  3. otherwise (a shallow clone with no remote — a shallow boundary commit
//     is a truncation artifact of that particular clone's depth, not a
//     stable identity, so root commits are not trusted here), the absolute,
//     symlink-resolved repository path.
func Identify(repo gitx.Repo) (string, error) {
	if url, ok, err := repo.RemoteURL("origin"); err != nil {
		return "", err
	} else if ok {
		return url, nil
	}

	shallow, err := repo.IsShallow()
	if err != nil {
		return "", err
	}
	if !shallow {
		roots, err := repo.RootCommits()
		if err != nil {
			return "", err
		}
		if len(roots) > 0 {
			sorted := slices.Clone(roots)
			slices.Sort(sorted)
			return strings.Join(sorted, "+"), nil
		}
	}

	abs, err := filepath.Abs(repo.Dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}
