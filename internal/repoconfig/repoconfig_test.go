package repoconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
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

func newCfg(workspace string, roots ...string) *config.Config {
	if roots == nil {
		roots = []string{}
	}
	return &config.Config{
		Workspace:            workspace,
		RepositoryRoots:      roots,
		RepositoryResolution: config.RepositoryResolution{Locators: []string{}},
	}
}

// --- Roots ---

func TestListRootsEmptyIsEmptyNotNil(t *testing.T) {
	cfg := newCfg("")
	got := ListRoots(cfg)
	if got == nil || len(got) != 0 {
		t.Errorf("ListRoots = %#v, want empty non-nil slice", got)
	}
}

func TestAddRootsAbsolutizesAndDedups(t *testing.T) {
	cfg := newCfg("")
	d1, d2 := t.TempDir(), t.TempDir()
	if err := AddRoots(cfg, []string{d1, d2, d1}); err != nil {
		t.Fatalf("AddRoots: %v", err)
	}
	if len(cfg.RepositoryRoots) != 2 {
		t.Fatalf("roots = %v, want 2 entries (dedup)", cfg.RepositoryRoots)
	}
	if cfg.RepositoryRoots[0] != d1 || cfg.RepositoryRoots[1] != d2 {
		t.Errorf("roots = %v, want [%s %s] in order", cfg.RepositoryRoots, d1, d2)
	}
}

func TestAddRootsRejectsNonDirectory(t *testing.T) {
	cfg := newCfg("")
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, []byte("x"), 0o644)
	err := AddRoots(cfg, []string{f})
	wantToken(t, err, diag.Usage)
	if len(cfg.RepositoryRoots) != 0 {
		t.Errorf("cfg mutated on rejection: %v", cfg.RepositoryRoots)
	}
}

func TestAddRootsRejectsMissingDirectory(t *testing.T) {
	cfg := newCfg("")
	err := AddRoots(cfg, []string{filepath.Join(t.TempDir(), "nope")})
	wantToken(t, err, diag.Usage)
}

func TestAddRootsPartialBatchLeavesCfgUntouched(t *testing.T) {
	cfg := newCfg("")
	good := t.TempDir()
	bad := filepath.Join(t.TempDir(), "missing")
	err := AddRoots(cfg, []string{good, bad})
	wantToken(t, err, diag.Usage)
	if len(cfg.RepositoryRoots) != 0 {
		t.Errorf("partial write leaked into cfg: %v", cfg.RepositoryRoots)
	}
}

func TestAddRootsRejectsOverlapWithWorkspaceRootInsideRoot(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")
	os.MkdirAll(ws, 0o755)
	cfg := newCfg(ws)
	err := AddRoots(cfg, []string{root})
	wantToken(t, err, diag.Usage)
}

func TestAddRootsRejectsOverlapWithRootInsideWorkspace(t *testing.T) {
	ws := t.TempDir()
	root := filepath.Join(ws, "clones")
	os.MkdirAll(root, 0o755)
	cfg := newCfg(ws)
	err := AddRoots(cfg, []string{root})
	wantToken(t, err, diag.Usage)
}

func TestAddRootsAllowsUnrelatedWorkspace(t *testing.T) {
	cfg := newCfg(t.TempDir())
	if err := AddRoots(cfg, []string{t.TempDir()}); err != nil {
		t.Fatalf("AddRoots: %v", err)
	}
}

func TestAddRootsExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	sub, err := os.MkdirTemp(home, "repoconfig-test-*")
	if err != nil {
		t.Skip("cannot create dir under home:", err)
	}
	t.Cleanup(func() { os.RemoveAll(sub) })

	cfg := newCfg("")
	tilde := "~/" + filepath.Base(sub)
	if err := AddRoots(cfg, []string{tilde}); err != nil {
		t.Fatalf("AddRoots: %v", err)
	}
	if len(cfg.RepositoryRoots) != 1 || !filepath.IsAbs(cfg.RepositoryRoots[0]) {
		t.Errorf("roots = %v, want one absolute path", cfg.RepositoryRoots)
	}
}

func TestRemoveRootsCanonicalMatchAndNoOpOnAbsent(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	cfg := newCfg("", d1, d2)
	if err := RemoveRoots(cfg, []string{d1, "/never/configured"}); err != nil {
		t.Fatalf("RemoveRoots: %v", err)
	}
	if len(cfg.RepositoryRoots) != 1 || cfg.RepositoryRoots[0] != d2 {
		t.Errorf("roots = %v, want [%s]", cfg.RepositoryRoots, d2)
	}
}

