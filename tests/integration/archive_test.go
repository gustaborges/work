//go:build unix

// Interactive (pty) coverage for `work archive`: the multi-select picker with
// its confirmation view destroys exactly the chosen worktrees and leaves the
// rest untouched, and an explicit id still has to pass the confirmation.
// Covers US2 #1, #2, #3; SC-003, SC-004; FR-009, FR-010, FR-012, FR-014.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/work"
)

func snapStatus(t *testing.T, snapshot string) string {
	t.Helper()
	st, err := work.Read(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return st.Work.Status
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	out, _ := exec.Command("git", "-C", repo, "branch", "--list", branch).CombinedOutput()
	return strings.Contains(string(out), branch)
}

func TestArchiveMultiSelectAndConfirm(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "alpha", "bravo", "charlie")
	// Recency order, most recent first: charlie, bravo, alpha.

	inProgress := filepath.Join(ws, "in-progress")
	stamp := time.Now().Format("20060102")

	// First run: check charlie and alpha, reach the confirmation view, then
	// cancel — nothing must change before the confirm (SC-004).
	c := newConsole(t, bin, env, "archive")
	c.expect("Archive Works")
	c.expect("demo  charlie")
	c.send(" ")            // check charlie (cursor at row 0)
	c.send("\x1b[B\x1b[B") // down to alpha
	c.send(" ")            // check alpha
	c.send("\r")           // -> confirmation view
	c.expect("2 worktree(s) will be destroyed")
	c.expect("Branches are kept")
	c.send("\x03") // Ctrl-C: cancel the whole operation
	if code := c.wait(); code != 20 {
		t.Fatalf("cancelling archive exited %d, want 20", code)
	}
	for _, slug := range []string{"alpha", "bravo", "charlie"} {
		if _, err := os.Stat(filepath.Join(inProgress, "demo_"+slug, "worktree")); err != nil {
			t.Fatalf("cancelled archive removed %s worktree: %v", slug, err)
		}
	}

	// Second run: check charlie and alpha, confirm.
	c2 := newConsole(t, bin, env, "archive")
	c2.expect("demo  charlie")
	c2.send(" ")
	c2.send("\x1b[B\x1b[B")
	c2.send(" ")
	c2.send("\r")
	c2.expect("2 worktree(s) will be destroyed")
	c2.send("\r") // confirm
	c2.expect("work: archived 2 of 2")
	if code := c2.wait(); code != 0 {
		t.Fatalf("archive exited %d, want 0", code)
	}

	// alpha and charlie are archived; bravo is untouched.
	for _, slug := range []string{"alpha", "charlie"} {
		if _, err := os.Stat(filepath.Join(inProgress, "demo_"+slug)); !os.IsNotExist(err) {
			t.Errorf("%s in-progress dir survived: %v", slug, err)
		}
		archived := filepath.Join(ws, "archived", stamp+"-demo_"+slug)
		if got := snapStatus(t, filepath.Join(archived, "work-state.json")); got != "archived" {
			t.Errorf("%s archived snapshot status = %q", slug, got)
		}
		if _, err := os.Stat(filepath.Join(archived, "worktree")); !os.IsNotExist(err) {
			t.Errorf("%s worktree survived under archived/", slug)
		}
		if !branchExists(t, repo, slug) {
			t.Errorf("%s branch was deleted by archiving", slug)
		}
	}
	if got := snapStatus(t, filepath.Join(inProgress, "demo_bravo", "work-state.json")); got != "in-progress" {
		t.Errorf("bravo status = %q, want in-progress", got)
	}

	// A later resume no longer lists the archived Works.
	c3 := newConsole(t, bin, env, "resume")
	c3.expect("demo  bravo")
	s := c3.snapshot()
	if strings.Contains(s, "demo  alpha") || strings.Contains(s, "demo  charlie") {
		t.Errorf("resume still lists an archived Work:\n%s", s)
	}
	c3.send("q")
	c3.wait()
}

