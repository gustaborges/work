//go:build unix

// PTY coverage for start modes (F4 Phase 5, US3): with specific-starter
// returning start_modes:["contribution","fork"] and a base_branch, the Mode
// step offers exactly those two; fork runs the full slug/prefix/base
// sequence (skipping the base-branch prompt, since the Starter already
// supplied one); contribution skips slug/prefix/base entirely and checks out
// the existing branch directly, persisting no branch_convention (quickstart
// S8, S9).
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// startModesFixture installs specific-starter and prepares a source repo with
// the branch its response names as base_branch ("feature/source-branch"),
// reachable via a configured repository root under "demo-pr-1" — the token
// the fixture's pattern matches.
func startModesFixture(t *testing.T) (bin string, env []string, repo, ws string) {
	t.Helper()
	needSeed(t)
	bin = buildWorkBin(t)
	var homeDir, workHome string
	env, homeDir, workHome = ptyEnv(t)

	installPluginFixture(t, bin, env, "specific-starter")

	root := filepath.Join(homeDir, "src")
	repo = filepath.Join(root, "demo-pr-1")
	makeRepo(t, repo)
	gitIn(t, repo, "branch", "feature/source-branch")
	writeRepositoryRoots(t, workHome, root)
	ws = filepath.Join(homeDir, "ws")
	return bin, env, repo, ws
}

func TestStartModeForkRunsFullJourney(t *testing.T) {
	bin, env, _, ws := startModesFixture(t)

	c := newConsole(t, bin, env, "start", "demo-pr-1",
		"--workspace", ws, "--slug", "forked", "--prefix", "{slug}", "--yes")
	c.expect("Mode")
	c.expect("contribution")
	c.expect("fork")
	c.send("\x1b[B") // the fixture lists "contribution" first; move down to "fork"
	c.send("\r")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("fork mode exited %d, want 0\n%s", code, c.screen())
	}

	screen := c.screen()
	if strings.Contains(screen, "Base branch") {
		t.Errorf("fork mode must not prompt for a base branch when the Starter supplied one:\n%s", screen)
	}

	dir := filepath.Join(ws, "in-progress", "demo-pr-1_forked")
	st := readSnapshot(t, filepath.Join(dir, "work-state.json"))
	if st.Schema != 3 {
		t.Errorf("schema = %d, want 3", st.Schema)
	}
	if st.Work.StartMode != "fork" {
		t.Errorf("start_mode = %q, want %q", st.Work.StartMode, "fork")
	}
	if st.Work.BaseBranch != "feature/source-branch" {
		t.Errorf("base_branch = %q, want the Starter-supplied branch", st.Work.BaseBranch)
	}
	if st.Work.BranchConvention == "" {
		t.Errorf("fork mode must persist a branch_convention, got none")
	}
	if st.Work.Slug != "forked" {
		t.Errorf("slug = %q, want forked", st.Work.Slug)
	}
}

func TestStartModeContributionSkipsSlugConventionPrefix(t *testing.T) {
	bin, env, repo, ws := startModesFixture(t)

	c := newConsole(t, bin, env, "start", "demo-pr-1", "--workspace", ws, "--yes")
	c.expect("Mode")
	c.expect("contribution")
	c.send("\r") // "contribution" is the first (focused) option
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("contribution mode exited %d, want 0\n%s", code, c.screen())
	}

	// "Slug" itself is checked via its unique field description rather than
	// the bare word, since the materialized worktree path this same screen
	// prints legitimately embeds this test's own name (which contains
	// "Slug").
	screen := c.screen()
	for _, unwanted := range []string{"a short identifier for this Work", "Branch prefix", "Base branch"} {
		if strings.Contains(screen, unwanted) {
			t.Errorf("contribution mode must not show a %q step:\n%s", unwanted, screen)
		}
	}

	dir := filepath.Join(ws, "in-progress", "demo-pr-1_feature-source-branch")
	st := readSnapshot(t, filepath.Join(dir, "work-state.json"))
	if st.Work.StartMode != "contribution" {
		t.Errorf("start_mode = %q, want %q", st.Work.StartMode, "contribution")
	}
	if st.Work.Slug != "" {
		t.Errorf("contribution mode must persist no slug, got %q", st.Work.Slug)
	}
	if st.Work.BranchConvention != "" {
		t.Errorf("contribution mode must persist no branch_convention, got %q", st.Work.BranchConvention)
	}
	if st.Work.Branch != "feature/source-branch" {
		t.Errorf("branch = %q, want the Starter-resolved branch", st.Work.Branch)
	}
	if st.Work.BaseBranch != st.Work.Branch {
		t.Errorf("base_branch = %q, want it to equal branch %q", st.Work.BaseBranch, st.Work.Branch)
	}

	wt := filepath.Join(dir, "worktree")
	if head := gitIn(t, wt, "rev-parse", "--abbrev-ref", "HEAD"); head != "feature/source-branch" {
		t.Errorf("checked out branch = %q, want feature/source-branch", head)
	}

	// The branch is the repository's own pre-existing one, not created by this
	// Work — it must still be visible from the source repo itself.
	if out, err := exec.Command("git", "-C", repo, "branch", "--list", "feature/source-branch").CombinedOutput(); err != nil || len(out) == 0 {
		t.Errorf("source branch feature/source-branch missing after contribution create: %s (%v)", out, err)
	}
}

// TestStartModeContributionCancelledLeavesNoTraceAndBranchIntact (T045):
// declining the confirmation prompt in contribution mode leaves no
// worktree/dir/snapshot/index entry, and the Starter-resolved branch is still
// present in the source repository (SC-009, research R12).
func TestStartModeContributionCancelledLeavesNoTraceAndBranchIntact(t *testing.T) {
	bin, env, repo, ws := startModesFixture(t)

	c := newConsole(t, bin, env, "start", "demo-pr-1", "--workspace", ws)
	c.expect("Mode")
	c.expect("contribution")
	c.send("\r")
	c.expect("Create Work")
	c.send("n") // decline
	if code := c.wait(); code != 20 {
		t.Fatalf("declining contribution's confirm exited %d, want 20\n%s", code, c.screen())
	}

	dir := filepath.Join(ws, "in-progress", "demo-pr-1_feature-source-branch")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("declined contribution create left a Work directory: %v", err)
	}

	if out, err := exec.Command("git", "-C", repo, "branch", "--list", "feature/source-branch").CombinedOutput(); err != nil || len(out) == 0 {
		t.Errorf("cancelling must leave the pre-existing branch intact: %s (%v)", out, err)
	}
}

type snapshotStateFull struct {
	Schema int `json:"schema"`
	Work   struct {
		StartMode        string `json:"start_mode"`
		Starter          string `json:"starter"`
		Slug             string `json:"slug"`
		Branch           string `json:"branch"`
		BaseBranch       string `json:"base_branch"`
		BranchConvention string `json:"branch_convention"`
	} `json:"work"`
}

func readSnapshot(t *testing.T, path string) snapshotStateFull {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var s snapshotStateFull
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return s
}
