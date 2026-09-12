package locator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/tests/fixtures/locators"
)

func wantToken(t *testing.T, err error, c diag.Category) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with token %q, got nil", c.Token)
	}
	if got := diag.Token(err); got != c.Token {
		t.Fatalf("token = %q, want %q (%v)", got, c.Token, err)
	}
}

// setMatches points the ok/two/dupe/invalid fixtures at paths for this test.
func setMatches(t *testing.T, paths ...string) {
	t.Helper()
	if paths == nil {
		paths = []string{}
	}
	b, err := json.Marshal(paths)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCATOR_FIXTURE_MATCHES", string(b))
}

// newDeps builds Deps with a fresh registry and installs each named fixture
// under alias "fixture", using name == fixture unless overridden via
// installAs, and returns Deps with Policy in the given ref order.
func newDeps(t *testing.T, pluginsDir string, roots []string, refs ...policyEntry) Deps {
	t.Helper()
	reg := &registry.Registry{}
	var policy []string
	for _, e := range refs {
		locators.Install(t, reg, pluginsDir, e.alias, e.name, e.fixture, e.accepts)
		policy = append(policy, e.alias+"/"+e.name)
	}
	return Deps{PluginsDir: pluginsDir, Policy: policy, Roots: roots, Registry: reg}
}

type policyEntry struct {
	alias, name, fixture string
	accepts              []string
}

func entry(alias, fixture string, accepts ...string) policyEntry {
	return policyEntry{alias: alias, name: fixture, fixture: fixture, accepts: accepts}
}

func TestResolveEmptyReferenceIsNoEligibleLocator(t *testing.T) {
	plugins := t.TempDir()
	d := newDeps(t, plugins, nil, entry("fixture", "ok", "name"))
	_, err := Resolve(context.Background(), d, Reference{})
	wantToken(t, err, diag.NoEligibleLocator)
}

func TestResolveEmptyPolicyIsNoEligibleLocator(t *testing.T) {
	d := Deps{Registry: &registry.Registry{}}
	_, err := Resolve(context.Background(), d, Reference{Name: "payments"})
	wantToken(t, err, diag.NoEligibleLocator)
}

func TestResolveIneligiblePolicyIsNoEligibleLocator(t *testing.T) {
	plugins := t.TempDir()
	// The only policy entry accepts "name"; the reference carries only
	// git_fetch_urls, so it is never invoked (FR-006) and no run occurs.
	d := newDeps(t, plugins, nil, entry("fixture", "ok", "name"))
	_, err := Resolve(context.Background(), d, Reference{GitFetchURLs: []string{"https://example.com/x.git"}})
	wantToken(t, err, diag.NoEligibleLocator)
}

func TestResolveUnavailablePolicyEntryIsSkipped(t *testing.T) {
	plugins := t.TempDir()
	reg := &registry.Registry{}
	// A hand-edited policy naming a never-installed component (R12).
	d := Deps{PluginsDir: plugins, Policy: []string{"ghost/vanished"}, Registry: reg}
	_, err := Resolve(context.Background(), d, Reference{Name: "payments"})
	wantToken(t, err, diag.NoEligibleLocator)
}

func TestResolveSingleMatch(t *testing.T) {
	repo := gittest.Repo(t)
	setMatches(t, repo)

	plugins := t.TempDir()
	d := newDeps(t, plugins, []string{t.TempDir()}, entry("fixture", "ok", "name"))
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if out.Resolved != resolved {
		t.Errorf("Resolved = %q, want %q", out.Resolved, resolved)
	}
	if out.Candidates != nil {
		t.Errorf("Candidates = %v, want nil", out.Candidates)
	}
}

func TestResolveEmptyMatchesAdvancesToNextEntry(t *testing.T) {
	repo := gittest.Repo(t)
	setMatches(t, repo)

	plugins := t.TempDir()
	d := newDeps(t, plugins, nil,
		entry("fixtureA", "empty", "name"),
		entry("fixtureB", "ok", "name"),
	)
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if out.Resolved != resolved {
		t.Errorf("Resolved = %q, want %q", out.Resolved, resolved)
	}
}

func TestResolveAllEmptyIsNoRepositoryFound(t *testing.T) {
	plugins := t.TempDir()
	d := newDeps(t, plugins, nil, entry("fixture", "empty", "name"))
	_, err := Resolve(context.Background(), d, Reference{Name: "project"})
	wantToken(t, err, diag.NoRepositoryFound)
}

