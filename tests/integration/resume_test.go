//go:build unix

// Interactive (pty) coverage for `work resume`: the recency picker lists Works
// most-recently-accessed first, selecting one repositions the session and moves
// it to the top of the list, and the canonical snapshot agrees with the index.
// Covers US1 #1, #2, #5; SC-001, SC-002, SC-008; FR-002, FR-003, FR-004.
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

// seedWorks runs `work start` non-interactively for each slug against repo, into
// ws, spacing calls a second apart so each last_accessed_at is distinct.
// --workspace is passed only on the first call (it is rejected once the root is
// persisted).
func seedWorks(t *testing.T, bin string, env []string, repo, ws string, slugs ...string) {
	t.Helper()
	for i, slug := range slugs {
		if i > 0 {
			time.Sleep(1100 * time.Millisecond)
		}
		args := []string{"start", repo, "--base", "main", "--slug", slug, "--prefix", "{slug}", "--yes"}
		if i == 0 {
			args = append(args, "--workspace", ws)
		}
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed %s: %v\n%s", slug, err, out)
		}
	}
}

// envWith returns env with key set to val, replacing any existing entry.
func envWith(env []string, key, val string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, key+"=") {
			out = append(out, e)
		}
	}
	return append(out, key+"="+val)
}

func snapAccessed(t *testing.T, dir string) string {
	t.Helper()
	st, err := work.Read(filepath.Join(dir, "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return st.Work.LastAccessedAt
}

func TestResumeOrderingAndReposition(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	seedWorks(t, bin, env, repo, ws, "alpha", "bravo", "charlie")
	time.Sleep(1100 * time.Millisecond)
	// Access order, most recent first: charlie, bravo, alpha.

	alphaDir := filepath.Join(ws, "in-progress", "demo_alpha")
	cdFile := filepath.Join(homeDir, "cdfile")
	runEnv := envWith(env, "WORK_CD_FILE", cdFile)

	// Interactive resume: the picker lists charlie, bravo, alpha top-to-bottom.
	c := newConsole(t, bin, runEnv, "resume")
	c.expect("Resume a Work")
	c.expect("demo  charlie")
	c.expect("just now • alpha")
	s := c.snapshot()
	posC, posB, posA := strings.Index(s, "demo  charlie"), strings.Index(s, "demo  bravo"), strings.Index(s, "demo  alpha")
	if !(posC < posB && posB < posA) {
		t.Fatalf("picker order is not charlie, bravo, alpha:\n%s", s)
	}

	// Select alpha (least recent): two rows down, Enter.
	c.send("\x1b[B\x1b[B\r")
	c.expect("work: resumed ")
	if code := c.wait(); code != 0 {
		t.Fatalf("resume exited %d, want 0", code)
	}
	full := c.snapshot()
	if !strings.Contains(full, "work: path "+alphaDir+string(os.PathSeparator)+"worktree") {
		t.Fatalf("stdout missing the alpha worktree path:\n%s", full)
	}

	// The shell hand-off wrote the alpha worktree path.
	got, err := os.ReadFile(cdFile)
	if err != nil {
		t.Fatalf("WORK_CD_FILE not written: %v", err)
	}
	if want := filepath.Join(alphaDir, "worktree"); strings.TrimSpace(string(got)) != want {
		t.Errorf("WORK_CD_FILE = %q, want %q", got, want)
	}

	// alpha is now schema 2 and the most recently accessed of the three.
	st, err := work.Read(filepath.Join(alphaDir, "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Schema != 2 {
		t.Errorf("alpha snapshot schema = %d, want 2", st.Schema)
	}
	aAcc := st.Work.LastAccessedAt
	if aAcc <= snapAccessed(t, filepath.Join(ws, "in-progress", "demo_bravo")) ||
		aAcc <= snapAccessed(t, filepath.Join(ws, "in-progress", "demo_charlie")) {
		t.Errorf("alpha last_accessed_at %q is not newer than bravo/charlie", aAcc)
	}

	// A second resume lists alpha, charlie, bravo.
	c2 := newConsole(t, bin, env, "resume")
	c2.expect("demo  alpha")
	s2 := c2.snapshot()
	p2A, p2C, p2B := strings.Index(s2, "demo  alpha"), strings.Index(s2, "demo  charlie"), strings.Index(s2, "demo  bravo")
	if !(p2A < p2C && p2C < p2B) {
		t.Fatalf("second picker order is not alpha, charlie, bravo:\n%s", s2)
	}
	c2.send("q")
	if code := c2.wait(); code != 20 {
		t.Fatalf("q at the picker exited %d, want 20 (cancelled)", code)
	}
}

// Cancelling the picker (q) exits 20 and changes nothing.
func TestResumePickerCancel(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "solo")

	before := snapAccessed(t, filepath.Join(ws, "in-progress", "demo_solo"))

	c := newConsole(t, bin, env, "resume")
	c.expect("demo  solo")
	c.send("q")
	if code := c.wait(); code != 20 {
		t.Fatalf("q at the picker exited %d, want 20", code)
	}
	if after := snapAccessed(t, filepath.Join(ws, "in-progress", "demo_solo")); after != before {
		t.Errorf("cancelled resume bumped last_accessed_at: %q -> %q", before, after)
	}
}
