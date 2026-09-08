//go:build unix

package integration

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/creack/pty"
)

// Interactive bare `work` prints the static WORK brand to stdout and exits 0 —
// no selector, no Bubble Tea program. Wide terminals get the multi-line art with
// a colour gradient; narrow terminals get the compact word. Piped, it keeps the
// F1/F2 non-interactive usage failure (exit 2, no ANSI). (US2 #1, #2;
// FR-017–FR-019, FR-028; SC-009.)
func TestBrandInteractiveWide(t *testing.T) {
	bin := buildWorkBin(t)
	env, _, _ := ptyEnv(t)

	c := newConsoleSize(t, pty.Winsize{Rows: 40, Cols: 120}, bin, env)

	// The multi-line block art (its glyphs spell WORK) and the brand copy.
	c.expect("██")
	c.expect("Isolated work, ready when you are.")
	c.expect("Run 'work --help' to get started.")

	if code := c.wait(); code != 0 {
		t.Fatalf("interactive bare `work` exited %d, want 0", code)
	}

	// No selector chrome ever appeared.
	snap := c.snapshot()
	for _, chrome := range []string{"Start a Work", "❯ ", "select", "move"} {
		if strings.Contains(snap, chrome) {
			t.Errorf("bare `work` rendered selector chrome %q:\n%s", chrome, snap)
		}
	}

	// TERM=xterm-256color is colour-capable: the wide art carries a gradient.
	raw := string(rawOf(c))
	if !strings.Contains(raw, "\x1b[") {
		t.Errorf("wide colour-capable brand carries no styling:\n%q", raw)
	}
}

func TestBrandInteractiveNarrowCompact(t *testing.T) {
	bin := buildWorkBin(t)
	env, _, _ := ptyEnv(t)

	c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 30}, bin, env)

	c.expect("WORK")
	c.expect("Isolated work, ready when you are.")
	if code := c.wait(); code != 0 {
		t.Fatalf("narrow bare `work` exited %d, want 0", code)
	}
	if strings.Contains(c.snapshot(), "██") {
		t.Errorf("narrow terminal still got the block art:\n%s", c.snapshot())
	}
}

func TestBrandNoColorIsPlain(t *testing.T) {
	bin := buildWorkBin(t)
	env, _, _ := ptyEnv(t)
	env = append(env, "NO_COLOR=1")

	c := newConsoleSize(t, pty.Winsize{Rows: 40, Cols: 120}, bin, env)
	c.expect("Run 'work --help' to get started.")
	if code := c.wait(); code != 0 {
		t.Fatalf("bare `work` with NO_COLOR exited %d, want 0", code)
	}
	if raw := string(rawOf(c)); strings.Contains(raw, "\x1b[3") || strings.Contains(raw, "\x1b[1m") {
		t.Errorf("NO_COLOR brand still emits SGR colour/bold:\n%q", raw)
	}
}

func TestBrandPipedIsUsageExit2(t *testing.T) {
	bin := buildWorkBin(t)
	env, _, _ := ptyEnv(t)

	cmd := exec.Command(bin)
	cmd.Env = env
	out, err := cmd.CombinedOutput()

	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 2 {
		t.Fatalf("piped bare `work` exit = %v, want 2\n%s", err, out)
	}
	if strings.ContainsRune(string(out), '\x1b') {
		t.Errorf("piped bare `work` emitted an escape sequence:\n%q", out)
	}
	for _, want := range []string{"work start", "work resume", "work archive", "work --help"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("piped bare `work` summary missing %q:\n%s", want, out)
		}
	}
}

// rawOf returns the console's raw (un-stripped) pty bytes.
func rawOf(c *console) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.raw...)
}
