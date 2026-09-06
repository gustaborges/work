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

// T043 — `work` no-args opens the home listing only "Start a Work"; q exits 0
// with no state change; selecting the entry reaches the path prompt.
func TestHomeReachability(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, _, workHome := ptyEnv(t)

	// Quit the home immediately: exit 0, nothing created.
	c := newConsole(t, bin, env)
	c.expect("Start a Work")
	if s := c.snapshot(); strings.Contains(s, "Resume") || strings.Contains(s, "Archive") {
		t.Fatalf("home exposes later-slice actions:\n%s", s)
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
