//go:build unix

// Interactive (pty) coverage for the guided journey: `work start` with no
// SOURCE prompts for the path and then converges to the same guarantees as the
// flag form. The bare `work` brand is covered by brand_test.go.
package integration

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/gustaborges/work/seed"
)

var (
	workBinOnce sync.Once
	workBinPath string
	workBinErr  error
)

// buildWorkBin compiles a real `work` binary once per test run; pty tests need
// a genuine process, not the in-process testscript entrypoint.
func buildWorkBin(t *testing.T) string {
	t.Helper()
	workBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "workbin")
		if err != nil {
			workBinErr = err
			return
		}
		bin := filepath.Join(dir, "work")
		cmd := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work")
		if out, err := cmd.CombinedOutput(); err != nil {
			workBinErr = errors.New(string(out))
			return
		}
		workBinPath = bin
	})
	if workBinErr != nil {
		t.Fatalf("build work binary: %v", workBinErr)
	}
	return workBinPath
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)|\x1b[()][A-B0-9]|\x1b[=>]")

// console drives a child process over a pty with a simple expect/send loop.
type console struct {
	t    *testing.T
	f    *os.File
	cmd  *exec.Cmd
	rows int
	cols int
	mu   sync.Mutex
	buf  strings.Builder // ANSI stripped, for substring expectations
	raw  []byte          // untouched, for screen reconstruction
	done chan struct{}
}

func newConsole(t *testing.T, bin string, env []string, args ...string) *console {
	return newConsoleSize(t, pty.Winsize{Rows: 40, Cols: 120}, bin, env, args...)
}

// newConsoleSize is newConsole with an explicit terminal size, for the geometry
// and selector-collapse checks that must run at a known rows×cols.
func newConsoleSize(t *testing.T, ws pty.Winsize, bin string, env []string, args ...string) *console {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	// A concrete window size is required: bubbletea renders nothing into a 0x0
	// terminal, which is what an unsized pty reports on Linux.
	f, err := pty.StartWithSize(cmd, &ws)
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	c := &console{t: t, f: f, cmd: cmd, rows: int(ws.Rows), cols: int(ws.Cols), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		b := make([]byte, 4096)
		for {
			n, err := f.Read(b)
			if n > 0 {
				c.mu.Lock()
				c.raw = append(c.raw, b[:n]...)
				c.buf.Write([]byte(ansiRe.ReplaceAllString(string(b[:n]), "")))
				c.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = c.f.Close()
		_ = cmd.Process.Kill()
	})
	return c
}

func (c *console) snapshot() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// screen replays the raw pty stream through a tiny terminal emulator and
// returns what a real terminal would actually show: the scrollback above the
// viewport plus the current grid, with overwritten and erased content gone.
// This is what makes "no rejected value survives" assertable — the ANSI-
// stripped snapshot still contains every historical repaint.
func (c *console) screen() string {
	c.mu.Lock()
	raw := append([]byte(nil), c.raw...)
	c.mu.Unlock()
	v := newVT(c.rows, c.cols)
	v.write(raw)
	return v.String()
}

// expect waits until sub appears in the output, failing the test on timeout.
func (c *console) expect(sub string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(c.snapshot(), sub) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("timed out waiting for %q\n--- output ---\n%s", sub, c.snapshot())
}

func (c *console) send(s string) {
	c.t.Helper()
	if _, err := io.WriteString(c.f, s); err != nil {
		c.t.Fatalf("write %q: %v", s, err)
	}
}

// wait returns the child's exit code once it has exited.
func (c *console) wait() int {
	c.t.Helper()
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
		c.t.Fatalf("timed out waiting for exit\n--- output ---\n%s", c.snapshot())
	}
	err := c.cmd.Wait()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	c.t.Fatalf("wait: %v", err)
	return -1
}

// ptyEnv builds an isolated environment: scratch HOME and WORK_HOME, a git
// identity, and no global/system git config.
func ptyEnv(t *testing.T) (env []string, home, workHome string) {
	t.Helper()
	base := t.TempDir()
	// macOS puts TempDir under /var/folders, a symlink to /private/var/folders.
	// `work` normalizes the repo path (EvalSymlinks) before it reaches a receipt,
	// so resolve here too or the pty screen assertions compare unequal spellings
	// of the same directory.
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	home = filepath.Join(base, "home")
	workHome = filepath.Join(base, "dothome")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"USERPROFILE=" + home,
		"TERM=xterm-256color",
		"WORK_HOME=" + workHome,
		"WORK_CD_FILE=",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=" + filepath.Join(base, "gitconfig"),
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
	}
	return env, home, workHome
}

func makeRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gc"),
			"GIT_CONFIG_SYSTEM="+os.DevNull,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func needSeed(t *testing.T) {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}
}

