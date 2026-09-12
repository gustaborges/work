//go:build unix

// PTY coverage for `work start <name>` (F3 Phase 3, US1): a single matching
// local clone resolves silently through the Repository Resolution Policy —
// no path prompt, no picker — and the wizard continues the identical F1
// creation journey. The disambiguation picker (>= 2 matches) lands in Phase 4.
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"

	"github.com/gustaborges/work/internal/config"
)

// writeRepositoryRoots pre-configures repository_roots in workHome's
// config/work.json — Phase 3 has no `work repository root add` CLI yet.
func writeRepositoryRoots(t *testing.T, workHome string, roots ...string) {
	t.Helper()
	cfgPath := filepath.Join(workHome, "config", "work.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.RepositoryRoots = roots
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
}

// TestStartByNameSingleMatchPTY: `work start payments` in an 80x24 PTY
// resolves the one clone under the configured search root silently — no
// "Local repository path" prompt, no selector — and the wizard continues at
// the slug/base/confirm steps exactly as F1 (quickstart S3).
func TestStartByNameSingleMatchPTY(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "payments")
	makeRepo(t, repo)
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "start", "payments",
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("start by name exited %d, want 0\n%s", code, c.screen())
	}

	screen := c.screen()
	if strings.Contains(screen, "Local repository path") {
		t.Errorf("a single match must not show the path prompt:\n%s", screen)
	}

	wt := filepath.Join(ws, "in-progress", "payments_guided", "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}
}

// TestStartByNameAmbiguousPickerPTY: two matching clones — a Repository
// select step appears with both candidates (primary = absolute path); picking
// one materialises the Work against exactly that repository and not the
// other, and the chain is not resumed (quickstart S5).
func TestStartByNameAmbiguousPickerPTY(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	root := filepath.Join(homeDir, "src")
	r1 := filepath.Join(root, "primary", "payments")
	r2 := filepath.Join(root, "mirror", "payments")
	makeRepo(t, r1)
	makeRepo(t, r2)
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, "start", "payments",
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	c.expect("Repository")
	// The primary line is the absolute path, which a long test tempdir (macOS
	// runners in particular) can truncate past the "primary"/"mirror" segment
	// that distinguishes the two candidates — so match on the short secondary
	// (parent-dir) line instead of the full path.
	c.expect("mirror")
	c.expect("primary")
	c.send("\x1b[B") // move focus to the second row
	c.send("\r")     // accept
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("ambiguous start exited %d, want 0\n%s", code, c.screen())
	}

	// Exactly one candidate repository received the new branch — the chosen
	// one — and the chain was not resumed against the other.
	r1Has := gitIn(t, r1, "branch", "--list", "guided") != ""
	r2Has := gitIn(t, r2, "branch", "--list", "guided") != ""
	if r1Has == r2Has {
		t.Fatalf("want exactly one candidate to receive the branch: r1=%v r2=%v", r1Has, r2Has)
	}

	var chosen string
	if r1Has {
		chosen = r1
	} else {
		chosen = r2
	}
	wt := filepath.Join(ws, "in-progress", "payments_guided", "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}
	if tip := gitIn(t, wt, "rev-parse", "HEAD"); tip != gitIn(t, chosen, "rev-parse", "HEAD") {
		t.Errorf("materialized worktree does not track the chosen repository %s", chosen)
	}
}

// TestStartByNameAmbiguousPickerGeometry: the Repository selector holds a
// bounded, column-stable geometry at 40x10, 80x24, and 160x50 (SC-003, SC-004
// parity with the other F2.5 selectors).
func TestStartByNameAmbiguousPickerGeometry(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)

	for _, sz := range []pty.Winsize{{Rows: 10, Cols: 40}, {Rows: 24, Cols: 80}, {Rows: 50, Cols: 160}} {
		t.Run(fmt.Sprintf("%dx%d", sz.Cols, sz.Rows), func(t *testing.T) {
			env, homeDir, workHome := ptyEnv(t)
			root := filepath.Join(homeDir, "src")
			r1 := filepath.Join(root, "primary", "payments")
			r2 := filepath.Join(root, "mirror", "payments")
			makeRepo(t, r1)
			makeRepo(t, r2)
			writeRepositoryRoots(t, workHome, root)
			ws := filepath.Join(homeDir, "ws")

			c := newConsoleSize(t, sz, bin, env, "start", "payments",
				"--workspace", ws, "--base", "main", "--slug", "g", "--prefix", "{slug}", "--yes")
			c.expect("Repository")
			settle()
			assertNoLineExceeds(t, c.screen(), int(sz.Cols))

			c.send("\r")
			if code := c.wait(); code != 0 {
				t.Fatalf("exited %d, want 0\n%s", code, c.screen())
			}
		})
	}
}

// TestStartPathAndNameProduceIdenticalWork: `work start <path>` and
// `work start <name>` for the same repository produce identical
// work-state.json fields (normalising the fields that legitimately differ:
// id, slug, branch, worktree/dir paths, timestamps) — SC-008, quickstart S4.
func TestStartPathAndNameProduceIdenticalWork(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "payments")
	makeRepo(t, repo)
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	byPath := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "viapath", "--prefix", "{slug}", "--yes")
	byPath.expect("work: created ")
	if code := byPath.wait(); code != 0 {
		t.Fatalf("start by path exited %d, want 0", code)
	}

	byName := newConsole(t, bin, env, "start", "payments",
		"--base", "main", "--slug", "byname", "--prefix", "{slug}", "--yes")
	byName.expect("work: created ")
	if code := byName.wait(); code != 0 {
		t.Fatalf("start by name exited %d, want 0\n%s", code, byName.screen())
	}

	pathState := readState(t, filepath.Join(ws, "in-progress", "payments_viapath", "work-state.json"))
	nameState := readState(t, filepath.Join(ws, "in-progress", "payments_byname", "work-state.json"))

	if pathState.Schema != nameState.Schema {
		t.Errorf("schema differs: by-path=%d, by-name=%d", pathState.Schema, nameState.Schema)
	}
	pw, nw := pathState.Work, nameState.Work
	for name, pair := range map[string][2]string{
		"status":            {pw.Status, nw.Status},
		"start_mode":        {pw.StartMode, nw.StartMode},
		"base_branch":       {pw.BaseBranch, nw.BaseBranch},
		"branch_convention": {pw.BranchConvention, nw.BranchConvention},
		"starter":           {pw.Starter, nw.Starter},
	} {
		if pair[0] != pair[1] {
			t.Errorf("field %q differs: by-path=%q, by-name=%q", name, pair[0], pair[1])
		}
	}
}

type snapshotState struct {
	Schema int `json:"schema"`
	Work   struct {
		Status           string `json:"status"`
		StartMode        string `json:"start_mode"`
		Starter          string `json:"starter"`
		BaseBranch       string `json:"base_branch"`
		BranchConvention string `json:"branch_convention"`
	} `json:"work"`
}

func readState(t *testing.T, path string) snapshotState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var s snapshotState
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return s
}
