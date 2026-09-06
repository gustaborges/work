package gitx

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo makes a repo with one commit on branch main.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func TestVersionAndPreflight(t *testing.T) {
	v, err := Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if !strings.Contains(v, ".") {
		t.Errorf("Version = %q", v)
	}
	if err := Preflight(); err != nil {
		t.Errorf("Preflight: %v", err)
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in       string
		maj, min int
	}{
		{"2.43.0", 2, 43},
		{"2.5", 2, 5},
		{"2.39.3.windows.1", 2, 39},
		{"2.39.3 (Apple Git-145)", 2, 39},
	}
	for _, c := range cases {
		maj, min, err := parseVersion(c.in)
		if err != nil {
			t.Errorf("parseVersion(%q): %v", c.in, err)
			continue
		}
		if maj != c.maj || min != c.min {
			t.Errorf("parseVersion(%q) = %d.%d, want %d.%d", c.in, maj, min, c.maj, c.min)
		}
	}
}

func TestIsWorkTreeAndBare(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)

	if ok, err := r.IsWorkTree(); err != nil || !ok {
		t.Errorf("IsWorkTree = %v, %v; want true, nil", ok, err)
	}
	if ok, err := r.IsBare(); err != nil || ok {
		t.Errorf("IsBare = %v, %v; want false, nil", ok, err)
	}

	notrepo := t.TempDir()
	if ok, err := Open(notrepo).IsWorkTree(); err != nil || ok {
		t.Errorf("IsWorkTree(non-repo) = %v, %v; want false, nil", ok, err)
	}

	bare := t.TempDir()
	git(t, bare, "init", "-q", "--bare")
	if ok, err := Open(bare).IsBare(); err != nil || !ok {
		t.Errorf("IsBare(bare) = %v, %v; want true, nil", ok, err)
	}
}

func TestHasCommit(t *testing.T) {
	repo := newRepo(t)
	if ok, err := Open(repo).HasCommit(); err != nil || !ok {
		t.Errorf("HasCommit = %v, %v; want true, nil", ok, err)
	}

	empty := t.TempDir()
	git(t, empty, "init", "-q", "-b", "main")
	if ok, err := Open(empty).HasCommit(); err != nil || ok {
		t.Errorf("HasCommit(unborn) = %v, %v; want false, nil", ok, err)
	}
}

func TestForEachRef(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "branch", "feature/x")
	// Fake a remote-tracking ref.
	git(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	git(t, repo, "update-ref", "refs/remotes/origin/HEAD", "HEAD")

	refs, err := Open(repo).ForEachRef("refs/heads", "refs/remotes")
	if err != nil {
		t.Fatalf("ForEachRef: %v", err)
	}
	got := map[string]string{}
	for _, r := range refs {
		got[r.Short] = r.Refname
	}
	if got["main"] != "refs/heads/main" {
		t.Errorf("missing local main: %+v", refs)
	}
	if got["feature/x"] != "refs/heads/feature/x" {
		t.Errorf("missing feature/x: %+v", refs)
	}
	if got["origin/main"] != "refs/remotes/origin/main" {
		t.Errorf("missing origin/main: %+v", refs)
	}
	if _, ok := got["origin"]; ok {
		t.Errorf("refs/remotes/origin/HEAD was not dropped: %+v", refs)
	}
	for _, r := range refs {
		if r.ObjectShort == "" {
			t.Errorf("ref %s has empty ObjectShort", r.Short)
		}
	}
}

func TestCheckRefFormat(t *testing.T) {
	if err := CheckRefFormat("refs/heads/feature/valid-name"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}
	for _, bad := range []string{"refs/heads/has space", "refs/heads/x~1", "refs/heads/a..b", "refs/heads/end.lock", "refs/heads/ctrl\tchar"} {
		if err := CheckRefFormat(bad); err == nil {
			t.Errorf("bad name %q accepted", bad)
		}
	}
}

func TestShowRefVerify(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	if ok, err := r.ShowRefVerify("refs/heads/main"); err != nil || !ok {
		t.Errorf("ShowRefVerify(main) = %v, %v", ok, err)
	}
	if ok, err := r.ShowRefVerify("refs/heads/nope"); err != nil || ok {
		t.Errorf("ShowRefVerify(nope) = %v, %v", ok, err)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)

	wtParent := t.TempDir()
	wt := filepath.Join(wtParent, "worktree")
	if err := r.WorktreeAdd(wt, "feature/wt", "main"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	list, err := r.WorktreeList()
	if err != nil {
		t.Fatalf("WorktreeList: %v", err)
	}
	var found bool
	for _, w := range list {
		if w.Branch == "refs/heads/feature/wt" {
			found = true
		}
	}
	if !found {
		t.Errorf("new worktree not listed: %+v", list)
	}

	if got, err := Open(wt).CurrentBranch(); err != nil || got != "feature/wt" {
		t.Errorf("CurrentBranch = %q, %v; want feature/wt", got, err)
	}

	// Branch started from main's tip.
	mainTip := git(t, repo, "rev-parse", "main")
	mb, err := Open(wt).MergeBase("feature/wt", "main")
	if err != nil || mb != mainTip {
		t.Errorf("MergeBase = %q, %v; want %q", mb, err, mainTip)
	}

	if err := r.WorktreeRemove(wt); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if err := r.BranchDelete("feature/wt"); err != nil {
		t.Fatalf("BranchDelete: %v", err)
	}
	if ok, _ := r.ShowRefVerify("refs/heads/feature/wt"); ok {
		t.Errorf("branch still present after delete")
	}
}

func TestWorktreeAddCollisionIsError(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	if err := r.WorktreeAdd(filepath.Join(t.TempDir(), "wt"), "main", "main"); err == nil {
		t.Errorf("WorktreeAdd on existing branch: want error")
	}
}

func TestRevParse(t *testing.T) {
	repo := newRepo(t)
	full, err := Open(repo).RevParse("HEAD")
	if err != nil {
		t.Fatalf("RevParse: %v", err)
	}
	if len(full) != 40 && len(full) != 64 {
		t.Errorf("RevParse(HEAD) = %q, want a full object name", full)
	}
}
