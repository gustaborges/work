package contract

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runBinCtx runs bin with the given stdin under a hard deadline. A binary that
// blocks on stdin or loops forever trips the deadline and fails the test rather
// than hanging the suite (roadmap §4 "contrato de processo": never hang).
func runBinCtx(t *testing.T, bin string, stdin []byte) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s did not exit within the deadline on garbled input (hang)", bin)
	}

	res := runResult{stdout: out.Bytes(), stderr: errb.Bytes()}
	var ee *exec.ExitError
	switch {
	case err == nil:
		res.exitCode = 0
	case errors.As(err, &ee):
		res.exitCode = ee.ExitCode()
	default:
		t.Fatalf("running %s: %v", bin, err)
	}
	return res
}

// TestSeedBinariesRejectGarbledInput feeds both seed components malformed
// stdin — truncated JSON, non-UTF-8 bytes, NUL bytes, a large garbage blob, and
// empty input — and asserts each exits non-zero, writes nothing to stdout, and
// returns promptly. (T061; roadmap §4.)
func TestSeedBinariesRejectGarbledInput(t *testing.T) {
	cases := []struct {
		name  string
		stdin []byte
	}{
		{"truncated object", []byte(`{"arg":`)},
		{"truncated string", []byte(`{"arg":"/tmp/re`)},
		{"unbalanced braces", []byte(`{{{{`)},
		{"non-utf8 bytes", []byte{0xff, 0xfe, 0xfd, 0x00, 0x80, 0x81}},
		{"nul bytes only", bytes.Repeat([]byte{0x00}, 64)},
		{"empty stdin", []byte{}},
		{"bare newline", []byte("\n")},
		{"large garbage blob", bytes.Repeat([]byte("not-json not-json "), 128*1024)}, // ~2 MiB
		{"json array not object", []byte(`["arg","/tmp/x"]`)},
		{"trailing junk after object", []byte(`{"arg":"/tmp/x"} and then some`)},
	}

	for _, name := range []string{"starter", "locator"} {
		bin := seedBin(t, name)
		for _, c := range cases {
			t.Run(name+"/"+c.name, func(t *testing.T) {
				res := runBinCtx(t, bin, c.stdin)
				if res.exitCode == 0 {
					t.Fatalf("exit = 0 on garbled input; stdout = %q", res.stdout)
				}
				if len(res.stdout) != 0 {
					t.Errorf("wrote to stdout on failure: %q", res.stdout)
				}
				if len(bytes.TrimSpace(res.stderr)) == 0 {
					t.Errorf("no stderr diagnostic on failure")
				}
				if line := string(bytes.TrimSpace(res.stderr)); strings.Count(line, "\n") > 0 {
					t.Errorf("stderr is more than one line: %q", line)
				}
			})
		}
	}
}
