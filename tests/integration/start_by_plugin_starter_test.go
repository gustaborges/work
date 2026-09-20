//go:build unix

// PTY coverage for `work start <plugin-specific-argument>` (F4 Phase 3, US2):
// with a plugin-installed specific Starter matching the argument, the full
// new-Work journey completes exactly as an F1 direct-path/name resolution
// would — the specific Starter was invoked (not the reference fallback), and
// work.starter names it. specific-starter's start_modes/base_branch are
// consumed starting Phase 5 (US3): TestStartByPluginStarterFullJourney below
// selects "fork" at the resulting Mode step to keep exercising the ordinary
// new-Work journey end to end; contribution/fork-specific coverage lives in
// start_modes_test.go.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// installPluginFixture runs `work plugin install <fixture-dir>` to
// completion (not through the pty — install itself is non-interactive) so a
// later `work start` pty session sees it already registered.
func installPluginFixture(t *testing.T, bin string, env []string, fixture string) {
	t.Helper()
	dir := fixtures.Prepare(t, fixture)
	cmd := exec.Command(bin, "plugin", "install", dir)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin install %s: %v\n%s", fixture, err, out)
	}
}

// TestStartByPluginStarterFullJourney: specific-starter's pattern
// (^demo-pr-1$) matches the SOURCE argument directly — no fallback, no
// selection step — and the resulting Work is indistinguishable in shape from
// an F1 direct-path/name creation, except work.starter naming the plugin
// Starter that actually ran (quickstart S7, SC-002) and work.start_mode
// reflecting the chosen mode (Phase 5 consumes start_modes/base_branch:
// selecting "fork" at the Mode step runs the identical new-Work journey,
// using the Starter's base_branch with no base-branch prompt, FR-024).
func TestStartByPluginStarterFullJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	installPluginFixture(t, bin, env, "specific-starter")

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "demo-pr-1")
	makeRepo(t, repo)
	gitIn(t, repo, "branch", "feature/source-branch")
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	c := newConsole(t, bin, env, "start", "demo-pr-1",
		"--workspace", ws, "--slug", "guided", "--yes")
	c.expect("Mode")
	c.expect("contribution")
	c.expect("fork")
	c.send("\x1b[B") // the fixture lists "contribution" first; move down to "fork"
	c.send("\r")
	// specific-starter's manifest declares a "gitflow" convention alongside
	// the reference package's "freeform" (2 enabled): a fresh repository's
	// first fork-mode use requires an explicit Convention choice (F4, US5).
	// Which one is irrelevant to this test, so accept whichever is focused,
	// then whichever prefix that convention offers.
	c.expect("Convention")
	c.send("\r")
	c.expect("Branch prefix")
	c.send("\r")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("start via plugin starter exited %d, want 0\n%s", code, c.screen())
	}

	// The directory name embeds the derived branch, which depends on which of
	// the 2 enabled conventions (freeform/gitflow) the Convention step's
	// default focus accepted — glob for it rather than assuming one.
	dir := globOneInProgress(t, ws, "demo-pr-1_*guided")
	wt := filepath.Join(dir, "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "work-state.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var st struct {
		Schema int `json:"schema"`
		Work   struct {
			StartMode string `json:"start_mode"`
			Starter   string `json:"starter"`
		} `json:"work"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if st.Schema != 3 {
		t.Errorf("schema = %d, want 3", st.Schema)
	}
	if st.Work.StartMode != "fork" {
		t.Errorf("start_mode = %q, want %q", st.Work.StartMode, "fork")
	}
	if st.Work.Starter != "specific-starter" {
		t.Errorf("starter = %q, want the plugin Starter's bare name (no collision to qualify)", st.Work.Starter)
	}
}

// TestStartFallsBackWhenOnlyReferencePackageInstalled is the F3-regression
// half of US2 (T043): with no specific Starter installed, Match always falls
// back to the reference package — `work start <path>` stays byte-identical
// to F3.
func TestStartFallsBackWhenOnlyReferencePackageInstalled(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src", "payments")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	c := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	answerFirstRunRootPrompt(c, t.TempDir())
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("exited %d, want 0\n%s", code, c.screen())
	}

	data, err := os.ReadFile(filepath.Join(ws, "in-progress", "payments_guided", "work-state.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var st struct {
		Work struct {
			Starter string `json:"starter"`
		} `json:"work"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if st.Work.Starter != "local-path-starter" {
		t.Errorf("starter = %q, want the reference fallback", st.Work.Starter)
	}
}
