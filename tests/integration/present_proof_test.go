//go:build unix

// Phase 1 de-risking probe (research R18 step 1): prove on a real PTY that a
// Bubble Tea inline model which sets its final View to a compact receipt before
// tea.Quit leaves only that receipt in the terminal — an in-frame `✘` error is
// replaced on the next keystroke and does not survive into the final frame.
//
// This test and tests/integration/testdata/presentproof are deleted in Phase 2
// once internal/present.Input exists and carries its own model tests.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

func buildPresentProof(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "presentproof")
	cmd := exec.Command("go", "build", "-o", bin,
		"github.com/gustaborges/work/tests/integration/testdata/presentproof")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build presentproof: %v\n%s", err, out)
	}
	return bin
}

func TestPresentProofFinalFrameIsReceiptOnly(t *testing.T) {
	bin := buildPresentProof(t)

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	defer func() { _ = f.Close(); _ = cmd.Process.Kill() }()

	var mu sync.Mutex
	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		b := make([]byte, 4096)
		for {
			n, rerr := f.Read(b)
			if n > 0 {
				mu.Lock()
				buf.Write([]byte(ansiRe.ReplaceAllString(string(b[:n]), "")))
				mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()
	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return strings.ReplaceAll(buf.String(), "\r", "")
	}
	expect := func(sub string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if strings.Contains(snapshot(), sub) {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %q\n--- output ---\n%s", sub, snapshot())
	}

	expect("Local repository path")

	// A rejected attempt shows the in-frame error.
	if _, err := f.WriteString("zzz\r"); err != nil {
		t.Fatal(err)
	}
	expect("✘ that path does not exist")

	// The next keystrokes replace the error and lead to an accepted value.
	if _, err := f.WriteString("good\r"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("child did not exit\n--- output ---\n%s", snapshot())
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("presentproof exited with error: %v\n%s", err, snapshot())
	}

	out := snapshot()

	// Bubble Tea's graceful final render committed the compact receipt frame.
	if !strings.Contains(out, "Local repository path\n  ✔ good") {
		t.Fatalf("final receipt not found in terminal output:\n%s", out)
	}
	if n := strings.Count(out, "✔ good"); n != 1 {
		t.Fatalf("receipt rendered %d times, want 1:\n%s", n, out)
	}

	// Nothing after the receipt: no error debris, no leftover editing prompt,
	// no restated rejected value. The intermediate `✘` frame did not survive.
	rest := out[strings.LastIndex(out, "  ✔ good")+len("  ✔ good"):]
	for _, debris := range []string{"✘", "does not exist", "> "} {
		if strings.Contains(rest, debris) {
			t.Errorf("%q survives into the final frame after the receipt:\n%q", debris, rest)
		}
	}
}
