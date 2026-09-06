package reporef

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
)

func wantCategory(t *testing.T, err error, c diag.Category) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with token %q, got nil", c.Token)
	}
	if got := diag.Token(err); got != c.Token {
		t.Fatalf("token = %q, want %q (%v)", got, c.Token, err)
	}
}

func TestValidatePathHappy(t *testing.T) {
	repo := gittest.Repo(t)
	got, err := ValidatePath(repo)
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if got != resolved {
		t.Errorf("got %q, want %q", got, resolved)
	}
}

func TestValidatePathWithSpacesAndUnicode(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "my repo — café")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	gittest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	if _, err := ValidatePath(dir); err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
}

func TestValidatePathSymlink(t *testing.T) {
	repo := gittest.Repo(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	got, err := ValidatePath(link)
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if got != resolved {
		t.Errorf("got %q, want %q", got, resolved)
	}
}

func TestValidatePathRejections(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := ValidatePath("  ")
		wantCategory(t, err, diag.InvalidPath)
	})
	t.Run("missing", func(t *testing.T) {
		_, err := ValidatePath(filepath.Join(t.TempDir(), "nope"))
		wantCategory(t, err, diag.InvalidPath)
	})
	t.Run("dot-dot to nowhere", func(t *testing.T) {
		_, err := ValidatePath(filepath.Join(t.TempDir(), "..", "definitely-missing-xyz"))
		wantCategory(t, err, diag.InvalidPath)
	})
	t.Run("file not dir", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "f")
		os.WriteFile(f, []byte("x"), 0o644)
		_, err := ValidatePath(f)
		wantCategory(t, err, diag.InvalidPath)
	})
	t.Run("plain dir not a repo", func(t *testing.T) {
		_, err := ValidatePath(t.TempDir())
		wantCategory(t, err, diag.UnusableRepo)
	})
	t.Run("bare repo", func(t *testing.T) {
		bare := t.TempDir()
		gittest.Git(t, bare, "init", "-q", "--bare")
		_, err := ValidatePath(bare)
		wantCategory(t, err, diag.UnusableRepo)
	})
	t.Run("no commits", func(t *testing.T) {
		empty := t.TempDir()
		gittest.Git(t, empty, "init", "-q", "-b", "main")
		_, err := ValidatePath(empty)
		wantCategory(t, err, diag.UnusableRepo)
	})
}
