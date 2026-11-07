package gitx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gustaborges/work/internal/gittest"
)

// publish creates a repository with one branch and returns it with its file URL.
func publish(t *testing.T, branch string) (dir, url string) {
	t.Helper()
	dir = t.TempDir()
	gittest.Git(t, dir, "init", "--quiet", "-b", branch)
	gittest.Git(t, dir, "config", "user.email", "t@example.org")
	gittest.Git(t, dir, "config", "user.name", "T")
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	return dir, "file://" + filepath.ToSlash(dir)
}

func commitFile(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "--quiet", "-m", "c")
	return trim(gittest.Git(t, dir, "rev-parse", "HEAD"))
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

func TestDefaultBranchAndResolve(t *testing.T) {
	dir, url := publish(t, "trunk")
	c1 := commitFile(t, dir, "a.txt", "1", 0o644)
	ctx := context.Background()

	if b, err := DefaultBranch(ctx, url); err != nil || b != "trunk" {
		t.Fatalf("DefaultBranch = %q, %v", b, err)
	}
	if sha, err := ResolveBranch(ctx, url, "trunk"); err != nil || sha != c1 {
		t.Fatalf("ResolveBranch = %q, %v (want %s)", sha, err, c1)
	}
	for _, bad := range []string{"missing", "bad..name", ""} {
		if _, err := ResolveBranch(ctx, url, bad); !errors.Is(err, ErrUnavailable) {
			t.Errorf("ResolveBranch(%q) = %v, want ErrUnavailable", bad, err)
		}
	}
	if _, err := DefaultBranch(ctx, "file://"+filepath.ToSlash(t.TempDir())); !errors.Is(err, ErrUnavailable) {
		t.Errorf("DefaultBranch(non-repo) = %v", err)
	}
}

func TestUnavailableErrorNeverLeaksTheSource(t *testing.T) {
	secret := "https://user:s3cret@127.0.0.1:1/x.git"
	_, err := ResolveBranch(context.Background(), secret, "main")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if msg := err.Error(); msg != ErrUnavailable.Error() {
		t.Errorf("error leaks detail: %q", msg)
	}
	var ce *CommandError
	if errors.As(err, &ce) {
		t.Error("projection must not retain the raw CommandError")
	}
}

func TestFetchExactCommitIgnoresNewerTip(t *testing.T) {
	dir, url := publish(t, "main")
	c1 := commitFile(t, dir, "a.txt", "one", 0o644)
	c2 := commitFile(t, dir, "a.txt", "two", 0o644)
	ctx := context.Background()

	f, err := FetchCommit(ctx, url, "main", c1, t.TempDir())
	if err != nil {
		t.Fatalf("FetchCommit(old) = %v", err)
	}
	defer f.Close()
	dest := filepath.Join(t.TempDir(), "out")
	if _, err := f.Materialize(ctx, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "a.txt")); string(b) != "one" {
		t.Errorf("materialized %q, want the old commit's content (tip is %s)", b, c2)
	}
	if _, err := FetchCommit(ctx, url, "main", "0123456789abcdef0123456789abcdef01234567", t.TempDir()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unknown commit = %v, want ErrUnavailable and no fall-forward", err)
	}
	if _, err := FetchCommit(ctx, url, "main", "main", t.TempDir()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("non-object name = %v", err)
	}
}

func TestMaterializeRecordsExecutableBitAndLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and modes need Unix")
	}
	dir, url := publish(t, "main")
	commitFile(t, dir, "bin/run", "#!/bin/sh\n", 0o755)
	commitFile(t, dir, "plain.txt", "x", 0o644)
	if err := os.Symlink("plain.txt", filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "--quiet", "-m", "link")
	sha := trim(gittest.Git(t, dir, "rev-parse", "HEAD"))

	f, err := FetchCommit(context.Background(), url, "main", sha, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dest := filepath.Join(t.TempDir(), "out")
	exec, err := f.Materialize(context.Background(), dest)
	if err != nil {
		t.Fatal(err)
	}
	if !exec["bin/run"] || exec["plain.txt"] || len(exec) != 1 {
		t.Errorf("exec bits = %v", exec)
	}
	if target, err := os.Readlink(filepath.Join(dest, "alias")); err != nil || target != "plain.txt" {
		t.Errorf("link = %q, %v", target, err)
	}
}

func TestMaterializeRejectsSubmodules(t *testing.T) {
	dir, url := publish(t, "main")
	sub, _ := publish(t, "main")
	commitFile(t, sub, "s.txt", "s", 0o644)
	commitFile(t, dir, "a.txt", "a", 0o644)
	gittest.Git(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", sub, "vendor")
	gittest.Git(t, dir, "commit", "--quiet", "-m", "sub")
	sha := trim(gittest.Git(t, dir, "rev-parse", "HEAD"))
	f, err := FetchCommit(context.Background(), url, "main", sha, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Materialize(context.Background(), filepath.Join(t.TempDir(), "out")); !errors.Is(err, ErrUnsafeContent) {
		t.Errorf("submodule = %v, want ErrUnsafeContent", err)
	}
}

func TestCancellationIsNotUnavailable(t *testing.T) {
	_, url := publish(t, "main")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ResolveBranch(ctx, url, "main"); !errors.Is(err, context.Canceled) {
		t.Errorf("ResolveBranch = %v, want context.Canceled", err)
	}
}