func TestArchiveExplicitTargetStillConfirms(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "solo")

	st, err := work.Read(filepath.Join(ws, "in-progress", "demo_solo", "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Explicit id: no multi-select list, but the confirmation is still shown.
	c := newConsole(t, bin, env, "archive", st.Work.ID)
	c.expect("worktree(s) will be destroyed")
	if s := c.snapshot(); strings.Contains(s, "[ ]") {
		t.Errorf("explicit archive rendered the multi-select list:\n%s", s)
	}
	c.send("n") // decline
	if code := c.wait(); code != 20 {
		t.Fatalf("declining the confirmation exited %d, want 20", code)
	}
	if got := snapStatus(t, filepath.Join(ws, "in-progress", "demo_solo", "work-state.json")); got != "in-progress" {
		t.Errorf("declined archive changed the snapshot: status = %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "demo_solo", "worktree")); err != nil {
		t.Errorf("declined archive removed the worktree: %v", err)
	}
}

func TestArchiveDirtyExplicitAcknowledgement(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)
	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "solo")
	dir := filepath.Join(ws, "in-progress", "demo_solo")
	if err := os.WriteFile(filepath.Join(dir, "worktree", "UNTRACKED"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := work.Read(filepath.Join(dir, "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Neither --yes nor --force-dirty is needed after both TUI confirmations.
	c := newConsole(t, bin, env, "archive", st.Work.ID)
	c.expect("1 worktree(s) will be destroyed")
	c.send("y")
	c.expect("Archive anyway")
	c.send("y")
	c.expect("work: archived 1 of 1")
	if code := c.wait(); code != 0 {
		t.Fatalf("archive exited %d, want 0", code)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("acknowledged dirty Work stayed active: %v", err)
	}
	archived := filepath.Join(ws, "archived", time.Now().Format("20060102")+"-demo_solo")
	if got := snapStatus(t, filepath.Join(archived, "work-state.json")); got != "archived" {
		t.Errorf("snapshot status = %q, want archived", got)
	}
	if _, err := os.Stat(filepath.Join(archived, "worktree")); !os.IsNotExist(err) {
		t.Errorf("acknowledged dirty worktree survived: %v", err)
	}
	if !branchExists(t, repo, "solo") {
		t.Error("archiving deleted the branch")
	}
}

func TestArchiveDirtyPickerAcknowledgementsArePerWork(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)
	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "clean", "late", "declined", "accepted")
	inProgress := filepath.Join(ws, "in-progress")
	dirtyPath := func(slug string) string {
		return filepath.Join(inProgress, "demo_"+slug, "worktree", "UNTRACKED")
	}
	for _, slug := range []string{"accepted", "declined"} {
		if err := os.WriteFile(dirtyPath(slug), []byte(slug), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]string{}
	for _, slug := range []string{"declined", "late"} {
		b, err := os.ReadFile(filepath.Join(inProgress, "demo_"+slug, "work-state.json"))
		if err != nil {
			t.Fatal(err)
		}
		before[slug] = string(b)
	}

	c := newConsole(t, bin, env, "archive")
	c.expect("demo  accepted")
	// Select all four in recency order: accepted, declined, late, clean.
	c.send(" \x1b[B \x1b[B \x1b[B ")
	c.send("\r")
	c.expect("4 worktree(s) will be destroyed")
	c.send("y")
	c.expect("Archive anyway")
	c.expect("accepted  (accepted)")
	// The dirty preflight has finished. A different Work becoming dirty now
	// must not inherit the acknowledgement for accepted.
	if err := os.WriteFile(dirtyPath("late"), []byte("late"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.send("y")
	c.expect("declined  (declined)")
	c.send("n")
	c.expect("work: archived 2 of 4")
	if code := c.wait(); code != 0 {
		t.Fatalf("archive exited %d, want 0", code)
	}
	for _, slug := range []string{"accepted", "clean"} {
		if _, err := os.Stat(filepath.Join(inProgress, "demo_"+slug)); !os.IsNotExist(err) {
			t.Errorf("%s stayed active: %v", slug, err)
		}
		archived := filepath.Join(ws, "archived", time.Now().Format("20060102")+"-demo_"+slug)
		if got := snapStatus(t, filepath.Join(archived, "work-state.json")); got != "archived" {
			t.Errorf("%s status = %q, want archived", slug, got)
		}
		if _, err := os.Stat(filepath.Join(archived, "worktree")); !os.IsNotExist(err) {
			t.Errorf("%s worktree survived: %v", slug, err)
		}
		if !branchExists(t, repo, slug) {
			t.Errorf("archiving deleted %s branch", slug)
		}
	}
	for _, slug := range []string{"declined", "late"} {
		b, err := os.ReadFile(filepath.Join(inProgress, "demo_"+slug, "work-state.json"))
		if err != nil || string(b) != before[slug] {
			t.Errorf("%s snapshot changed: %v", slug, err)
		}
		if b, err := os.ReadFile(dirtyPath(slug)); err != nil || string(b) != slug {
			t.Errorf("%s unacknowledged changes were lost: %q, %v", slug, b, err)
		}
	}
}
