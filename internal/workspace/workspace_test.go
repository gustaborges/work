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
	// Validate canonicalises the existing prefix (symlinks on macOS, 8.3 short
	// names on Windows), so compare against the same resolution.
	want := root
	if r, err := filepath.EvalSymlinks(filepath.Dir(root)); err == nil {
		want = filepath.Join(r, filepath.Base(root))
	}
	if got != want {
		t.Errorf("got %q want %q", got, want)
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

func TestValidateRejectsInsideGitRepo(t *testing.T) {
	repo := gittest.Repo(t)
	inside := filepath.Join(repo, "nested", "ws")
	if _, err := Validate(inside, nil); diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v", err)
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
