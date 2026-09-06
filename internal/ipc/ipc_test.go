package ipc

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	echoOnce sync.Once
	echoBin  string
	echoErr  error
)

// buildEcho compiles the testdata helper once for the whole package. The output
// lives in an OS temp dir (not t.TempDir, which would be removed after the
// first test).
func buildEcho(t *testing.T) string {
	t.Helper()
	echoOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ipc-echo-")
		if err != nil {
			echoErr = err
			return
		}
		bin := filepath.Join(dir, "echo")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		out, buildErr := exec.Command("go", "build", "-o", bin, "./testdata/echo").CombinedOutput()
		if buildErr != nil {
			echoErr = buildErr
			t.Logf("building echo helper: %s", out)
			return
		}
		echoBin = bin
	})
	if echoErr != nil {
		t.Fatalf("building echo helper: %v", echoErr)
	}
	return echoBin
}

func TestRunPassthrough(t *testing.T) {
	echo := buildEcho(t)
	res, err := Run(echo, []byte(`{"arg":"x"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 || string(res.Stdout) != `{"arg":"x"}` {
		t.Errorf("res = %+v", res)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "fail")
	res, err := Run(echo, nil)
	if err == nil {
		t.Fatal("Run: want error on non-zero exit")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if res.Stderr == "" {
		t.Errorf("Stderr empty, want captured diagnostics")
	}
}

func TestRunMissingEntrypoint(t *testing.T) {
	_, err := Run(filepath.Join(t.TempDir(), "nope"), nil)
	if err == nil {
		t.Fatal("Run(missing): want error")
	}
}

func TestInvokeStarterIgnoresUnknownKeys(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "stdout")
	t.Setenv("ECHO_STDOUT", `{"repository":{"path":"/abs/repo"},"id":"x","status":"in-progress","branch":"evil"}`)

	resp, err := InvokeStarter(echo, StarterInput{Arg: "/abs/repo"})
	if err != nil {
		t.Fatalf("InvokeStarter: %v", err)
	}
	if resp.Repository.Path != "/abs/repo" {
		t.Errorf("Repository.Path = %q", resp.Repository.Path)
	}
	// The typed struct has no id/status/branch fields, so a malicious Starter
	// cannot smuggle governed work.* values through this boundary.
}

func TestInvokeStarterParsesBaseAndModes(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "stdout")
	t.Setenv("ECHO_STDOUT", `{"repository":{"path":"/r"},"base_branch":"main","start_modes":["new"]}`)
	resp, err := InvokeStarter(echo, StarterInput{Arg: "/r"})
	if err != nil {
		t.Fatalf("InvokeStarter: %v", err)
	}
	if resp.BaseBranch != "main" || len(resp.StartModes) != 1 || resp.StartModes[0] != "new" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestInvokeStarterRejectsGarbage(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "garbage")
	if _, err := InvokeStarter(echo, StarterInput{Arg: "x"}); err == nil {
		t.Fatal("InvokeStarter(garbage stdout): want error")
	}
}

func TestInvokeStarterRejectsNonZero(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "fail")
	if _, err := InvokeStarter(echo, StarterInput{Arg: "x"}); err == nil {
		t.Fatal("InvokeStarter(exit 3): want error")
	}
}

func TestInvokeLocatorHappy(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "stdout")
	t.Setenv("ECHO_STDOUT", `{"matches":[{"repo_path":"/a"},{"repo_path":"/b"}]}`)
	resp, err := InvokeLocator(echo, LocatorInput{})
	if err != nil {
		t.Fatalf("InvokeLocator: %v", err)
	}
	if len(resp.Matches) != 2 || resp.Matches[0].RepoPath != "/a" {
		t.Errorf("Matches = %+v", resp.Matches)
	}
}
