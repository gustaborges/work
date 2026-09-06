// Package gittest builds throwaway Git repositories for tests. It is imported
// only from _test.go files.
package gittest

import (
	"os/exec"
	"strings"
	"testing"
)

// Git runs `git -C dir args...` with a fixed identity and fails the test on
// error, returning trimmed combined output.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Repo creates a repository with a single empty commit on branch main and
// returns its path.
func Repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	Git(t, dir, "init", "-q", "-b", "main")
	Git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}