func TestReplaceRootsValidatesWholeSetAtomically(t *testing.T) {
	d1 := t.TempDir()
	cfg := newCfg("", d1)
	bad := filepath.Join(t.TempDir(), "missing")
	err := ReplaceRoots(cfg, []string{t.TempDir(), bad})
	wantToken(t, err, diag.Usage)
	if len(cfg.RepositoryRoots) != 1 || cfg.RepositoryRoots[0] != d1 {
		t.Errorf("roots changed on rejected replace: %v", cfg.RepositoryRoots)
	}
}

func TestReplaceRootsAppliesNewSet(t *testing.T) {
	cfg := newCfg("", t.TempDir())
	d2 := t.TempDir()
	if err := ReplaceRoots(cfg, []string{d2}); err != nil {
		t.Fatalf("ReplaceRoots: %v", err)
	}
	if len(cfg.RepositoryRoots) != 1 || cfg.RepositoryRoots[0] != d2 {
		t.Errorf("roots = %v, want [%s]", cfg.RepositoryRoots, d2)
	}
}

func TestNeedsSetup(t *testing.T) {
	cases := []struct {
		cfg           *config.Config
		wantWorkspace bool
		wantRoot      bool
	}{
		{newCfg(""), true, true},
		{newCfg("/ws"), false, true},
		{newCfg("", "/root"), true, false},
		{newCfg("/ws", "/root"), false, false},
	}
	for _, c := range cases {
		gotWS, gotRoot := NeedsSetup(c.cfg)
		if gotWS != c.wantWorkspace || gotRoot != c.wantRoot {
			t.Errorf("NeedsSetup(%+v) = (%v, %v), want (%v, %v)",
				c.cfg, gotWS, gotRoot, c.wantWorkspace, c.wantRoot)
		}
	}
}

func TestValidateRootNoPresentImport(t *testing.T) {
	// Purely a compile-time guarantee (data-model.md §5): ValidateRoot must
	// not need an interactive session. Exercised here as a normal call.
	cfg := newCfg("")
	dir := t.TempDir()
	abs, err := ValidateRoot(cfg, dir)
	if err != nil || abs != dir {
		t.Fatalf("ValidateRoot(%q) = (%q, %v)", dir, abs, err)
	}
}

// --- Policy ---

func locatorComponent(alias, name string, accepts ...string) registry.Component {
	return registry.Component{
		Alias: alias, Name: name, Role: registry.RoleRepositoryLocator,
		Accepts: accepts, DisplayName: name,
	}
}

func TestListPolicyMarksUnavailableWithoutDropping(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		locatorComponent("work-reference", "filesystem-repository-locator", "name"),
	}}
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{
		"work-reference/filesystem-repository-locator",
		"ghost/vanished",
	}
	got := ListPolicy(cfg, reg)
	if len(got) != 2 {
		t.Fatalf("ListPolicy = %+v, want 2 entries", got)
	}
	if !got[0].Available || got[0].Position != 1 {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].Available || got[1].Position != 2 {
		t.Errorf("unavailable entry should be kept and marked: %+v", got[1])
	}
}

func TestAddPolicyRejectsUnknownLocator(t *testing.T) {
	cfg := newCfg("")
	reg := &registry.Registry{}
	err := AddPolicy(cfg, reg, "acme/corp-index", "", "")
	wantToken(t, err, diag.Usage)
}

func TestAddPolicyAppendsAndIsNoOpWhenPresent(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		locatorComponent("a", "one"), locatorComponent("b", "two"),
	}}
	cfg := newCfg("")
	if err := AddPolicy(cfg, reg, "a/one", "", ""); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	if err := AddPolicy(cfg, reg, "a/one", "", ""); err != nil {
		t.Fatalf("AddPolicy (no-op): %v", err)
	}
	if len(cfg.RepositoryResolution.Locators) != 1 {
		t.Fatalf("locators = %v, want 1 (no duplicate)", cfg.RepositoryResolution.Locators)
	}
}

func TestAddPolicyBareComponentResolvesWhenUnambiguous(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{locatorComponent("acme", "corp-index")}}
	cfg := newCfg("")
	if err := AddPolicy(cfg, reg, "corp-index", "", ""); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	if cfg.RepositoryResolution.Locators[0] != "acme/corp-index" {
		t.Errorf("locators = %v, want qualified acme/corp-index", cfg.RepositoryResolution.Locators)
	}
}