// gitIn runs `git -C dir args...` with the test git identity.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gc"),
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// `work start` with no SOURCE prompts for the path, then converges to
// the same materialization as the flag form once the path is supplied.
func TestStartNoSourcePrompts(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	// Every required value except SOURCE is a flag, so only the path is asked.
	c := newConsole(t, bin, env, "start",
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	c.expect("repository path")
	c.send(repo + "\r")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("guided start exited %d, want 0", code)
	}

	wt := filepath.Join(ws, "in-progress", "src_guided", "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "src_guided", "work-state.json")); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
}

// T056 — in an interactive terminal a bad path, a bad branch name, and a branch
// collision each return to the prompt that produced them; a later valid choice
// completes the same journey without restarting `work start`. (US3 #1–#3.)
func TestInteractiveRecovery(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	gitIn(t, repo, "branch", "taken")
	ws := filepath.Join(homeDir, "ws")

	// Only SOURCE and the slug are prompted.
	c := newConsole(t, bin, env, "start",
		"--workspace", ws, "--base", "main", "--prefix", "{slug}", "--yes")

	// Bad path -> the error is shown inside the live field; the step does not
	// exit and the field stays open for a correction.
	c.expect("repository path")
	c.send("/no/such/path\r")
	c.expect("does not exist")
	c.expect("repository path")
	c.send("\x15" + repo + "\r") // ctrl-u clears the field, then the valid path

	// Slug that git rejects as a ref -> the error appears in-frame.
	c.expect("Slug")
	c.send("bad:slug\r")
	c.expect("not a valid branch name")
	c.expect("Slug")

	// Slug that collides with an existing branch -> in-frame collision message.
	c.send("\x15taken\r")
	c.expect("already exists")
	c.expect("Slug")

	// A free slug completes the journey.
	c.send("\x15fresh\r")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("recovered start exited %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "src_fresh", "worktree")); err != nil {
		t.Fatalf("worktree missing after recovery: %v", err)
	}
	// The rejected choices left no branches behind: only main, taken, and fresh.
	out, _ := exec.Command("git", "-C", repo, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	got := strings.Fields(string(out))
	sort.Strings(got)
	if want := []string{"fresh", "main", "taken"}; !slices.Equal(got, want) {
		t.Errorf("branches after recovery = %v, want %v", got, want)
	}
}

// T055 — declining the confirmation prompt exits 20 and materializes nothing.
func TestInteractiveCancelAtConfirm(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	// No --yes, so the confirm prompt is shown.
	c := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "cancelme", "--prefix", "{slug}")
	c.expect("Create Work")
	c.send("n") // Reject -> submit
	if code := c.wait(); code != 20 {
		t.Fatalf("declining the confirm exited %d, want 20", code)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "src_cancelme")); !os.IsNotExist(err) {
		t.Fatalf("declined create left a Work directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workHome, "state", "work.db")); err == nil {
		t.Fatalf("declined create left a projection database")
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", "cancelme").CombinedOutput(); len(out) != 0 {
		t.Fatalf("declined create left branch cancelme: %s", out)
	}

	// An interrupt delivered before the commit step also rolls back to exit 20.
	// The workspace root is persisted only after the wizard is accepted (p7), so
	// the declined run above wrote no config — this run supplies --workspace too.
	c2 := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "intr", "--prefix", "{slug}")
	c2.expect("Create Work")
	c2.send("\x03") // Ctrl-C
	if code := c2.wait(); code != 20 {
		t.Fatalf("Ctrl-C at the confirm exited %d, want 20", code)
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", "intr").CombinedOutput(); len(out) != 0 {
		t.Fatalf("interrupted create left branch intr: %s", out)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "src_intr")); !os.IsNotExist(err) {
		t.Fatalf("interrupted create left a Work directory: %v", err)
	}
}

// Two-phase base-branch picker: the guided flow presents Remote / Local tabs and
// the new branch starts from exactly the ref chosen there. Covers FR-009 (the
// selection staged as source-then-branch); mirrors create_base_branch.txtar.
func TestInteractiveBaseBranchTabs(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	origin := filepath.Join(homeDir, "origin")
	makeRepo(t, origin)
	clone := filepath.Join(homeDir, "clone")
	gitIn(t, homeDir, "clone", "-q", origin, clone)
	// Diverge local main from origin/main so "exactly the chosen revision" bites.
	gitIn(t, clone, "commit", "-q", "--allow-empty", "-m", "local only")
	originMain := gitIn(t, clone, "rev-parse", "origin/main")
	localMain := gitIn(t, clone, "rev-parse", "main")
	if originMain == localMain {
		t.Fatal("fixture did not diverge local main from origin/main")
	}
	ws := filepath.Join(homeDir, "ws")

	// Only the base branch is prompted; the picker groups choices into Local /
	// Remote tabs. Switch to the Remote tab (its only row is origin/main) and
	// select it.
	c := newConsole(t, bin, env, "start", clone,
		"--workspace", ws, "--slug", "picked", "--prefix", "{slug}", "--yes")
	c.expect("Base branch")
	c.expect("Local")
	c.expect("Remote")
	c.send("\t") // Local -> Remote
	c.expect("origin/main")
	c.send("\r")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("start exited %d, want 0", code)
	}

	dir := filepath.Join(ws, "in-progress", "clone_picked")
	b, err := os.ReadFile(filepath.Join(dir, "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"base_branch": "origin/main"`) {
		t.Errorf("snapshot base_branch is not origin/main:\n%s", b)
	}
	if tip := gitIn(t, filepath.Join(dir, "worktree"), "rev-parse", "HEAD"); tip != originMain {
		t.Errorf("branch tip = %s, want origin/main %s", tip, originMain)
	}
}
