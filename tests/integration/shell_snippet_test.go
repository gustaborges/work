package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/shellintegration"
	"github.com/gustaborges/work/seed"
)

// TestBashSnippetRepositionsSession sources the emitted bash snippet through a
// fake shell and asserts the shell's cwd ends up inside the new worktree.
func TestBashSnippetRepositionsSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash snippet test is POSIX-only")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}

	tmp := t.TempDir()
	repoRoot := repoRootDir(t)

	// Build the work binary.
	workBin := filepath.Join(tmp, "work")
	build := exec.Command("go", "build", "-o", workBin, "./cmd/work")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build work: %v\n%s", err, out)
	}

	// Write the bash snippet to a file.
	snippet, err := shellintegration.Snippet("bash")
	if err != nil {
		t.Fatal(err)
	}
	snippetFile := filepath.Join(tmp, "work.bash")
	if err := os.WriteFile(snippetFile, []byte(snippet), 0o644); err != nil {
		t.Fatal(err)
	}

	// A source repo and an isolated home/workspace.
	src := filepath.Join(tmp, "src")
	mustGit(t, src, "init", "-q", "-b", "main")
	mustGit(t, src, "commit", "-q", "--allow-empty", "-m", "init")
	home := filepath.Join(tmp, "home")
	ws := filepath.Join(tmp, "ws")

	fakeshell := filepath.Join(repoRoot, "tests", "fixtures", "fakeshell", "fakeshell.sh")
	cmd := exec.Command(bash, fakeshell, workBin, snippetFile, "--",
		"start", src, "--workspace", ws, "--base", "main", "--slug", "moved", "--prefix", "{slug}", "--yes")
	cmd.Env = append(os.Environ(),
		"WORK_HOME="+filepath.Join(tmp, "dothome"),
		"HOME="+home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fakeshell: %v\n%s", err, out)
	}

	wantPWD := filepath.Join(ws, "in-progress", "src_moved", "worktree")
	if !strings.Contains(string(out), "FAKESHELL_PWD="+wantPWD) {
		t.Errorf("shell did not move into the worktree\nwant FAKESHELL_PWD=%s\ngot:\n%s", wantPWD, out)
	}
	if strings.Contains(string(out), "this shell session was not moved") {
		t.Errorf("FR-023 notice printed even though integration was active:\n%s", out)
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
