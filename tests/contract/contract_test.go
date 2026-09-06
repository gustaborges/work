// Package contract drives the built seed component binaries with golden
// stdin/stdout JSON and asserts the process contract from
// specs/001-first-local-work/contracts/ipc-*.md.
package contract

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// seedBin returns the path to a built seed binary ("starter" / "locator") for
// the host platform, skipping the test when `make seed` has not run.
func seedBin(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	p := filepath.Join(repoRoot, "seed", "dist",
		runtime.GOOS+"_"+runtime.GOARCH, name+ext)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("seed binary %s missing; run `make seed`", p)
	}
	return p
}

type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func runBin(t *testing.T, bin string, stdin string) runResult {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader([]byte(stdin))
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
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
