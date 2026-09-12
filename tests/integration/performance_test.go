package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/convention"
	"github.com/gustaborges/work/internal/create"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work/verify"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
)

// budget is the ceiling for the machine portion of `work start` from SC-001:
// warm bootstrap + git worktree + snapshot + projection, on a ~1k-commit repo.
const budget = 5 * time.Second

// TestStartMachinePortionUnderBudget builds a repository with ~1000 commits,
// warms the bootstrap, then times only the create pipeline (worktree +
// snapshot + db) and asserts it stays well under the SC-001 budget.
func TestStartMachinePortionUnderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 1000-commit repo; skipped under -short")
	}
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}

	root := t.TempDir()
	t.Setenv("WORK_HOME", filepath.Join(root, "dothome"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	home, err := workhome.Resolve()
	if err != nil {
		t.Fatal(err)
	}

	// Warm the seed install so the timed section excludes first-run extraction.
	if err := bootstrap.EnsureSeed(home); err != nil {
		t.Fatalf("warm bootstrap: %v", err)
	}

	src := buildLargeRepo(t, root, 1000)
	workspaceRoot := filepath.Join(root, "workspaces")

	// The workspace root would normally be resolved/persisted by the CLI.
	cfg := config.Default()
	cfg.Workspace = workspaceRoot
	if err := config.Save(home.ConfigFile(), cfg); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	res, err := create.Run(context.Background(), create.Params{
		Home:            home,
		SourceRepo:      src,
		RepoName:        filepath.Base(src),
		WorkspaceRoot:   workspaceRoot,
		Slug:            "perf-check",
		Branch:          "perf-check",
		BaseRefname:     "refs/heads/main",
		BaseBranchShort: "main",
		Convention:      convention.Freeform,
		Starter:         "local-path-starter",
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("create.Run: %v", err)
	}

	t.Logf("machine portion: %s (budget %s)", elapsed.Round(time.Millisecond), budget)
	if elapsed > budget {
		t.Errorf("machine portion %s exceeds the %s budget", elapsed, budget)
	}

	db, err := projection.Open(home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rep, err := verify.Check(db, res.WorkID)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Errorf("created Work is incoherent: %s", strings.Join(rep.Problems, "; "))
	}
}

// resolutionBudget is SC-012: a single-match name lookup over a large search
// tree completes well under this ceiling on the reference runner.
const resolutionBudget = 2 * time.Second

// TestStartByUniqueNameOver500ReposUnderBudget builds a search root of 500
// sibling directories that merely *look* like repositories to the Locator's
// walk (a bare .git entry, no init) plus one real, uniquely-named repository,
// and asserts non-interactive `work start <unique-name>` resolves and
// completes within the SC-012 budget. The expensive part (reporef.ValidatePath)
// scales with matches, not with the size of the search tree — a unique name
// returns exactly one (research R18).
func TestStartByUniqueNameOver500ReposUnderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 500-entry search tree; skipped under -short")
	}
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}

	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	root := t.TempDir()
	workBin := filepath.Join(root, "work")
	if runtime.GOOS == "windows" {
		workBin += ".exe"
	}
	buildCmd := exec.Command("go", "build", "-o", workBin, "github.com/gustaborges/work/cmd/work")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build work binary: %v\n%s", err, out)
	}

	searchRoot := filepath.Join(root, "src")
	for i := range 500 {
		dir := filepath.Join(searchRoot, fmt.Sprintf("repo-%d", i))
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(searchRoot, "unique-target-payments")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, target, "init", "-q", "-b", "main")
	gitRun(t, target, "commit", "-q", "--allow-empty", "-m", "init")

	home := filepath.Join(root, "dothome")
	cfgPath := filepath.Join(home, "config", "work.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.RepositoryRoots = []string{searchRoot}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(workBin, "start", "unique-target-payments",
		"--workspace", filepath.Join(root, "ws"), "--base", "main",
		"--slug", "perf", "--prefix", "{slug}", "--yes")
	cmd.Env = append(os.Environ(),
		"WORK_HOME="+home, "HOME="+filepath.Join(root, "home"),
		"GIT_CONFIG_GLOBAL="+filepath.Join(root, "gitconfig"), "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")

	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("work start: %v\n%s", err, out)
	}

	t.Logf("name resolution + create over 500 repos: %s (budget %s)", elapsed.Round(time.Millisecond), resolutionBudget)
	if elapsed > resolutionBudget {
		t.Errorf("resolution + create %s exceeds the %s budget", elapsed, resolutionBudget)
	}
}

// buildLargeRepo creates a git repo at <parent>/big with n commits on main,
// using a single `git fast-import` stream so setup stays fast on every OS.
func buildLargeRepo(t *testing.T, parent string, n int) string {
	t.Helper()
	dir := filepath.Join(parent, "big")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")

	var b strings.Builder
	fmt.Fprintf(&b, "blob\nmark :1\ndata 6\nhello\n\n")
	for i := 1; i <= n; i++ {
		msg := fmt.Sprintf("commit %d\n", i)
		fmt.Fprintf(&b, "commit refs/heads/main\ncommitter t <t@t> %d +0000\ndata %d\n%s",
			1_000_000_000+i, len(msg), msg)
		// First commit is a root; fast-import tracks the branch head after that.
		fmt.Fprintf(&b, "M 100644 :1 file.txt\n\n")
	}
	b.WriteString("done\n")

	cmd := exec.Command("git", "-C", dir, "fast-import", "--quiet", "--done")
	cmd.Stdin = strings.NewReader(b.String())
	cmd.Env = append(os.Environ(), "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git fast-import: %v\n%s", err, out)
	}
	gitRun(t, dir, "reset", "-q", "--hard", "main")

	count := strings.TrimSpace(gitOut(t, dir, "rev-list", "--count", "main"))
	if count != fmt.Sprint(n) {
		t.Fatalf("built %s commits, want %d", count, n)
	}
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
