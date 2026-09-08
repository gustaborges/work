//go:build unix

package integration

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The diagnostic border renders a failure once, and differently per channel
// (contracts/diagnostics.md, FR-014–FR-016, SC-007):
//   - interactively: a single "✘ <message>" (+ "  → <hint>"), never the
//     "error: <token>:" line, never a wrapped cause chain;
//   - non-interactively: the byte-for-byte frozen "error: <token>: <message>"
//     line and the same exit code.
func TestDiagBorderInteractiveVsPiped(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)

	t.Run("resume-unknown-id", func(t *testing.T) {
		env, _, _ := ptyEnv(t)

		// Interactive: the clean one-liner, no "error:" prefix, exit 21.
		c := newConsole(t, bin, env, "resume", "01000000000000000000000000")
		if code := c.wait(); code != 21 {
			t.Fatalf("interactive resume unknown-id exited %d, want 21\n%s", code, c.screen())
		}
		screen := c.screen()
		if !strings.Contains(screen, "✘ no Work has that id") {
			t.Errorf("interactive diagnostic missing the ✘ line:\n%s", screen)
		}
		if !strings.Contains(screen, "→ run `work resume`") {
			t.Errorf("interactive diagnostic missing the hint line:\n%s", screen)
		}
		if strings.Contains(screen, "error: target-not-found") {
			t.Errorf("the non-interactive form leaked into the interactive render:\n%s", screen)
		}

		// Piped: byte-for-byte the frozen contract line, exit 21.
		cmd := exec.Command(bin, "resume", "01000000000000000000000000")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if code := exitCode(err); code != 21 {
			t.Fatalf("piped resume unknown-id exited %d, want 21\n%s", code, out)
		}
		if got := strings.TrimRight(string(out), "\n"); got != "error: target-not-found: no Work has that id" {
			t.Errorf("piped diagnostic = %q", got)
		}
		if strings.Contains(string(out), "✘") {
			t.Errorf("piped diagnostic used the interactive mark: %q", out)
		}
	})

	t.Run("start-materialization-failure", func(t *testing.T) {
		// Each run starts from a clean home so the workspace-root write the
		// failed run persists does not turn the next run into a usage error.
		run := func(interactive bool) (string, int) {
			env, homeDir, _ := ptyEnv(t)
			env = append(env, "WORK_FAIL_AT=worktree")
			repo := filepath.Join(homeDir, "src")
			makeRepo(t, repo)
			ws := filepath.Join(homeDir, "ws")
			args := []string{"start", repo, "--workspace", ws, "--base", "main",
				"--slug", "boom", "--prefix", "{slug}", "--yes"}
			if interactive {
				c := newConsole(t, bin, env, args...)
				code := c.wait()
				return c.screen(), code
			}
			cmd := exec.Command(bin, args...)
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			return string(out), exitCode(err)
		}

		screen, code := run(true)
		if code != 17 {
			t.Fatalf("interactive start failure exited %d, want 17\n%s", code, screen)
		}
		if !strings.Contains(screen, "✘ ") {
			t.Errorf("interactive failure missing the ✘ mark:\n%s", screen)
		}
		if strings.Contains(screen, "error: materialization-failed") {
			t.Errorf("the non-interactive form leaked into the interactive render:\n%s", screen)
		}

		out, code := run(false)
		if code != 17 {
			t.Fatalf("piped start failure exited %d, want 17\n%s", code, out)
		}
		if line := firstErrorLine(out); !strings.HasPrefix(line, "error: materialization-failed: ") {
			t.Errorf("piped failure line = %q, want the frozen token form", line)
		}
	})
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func firstErrorLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "error: ") {
			return l
		}
	}
	return ""
}