func TestResolveFirstNonEmptyEndsTraversalNoFallback(t *testing.T) {
	repoA, repoB := gittest.Repo(t), gittest.Repo(t)
	setMatches(t, repoA, repoB)

	plugins := t.TempDir()
	// "two" (first) returns two matches and ends the chain; "boom" (second)
	// must never be consulted, or this would fail with locator-failed
	// instead of an ambiguous outcome (FR-011, no fallback after results).
	d := newDeps(t, plugins, nil,
		entry("fixtureA", "two", "name"),
		entry("fixtureB", "boom", "name"),
	)
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(out.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want 2 entries", out.Candidates)
	}
}

func TestResolveOperationalFailureHaltsNoFallback(t *testing.T) {
	repo := gittest.Repo(t)
	setMatches(t, repo)

	plugins := t.TempDir()
	// "boom" (first) halts the chain; "ok" (second) must never run, or this
	// would resolve instead of failing (ADR-0015: no continue_on_locator_error).
	d := newDeps(t, plugins, nil,
		entry("fixtureA", "boom", "name"),
		entry("fixtureB", "ok", "name"),
	)
	_, err := Resolve(context.Background(), d, Reference{Name: "project"})
	wantToken(t, err, diag.LocatorFailed)
}

func TestResolveDedupCollapsesRepeatedPath(t *testing.T) {
	repo := gittest.Repo(t)
	setMatches(t, repo)

	plugins := t.TempDir()
	// "dupe" returns repo twice; after dedup only one valid candidate
	// remains, so the outcome is a single resolved match, not ambiguous
	// (FR-017, SC-009).
	d := newDeps(t, plugins, nil, entry("fixture", "dupe", "name"))
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if out.Resolved != resolved {
		t.Errorf("Resolved = %q, want %q (single after dedup)", out.Resolved, resolved)
	}
}

func TestResolveDedupCollapsesSymlinkVariant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privileges on windows")
	}
	repo := gittest.Repo(t)
	link := filepath.Join(t.TempDir(), "link-to-repo")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	setMatches(t, repo, link)

	plugins := t.TempDir()
	d := newDeps(t, plugins, nil, entry("fixture", "two", "name"))
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(repo)
	if out.Resolved != resolved {
		t.Errorf("Resolved = %q, want %q (path/symlink collapse)", out.Resolved, resolved)
	}
}

func TestResolveAllCandidatesInvalidIsCandidateInvalid(t *testing.T) {
	notARepo := t.TempDir() // no .git — reporef.ValidatePath rejects it
	setMatches(t, notARepo)

	plugins := t.TempDir()
	d := newDeps(t, plugins, nil, entry("fixture", "invalid", "name"))
	_, err := Resolve(context.Background(), d, Reference{Name: "project"})
	wantToken(t, err, diag.RepositoryCandidateInvalid)
}

func TestResolveAmbiguousCandidatesCarrySecondaryLine(t *testing.T) {
	repoA, repoB := gittest.Repo(t), gittest.Repo(t)
	setMatches(t, repoA, repoB)

	plugins := t.TempDir()
	d := newDeps(t, plugins, nil, entry("fixture", "two", "name"))
	out, err := Resolve(context.Background(), d, Reference{Name: "project"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(out.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want 2", out.Candidates)
	}
	for _, c := range out.Candidates {
		if c.Remote == "" {
			t.Errorf("candidate %+v has empty secondary line", c)
		}
	}
}

func TestEligibleProjectsOnlyAcceptedAndPresentFields(t *testing.T) {
	comp := registry.Component{Accepts: []string{"name", "query"}}
	proj, ok := eligible(comp, Reference{Name: "acme", Query: "billing", GitFetchURLs: []string{"u"}})
	if !ok {
		t.Fatal("want eligible")
	}
	if proj.Name != "acme" || proj.Query != "billing" {
		t.Errorf("proj = %+v", proj)
	}
	if len(proj.GitFetchURLs) != 0 {
		t.Errorf("proj carries an unaccepted field: %+v", proj)
	}
}

func TestEligibleFalseWhenNoIntersection(t *testing.T) {
	comp := registry.Component{Accepts: []string{"name"}}
	_, ok := eligible(comp, Reference{Query: "billing"})
	if ok {
		t.Fatal("want ineligible: accepts and present fields do not intersect")
	}
}

func TestAvailableComponentRequiresRepositoryLocatorRole(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		{Alias: "a", Name: "b", Role: registry.RoleStarter},
	}}
	if _, ok := availableComponent(reg, "a/b"); ok {
		t.Fatal("want unavailable: wrong role")
	}
}

func TestAvailableComponentRejectsMalformedRef(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		{Alias: "a", Name: "b", Role: registry.RoleRepositoryLocator},
	}}
	if _, ok := availableComponent(reg, "no-slash"); ok {
		t.Fatal("want unavailable: malformed ref")
	}
}
