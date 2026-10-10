//go:build unix

package integration

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// launchedRoles returns the fixture components that started, in order, from
// the WORK_FIXTURE_LOG file.
func launchedRoles(t *testing.T, logPath string) []string {
	t.Helper()
	f, err := os.Open(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var roles []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, `"role":"`); i >= 0 {
			rest := line[i+len(`"role":"`):]
			roles = append(roles, rest[:strings.Index(rest, `"`)])
		}
	}
	return roles
}

// contextSuiteWorkspace prepares an isolated home with the context-suite
// plugin installed and a repository its Starter resolves by name.
func contextSuiteWorkspace(t *testing.T, bin string, env []string, homeDir, workHome string) (ws string) {
	t.Helper()
	installPluginFixture(t, bin, env, "context-suite")
	root := filepath.Join(homeDir, "src")
	makeRepo(t, filepath.Join(root, "demo-ctx-1"))
	writeRepositoryRoots(t, workHome, root)
	return filepath.Join(homeDir, "ws")
}

// TestInterruptDuringAutomaticPhasesKeepsTheWork: the first
// SIGINT sent to work while a Linker runs kills that Linker, skips everything
// after it, reports one interruption, and still ends the start successfully
// with the Work intact and the shell repositioned into it.
func TestInterruptDuringAutomaticPhasesKeepsTheWork(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	logPath := filepath.Join(homeDir, "launches.log")
	cdFile := filepath.Join(homeDir, "cdfile")
	env = append(env,
		"WORK_FIXTURE_MODE=linker-hang,linker2-value,importer-ok",
		"WORK_FIXTURE_LOG="+logPath,
		"WORK_CD_FILE="+cdFile)

	c := newConsole(t, bin, env, "start", "demo-ctx-1",
		"--workspace", ws, "--base", "main", "--slug", "hang", "--prefix", "{slug}", "--yes")
	c.expect("work: running context-suite/linker (discover)")
	if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	c.expect("interrupted")
	if code := c.wait(); code != 0 {
		t.Fatalf("start exited %d after an interrupt of the automatic phases, want 0\n%s", code, c.screen())
	}

	dir := filepath.Join(ws, "in-progress", "demo-ctx-1_hang")
	if _, err := os.Stat(filepath.Join(dir, "worktree")); err != nil {
		t.Errorf("the Work was not kept: %v", err)
	}
	st := readSnapshot(t, filepath.Join(dir, "work-state.json"))
	if st.Work.Branch != "hang" {
		t.Errorf("snapshot branch = %q", st.Work.Branch)
	}
	if roles := launchedRoles(t, logPath); len(roles) == 0 || roles[len(roles)-1] != "linker" || len(roles) != 2 {
		// starter, linker — and nothing after the interrupted Linker.
		t.Errorf("launched = %v, want the Starter then only the interrupted Linker", roles)
	}
	got, err := os.ReadFile(cdFile)
	if err != nil || !strings.Contains(string(got), "worktree") {
		t.Errorf("shell integration target = %q, %v; want the worktree path", got, err)
	}
}
