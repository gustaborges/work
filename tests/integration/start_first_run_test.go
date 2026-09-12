//go:build unix

// PTY coverage for the F3 first-run setup wizard: on a fresh WORK_HOME, an
// interactive `work start` asks for the workspace root and a repository
// search root, each with a purpose line, before any other step; both persist
// in a single write; a configured install shows neither prompt again
// (contracts/cli-work-start.md §First-run setup, research R21, quickstart S13).
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"

	"github.com/gustaborges/work/internal/config"
)

func TestStartFirstRunSetupPromptsOnceThenNeverAgain(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "payments")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "start", "payments")
	c.expect("Workspace root")
	if !strings.Contains(c.snapshot(), "worktrees") {
		t.Errorf("workspace prompt missing a purpose line:\n%s", c.snapshot())
	}
	c.send("\x15" + ws + "\r") // ctrl-u clears the pre-filled suggested default

	c.expect("Repository search root")
	if !strings.Contains(c.snapshot(), "find them") {
		t.Errorf("search-root prompt missing a purpose line:\n%s", c.snapshot())
	}
	c.send(root + "\r")

	// Resolution then finds $root/payments and the wizard continues at
	// prefix/slug/base as in the plain single-match flow (quickstart S13).
	c.expect("Slug")
	c.send("guided\r")
	c.expect("Base branch")
	c.send("\r")
	c.expect("Create Work")
	c.send("y")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("first-run start exited %d, want 0\n%s", code, c.screen())
	}

	cfgData, err := os.ReadFile(filepath.Join(workHome, "config", "work.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfgData), ws) || !strings.Contains(string(cfgData), root) {
		t.Errorf("config missing workspace/root:\n%s", cfgData)
	}

	wt := filepath.Join(ws, "in-progress", "payments_guided", "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}

	// A second run against the same (now-configured) home skips both prompts.
	c2 := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "start", "ledger")
	makeRepo(t, filepath.Join(root, "ledger"))
	c2.expect("Slug")
	if strings.Contains(c2.snapshot(), "Workspace root") || strings.Contains(c2.snapshot(), "Repository search root") {
		t.Errorf("first-run prompts reappeared on a configured install:\n%s", c2.snapshot())
	}
	c2.send("second\r")
	c2.expect("Base branch")
	c2.send("\r")
	c2.expect("Create Work")
	c2.send("y")
	c2.expect("work: created ")
	if code := c2.wait(); code != 0 {
		t.Fatalf("second start exited %d, want 0\n%s", code, c2.screen())
	}
}

func TestStartFirstRunSetupOverlapRejectedInFrame(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)
	ws := filepath.Join(homeDir, "ws")
	root := filepath.Join(homeDir, "src")
	makeRepo(t, filepath.Join(root, "payments"))

	c := newConsole(t, bin, env, "start", "payments")
	c.expect("Workspace root")
	c.send("\x15" + ws + "\r")

	c.expect("Repository search root")
	c.send(filepath.Join(ws, "in-progress") + "\r")
	c.expect("overlaps")
	c.expect("Repository search root") // re-promptable, same step
	c.send("\x15" + root + "\r")

	c.expect("Slug")
	c.send("s\r")
	c.expect("Base branch")
	c.send("\r")
	c.expect("Create Work")
	c.send("y")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("exited %d, want 0\n%s", code, c.screen())
	}
}

func TestStartFirstRunSetupCancelPersistsNothing(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := filepath.Join(homeDir, "ws")

	// bootstrap.EnsureSeed itself writes config/work.json on first run (to
	// seed the default policy — unrelated to this wizard), so the assertion
	// here is on the workspace/root *fields*, not the file's mere existence.
	assertUnconfigured := func() {
		t.Helper()
		cfg, err := config.Load(filepath.Join(workHome, "config", "work.json"))
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		if cfg.Workspace != "" || len(cfg.RepositoryRoots) != 0 {
			t.Errorf("cancelling persisted setup: workspace=%q roots=%v", cfg.Workspace, cfg.RepositoryRoots)
		}
	}

	c := newConsole(t, bin, env, "start", "payments")
	c.expect("Workspace root")
	c.send("\x03") // Ctrl-C
	if code := c.wait(); code != 20 {
		t.Fatalf("Ctrl-C at workspace root exited %d, want 20\n%s", code, c.screen())
	}
	assertUnconfigured()

	c2 := newConsole(t, bin, env, "start", "payments")
	c2.expect("Workspace root")
	c2.send("\x15" + ws + "\r")
	c2.expect("Repository search root")
	c2.send("\x03") // Ctrl-C
	if code := c2.wait(); code != 20 {
		t.Fatalf("Ctrl-C at search root exited %d, want 20\n%s", code, c2.screen())
	}
	assertUnconfigured()
}
