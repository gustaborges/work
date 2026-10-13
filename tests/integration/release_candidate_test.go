package integration

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func releaseGit(t *testing.T, h *f6Harness, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, h.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func releaseRepo(t *testing.T, h *f6Harness, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, h, path, "init", "-q", "-b", "main")
	releaseGit(t, h, path, "commit", "-q", "--allow-empty", "-m", "initial")
}

// This cumulative F7 journey uses product commands and offline native sources.
// Earlier demonstration tests remain independent regression gates.
func TestF7RoadmapJourney(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	ctx := context.Background()
	repo := filepath.Join(h.root, "repos", "demo")
	releaseRepo(t, h, repo)
	// Inspection precedes setup and does not even create the home.
	h.run(t, h.root, 0, "plugin", "list", "--json")
	if _, err := os.Stat(home.Root()); !os.IsNotExist(err) {
		t.Fatal("read initialized home", err)
	}
	ws := filepath.Join(h.root, "workspace")
	h.run(t, h.root, 0, "start", repo, "--workspace", ws, "--base", "main", "--slug", "before", "--prefix", "{slug}", "--yes")
	s, err := adminstate.Read(ctx, home)
	if err != nil || !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, []string{bootstrap.Alias + "/filesystem-repository-locator"}) {
		t.Fatal(s, err)
	}
	work := filepath.Join(ws, "in-progress", "demo_before")
	workBefore := adminTree(t, work)
	local := fixtures.Prepare(t, "admin-suite")
	remote := fixtures.Prepare(t, "admin-suite")
	fixtures.Revise(t, remote, "release-blue", func(m map[string]any) { m["conventions"] = []any{} })
	adminCommitSource(t, remote)
	remotePath := filepath.ToSlash(remote)
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}
	remoteURL := (&url.URL{Scheme: "file", Path: remotePath}).String()
	h.run(t, h.root, 0, "plugin", "install", local, "--as", "local-admin")
	h.run(t, h.root, 0, "plugin", "install", remoteURL, "--as", "remote-admin")
	h.run(t, h.root, 0, "repository", "policy", "replace", "local-admin/clone", "remote-admin/clone")
	h.run(t, h.root, 0, "repository", "root", "add", filepath.Dir(repo))
	h.run(t, repo, 0, "convention", "set", "admin-team")
	policy, _ := os.ReadFile(home.ConfigFile())
	h.run(t, h.root, 0, "plugin", "disable", "local-admin")
	out, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if !strings.Contains(out, "package-disabled") {
		t.Fatal(out)
	}
	out, _ = h.run(t, repo, 0, "convention", "show", "--json")
	if !strings.Contains(out, "provider-unavailable") || !strings.Contains(out, "admin-team") {
		t.Fatal(out)
	}
	h.run(t, h.root, 0, "plugin", "enable", "local-admin")
	if b, _ := os.ReadFile(home.ConfigFile()); string(b) != string(policy) {
		t.Fatal("toggle changed policy")
	}
	fixtures.Revise(t, local, "release-green", nil)
	fixtures.Revise(t, remote, "release-green", nil)
	adminCommitSource(t, remote)
	before := adminTree(t, home.Root())
	h.run(t, h.root, 0, "plugin", "update", "--check", "local-admin", "remote-admin", "--json")
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("check changed home")
	}
	h.run(t, h.root, 0, "plugin", "update", "local-admin", "remote-admin", "--yes")
	s, err = adminstate.Read(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"local-admin", "remote-admin"} {
		p, ok := s.Registry.PackageByAlias(alias)
		if !ok || p.Version != "release-green" || p.ContentDigest == "" {
			t.Fatal(p)
		}
	}
	before = adminTree(t, home.Root())
	h.run(t, h.root, 52, "plugin", "uninstall", "local-admin", "remote-admin", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("refusal changed home")
	}
	h.run(t, h.root, 0, "plugin", "uninstall", "local-admin", "remote-admin", "--yes", "--remove-policy-references")
	out, _ = h.run(t, repo, 0, "convention", "show", "--json")
	if !strings.Contains(out, "provider-unavailable") {
		t.Fatal(out)
	}
	h.run(t, repo, 0, "convention", "set", "freeform")
	h.run(t, h.root, 0, "plugin", "uninstall", bootstrap.Alias, "--yes")
	if err := bootstrap.EnsureSeed(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home.PluginsDir(), bootstrap.Alias)); !os.IsNotExist(err) {
		t.Fatal("removed seed resurrected", err)
	}
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	s, err = adminstate.Read(ctx, home)
	if err != nil || len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Fatal(s, err)
	}
	if !reflect.DeepEqual(workBefore, adminTree(t, work)) {
		t.Fatal("administration changed existing Work")
	}
	// Existing reconcile restores the index without rewriting canonical state.
	if err := os.Remove(home.DBFile()); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 2, "resume")
	if !reflect.DeepEqual(workBefore, adminTree(t, work)) {
		t.Fatal("rebuild changed Work")
	}
	db, err := projection.Open(home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version, err := db.UserVersion()
	if err != nil || version != 3 {
		t.Fatal(version, err)
	}
	rows, err := db.List()
	if err != nil || len(rows) != 1 || rows[0].DirPath != work {
		t.Fatal(rows, err)
	}
	var snapshot struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(workBefore["work-state.json"], "file:")), &snapshot); err != nil || snapshot.Schema != 3 {
		t.Fatal(snapshot, err)
	}
}

// Divergent real local and remote refs with the same short name must retain
// their explicit identity across administration and Work creation.
func TestReleaseCandidateLocalRemoteBranches(t *testing.T) {
	h := newAdminHarness(t)
	upstream, repo := filepath.Join(h.root, "upstream"), filepath.Join(h.root, "clone")
	releaseRepo(t, h, upstream)
	releaseGit(t, h, h.root, "clone", "-q", upstream, repo)
	releaseGit(t, h, repo, "commit", "-q", "--allow-empty", "-m", "local divergence")
	local, remote := releaseGit(t, h, repo, "rev-parse", "main"), releaseGit(t, h, repo, "rev-parse", "origin/main")
	if local == remote {
		t.Fatal("fixture refs did not diverge")
	}
	ws := filepath.Join(h.root, "workspace")
	for i, test := range []struct{ base, slug, tip string }{{"main", "local", local}, {"origin/main", "remote", remote}} {
		args := []string{"start", repo, "--base", test.base, "--slug", test.slug, "--prefix", "{slug}", "--yes"}
		if i == 0 {
			args = append(args, "--workspace", ws)
		}
		h.run(t, h.root, 0, args...)
		path := filepath.Join(ws, "in-progress", "clone_"+test.slug)
		if tip := releaseGit(t, h, filepath.Join(path, "worktree"), "rev-parse", "HEAD"); tip != test.tip {
			t.Fatal(tip, test.tip)
		}
		b, err := os.ReadFile(filepath.Join(path, "work-state.json"))
		if err != nil || !strings.Contains(string(b), `"base_branch": "`+test.base+`"`) {
			t.Fatal(string(b), err)
		}
	}
	before := adminTree(t, ws)
	h.run(t, h.root, 14, "start", repo, "--base", "main", "--slug", "local", "--prefix", "{slug}", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, ws)) {
		t.Fatal("branch collision changed existing Works")
	}
}
