//go:build unix

// PTY coverage for `work convention` with no subcommand — this codebase's
// first genuinely interactive hub (F4 Phase 7, US5; ADR-0011/FR-030, research
// R16): unlike `work plugin`/`work repository`, it is not deferred to F7.
package integration

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConventionHubShowsCurrentChangesAndReceiptsTheEquivalentCommand (T062):
// the hub shows the current choice (or "not set"), changing it persists
// immediately and prints the `work convention set <name>` receipt, and
// leaving it unchanged persists nothing (quickstart S14).
func TestConventionHubShowsCurrentChangesAndReceiptsTheEquivalentCommand(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	// specific-starter declares "gitflow" alongside the reference package's
	// "freeform" — the hub needs 2+ enabled conventions to offer a real
	// choice. Installed before any bootstrap, so the fixture's own
	// convention is registered first.
	installPluginFixture(t, bin, env, "specific-starter")

	repo := filepath.Join(homeDir, "src", "demo")
	makeRepo(t, repo)

	// First run: nothing memoized yet.
	c := newConsoleIn(t, repo, bin, env, "convention")
	c.expect("Convention")
	c.expect("current: not set")
	c.expect("(leave unchanged)")
	c.expect("gitflow")
	c.send("\x1b[B") // down from "(leave unchanged)" to the first named option
	c.send("\r")
	c.expect("work: convention set to gitflow")
	c.expect("work: (equivalent: `work convention set gitflow`)")
	if code := c.wait(); code != 0 {
		t.Fatalf("hub exited %d, want 0\n%s", code, c.screen())
	}

	assertConventionShow(t, bin, env, repo, "work: convention gitflow")

	// Second run: the current choice is shown and pre-marked; leaving it
	// unchanged (accepting the default "(leave unchanged)" option) persists
	// nothing and prints no receipt.
	c = newConsoleIn(t, repo, bin, env, "convention")
	c.expect("Convention")
	c.expect("current: gitflow")
	c.expect("gitflow (current)")
	c.send("\r") // "(leave unchanged)" is focused by default
	if code := c.wait(); code != 0 {
		t.Fatalf("hub exited %d, want 0\n%s", code, c.screen())
	}
	screen := c.screen()
	if strings.Contains(screen, "work: convention set to") {
		t.Errorf("leaving the choice unchanged must print no receipt:\n%s", screen)
	}
	assertConventionShow(t, bin, env, repo, "work: convention gitflow")

	// Third run: change to "freeform" (the second named option).
	c = newConsoleIn(t, repo, bin, env, "convention")
	c.expect("Convention")
	c.send("\x1b[B")
	c.send("\x1b[B")
	c.expect("freeform")
	c.send("\r")
	c.expect("work: convention set to freeform")
	c.expect("work: (equivalent: `work convention set freeform`)")
	if code := c.wait(); code != 0 {
		t.Fatalf("hub exited %d, want 0\n%s", code, c.screen())
	}
	assertConventionShow(t, bin, env, repo, "work: convention freeform")
}

// TestConventionHubNonInteractiveFailsUsageWithNoSelector (T062): a
// non-interactive bare `work convention` never opens the hub.
func TestConventionHubNonInteractiveFailsUsageWithNoSelector(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)
	repo := filepath.Join(homeDir, "src", "demo")
	makeRepo(t, repo)

	cmd := exec.Command(bin, "convention")
	cmd.Env = env
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected a non-zero exit, got err=%v\n%s", err, out)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("exit = %d, want 2\n%s", ee.ExitCode(), out)
	}
	if !strings.Contains(string(out), "usage:") {
		t.Errorf("stderr missing usage token: %s", out)
	}
}

// assertConventionShow runs `work convention show` non-interactively from dir
// and asserts its stdout contains want.
func assertConventionShow(t *testing.T, bin string, env []string, dir, want string) {
	t.Helper()
	cmd := exec.Command(bin, "convention", "show")
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convention show: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), want) {
		t.Errorf("convention show = %q, want to contain %q", out, want)
	}
}
