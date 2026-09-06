package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gittest"
)

// TestStartDiagnosticsContract drives `work start` into each failure category it
// can reach non-interactively and asserts the FR-025/FR-027 contract: the exit
// code and stderr token are exactly the ones diag defines, the message names
// only the user's own input, and no home path or repository internals leak.
func TestStartDiagnosticsContract(t *testing.T) {
	needSeed(t)

	repoWithBranch := func(t *testing.T, extra ...string) string {
		r := gittest.Repo(t)
		for _, b := range extra {
			gittest.Git(t, r, "branch", b)
		}
		return r
	}

	cases := []struct {
		name  string
		token string
		code  int
		args  func(t *testing.T) []string
		env   map[string]string
	}{
		{
			name:  "usage/missing-slug",
			token: "usage", code: 2,
			args: func(t *testing.T) []string {
				return []string{"start", gittest.Repo(t), "--workspace", t.TempDir(),
					"--base", "main", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "invalid-path/missing",
			token: "invalid-path", code: 10,
			args: func(t *testing.T) []string {
				return []string{"start", filepath.Join(t.TempDir(), "nope"),
					"--workspace", t.TempDir(), "--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "unusable-repo/plain-dir",
			token: "unusable-repo", code: 11,
			args: func(t *testing.T) []string {
				return []string{"start", t.TempDir(),
					"--workspace", t.TempDir(), "--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "no-base-branch",
			token: "no-base-branch", code: 12,
			args: func(t *testing.T) []string {
				r := gittest.Repo(t)
				gittest.Git(t, r, "checkout", "-q", "--detach", "HEAD")
				gittest.Git(t, r, "branch", "-D", "main")
				return []string{"start", r, "--workspace", t.TempDir(),
					"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "invalid-branch-name",
			token: "invalid-branch-name", code: 13,
			args: func(t *testing.T) []string {
				return []string{"start", gittest.Repo(t), "--workspace", t.TempDir(),
					"--base", "main", "--slug", "has spaces", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "branch-collision",
			token: "branch-collision", code: 14,
			args: func(t *testing.T) []string {
				return []string{"start", repoWithBranch(t, "taken"), "--workspace", t.TempDir(),
					"--base", "main", "--slug", "taken", "--prefix", "{slug}", "--yes"}
			},
		},
		{
			name:  "materialization-failed",
			token: "materialization-failed", code: 17,
			env: map[string]string{"WORK_FAIL_AT": "worktree"},
			args: func(t *testing.T) []string {
				return []string{"start", gittest.Repo(t), "--workspace", t.TempDir(),
					"--base", "main", "--slug", "boom", "--prefix", "{slug}", "--yes"}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			home := filepath.Join(t.TempDir(), "dothome")
			t.Setenv("WORK_HOME", home)

			_, stderr, code := runWork(t, tc.args(t)...)

			if code != tc.code {
				t.Fatalf("exit = %d, want %d\nstderr: %s", code, tc.code, stderr)
			}
			line := errorLine(stderr)
			if line == "" {
				t.Fatalf("no 'error: ' line on stderr:\n%s", stderr)
			}
			if !strings.HasPrefix(line, "error: "+tc.token+": ") {
				t.Errorf("stderr line = %q, want token %q", line, tc.token)
			}
			if strings.Contains(line, home) {
				t.Errorf("diagnostic leaks the Work home path: %q", line)
			}
			if strings.Contains(line, ".git/objects") || strings.Contains(line, "refs/remotes/") {
				t.Errorf("diagnostic leaks repository internals: %q", line)
			}
		})
	}
}

// errorLine returns the first line beginning with "error: ".
func errorLine(s string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(l, "error: ") {
			return l
		}
	}
	return ""
}
