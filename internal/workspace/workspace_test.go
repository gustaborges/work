package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/workhome"
)

func TestValidateCreatableDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	got, err := Validate(root, nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Validate returns the plain absolute form, not a symlink-resolved one.
	if got != root {
		t.Errorf("got %q want %q", got, root)
	}
}

func TestValidateExistingWritableDir(t *testing.T) {
	root := t.TempDir()
	if _, err := Validate(root, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f")
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := Validate(f, nil); diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateAllowsInsideGitRepo(t *testing.T) {
	repo := gittest.Repo(t)
	inside := filepath.Join(repo, "nested", "ws")
	got, err := Validate(inside, nil)
	if err != nil {
		t.Fatalf("Validate rejected a git-enclosed root: %v", err)
	}
	if got != inside {
		t.Errorf("got %q want %q", got, inside)
	}
}

func TestValidateRejectsInsideRepositoryRoot(t *testing.T) {
	rootsParent := t.TempDir()
	repoRoot := filepath.Join(rootsParent, "src")
	os.MkdirAll(repoRoot, 0o755)
	ws := filepath.Join(repoRoot, "work")
	if _, err := Validate(ws, []string{repoRoot}); diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v", err)
	}
}

// TestValidateRejectsRootInsideWorkspace is the mirror of
// TestValidateRejectsInsideRepositoryRoot: a configured repository root
// nested *inside* the candidate workspace root is rejected too (research
// R13, FR-020) — overlap is bidirectional.
func TestValidateRejectsRootInsideWorkspace(t *testing.T) {
	ws := t.TempDir()
	root := filepath.Join(ws, "clones")
	os.MkdirAll(root, 0o755)
	if _, err := Validate(ws, []string{root}); diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v, want usage", err)
	}
}

func TestValidateRejectsEqualPaths(t *testing.T) {
	same := t.TempDir()
	if _, err := Validate(same, []string{same}); diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v, want usage", err)
	}
}

func TestOverlapsBothDirections(t *testing.T) {
	parent := t.TempDir()
	inner := filepath.Join(parent, "inner")
	os.MkdirAll(inner, 0o755)
	unrelated := t.TempDir()

	if ok, err := Overlaps(parent, inner); err != nil || !ok {
		t.Errorf("Overlaps(parent, inner) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := Overlaps(inner, parent); err != nil || !ok {
		t.Errorf("Overlaps(inner, parent) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := Overlaps(parent, parent); err != nil || !ok {
		t.Errorf("Overlaps(parent, parent) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := Overlaps(parent, unrelated); err != nil || ok {
		t.Errorf("Overlaps(parent, unrelated) = %v, %v; want false, nil", ok, err)
	}
}

func TestPersistWritesConfigAndLayout(t *testing.T) {
	h := workhome.At(filepath.Join(t.TempDir(), "dotwork"))
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "ws")
	if err := Persist(h, root); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workspace != root {
		t.Errorf("config.workspace = %q, want %q", cfg.Workspace, root)
	}
	for _, sub := range []string{InProgressDir, ArchivedDir} {
		if _, err := os.Stat(filepath.Join(root, sub)); err != nil {
			t.Errorf("missing %s: %v", sub, err)
		}
	}
}

func TestSuggestDefault(t *testing.T) {
	got, err := SuggestDefault()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "work" {
		t.Errorf("SuggestDefault = %q", got)
	}
}
