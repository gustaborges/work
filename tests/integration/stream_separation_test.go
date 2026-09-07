//go:build unix

package integration

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// A full interactive `work start` must keep the two output channels disjoint
// (FR-026, SC-007): every frame, receipt, help line and diagnostic goes to the
// UI channel (stderr), and stdout carries only the three stable F1 result lines,
// byte-identical to the flag form. Stdin and stdout share one pty so the process
// still detects an interactive terminal (present.IsInteractive checks both);
// stderr is a second pty read separately.
func TestStdoutCarriesOnlyStableLines(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	s := startSplit(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "split", "--prefix", "{slug}")

	s.expectUI("Create Work")
	s.send("y")
	s.expectOut("work: created ")
	if code := s.wait(); code != 0 {
		t.Fatalf("split start exited %d, want 0\nUI:\n%s\nOUT:\n%s", code, s.ui(), s.out())
	}

	out := stripANSI(s.out())
	ui := s.ui()

	// stdout: exactly the three F1 lines, nothing else.
	lines := splitNonEmpty(out)
	if len(lines) != 3 {
		t.Fatalf("stdout is not the 3-line F1 contract, got %d lines:\n%q", len(lines), out)
	}
	wt := filepath.Join(ws, "in-progress", "src_split", "worktree")
	want := []*regexp.Regexp{
		regexp.MustCompile(`^work: created [0-9A-HJKMNP-TV-Z]{26}$`),
		regexp.MustCompile(`^work: branch split  \(from main @ [0-9a-f]{7,}\)$`),
		regexp.MustCompile(`^work: path ` + regexp.QuoteMeta(wt) + `$`),
	}
	for i, re := range want {
		if !re.MatchString(lines[i]) {
			t.Errorf("stdout line %d = %q, want %s", i+1, lines[i], re)
		}
	}

	// No interactive chrome ever reached stdout.
	for _, leak := range []string{"Local repository path", "✔", "Create Work", "Base branch", "❯"} {
		if strings.Contains(out, leak) {
			t.Errorf("interactive chrome %q leaked onto stdout:\n%q", leak, out)
		}
	}

	// The receipts and the confirmation receipt are on the UI channel.
	for _, w := range []string{"Create Work", "✔ Create Work confirmed"} {
		if !strings.Contains(ui, w) {
			t.Errorf("UI channel missing %q:\n%s", w, ui)
		}
	}
	// The FR-023 notice is a human message, so it belongs on the UI channel.
	if !strings.Contains(ui, "this shell session was not moved") {
		t.Errorf("FR-023 notice not on the UI channel:\n%s", ui)
	}
}

// splitProc runs the child with stdin+stdout on one pty and stderr on another.
type splitProc struct {
	t      *testing.T
	cmd    *exec.Cmd
	inout  *os.File
	mu     sync.Mutex
	outBuf strings.Builder
	uiBuf  strings.Builder
	done   chan struct{}
}

func startSplit(t *testing.T, bin string, env []string, args ...string) *splitProc {
	t.Helper()
	ws := &pty.Winsize{Rows: 24, Cols: 80}

	ptmxIO, ttyIO, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open io: %v", err)
	}
	ptmxErr, ttyErr, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open err: %v", err)
	}
	_ = pty.Setsize(ptmxIO, ws)
	_ = pty.Setsize(ptmxErr, ws)

	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin = ttyIO
	cmd.Stdout = ttyIO
	cmd.Stderr = ttyErr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	// The child holds its own copies of the slave fds now.
	_ = ttyIO.Close()
	_ = ttyErr.Close()

	s := &splitProc{t: t, cmd: cmd, inout: ptmxIO, done: make(chan struct{})}
	var wg sync.WaitGroup
	wg.Add(2)
	pump := func(src *os.File, dst *strings.Builder) {
		defer wg.Done()
		b := make([]byte, 4096)
		for {
			n, err := src.Read(b)
			if n > 0 {
				s.mu.Lock()
				dst.Write(b[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}
	go pump(ptmxIO, &s.outBuf)
	go pump(ptmxErr, &s.uiBuf)
	go func() { wg.Wait(); close(s.done) }()

	t.Cleanup(func() {
		_ = ptmxIO.Close()
		_ = ptmxErr.Close()
		_ = cmd.Process.Kill()
	})
	return s
}

func (s *splitProc) out() string { s.mu.Lock(); defer s.mu.Unlock(); return s.outBuf.String() }
func (s *splitProc) ui() string  { s.mu.Lock(); defer s.mu.Unlock(); return stripANSI(s.uiBuf.String()) }

func (s *splitProc) send(str string) {
	s.t.Helper()
	if _, err := s.inout.WriteString(str); err != nil {
		s.t.Fatalf("send %q: %v", str, err)
	}
}

func (s *splitProc) expectUI(sub string) { s.expect(sub, s.ui) }
func (s *splitProc) expectOut(sub string) {
	s.expect(sub, func() string { return stripANSI(s.out()) })
}

func (s *splitProc) expect(sub string, read func() string) {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(read(), sub) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %q\nUI:\n%s\nOUT:\n%s", sub, s.ui(), s.out())
}

func (s *splitProc) wait() int {
	s.t.Helper()
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		s.t.Fatalf("timed out waiting for exit\nUI:\n%s\nOUT:\n%s", s.ui(), s.out())
	}
	err := s.cmd.Wait()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	s.t.Fatalf("wait: %v", err)
	return -1
}

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimRight(l, "\r"); t != "" {
			out = append(out, t)
		}
	}
	return out
}
