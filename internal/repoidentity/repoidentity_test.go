package repoidentity

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gitx"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func TestIdentifyOriginRemote(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "remote", "add", "origin", "https://example.com/project.git")

	id, err := Identify(gitx.Open(dir))
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if id != "https://example.com/project.git" {
		t.Errorf("Identify = %q, want the origin URL", id)
	}
}

func TestIdentifyNoRemoteSingleRoot(t *testing.T) {
	dir := newRepo(t)
	repo := gitx.Open(dir)
	want, err := repo.RevParse("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	id, err := Identify(repo)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if id != want {
		t.Errorf("Identify = %q, want root commit %q", id, want)
	}
}

func TestIdentifyNoRemoteMergedUnrelatedHistoriesDeterministic(t *testing.T) {
	// Two independent repos, each the source of a root commit; merge them in
	// both possible orders. Sorting before joining must make the resulting
	// composite key identical regardless of which side did the merging.
	mergeForward := func() string {
		repo1 := newRepo(t)
		repo2 := newRepo(t)
		git(t, repo2, "commit", "-q", "--amend", "--allow-empty", "-m", "root-2")
		git(t, repo1, "remote", "add", "other", repo2)
		git(t, repo1, "fetch", "-q", "other")
		git(t, repo1, "merge", "-q", "--allow-unrelated-histories", "-m", "merge", "other/main")
		id, err := Identify(gitx.Open(repo1))
		if err != nil {
			t.Fatalf("Identify: %v", err)
		}
		return id
	}
	mergeBackward := func() string {
		repo1 := newRepo(t)
		repo2 := newRepo(t)
		git(t, repo2, "commit", "-q", "--amend", "--allow-empty", "-m", "root-2")
		git(t, repo2, "remote", "add", "other", repo1)
		git(t, repo2, "fetch", "-q", "other")
		git(t, repo2, "merge", "-q", "--allow-unrelated-histories", "-m", "merge", "other/main")
		id, err := Identify(gitx.Open(repo2))
		if err != nil {
			t.Fatalf("Identify: %v", err)
		}
		return id
	}

	idForward := mergeForward()
	idBackward := mergeBackward()
	if idForward == "" || !strings.Contains(idForward, "+") {
		t.Fatalf("Identify(merged) = %q, want a '+'-joined composite key", idForward)
	}
	if idForward != idBackward {
		t.Errorf("composite keys differ by merge order: forward=%q backward=%q, want equal", idForward, idBackward)
	}
}

func TestIdentifyShallowNoRemoteFallsBackToPath(t *testing.T) {
	dir := newRepo(t)
	shallow := filepath.Join(t.TempDir(), "shallow")
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+dir, shallow)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1 file://: %v\n%s", err, out)
	}
	git(t, shallow, "remote", "remove", "origin")

	id, err := Identify(gitx.Open(shallow))
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	want, err := filepath.EvalSymlinks(shallow)
	if err != nil {
		t.Fatal(err)
	}
	if id != want {
		t.Errorf("Identify(shallow, no remote) = %q, want absolute path %q", id, want)
	}
}

func TestIdentifyStableAcrossReclone(t *testing.T) {
	origin := newRepo(t)
	git(t, origin, "remote", "add", "origin", "https://example.com/stable.git")
	id1, err := Identify(gitx.Open(origin))
	if err != nil {
		t.Fatal(err)
	}

	// A second "clone" of the same logical repository (recreated fresh, same
	// origin configured) yields the same identity.
	clone := newRepo(t)
	git(t, clone, "remote", "add", "origin", "https://example.com/stable.git")
	id2, err := Identify(gitx.Open(clone))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("Identify not stable across clones: %q vs %q", id1, id2)
	}
}
