//go:build unix

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/charmbracelet/x/ansi"
)

// The migrated resume and archive selectors hold a bounded, column-stable
// geometry on a real terminal (contracts/interaction.md §4–5, SC-003, SC-004):
// no rendered line is wider than the viewport at any size, every Work row's
// primary text starts in the same column regardless of which row is focused or
// checked, and the focused row stays identifiable with colour forced off. The
// per-keystroke column invariants across filter/tab changes are pinned by the
// model tests in internal/present.
func TestSelectorGeometryIsStable(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := homeDir + "/demo"
	makeRepo(t, repo)
	ws := homeDir + "/ws"
	slugs := []string{
		"short", "a-considerably-longer-feature-branch-name-for-truncation",
		"mid-length-name", "another-long-one-that-should-need-eliding-somewhere",
		"tiny", "sixth-work-entry",
	}
	seedWorks(t, bin, env, repo, ws, slugs...)

	for _, sz := range []pty.Winsize{{Rows: 10, Cols: 40}, {Rows: 24, Cols: 80}, {Rows: 50, Cols: 160}} {
		t.Run(fmt.Sprintf("resume/%dx%d", sz.Cols, sz.Rows), func(t *testing.T) {
			c := newConsoleSize(t, sz, bin, env, "resume")
			c.expect("Resume a Work")
			c.expect("demo  ")
			settle()

			assertNoLineExceeds(t, c.screen(), int(sz.Cols))
			assertRowsShareAColumn(t, c.screen(), "initial frame")

			// Focus the third row; the marker slot is fixed width, so every
			// row's primary column is unchanged.
			c.send("\x1b[B\x1b[B")
			settle()
			assertNoLineExceeds(t, c.screen(), int(sz.Cols))
			assertRowsShareAColumn(t, c.screen(), "focus on row 3")

			c.send("q")
			if code := c.wait(); code != 20 {
				t.Fatalf("resume cancel exited %d, want 20\n%s", code, c.screen())
			}
		})
	}

	t.Run("archive/checkbox-and-focus-independent", func(t *testing.T) {
		c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "archive")
		c.expect("Archive Works")
		c.expect("demo  ")
		settle()
		if !strings.Contains(c.screen(), "[ ] ") {
			t.Errorf("archive rows missing the fixed checkbox slot:\n%s", c.screen())
		}
		assertRowsShareAColumn(t, c.screen(), "archive initial")

		c.send(" ")            // check the focused row
		c.send("\x1b[B\x1b[B") // move focus two rows down
		settle()
		// A checked row and a moved focus must not shift any primary column.
		assertRowsShareAColumn(t, c.screen(), "after check + focus move")
		assertNoLineExceeds(t, c.screen(), 80)

		c.send("\x03")
		if code := c.wait(); code != 20 {
			t.Fatalf("archive cancel exited %d, want 20", code)
		}
	})

	t.Run("resume/focus-visible-without-colour", func(t *testing.T) {
		c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin,
			envWith(env, "NO_COLOR", "1"), "resume")
		c.expect("Resume a Work")
		c.expect("demo  ")
		c.send("\x1b[B")
		settle()
		if !strings.Contains(c.screen(), "❯ ") {
			t.Errorf("focused row not identifiable with NO_COLOR=1 (no ❯ marker):\n%s", c.screen())
		}
		c.send("q")
		c.wait()
	})
}

// settle lets the pty catch up after a burst of keystrokes so c.screen()
// reflects the final frame rather than an in-flight repaint.
func settle() { time.Sleep(300 * time.Millisecond) }

// assertRowsShareAColumn checks that every visible Work row line in one frame
// starts its primary text ("demo …") in the same display column — the fixed
// 2-cell focus-marker slot in action (SC-003).
func assertRowsShareAColumn(t *testing.T, screen, when string) {
	t.Helper()
	var cols []int
	for line := range strings.SplitSeq(screen, "\n") {
		i := strings.Index(line, "demo  ")
		if i < 0 {
			continue
		}
		cols = append(cols, ansi.StringWidth(line[:i]))
	}
	if len(cols) < 2 {
		t.Fatalf("%s: parsed %d Work rows, need at least 2:\n%s", when, len(cols), screen)
	}
	for _, c := range cols[1:] {
		if c != cols[0] {
			t.Errorf("%s: primary columns are not aligned: %v", when, cols)
			return
		}
	}
}

func assertNoLineExceeds(t *testing.T, screen string, cols int) {
	t.Helper()
	for line := range strings.SplitSeq(screen, "\n") {
		if w := ansi.StringWidth(line); w > cols {
			t.Errorf("rendered line is %d cells wide, viewport is %d:\n%q", w, cols, line)
		}
	}
}
