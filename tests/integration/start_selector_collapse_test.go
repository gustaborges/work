//go:build unix

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/creack/pty"
)

// The base-branch selector is a scrolling list while active and collapses on
// Enter to a two-line receipt with a single blank separator — no list rows and
// no padded frame survive, and the confirmation block starts right after it.
// Checked at a small and a large terminal. (US1 #2; SC-002, SC-004.)
func TestStartSelectorCollapsesToReceipt(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)

	sizes := []pty.Winsize{{Rows: 24, Cols: 80}, {Rows: 50, Cols: 160}}
	for _, ws := range sizes {
		ws := ws
		t.Run(fmt.Sprintf("%dx%d", ws.Cols, ws.Rows), func(t *testing.T) {
			env, homeDir, _ := ptyEnv(t)
			repo := filepath.Join(homeDir, "src")
			makeRepo(t, repo)
			for _, b := range []string{"feature-a", "feature-b", "feature-c", "release-1", "release-2", "hotfix"} {
				gitIn(t, repo, "branch", b)
			}
			wsRoot := filepath.Join(homeDir, "ws")

			c := newConsoleSize(t, ws, bin, env, "start", repo,
				"--workspace", wsRoot, "--slug", "picked", "--prefix", "{slug}")

			// Active: a multi-row list with the fixed focus marker.
			c.expect("Base branch")
			c.expect("❯ ")
			c.expect("feature-c")
			active := c.screen()
			if !strings.Contains(active, "feature-a") || !strings.Contains(active, "feature-c") {
				t.Fatalf("selector is not showing a multi-row list:\n%s", active)
			}
			if !strings.Contains(active, "select") {
				t.Errorf("selector help line missing:\n%s", active)
			}

			// Enter accepts the focused row and the frame collapses.
			c.send("\r")
			c.expect("Create Work")
			screen := c.screen()

			collapsed := regexp.MustCompile(`Base branch\n {2}✔ [^\n]+\n\nCreate Work`)
			if !collapsed.MatchString(screen) {
				t.Errorf("selector did not collapse to <receipt><blank>Create Work:\n%s", screen)
			}
			// The list chrome — focus marker, unfocused rows, help line — is gone.
			for _, gone := range []string{"❯ ", "enter select", "feature-b", "release-1", "hotfix"} {
				if strings.Contains(screen, gone) {
					t.Errorf("selector debris %q survived the collapse:\n%s", gone, screen)
				}
			}

			c.send("\x03") // cancel the confirm; nothing was created
			if code := c.wait(); code != 20 {
				t.Fatalf("Ctrl-C at the confirm exited %d, want 20", code)
			}
			if _, err := os.Stat(filepath.Join(wsRoot, "in-progress", "src_picked")); err == nil {
				t.Errorf("a cancelled run left a Work directory")
			}
		})
	}
}
