// Command locator is the official reference "filesystem-repository-locator"
// seed component. It reads a repository query plus search roots on stdin, walks
// each root to a bounded depth, and emits every matching local checkout as
// {"matches":[{"repo_path":"<abs>"},...]}. It never ranks results. See
// specs/001-first-local-work/contracts/ipc-repository-locator.md.
//
// On the F1 happy path this component is not executed (the Starter returns a
// path directly), but it is built, embedded, and contract-tested now.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// maxDepth bounds how far below each search root the walk descends.
const maxDepth = 6

type input struct {
	Repository struct {
		Name         string   `json:"name"`
		GitFetchURLs []string `json:"git_fetch_urls"`
		Query        string   `json:"query"`
	} `json:"repository"`
	RepositoryRoots []string `json:"repository_roots"`
}

type match struct {
	RepoPath string `json:"repo_path"`
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "filesystem-repository-locator:", err)
		os.Exit(1)
	}
}

func run(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("invalid input: not a JSON object")
	}

	matches := []match{}
	seen := map[string]bool{}

	for _, root := range in.RepositoryRoots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("resolving root %q: %w", root, err)
		}
		if _, err := os.ReadDir(abs); err != nil {
			return fmt.Errorf("reading root %q: %w", root, err)
		}
		err = walk(abs, 0, func(repoDir string) {
			if seen[repoDir] || !matchesRepo(repoDir, in) {
				return
			}
			seen[repoDir] = true
			matches = append(matches, match{RepoPath: repoDir})
		})
		if err != nil {
			return err
		}
	}

	return json.NewEncoder(stdout).Encode(map[string]any{"matches": matches})
}

// walk descends dir up to maxDepth. When a directory looks like a git repo its
// callback fires and the walk does not descend into it.
func walk(dir string, depth int, onRepo func(string)) error {
	if depth > maxDepth {
		return nil
	}
	if isRepo(dir) {
		onRepo(dir)
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Deeper unreadable directories are skipped, not fatal.
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := walk(filepath.Join(dir, e.Name()), depth+1, onRepo); err != nil {
			return err
		}
	}
	return nil
}

func isRepo(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	return false
}

func matchesRepo(repoDir string, in input) bool {
	base := filepath.Base(repoDir)
	if in.Repository.Name != "" && base == in.Repository.Name {
		return true
	}
	if in.Repository.Query != "" && strings.Contains(base, in.Repository.Query) {
		return true
	}
	if len(in.Repository.GitFetchURLs) > 0 && fetchURLMatches(repoDir, in.Repository.GitFetchURLs) {
		return true
	}
	return false
}

// fetchURLMatches compares the fetch URLs of every local remote against want,
// with no normalization (ADR-0016).
func fetchURLMatches(repoDir string, want []string) bool {
	out, err := exec.Command("git", "-C", repoDir, "remote").Output()
	if err != nil {
		return false
	}
	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	for remote := range strings.FieldsSeq(string(out)) {
		u, err := exec.Command("git", "-C", repoDir, "remote", "get-url", remote).Output()
		if err != nil {
			continue
		}
		if wantSet[strings.TrimSpace(string(u))] {
			return true
		}
	}
	return false
}
