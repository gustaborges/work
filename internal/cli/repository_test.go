package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/config"
)

func TestRepositoryNoSubcommandPrintsHelpExitZero(t *testing.T) {
	needSeed(t)
	out, _, code := runWork(t, "repository")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"locator", "policy", "root"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

func TestRepositoryJSONOnParentIsUsage(t *testing.T) {
	needSeed(t)
	_, _, code := runWork(t, "repository", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRepositoryLocatorListShowsSeedLocator(t *testing.T) {
	needSeed(t)
	out, _, code := runWork(t, "repository", "locator", "list")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "work-reference/filesystem-repository-locator") {
		t.Errorf("locator list missing the seed entry:\n%s", out)
	}
	if !strings.Contains(out, "(in policy)") {
		t.Errorf("seed locator should be marked in policy:\n%s", out)
	}
}

func TestRepositoryLocatorListJSON(t *testing.T) {
	needSeed(t)
	out, _, code := runWork(t, "repository", "locator", "list", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{`"ref"`, `"accepts"`, `"in_policy": true`} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %q:\n%s", want, out)
		}
	}
}

func TestRepositoryPolicyListShowsSeedAtPosition1(t *testing.T) {
	needSeed(t)
	out, _, code := runWork(t, "repository", "policy", "list")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "1  work-reference/filesystem-repository-locator  (available)") {
		t.Errorf("policy list = %q", out)
	}
}

func TestRepositoryPolicyMoveRequiresExactlyOneDirection(t *testing.T) {
	needSeed(t)
	if _, _, code := runWork(t, "repository", "policy", "move", "work-reference/filesystem-repository-locator"); code != 2 {
		t.Fatalf("neither flag: exit = %d, want 2", code)
	}
	if _, _, code := runWork(t, "repository", "policy", "move", "work-reference/filesystem-repository-locator",
		"--before", "x", "--after", "y"); code != 2 {
		t.Fatalf("both flags: exit = %d, want 2", code)
	}
}

func TestRepositoryPolicyAddUnknownLocatorIsUsage(t *testing.T) {
	needSeed(t)
	_, _, code := runWork(t, "repository", "policy", "add", "nonexistent/thing")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRepositoryPolicyRemoveKeepsComponentInstalled(t *testing.T) {
	needSeed(t)
	home := filepath.Join(t.TempDir(), "dothome")
	if _, _, code := runWorkHome(t, home, "repository", "policy", "remove",
		"work-reference/filesystem-repository-locator"); code != 0 {
		t.Fatalf("remove: exit = %d, want 0", code)
	}
	out, _, code := runWorkHome(t, home, "repository", "locator", "list")
	if code != 0 {
		t.Fatalf("locator list: exit = %d, want 0", code)
	}
	if !strings.Contains(out, "work-reference/filesystem-repository-locator") {
		t.Errorf("component should still be listed after policy removal:\n%s", out)
	}
	if strings.Contains(out, "(in policy)") {
		t.Errorf("component should show in_policy=false after removal:\n%s", out)
	}
}

func TestRepositoryPolicyMutationRejectsJSON(t *testing.T) {
	needSeed(t)
	if _, _, code := runWork(t, "repository", "policy", "add", "x/y", "--json"); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRepositoryRootAddListRemoveReplace(t *testing.T) {
	needSeed(t)
	home := filepath.Join(t.TempDir(), "dothome")
	d1, d2 := t.TempDir(), t.TempDir()

	if _, _, code := runWorkHome(t, home, "repository", "root", "add", d1, d2); code != 0 {
		t.Fatalf("add: exit = %d, want 0", code)
	}
	out, _, code := runWorkHome(t, home, "repository", "root", "list")
	if code != 0 || !strings.Contains(out, d1) || !strings.Contains(out, d2) {
		t.Fatalf("list = %q (exit %d)", out, code)
	}

	// Re-adding is a no-op.
	if _, _, code := runWorkHome(t, home, "repository", "root", "add", d1); code != 0 {
		t.Fatalf("re-add: exit = %d, want 0", code)
	}

	if _, _, code := runWorkHome(t, home, "repository", "root", "remove", d1); code != 0 {
		t.Fatalf("remove: exit = %d, want 0", code)
	}
	out, _, _ = runWorkHome(t, home, "repository", "root", "list")
	if strings.Contains(out, d1) {
		t.Errorf("removed root still listed: %s", out)
	}

	d3 := t.TempDir()
	if _, _, code := runWorkHome(t, home, "repository", "root", "replace", d3); code != 0 {
		t.Fatalf("replace: exit = %d, want 0", code)
	}
	out, _, _ = runWorkHome(t, home, "repository", "root", "list")
	if strings.TrimSpace(out) != d3 {
		t.Errorf("after replace, list = %q, want only %q", out, d3)
	}
}

func TestRepositoryRootAddRejectsWorkspaceOverlap(t *testing.T) {
	needSeed(t)
	home := filepath.Join(t.TempDir(), "dothome")
	ws := t.TempDir()

	// Configure a workspace root directly (there is no `work workspace set`
	// command; work start's own flag path covers persisting it in practice).
	cfgPath := filepath.Join(home, "config", "work.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Workspace = ws
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(ws, "clones")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runWorkHome(t, home, "repository", "root", "add", nested); code != 2 {
		t.Fatalf("root inside workspace: exit = %d, want 2", code)
	}
	if _, _, code := runWorkHome(t, home, "repository", "root", "add", ws); code != 2 {
		t.Fatalf("root == workspace: exit = %d, want 2", code)
	}
}

func TestRepositoryRootMutationRejectsJSON(t *testing.T) {
	needSeed(t)
	if _, _, code := runWork(t, "repository", "root", "add", t.TempDir(), "--json"); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRepositoryRootListEmptyPrintsNote(t *testing.T) {
	needSeed(t)
	_, errb, code := runWork(t, "repository", "root", "list")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(errb, "note:") {
		t.Errorf("stderr missing note: %s", errb)
	}
}
