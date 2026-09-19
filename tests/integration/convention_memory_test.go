//go:build unix

// PTY coverage for the generalized branch-convention step (F4 Phase 7, US5;
// FR-025, FR-026, ADR-0011): with specific-starter's "gitflow" installed
// alongside the reference package's "freeform" (2 enabled), a fresh
// repository's first fork-mode `work start` shows a Convention step once and
// memoizes the choice; a second clone of the same repository shows no step
// and reuses it, reported by `work convention show` (quickstart S13).
package integration

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConventionMemoizedOnFirstUseAndReusedFromASecondClone(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	installPluginFixture(t, bin, env, "specific-starter")

	// The ADR-0011 identity's first layer is the "origin" remote's fetch URL:
	// two independent clones of the same canonical repository share an
	// identity only when both actually carry that same URL (research R13) —
	// cloning "repo" itself would instead give the second clone an origin
	// pointing at "repo"'s own path, a different identity. So both "repo"
	// (used for `work start`) and "clone" (used for `work convention show`)
	// are cloned from one shared canonical origin.
	canonical := filepath.Join(homeDir, "canonical")
	makeRepo(t, canonical)

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "demo")
	if out, err := exec.Command("git", "clone", "-q", canonical, repo).CombinedOutput(); err != nil {
		t.Fatalf("git clone canonical -> repo: %v\n%s", err, out)
	}
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	// First use: base/prefix/slug supplied by flag, but 2 conventions are
	// enabled and unmemoized for this repository — the Convention step must
	// appear exactly once, before the (also generalized) prefix choice.
	c := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "s", "--yes")
	c.expect("Convention")
	c.expect("gitflow")
	c.expect("freeform")
	c.send("\x1b[B") // move off gitflow (listed first) onto freeform
	c.send("\r")
	// freeform has a single prefix ("{slug}"): the Prefix step collapses
	// silently, straight to the (--yes-skipped) confirmation and creation.
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("first fork-mode start exited %d, want 0\n%s", code, c.screen())
	}

	// A second clone of the same canonical repository (same "origin" fetch
	// URL, hence the same ADR-0011 identity): `work convention show` reports
	// the memoized choice without any `work start` having touched the clone.
	clone := filepath.Join(homeDir, "demo-clone-2")
	if out, err := exec.Command("git", "clone", "-q", canonical, clone).CombinedOutput(); err != nil {
		t.Fatalf("git clone canonical -> clone: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "convention", "show")
	cmd.Env = env
	cmd.Dir = clone
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convention show: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "work: convention freeform") {
		t.Errorf("convention show (clone) = %q, want the first clone's memoized choice", out)
	}

	// A second fork-mode `work start` against the same repository (from the
	// original clone) sees the memoized choice and shows no Convention step.
	// The workspace root is already configured from the first run, so
	// --workspace is omitted here (repeating it would be rejected).
	c2 := newConsole(t, bin, env, "start", repo,
		"--base", "main", "--slug", "s2", "--prefix", "{slug}", "--yes")
	c2.expect("work: created ")
	if code := c2.wait(); code != 0 {
		t.Fatalf("second start exited %d, want 0\n%s", code, c2.screen())
	}
	// "Convention" itself is not checked: the test's own name (embedded in
	// the temp worktree path this same screen legitimately prints) contains
	// that word too. The select-step help line is a step-agnostic signal
	// that no interactive step rendered at all.
	if screen := c2.screen(); strings.Contains(screen, "move · enter select") {
		t.Errorf("a memoized repository must show no interactive step at all:\n%s", screen)
	}
}
