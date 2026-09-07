//go:build unix

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Interactive cancellation of the resume and archive selectors: the terminal
// shows exactly one "✘ Operation cancelled" line (never the non-interactive
// "error:" form), the process exits 20, and no Work changed — no
// last_accessed_at bump, no worktree, branch, or directory mutation.
// (US4 #1; FR-012, FR-013; SC-010.)
func TestCancellationLeavesOneNoticeAndNoMutation(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)

	for _, key := range []struct{ name, seq string }{
		{"q", "q"},
		{"esc", "\x1b"},
		{"ctrl-c", "\x03"},
	} {
		t.Run("resume/"+key.name, func(t *testing.T) {
			env, homeDir, _ := ptyEnv(t)
			repo := filepath.Join(homeDir, "demo")
			makeRepo(t, repo)
			ws := filepath.Join(homeDir, "ws")
			seedWorks(t, bin, env, repo, ws, "alpha", "bravo")
			bravoDir := filepath.Join(ws, "in-progress", "demo_bravo")
			before := snapAccessed(t, bravoDir)

			c := newConsole(t, bin, env, "resume")
			c.expect("Resume a Work")
			c.expect("demo  bravo")
			c.send(key.seq)
			if code := c.wait(); code != 20 {
				t.Fatalf("resume cancel (%s) exited %d, want 20\n%s", key.name, code, c.screen())
			}
			assertOneCancelNotice(t, c.screen())
			if after := snapAccessed(t, bravoDir); after != before {
				t.Errorf("cancelled resume bumped last_accessed_at: %q -> %q", before, after)
			}
		})
	}

	t.Run("archive/confirm-then-ctrl-c", func(t *testing.T) {
		env, homeDir, _ := ptyEnv(t)
		repo := filepath.Join(homeDir, "demo")
		makeRepo(t, repo)
		ws := filepath.Join(homeDir, "ws")
		seedWorks(t, bin, env, repo, ws, "alpha", "bravo")

		c := newConsole(t, bin, env, "archive")
		c.expect("Archive Works")
		c.expect("demo  ")
		c.send(" ")  // check the focused row
		c.send("\r") // -> confirmation sub-state
		c.expect("worktree(s) will be destroyed")
		c.send("\x03") // Ctrl-C from the confirmation
		if code := c.wait(); code != 20 {
			t.Fatalf("archive cancel exited %d, want 20\n%s", code, c.screen())
		}
		assertOneCancelNotice(t, c.screen())

		for _, slug := range []string{"alpha", "bravo"} {
			if _, err := os.Stat(filepath.Join(ws, "in-progress", "demo_"+slug, "worktree")); err != nil {
				t.Errorf("%s worktree gone after a cancelled archive: %v", slug, err)
			}
			if !branchExists(t, repo, slug) {
				t.Errorf("%s branch gone after a cancelled archive", slug)
			}
			if got := snapStatus(t, filepath.Join(ws, "in-progress", "demo_"+slug, "work-state.json")); got != "in-progress" {
				t.Errorf("%s status = %q after a cancelled archive, want in-progress", slug, got)
			}
		}
	})
}

func assertOneCancelNotice(t *testing.T, screen string) {
	t.Helper()
	const notice = "✘ Operation cancelled"
	if n := strings.Count(screen, notice); n != 1 {
		t.Errorf("want exactly one %q, got %d:\n%s", notice, n, screen)
	}
	if strings.Contains(screen, "error: cancelled") {
		t.Errorf("the non-interactive 'error:' form leaked into an interactive cancel:\n%s", screen)
	}
}
