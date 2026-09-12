package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/seed"
)

// seedRepoAt creates a git repository at exactly dir (unlike gittest.Repo,
// which always uses a fresh t.TempDir()), so its basename can be chosen to
// match a `work start <name>` lookup.
func seedRepoAt(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	gittest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

// writeRepositoryRoot pre-configures repository_roots = [root] in home's
// config/work.json — Phase 3 has no `work repository root add` CLI yet, so
// tests write the file directly.
func writeRepositoryRoot(t *testing.T, home, root string) {
	t.Helper()
	cfgPath := filepath.Join(home, "config", "work.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.RepositoryRoots = []string{root}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
}

func needSeed(t *testing.T) {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}
}

// runWorkHome executes `work` with args against the given WORK_HOME and
// returns stdout, stderr, and the mapped exit code.
func runWorkHome(t *testing.T, home string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("WORK_HOME", home)
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

// runWork executes `work` with args in a fresh, isolated WORK_HOME.
func runWork(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runWorkHome(t, filepath.Join(t.TempDir(), "dothome"), args...)
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

func TestStartByNameSingleMatchNonInteractive(t *testing.T) {
	needSeed(t)
	home := filepath.Join(t.TempDir(), "dothome")
	root := t.TempDir()
	seedRepoAt(t, filepath.Join(root, "payments"))
	writeRepositoryRoot(t, home, root)

	out, errb, code := runWorkHome(t, home, "start", "payments",
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, errb)
	}
	if !strings.Contains(out, "work: created") {
		t.Errorf("stdout missing success line: %s", out)
	}
}

func TestStartByNameNoRootsIsNoRepositoryFound(t *testing.T) {
	needSeed(t)
	_, errb, code := runWork(t, "start", "payments",
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code != 26 {
		t.Fatalf("exit = %d, want 26\nstderr: %s", code, errb)
	}
}

func TestStartByNameAmbiguousIsTerminalInPhase3(t *testing.T) {
	// Phase 3 (US1) only wires the single-match outcome; the ambiguity picker
	// and the non-interactive repository-ambiguous exit (30) land in Phase 4.
	// Until then, >= 2 matches is a terminal diagnostic (T029).
	needSeed(t)
	home := filepath.Join(t.TempDir(), "dothome")
	root := t.TempDir()
	seedRepoAt(t, filepath.Join(root, "a", "payments"))
	seedRepoAt(t, filepath.Join(root, "b", "payments"))
	writeRepositoryRoot(t, home, root)

	_, _, code := runWorkHome(t, home, "start", "payments",
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code == 0 {
		t.Fatalf("exit = 0, want a non-zero (non-MVP-scoped) outcome for an ambiguous name")
	}
}

func TestStartPathStillBypassesLocator(t *testing.T) {
	// work start <path> must remain byte-for-byte the F1 journey: no
	// repository_roots configured at all, and the path still resolves.
	needSeed(t)
	repo := gittest.Repo(t)
	out, errb, code := runWork(t, "start", repo,
		"--workspace", filepath.Join(t.TempDir(), "ws"),
		"--base", "main", "--slug", "s", "--prefix", "{slug}", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, errb)
	}
	if !strings.Contains(out, "work: created") {
		t.Errorf("stdout missing success line: %s", out)
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
