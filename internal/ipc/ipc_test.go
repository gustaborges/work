package ipc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
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
	res, err := Run(Target{Path: echo}, []byte(`{"arg":"x"}`))
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
	res, err := Run(Target{Path: echo}, nil)
	if err == nil {
		t.Fatal("Run: want error on non-zero exit")
	}
	if res.ExitCode != 3 || !res.Exited {
		t.Errorf("ExitCode = %d, Exited = %v, want 3, true", res.ExitCode, res.Exited)
	}
	if res.Stderr == "" {
		t.Errorf("Stderr empty, want captured diagnostics")
	}
}

func TestRunMissingEntrypoint(t *testing.T) {
	_, err := Run(Target{Path: filepath.Join(t.TempDir(), "nope")}, nil)
	if err == nil {
		t.Fatal("Run(missing): want error")
	}
}

func TestInvokeStarterIgnoresUnknownKeys(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "stdout")
	t.Setenv("ECHO_STDOUT", `{"repository":{"path":"/abs/repo"},"id":"x","status":"in-progress","branch":"evil"}`)

	resp, err := InvokeStarter(Target{Path: echo}, StarterInput{Arg: "/abs/repo"})
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
	resp, err := InvokeStarter(Target{Path: echo}, StarterInput{Arg: "/r"})
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
	if _, err := InvokeStarter(Target{Path: echo}, StarterInput{Arg: "x"}); err == nil {
		t.Fatal("InvokeStarter(garbage stdout): want error")
	}
}

func TestInvokeStarterRejectsNonZero(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "fail")
	if _, err := InvokeStarter(Target{Path: echo}, StarterInput{Arg: "x"}); err == nil {
		t.Fatal("InvokeStarter(exit 3): want error")
	}
}

func TestInvokeLocatorHappy(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "stdout")
	t.Setenv("ECHO_STDOUT", `{"matches":[{"repo_path":"/a"},{"repo_path":"/b"}]}`)
	resp, err := InvokeLocator(Target{Path: echo}, LocatorInput{})
	if err != nil {
		t.Fatalf("InvokeLocator: %v", err)
	}
	if len(resp.Matches) != 2 || resp.Matches[0].RepoPath != "/a" {
		t.Errorf("Matches = %+v", resp.Matches)
	}
}

func TestRunWithRuntimeIgnoresShebangAndExecBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX sh")
	}
	script := filepath.Join(t.TempDir(), "s.sh")
	if err := os.WriteFile(script, []byte("cat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(Target{Runtime: "sh", Path: script}, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(res.Stdout) != `{"a":1}` {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if _, err := Run(Target{Path: script}, nil); err == nil {
		t.Error("a non-executable script ran without a runtime")
	}
}

func TestRunContextCancellationKillsTheChild(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "hang")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	res, err := RunContext(ctx, Target{Path: echo}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", res.ExitCode)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("cancellation did not stop the child promptly")
	}
}

func TestRunCapsStderrToItsTail(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "noisy")
	res, err := Run(Target{Path: echo}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Stderr) != 4<<10 || !strings.HasSuffix(res.Stderr, "END") {
		t.Errorf("stderr len = %d, want the last 4096 bytes ending in END", len(res.Stderr))
	}
}

func TestInvokeLinker(t *testing.T) {
	echo := buildEcho(t)
	tests := []struct {
		name    string
		stdout  string
		want    string
		invalid bool
	}{
		{"value", `{"value":"https://x/1"}`, "https://x/1", false},
		{"empty object", `{}`, "", false},
		{"empty stdout", ``, "", false},
		{"null value", `{"value":null}`, "", false},
		{"extra fields ignored", `{"value":"v","key":"other","x":1}`, "v", false},
		{"empty string", `{"value":""}`, "", true},
		{"non-string", `{"value":42}`, "", true},
		{"array", `[1]`, "", true},
		{"garbage", `nope`, "", true},
		{"two documents", `{} {}`, "", true},
	}
	for _, tc := range tests {
		t.Setenv("ECHO_MODE", "stdout")
		t.Setenv("ECHO_STDOUT", tc.stdout)
		got, _, err := InvokeLinker(context.Background(), Target{Path: echo}, LinkerInput{Inputs: map[string]any{"worktree_path": "/w"}})
		if tc.invalid {
			if !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("%s: err = %v, want ErrInvalidResponse", tc.name, err)
			}
			continue
		}
		if err != nil || got.Value != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got.Value, err, tc.want)
		}
	}
}

func TestInvokeLinkerDeliversOnlyTheInputs(t *testing.T) {
	echo := buildEcho(t)
	t.Setenv("ECHO_MODE", "passthrough")
	// passthrough echoes stdin, which is `{"inputs":...}` and so a valid
	// response document without a value.
	got, res, err := InvokeLinker(context.Background(), Target{Path: echo}, LinkerInput{Inputs: map[string]any{"worktree_path": "/w"}})
	if err != nil || got.Value != "" {
		t.Fatalf("got %q, %v", got.Value, err)
	}
	if string(res.Stdout) != `{"inputs":{"worktree_path":"/w"}}` {
		t.Errorf("delivered = %s", res.Stdout)
	}
}

func TestInvokeImporter(t *testing.T) {
	echo := buildEcho(t)
	for stdout, ok := range map[string]bool{``: true, `{}`: true, `{"anything":[1]}`: true, `[]`: false, `text`: false, `{}{}`: false} {
		t.Setenv("ECHO_MODE", "stdout")
		t.Setenv("ECHO_STDOUT", stdout)
		_, err := InvokeImporter(context.Background(), Target{Path: echo}, ImporterInput{OutputDir: "/o"})
		if ok && err != nil {
			t.Errorf("stdout %q: %v", stdout, err)
		}
		if !ok && !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("stdout %q: err = %v, want ErrInvalidResponse", stdout, err)
		}
	}

	t.Setenv("ECHO_MODE", "fail")
	res, err := InvokeImporter(context.Background(), Target{Path: echo}, ImporterInput{OutputDir: "/o"})
	if err == nil || res.ExitCode != 3 || errors.Is(err, ErrInvalidResponse) {
		t.Errorf("non-zero exit: res=%+v err=%v", res, err)
	}
}

func TestRunMissingEntrypointDidNotExit(t *testing.T) {
	res, err := Run(Target{Path: filepath.Join(t.TempDir(), "absent")}, nil)
	if err == nil {
		t.Fatal("Run: want error for a missing entrypoint")
	}
	if res.Exited || res.ExitCode != -1 {
		t.Errorf("Exited = %v, ExitCode = %d, want a start failure (false, -1)", res.Exited, res.ExitCode)
	}
}
