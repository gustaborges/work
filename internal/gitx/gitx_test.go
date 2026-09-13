package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestIsDirty(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	wt := filepath.Join(t.TempDir(), "worktree")
	if err := r.WorktreeAdd(wt, "feature/dirty", "main"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	w := Open(wt)

	if dirty, err := w.IsDirty(); err != nil || dirty {
		t.Errorf("clean worktree: IsDirty = %v, %v; want false, nil", dirty, err)
	}

	// Untracked-only is dirty.
	if err := os.WriteFile(filepath.Join(wt, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := w.IsDirty(); err != nil || !dirty {
		t.Errorf("untracked-only: IsDirty = %v, %v; want true, nil", dirty, err)
	}

	// Tracked-modified is dirty.
	git(t, wt, "add", "scratch.txt")
	git(t, wt, "commit", "-q", "-m", "add scratch")
	if err := os.WriteFile(filepath.Join(wt, "scratch.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := w.IsDirty(); err != nil || !dirty {
		t.Errorf("tracked-modified: IsDirty = %v, %v; want true, nil", dirty, err)
	}
}

func TestWorktreeRemoveForceOnDirty(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)

	clean := filepath.Join(t.TempDir(), "clean")
	if err := r.WorktreeAdd(clean, "feature/clean", "main"); err != nil {
		t.Fatal(err)
	}
	if err := r.WorktreeRemove(clean); err != nil {
		t.Errorf("WorktreeRemove(clean): %v", err)
	}

	dirty := filepath.Join(t.TempDir(), "dirty")
	if err := r.WorktreeAdd(dirty, "feature/dirtywt", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, "u.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.WorktreeRemove(dirty); err != nil {
		t.Errorf("WorktreeRemove(dirty): %v", err)
	}
	if _, err := os.Stat(dirty); !os.IsNotExist(err) {
		t.Errorf("dirty worktree dir still present: %v", err)
	}
}

func TestWorktreePruneAfterManualDelete(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	wt := filepath.Join(t.TempDir(), "gone")
	if err := r.WorktreeAdd(wt, "feature/gone", "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if err := r.WorktreePrune(); err != nil {
		t.Fatalf("WorktreePrune: %v", err)
	}
	list, err := r.WorktreeList()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range list {
		if w.Path == wt {
			t.Errorf("pruned worktree still listed: %+v", list)
		}
	}
}

func TestWorktreeAddCollisionIsError(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	if err := r.WorktreeAdd(filepath.Join(t.TempDir(), "wt"), "main", "main"); err == nil {
		t.Errorf("WorktreeAdd on existing branch: want error")
	}
}

func TestSourceRepoOfAndWorktreeAddExisting(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)
	wt := filepath.Join(t.TempDir(), "wt")
	if err := r.WorktreeAdd(wt, "feature/keep", "main"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	src, err := SourceRepoOf(wt)
	if err != nil {
		t.Fatalf("SourceRepoOf: %v", err)
	}
	if resolved, _ := filepath.EvalSymlinks(src); resolved != mustEval(t, repo) {
		t.Errorf("SourceRepoOf = %q, want %q", src, repo)
	}

	// Remove the worktree (branch ref survives), then re-attach it with
	// WorktreeAddExisting — the archive compensation path.
	if err := r.WorktreeRemove(wt); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if err := r.WorktreeAddExisting(wt, "feature/keep"); err != nil {
		t.Fatalf("WorktreeAddExisting on an existing branch: %v", err)
	}
	if head, _ := Open(wt).CurrentBranch(); head != "feature/keep" {
		t.Errorf("re-attached worktree HEAD = %q, want feature/keep", head)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
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

func TestRemotesAndRemoteURL(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)

	if names, err := r.Remotes(); err != nil || len(names) != 0 {
		t.Fatalf("Remotes(none) = %v, %v", names, err)
	}
	if _, ok, err := r.RemoteURL("origin"); err != nil || ok {
		t.Fatalf("RemoteURL(absent) = ok=%v, err=%v, want ok=false, err=nil", ok, err)
	}

	git(t, repo, "remote", "add", "origin", "https://example.com/project.git")
	names, err := r.Remotes()
	if err != nil || !slices.Contains(names, "origin") {
		t.Fatalf("Remotes() = %v, %v, want [origin]", names, err)
	}
	url, ok, err := r.RemoteURL("origin")
	if err != nil || !ok || url != "https://example.com/project.git" {
		t.Fatalf("RemoteURL(origin) = %q, %v, %v", url, ok, err)
	}
}

func TestRootCommitsSingleRoot(t *testing.T) {
	repo := newRepo(t)
	hashes, err := Open(repo).RootCommits()
	if err != nil {
		t.Fatalf("RootCommits: %v", err)
	}
	if len(hashes) != 1 {
		t.Fatalf("RootCommits = %v, want exactly 1", hashes)
	}
	want, _ := Open(repo).RevParse("HEAD")
	if hashes[0] != want {
		t.Errorf("RootCommits = %v, want [%s]", hashes, want)
	}
}

func TestRootCommitsMergedUnrelatedHistories(t *testing.T) {
	repoA := newRepo(t)
	repoB := newRepo(t)
	// newRepo's commits are otherwise byte-identical (same fixed author/
	// committer identity and date, same message): rewrite B's message so the
	// two root commits actually hash differently.
	git(t, repoB, "commit", "-q", "--amend", "--allow-empty", "-m", "init-b")
	rootA, _ := Open(repoA).RevParse("HEAD")
	rootB, _ := Open(repoB).RevParse("HEAD")

	git(t, repoA, "remote", "add", "other", repoB)
	git(t, repoA, "fetch", "-q", "other")
	git(t, repoA, "merge", "-q", "--allow-unrelated-histories", "-m", "merge unrelated", "other/main")

	hashes, err := Open(repoA).RootCommits()
	if err != nil {
		t.Fatalf("RootCommits: %v", err)
	}
	want := []string{rootA, rootB}
	slices.Sort(want)
	slices.Sort(hashes)
	if !slices.Equal(hashes, want) {
		t.Errorf("RootCommits = %v, want %v (sorted)", hashes, want)
	}
}

func TestIsShallowAndDiscoverRepoRoot(t *testing.T) {
	repo := newRepo(t)
	r := Open(repo)

	if shallow, err := r.IsShallow(); err != nil || shallow {
		t.Fatalf("IsShallow(full clone) = %v, %v, want false", shallow, err)
	}

	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := DiscoverRepoRoot(sub)
	if err != nil {
		t.Fatalf("DiscoverRepoRoot: %v", err)
	}
	if mustEval(t, root) != mustEval(t, repo) {
		t.Errorf("DiscoverRepoRoot(sub) = %q, want %q", root, repo)
	}

	// A local-path source clone ignores --depth (git treats it as a fast local
	// hardlink clone); force the file:// transport to actually get a shallow
	// clone to check IsShallow against.
	clone := filepath.Join(t.TempDir(), "shallow")
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+repo, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1 file://: %v\n%s", err, out)
	}
	if shallow, err := Open(clone).IsShallow(); err != nil || !shallow {
		t.Errorf("IsShallow(shallow clone) = %v, %v, want true", shallow, err)
	}
}

func TestClone(t *testing.T) {
	repo := newRepo(t)
	want, _ := Open(repo).RevParse("HEAD")

	dest := filepath.Join(t.TempDir(), "cloned")
	head, err := Clone(repo, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if head != want {
		t.Errorf("Clone head = %q, want %q", head, want)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Errorf("clone destination has no .git: %v", err)
	}
}
