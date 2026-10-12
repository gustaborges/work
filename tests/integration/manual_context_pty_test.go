//go:build unix

package integration

import (
	"fmt"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
	"github.com/gustaborges/work/internal/work"
)

func TestManualLinkJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	for _, mode := range []string{"both-omitted", "value-omitted", "direct", "cancel-selection", "cancel-value"} {
		t.Run(mode, func(t *testing.T) {
			env, home, _ := ptyEnv(t)
			installPluginFixture(t, bin, env, "manual-suite")
			repo := filepath.Join(home, "demo")
			makeRepo(t, repo)
			ws := filepath.Join(home, "ws")
			seedWorks(t, bin, env, repo, ws, "topic")
			dir := filepath.Join(ws, "in-progress", "demo_topic")
			path := filepath.Join(dir, "work-state.json")
			before, _ := os.ReadFile(path)
			args := []string{"link"}
			if mode == "value-omitted" || mode == "cancel-value" || mode == "direct" {
				args = append(args, "manual-suite/linker")
			}
			if mode == "direct" {
				args = append(args, "private-direct-value")
			}
			cmd := exec.Command(bin, args...)
			cmd.Env = env
			cmd.Dir = filepath.Join(dir, "worktree")
			c := startConsole(t, pty.Winsize{Rows: 24, Cols: 80}, cmd)
			if mode == "both-omitted" || mode == "cancel-selection" {
				c.expect("Choose a Linker")
				c.expect("manual-suite/linker")
				if mode == "cancel-selection" {
					c.send("\x1b")
				} else {
					c.send("\r")
				}
			}
			if mode == "both-omitted" || mode == "value-omitted" || mode == "cancel-value" {
				c.expect("Link value")
				if mode == "cancel-value" {
					c.send("\x1b")
				} else {
					c.send("\r")
					c.expect("enter a non-empty value")
					c.send("private-interactive-value")
					c.send("\r")
				}
			}
			code := c.wait()
			if strings.HasPrefix(mode, "cancel") {
				if code != 20 {
					t.Fatal(code, c.screen())
				}
				assertOneCancelNotice(t, c.screen())
				after, _ := os.ReadFile(path)
				if string(after) != string(before) {
					t.Fatal("cancel mutated canonical state")
				}
			} else {
				if code != 0 {
					t.Fatal(code, c.screen())
				}
				if !strings.Contains(c.screen(), "work: linked ") || strings.Contains(c.screen(), "private-") {
					t.Fatal(c.screen())
				}
				if strings.Contains(c.screen(), "enter a non-empty value") {
					t.Fatal("rejected field error survived acceptance", c.screen())
				}
				if (mode == "direct" || mode == "value-omitted") && strings.Contains(c.snapshot(), "Choose a Linker") {
					t.Fatal("explicit provider was prompted again")
				}
				if mode == "direct" && strings.Contains(c.snapshot(), "Link value") {
					t.Fatal("direct value was prompted again")
				}
				if mode != "direct" && !strings.Contains(c.screen(), "value supplied") {
					t.Fatal(c.screen())
				}
				st, err := work.Read(path)
				if err != nil || !strings.HasPrefix(st.Links["github.pull_request"], "private-") {
					t.Fatal(st, err)
				}
			}
		})
	}
}

// Colliding manual labels remain distinguishable by identity at every supported
// size. Filtering and resize use the same controls as the existing selectors.
func TestManualContextGeometry(t *testing.T) {
	h := newF6Harness(t)
	h.run(t, h.root, 0, "plugin", "install", fixtures.Prepare(t, "manual-suite"), "--as", "alternate")
	for _, size := range []pty.Winsize{{Rows: 10, Cols: 40}, {Rows: 24, Cols: 80}, {Rows: 50, Cols: 160}} {
		t.Run(fmt.Sprintf("%dx%d", size.Cols, size.Rows), func(t *testing.T) {
			cmd := exec.Command(h.bin, "link")
			cmd.Env = h.env
			cmd.Dir = h.dir
			c := startConsole(t, size, cmd)
			c.expect("Choose a Linker")
			c.expect("alternate/linker")
			c.expect("manual/linker")
			assertNoLineExceeds(t, c.screen(), int(size.Cols))
			c.send("\x1b[B\x1b[A")
			if err := pty.Setsize(c.f, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
				t.Fatal(err)
			}
			c.send("/manual")
			c.expect("manual")
			c.send("\r")
			c.expect("Link value")
			c.send("\r")
			c.expect("enter a non-empty value")
			c.send("private-geometry-value\r")
			if code := c.wait(); code != 0 {
				t.Fatal(code, c.screen())
			}
			// Replay the final screen at the resized dimensions.
			c.rows, c.cols = 24, 80
			screen := c.screen()
			if !strings.Contains(screen, "value supplied") || !strings.Contains(screen, "manual/linker") || strings.Contains(screen, "private-geometry-value") || strings.Contains(screen, "enter a non-empty value") {
				t.Fatal(screen)
			}
		})
	}
}

func TestManualEmptySelectors(t *testing.T) {
	h := newF6Harness(t)
	for _, tc := range []struct {
		command  string
		disabled bool
		guidance string
	}{
		{"import", false, "required inputs"}, {"import", true, "install or enable"}, {"link", true, "install or enable"},
	} {
		t.Run(fmt.Sprint(tc.command, tc.disabled), func(t *testing.T) {
			if err := fixtures.SetPackageEnabled(filepath.Join(h.root, "home", "state", "registry.json"), "manual", !tc.disabled); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(h.bin, tc.command)
			cmd.Env = h.env
			cmd.Dir = h.dir
			c := startConsole(t, pty.Winsize{Rows: 24, Cols: 80}, cmd)
			if code := c.wait(); code != 41 {
				t.Fatal(code, c.screen())
			}
			if !strings.Contains(c.screen(), tc.guidance) || strings.Contains(c.snapshot(), "Choose a") {
				t.Fatal(c.screen())
			}
		})
	}
}

func TestManualSelectionRevalidated(t *testing.T) {
	h := newF6Harness(t)
	for _, command := range []string{"link", "import"} {
		t.Run(command, func(t *testing.T) {
			path := filepath.Join(h.root, "home", "state", "registry.json")
			if err := fixtures.SetPackageEnabled(path, "manual", true); err != nil {
				t.Fatal(err)
			}
			h.run(t, h.dir, 0, "link", "manual/linker", "original")
			before, err := os.ReadFile(filepath.Join(h.dir, "work-state.json"))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(h.bin, command)
			cmd.Env = h.env
			cmd.Dir = h.dir
			c := startConsole(t, pty.Winsize{Rows: 24, Cols: 80}, cmd)
			if command == "link" {
				c.expect("Choose a Linker")
				c.send("\r")
				c.expect("Link value")
			} else {
				c.expect("Choose an Importer")
			}
			if err := fixtures.SetPackageEnabled(path, "manual", false); err != nil {
				t.Fatal(err)
			}
			if command == "link" {
				c.send("private-stale-value\r")
			} else {
				c.send("\r")
			}
			if code := c.wait(); code != 39 {
				t.Fatal(code, c.screen())
			}
			after, err := os.ReadFile(filepath.Join(h.dir, "work-state.json"))
			if err != nil || string(before) != string(after) {
				t.Fatal("stale selection mutated snapshot", err)
			}
			if _, err := os.Stat(h.log); !os.IsNotExist(err) {
				t.Fatal("stale selection ran a process", err)
			}
		})
	}
}
