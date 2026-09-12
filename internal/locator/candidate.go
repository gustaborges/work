package locator

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/reporef"
)

// validateAndDedup validates each raw repo_path with reporef.ValidatePath —
// the core's single path-to-usable-repo authority — and collapses path and
// symlink variants of the same repository to one entry, keyed on the
// resolved, symlink-evaluated absolute path (FR-016, FR-017, SC-009). Invalid
// matches are dropped silently; the caller decides what an empty result
// means.
func validateAndDedup(rawPaths []string) []string {
	seen := make(map[string]bool, len(rawPaths))
	valid := make([]string, 0, len(rawPaths))
	for _, raw := range rawPaths {
		resolved, err := reporef.ValidatePath(raw)
		if err != nil {
			continue
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		valid = append(valid, resolved)
	}
	return valid
}

// buildCandidates attaches the picker's secondary identifying line to each
// ambiguous candidate (research R15): the first remote fetch URL, else the
// parent directory name. Only called when there are >= 2 candidates, since
// the single-match case never shows a picker.
func buildCandidates(paths []string) []Candidate {
	out := make([]Candidate, len(paths))
	for i, p := range paths {
		out[i] = Candidate{Path: p, Remote: secondaryLine(p)}
	}
	return out
}

// secondaryLine returns the first remote's fetch URL for repoPath, or the
// basename of its parent directory when it has no remote or git is
// unavailable.
func secondaryLine(repoPath string) string {
	if out, err := exec.Command("git", "-C", repoPath, "remote").Output(); err == nil {
		if names := strings.Fields(string(out)); len(names) > 0 {
			if url, err := exec.Command("git", "-C", repoPath, "remote", "get-url", names[0]).Output(); err == nil {
				if u := strings.TrimSpace(string(url)); u != "" {
					return u
				}
			}
		}
	}
	return filepath.Base(filepath.Dir(repoPath))
}
