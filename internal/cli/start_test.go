package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/seed"
)

func needSeed(t *testing.T) {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}
}

// runWork executes `work` with args in an isolated WORK_HOME and returns
// stdout, stderr, and the mapped exit code.
func runWork(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("WORK_HOME", filepath.Join(t.TempDir(), "dothome"))
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	msg := errb.String()
	if err != nil {
		// Execute() would print this line to os.Stderr; fold it in for assertions.
		msg += diag.Format(err) + "\n"
	}
	return out.String(), msg, diag.ExitCode(err)
}

func TestStartRejectsJSON(t *testing.T) {
	_, _, code := runWork(t, "start", "x", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestStartNonInteractiveMissingSlug(t *testing.T) {
	repo := gittest.Repo(t)
	ws := filepath.Join(t.TempDir(), "ws")
	_, errb, code := runWork(t, "start", repo,
		"--workspace", ws, "--base", "main", "--prefix", "{slug}", "--yes")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if !strings.Contains(errb, "--slug") {
		t.Errorf("stderr does not name --slug: %s", errb)
	}
	// No workspace layout created, no Work directory.
	if _, err := os.Stat(ws); err == nil {
		t.Errorf("workspace root was created on a usage failure")
	}
}

func TestStartNonInteractiveMissingSource(t *testing.T) {
	_, errb, code := runWork(t, "start",
		"--workspace", "x", "--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(strings.ToLower(errb), "source") {
		t.Errorf("stderr does not mention SOURCE: %s", errb)
	}
}

func TestStartInvalidPath(t *testing.T) {
	needSeed(t)
	_, _, code := runWork(t, "start", filepath.Join(t.TempDir(), "missing"),
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code != 10 {
		t.Fatalf("exit = %d, want 10", code)
	}
}

func TestStartInvalidSlugReachesGit(t *testing.T) {
	needSeed(t)
	repo := gittest.Repo(t)
	_, _, code := runWork(t, "start", repo,
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "has spaces", "--prefix", "{slug}", "--yes")
	if code != 13 {
		t.Fatalf("exit = %d, want 13", code)
	}
}

func TestStartBranchCollision(t *testing.T) {
	needSeed(t)
	repo := gittest.Repo(t)
	gittest.Git(t, repo, "branch", "taken")
	_, _, code := runWork(t, "start", repo,
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "taken", "--prefix", "{slug}", "--yes")
	if code != 14 {
		t.Fatalf("exit = %d, want 14", code)
	}
}

func TestShellInitUnknownShell(t *testing.T) {
	_, errb, code := runWork(t, "shell-init", "frobnicate")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errb, "supported shells") {
		t.Errorf("stderr: %s", errb)
	}
}

func TestShellInitBash(t *testing.T) {
	out, _, code := runWork(t, "shell-init", "bash")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "WORK_CD_FILE") {
		t.Errorf("snippet missing WORK_CD_FILE:\n%s", out)
	}
}