func TestAddPolicyBareComponentAmbiguousIsUsageError(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		locatorComponent("a", "index"), locatorComponent("b", "index"),
	}}
	cfg := newCfg("")
	err := AddPolicy(cfg, reg, "index", "", "")
	wantToken(t, err, diag.Usage)
}

func TestAddPolicyPositioningBeforeAfter(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		locatorComponent("a", "one"), locatorComponent("b", "two"), locatorComponent("c", "three"),
	}}
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one", "b/two"}

	if err := AddPolicy(cfg, reg, "c/three", "b/two", ""); err != nil {
		t.Fatalf("AddPolicy --before: %v", err)
	}
	want := []string{"a/one", "c/three", "b/two"}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestAddPolicyBothBeforeAndAfterIsUsageError(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{locatorComponent("a", "one")}}
	cfg := newCfg("")
	err := AddPolicy(cfg, reg, "a/one", "x", "y")
	wantToken(t, err, diag.Usage)
}

func TestRemovePolicyDropsAndKeepsComponentInstalled(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one", "b/two"}
	if err := RemovePolicy(cfg, []string{"a/one"}); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}
	if !equal(cfg.RepositoryResolution.Locators, []string{"b/two"}) {
		t.Errorf("locators = %v", cfg.RepositoryResolution.Locators)
	}
}

func TestRemovePolicyAbsentIsNoOp(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one"}
	if err := RemovePolicy(cfg, []string{"z/nope"}); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}
	if !equal(cfg.RepositoryResolution.Locators, []string{"a/one"}) {
		t.Errorf("locators = %v, want unchanged", cfg.RepositoryResolution.Locators)
	}
}

func TestMovePolicyRequiresExactlyOneDirection(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one", "b/two"}
	wantToken(t, MovePolicy(cfg, "a/one", "", ""), diag.Usage)
	wantToken(t, MovePolicy(cfg, "a/one", "b/two", "b/two"), diag.Usage)
}

func TestMovePolicyReorders(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one", "b/two", "c/three"}
	if err := MovePolicy(cfg, "c/three", "a/one", ""); err != nil {
		t.Fatalf("MovePolicy: %v", err)
	}
	want := []string{"c/three", "a/one", "b/two"}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestMovePolicyUnknownRefIsUsageError(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one"}
	wantToken(t, MovePolicy(cfg, "z/nope", "a/one", ""), diag.Usage)
	wantToken(t, MovePolicy(cfg, "a/one", "z/nope", ""), diag.Usage)
}

func TestReplacePolicyValidatesAndDedups(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		locatorComponent("a", "one"), locatorComponent("b", "two"),
	}}
	cfg := newCfg("")
	if err := ReplacePolicy(cfg, reg, []string{"a/one", "b/two", "a/one"}); err != nil {
		t.Fatalf("ReplacePolicy: %v", err)
	}
	want := []string{"a/one", "b/two"}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestReplacePolicyRejectsUnknownLeavesCfgUntouched(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{locatorComponent("a", "one")}}
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"a/one"}
	err := ReplacePolicy(cfg, reg, []string{"a/one", "z/nope"})
	wantToken(t, err, diag.Usage)
	if !equal(cfg.RepositoryResolution.Locators, []string{"a/one"}) {
		t.Errorf("cfg mutated on rejected replace: %v", cfg.RepositoryResolution.Locators)
	}
}

func TestListLocatorsProjectsAcceptsAndInPolicy(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		{Alias: "work-reference", Name: "filesystem-repository-locator", Role: registry.RoleRepositoryLocator,
			Accepts: []string{"name", "git_fetch_urls", "query"}, DisplayName: "Filesystem repositories",
			Description: "Finds local Git repositories in configured search roots"},
	}}
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"work-reference/filesystem-repository-locator"}

	got := ListLocators(cfg, reg)
	if len(got) != 1 {
		t.Fatalf("ListLocators = %+v", got)
	}
	l := got[0]
	if l.Ref != "work-reference/filesystem-repository-locator" || !l.InPolicy {
		t.Errorf("locator = %+v", l)
	}
	if len(l.Accepts) != 3 {
		t.Errorf("Accepts = %v", l.Accepts)
	}
}

func TestListLocatorsNotInPolicy(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{locatorComponent("acme", "corp-index")}}
	cfg := newCfg("")
	got := ListLocators(cfg, reg)
	if len(got) != 1 || got[0].InPolicy {
		t.Errorf("locators = %+v, want InPolicy=false", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
