//go:build unix

// PTY coverage for `work start <plugin-specific-argument>` (F4 Phase 3, US2):
// with a plugin-installed specific Starter matching the argument, the full
// new-Work journey completes exactly as an F1 direct-path/name resolution
// would — the specific Starter was invoked (not the reference fallback), and
// work.starter names it.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// installPluginFixture runs `work plugin install <fixture-dir>` to
// completion (not through the pty — install itself is non-interactive) so a
// later `work start` pty session sees it already registered.
func installPluginFixture(t *testing.T, bin string, env []string, fixture string) {
	t.Helper()
	dir := fixtures.Prepare(t, fixture)
	cmd := exec.Command(bin, "plugin", "install", dir)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin install %s: %v\n%s", fixture, err, out)
	}
}

// TestStartByPluginStarterFullJourney: specific-starter's pattern
// (^demo-pr-1$) matches the SOURCE argument directly — no fallback, no
// selection step — and the resulting Work is indistinguishable in shape from
// an F1 direct-path/name creation, except work.starter naming the plugin
// Starter that actually ran (quickstart S7, SC-002). specific-starter's
// response also carries base_branch/start_modes, which Phase 4 reads but does
// not yet act on (consumption lands in Phase 5, research R8) — so the
// journey below is the ordinary new-Work one, driven by the usual flags.
func TestStartByPluginStarterFullJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)

	installPluginFixture(t, bin, env, "specific-starter")

	root := filepath.Join(homeDir, "src")
	repo := filepath.Join(root, "demo-pr-1")
	makeRepo(t, repo)
	writeRepositoryRoots(t, workHome, root)
	ws := filepath.Join(homeDir, "ws")

	c := newConsole(t, bin, env, "start", "demo-pr-1",
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("start via plugin starter exited %d, want 0\n%s", code, c.screen())
	}

	dir := filepath.Join(ws, "in-progress", "demo-pr-1_guided")
	wt := filepath.Join(dir, "worktree")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree not materialized at %s: %v", wt, err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "work-state.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var st struct {
		Schema int `json:"schema"`
		Work   struct {
			StartMode string `json:"start_mode"`
			Starter   string `json:"starter"`
		} `json:"work"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if st.Schema != 3 {
		t.Errorf("schema = %d, want 3", st.Schema)
	}
	if st.Work.StartMode != "new" {
		t.Errorf("start_mode = %q, want %q (start_modes consumption lands in Phase 5)", st.Work.StartMode, "new")
	}
	if st.Work.Starter != "specific-starter" {
		t.Errorf("starter = %q, want the plugin Starter's bare name (no collision to qualify)", st.Work.Starter)
	}
}

// TestStartFallsBackWhenOnlyReferencePackageInstalled is the F3-regression
// half of US2 (T043): with no specific Starter installed, Match always falls
// back to the reference package — `work start <path>` stays byte-identical
// to F3.
func TestStartFallsBackWhenOnlyReferencePackageInstalled(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src", "payments")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	c := newConsole(t, bin, env, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}", "--yes")
	answerFirstRunRootPrompt(c, t.TempDir())
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("exited %d, want 0\n%s", code, c.screen())
	}

	data, err := os.ReadFile(filepath.Join(ws, "in-progress", "payments_guided", "work-state.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var st struct {
		Work struct {
			Starter string `json:"starter"`
		} `json:"work"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if st.Work.Starter != "local-path-starter" {
		t.Errorf("starter = %q, want the reference fallback", st.Work.Starter)
	}
}
