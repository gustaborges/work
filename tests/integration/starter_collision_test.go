//go:build unix

// PTY and non-interactive coverage for Starter collision resolution (F4
// Phase 6, US4): two Starters matching the same argument never auto-resolve —
// an interactive present.Select names both before either runs; a
// non-interactive invocation fails starter-ambiguous (36) with no selector;
// no fallback and no match fails starter-not-matched (35) (quickstart S11,
// S12, ADR-0004).
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// installCollidingFixtures installs both specific-starter (pattern
// ^demo-pr-1$) and colliding-starter (pattern ^demo-pr-\d+$), which both
// match the literal argument "demo-pr-1".
func installCollidingFixtures(t *testing.T, bin string, env []string) {
	t.Helper()
	installPluginFixture(t, bin, env, "specific-starter")
	installPluginFixture(t, bin, env, "colliding-starter")
}

// TestStarterCollisionInteractiveSelection (T054): with both fixtures
// installed and matching, an interactive `work start demo-pr-1` shows a
// Starter selector — no ranking — before the SOURCE step's own validation
// completes; only the chosen component is invoked; running it twice may pick
// a different Starter each time (no memoization anywhere on disk, SC-006).
func TestStarterCollisionInteractiveSelection(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	installCollidingFixtures(t, bin, env)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "demo-pr-1")
	makeRepo(t, repo)
	// Both fixtures also return start_modes/base_branch (Phase 5); the
	// journey below picks "contribution" at the resulting Mode step, which
	// needs this branch to already exist in the source repository.
	gitIn(t, repo, "branch", "feature/source-branch")
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	// Order among colliding components is whatever registry.ByRole returns —
	// either way both names are asserted present and only one journey
	// completes.
	c := newConsole(t, bin, env, "start", "demo-pr-1", "--workspace", ws, "--yes")
	c.expect("Starter")
	c.expect("colliding-starter")
	c.expect("specific-starter")
	c.send("\r") // accept whichever is focused first
	c.expect("Mode")
	c.send("\r") // "contribution" is the first (focused) option
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("collision start exited %d, want 0\n%s", code, c.screen())
	}

	dir := filepath.Join(ws, "in-progress")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("in-progress entries = %v, want exactly one Work materialized", entries)
	}
	st := readSnapshot(t, filepath.Join(dir, entries[0].Name(), "work-state.json"))
	if st.Work.Starter != "specific-starter" && st.Work.Starter != "colliding-starter" {
		t.Errorf("starter = %q, want one of the two colliding components", st.Work.Starter)
	}
}

// TestStarterCollisionNonInteractiveFailsAmbiguous (T055): a non-interactive
// invocation whose explicit SOURCE matches both colliding fixtures fails
// starter-ambiguous (36), opening no selector and creating no Work (US4 AC4).
func TestStarterCollisionNonInteractiveFailsAmbiguous(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	installCollidingFixtures(t, bin, env)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "demo-pr-1")
	makeRepo(t, repo)
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	cmd := exec.Command(bin, "start", "demo-pr-1",
		"--workspace", ws, "--base", "main", "--slug", "picked", "--prefix", "{slug}", "--yes")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	if exitCode != 36 {
		t.Fatalf("exit = %d, want 36\n%s", exitCode, out)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "in-progress")); !os.IsNotExist(statErr) {
		t.Fatalf("non-interactive collision must create no Work: %v", statErr)
	}
}

// T056 (no pattern match and no fallback -> starter-not-matched, 35) is not
// reachable through the real `work start` CLI: bootstrap.EnsureSeed always
// installs the reference package's fallback Starter on first run, so a
// fallback is unconditionally present once any `work` command has run in a
// given WORK_HOME (quickstart S12 itself says to use a raw registry fixture
// for this case instead). It is already covered at that level by
// tests/contract/starter_match_test.go's "no match and no fallback is
// starter-not-matched (35)" case.
