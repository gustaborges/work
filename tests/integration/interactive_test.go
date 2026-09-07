//go:build unix

// Interactive (pty) coverage for the guided journey: the `work` home is
// reachable and reaches the path prompt, and `work start` with no SOURCE
// prompts for the path and then converges to the same guarantees as the flag
// form. Covers US2 scenarios 1 and 2; contracts/cli-work-home.md.
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
	mu   sync.Mutex
	buf  strings.Builder
	done chan struct{}
}

func newConsole(t *testing.T, bin string, env []string, args ...string) *console {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	// A concrete window size is required: bubbletea renders nothing into a 0x0
	// terminal, which is what an unsized pty reports on Linux.
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	c := &console{t: t, f: f, cmd: cmd, done: make(chan struct{})}
	go func() {
		defer close(c.done)
		b := make([]byte, 4096)
		for {
			n, err := f.Read(b)
			if n > 0 {
				c.mu.Lock()
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

// `work` no-args opens the home listing the shipped journeys ("Start a Work",
// "Resume a Work", "Archive Works"); q exits 0 with no state change; selecting
// "Start a Work" reaches the path prompt, "Resume a Work" reaches the recency
// picker, and "Archive Works" reaches the archive flow.
func TestHomeReachability(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, _, workHome := ptyEnv(t)

	// Quit the home immediately: exit 0, nothing created.
	c := newConsole(t, bin, env)
	c.expect("Start a Work")
	c.expect("Resume a Work")
	c.expect("Archive Works")
	for _, reserved := range []string{"status", "import", "link", "plugin", "repository", "convention"} {
		if strings.Contains(strings.ToLower(c.snapshot()), reserved) {
			t.Fatalf("home exposes a later-slice action %q:\n%s", reserved, c.snapshot())
		}
	}
	c.send("q")
	if code := c.wait(); code != 0 {
		t.Fatalf("quitting the home exited %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(workHome, "state", "work.db")); err == nil {
		t.Fatalf("quitting the home created a projection database")
	}

	// Selecting "Start a Work" enters the path prompt.
	c2 := newConsole(t, bin, env)
	c2.expect("Start a Work")
	c2.send("\r")
	c2.expect("repository path")
	c2.send("\x03") // Ctrl-C before anything is created
	if code := c2.wait(); code != 20 {
		t.Fatalf("Ctrl-C at the path prompt exited %d, want 20", code)
	}

	// Selecting "Resume a Work" enters the recency picker; with no Works it
	// prints the empty-list note and exits 0.
	c3 := newConsole(t, bin, env)
	c3.expect("Resume a Work")
	c3.send("\x1b[B") // arrow down to "Resume a Work"
	c3.send("\r")
	if code := c3.wait(); code != 0 {
		t.Fatalf("resume with no Works exited %d, want 0", code)
	}
	c3.expect("no Works to resume")

	// Selecting "Archive Works" enters the archive flow; with no Works it prints
	// the empty-list note and exits 0.
	c4 := newConsole(t, bin, env)
	c4.expect("Archive Works")
	c4.send("\x1b[B\x1b[B") // arrow down to "Archive Works"
	c4.send("\r")
	if code := c4.wait(); code != 0 {
		t.Fatalf("archive with no Works exited %d, want 0", code)
	}
	c4.expect("no active Works to archive")

	// Non-interactive `work` renders no TUI and exits 2 with the one-line
	// summary naming all three verbs (contracts/cli-work-home.md, FR-030).
	cmd := exec.Command(bin)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 2 {
		t.Fatalf("non-interactive `work` exit = %v, want 2\n%s", err, out)
	}
	for _, want := range []string{"work start", "work resume", "work archive", "work --help"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("non-interactive summary missing %q:\n%s", want, out)
		}
	}
}

// T039 — with Works present, arrowing to "Resume a Work" opens the recency
// picker and "Archive Works" opens the multi-select list; `q` leaves either
// without a state change. (contracts/cli-work-home.md, FR-030.)
func TestHomeReachesPopulatedPickers(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "demo")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")
	seedWorks(t, bin, env, repo, ws, "alpha", "bravo")

	// Home -> Resume a Work -> the recency picker lists a Work row.
	c := newConsole(t, bin, env)
	c.expect("Resume a Work")
	c.send("\x1b[B\r") // down to "Resume a Work", enter
	c.expect("demo  bravo")
	c.send("q")
	if code := c.wait(); code != 20 {
		t.Fatalf("q at the resume picker exited %d, want 20", code)
	}

	// Home -> Archive Works -> the multi-select list shows checkboxes.
	c2 := newConsole(t, bin, env)
	c2.expect("Archive Works")
	c2.send("\x1b[B\x1b[B\r") // down twice to "Archive Works", enter
	c2.expect("[ ]")
	c2.send("\x03") // Ctrl-C: cancel
	if code := c2.wait(); code != 20 {
		t.Fatalf("Ctrl-C at the archive picker exited %d, want 20", code)
	}

	// Nothing was archived.
	for _, slug := range []string{"alpha", "bravo"} {
		if _, err := os.Stat(filepath.Join(ws, "in-progress", "demo_"+slug, "worktree")); err != nil {
			t.Errorf("%s worktree gone after a cancelled archive: %v", slug, err)
		}
	}
}

// T044 — `work start` with no SOURCE prompts for the path, then converges to
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

	// Bad path -> notice + re-prompt, no exit.
	c.expect("repository path")
	c.send("/no/such/path\r")
	c.expect("invalid-path")
	c.expect("repository path")
	c.send(repo + "\r")

	// Slug that git rejects as a ref -> back to the slug prompt.
	c.expect("Slug")
	c.send("bad:slug\r")
	c.expect("invalid-branch-name")
	c.expect("Slug")

	// Slug that collides with an existing branch -> back to the slug prompt.
	c.send("taken\r")
	c.expect("branch-collision")
	c.expect("Slug")

	// A free slug completes the journey.
	c.send("fresh\r")
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
	// (The declined run above already persisted the workspace root.)
	c2 := newConsole(t, bin, env, "start", repo,
		"--base", "main", "--slug", "intr", "--prefix", "{slug}")
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

	// Only the base branch is prompted; the picker opens on the Remote tab whose
	// only row is origin/main. Filter to it, then select.
	c := newConsole(t, bin, env, "start", clone,
		"--workspace", ws, "--slug", "picked", "--prefix", "{slug}", "--yes")
	c.expect("Base branch")
	c.expect("Remote")
	c.expect("Local")
	// The picker carries huh's styled left rule, so coloring/theme is applied.
	if s := c.snapshot(); !strings.Contains(s, "┃") {
		t.Errorf("picker is unstyled (no left rule):\n%s", s)
	}
	c.send("/origin/main")
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
