package skillinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTargets(t *testing.T) {
	root := t.TempDir()
	targets, err := testTargets(t, root)
	if err != nil || len(targets) != 2 {
		t.Fatalf("%v %v", targets, err)
	}
	if _, err = Resolve(Options{Scope: "invalid", Vendor: "claude"}); err == nil {
		t.Fatal("accepted invalid scope")
	}
	if err = os.Symlink(t.TempDir(), filepath.Join(root, ".claude")); err != nil {
		t.Skip("symlink capability unavailable")
	}
	ds, err := testTargets(t, root)
	if err != nil {
		t.Fatal(err)
	}
	if r := Inspect(ds[0], "embedded"); r.State != "conflict" {
		t.Fatal("followed descendant link", r)
	}
}

// Exercise multi-target engine invariants independently of public CLI selection.
func testTargets(t *testing.T, root string) ([]Destination, error) {
	t.Helper()
	claude, err := resolveAt(Options{Scope: "local", Vendor: "claude"}, root)
	if err != nil {
		return nil, err
	}
	codex, err := resolveAt(Options{Scope: "local", Vendor: "codex"}, root)
	return append(claude, codex...), err
}

func TestClaudeUserOverrideAndLocalAnchor(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "custom-claude")
	user, err := resolveAt(Options{Scope: "user", Vendor: "claude"}, root)
	if err != nil {
		t.Fatal(err)
	}
	local, err := resolveAt(Options{Scope: "local", Vendor: "claude"}, root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range user {
		if user[i].Path != filepath.Join(root, "custom-claude", "skills", user[i].Skill) {
			t.Fatal(user[i])
		}
		if local[i].Path != filepath.Join(root, ".claude", "skills", local[i].Skill) {
			t.Fatal(local[i])
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("resolution wrote files", entries, err)
	}
}
