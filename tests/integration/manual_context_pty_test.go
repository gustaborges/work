//go:build unix

package integration

import (
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
