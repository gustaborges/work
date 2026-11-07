package repoconfig

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
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

func prov(plugin string) pluginid.Provider {
	return pluginid.Provider{Marketplace: "m", Plugin: plugin}
}

// ref is the fully qualified policy form of plugin/name in marketplace "m".
func ref(plugin, name string) string { return plugin + "@m/" + name }

func locatorComponent(plugin, name string, accepts ...string) registry.Component {
	return registry.Component{
		Provider: prov(plugin), Name: name, Role: registry.RoleRepositoryLocator, Entrypoint: name,
		Accepts: accepts, DisplayName: name,
	}
}

// regOf builds a valid registry holding the components, with one enabled
// package per provider except those named in off.
func regOf(off []string, comps ...registry.Component) *registry.Registry {
	r := registry.New()
	for _, c := range comps {
		if _, ok := r.PackageByProvider(c.Provider); !ok {
			r.UpsertPackage(registry.Package{
				Provider: c.Provider, InstallMode: registry.InstallModeCopy, Enabled: !slices.Contains(off, c.Provider.Plugin),
				Version: "1", ContentDigest: "sha256:x", DigestAlgorithm: registry.DigestPackageV1,
				Provenance: registry.Provenance{RegistrationID: "0123456789abcdef0123456789abcdef", Kind: pluginid.KindLocal, Source: "/s",
					MarketplaceName: "m", PluginName: c.Provider.Plugin, Path: "p", Revision: "r", CatalogDigest: "c"},
			})
		}
		r.UpsertComponent(c)
	}
	return r
}

func TestListPolicyMarksUnavailableWithoutDropping(t *testing.T) {
	reg := regOf(nil, locatorComponent("work-reference", "filesystem-repository-locator", "name"))
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{
		ref("work-reference", "filesystem-repository-locator"),
		ref("ghost", "vanished"),
		"not-a-reference",
	}
	got := ListPolicy(cfg, reg)
	if len(got) != 3 {
		t.Fatalf("ListPolicy = %+v, want 3 entries", got)
	}
	if !got[0].Available || got[0].Position != 1 {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].Available || got[1].Position != 2 || got[1].Installed {
		t.Errorf("unavailable entry should be kept and marked: %+v", got[1])
	}
	if got[2].Available || got[2].UnavailableReason == nil || *got[2].UnavailableReason != "reference-invalid" {
		t.Errorf("an unparsable entry must be retained as invalid: %+v", got[2])
	}
}

func TestAddPolicyRejectsUnknownLocator(t *testing.T) {
	cfg := newCfg("")
	wantToken(t, AddPolicy(cfg, registry.New(), ref("acme", "corp-index"), "", ""), diag.Usage)
	wantToken(t, AddPolicy(cfg, registry.New(), "acme/corp-index", "", ""), diag.Usage) // retired alias form
}

func TestAddPolicyAppendsAndIsNoOpWhenPresent(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "one"), locatorComponent("b", "two"))
	cfg := newCfg("")
	for range 2 {
		if err := AddPolicy(cfg, reg, ref("a", "one"), "", ""); err != nil {
			t.Fatalf("AddPolicy: %v", err)
		}
	}
	if len(cfg.RepositoryResolution.Locators) != 1 {
		t.Fatalf("locators = %v, want 1 (no duplicate)", cfg.RepositoryResolution.Locators)
	}
}

func TestAddPolicyBareComponentResolvesWhenUnambiguousAndStoresQualified(t *testing.T) {
	reg := regOf(nil, locatorComponent("acme", "corp-index"))
	cfg := newCfg("")
	if err := AddPolicy(cfg, reg, "corp-index", "", ""); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	if cfg.RepositoryResolution.Locators[0] != ref("acme", "corp-index") {
		t.Errorf("locators = %v, want the fully qualified reference", cfg.RepositoryResolution.Locators)
	}
}

func TestAddPolicyBareComponentAmbiguousIsUsageError(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "index"), locatorComponent("b", "index"))
	wantToken(t, AddPolicy(newCfg(""), reg, "index", "", ""), diag.Usage)
}

func TestHomonymousLocatorsAreAddressedByProvider(t *testing.T) {
	other := locatorComponent("a", "one")
	other.Provider = pluginid.Provider{Marketplace: "other", Plugin: "a"}
	reg := regOf(nil, locatorComponent("a", "one"))
	other2 := regOf(nil, other)
	reg.UpsertPackage(other2.Packages[0])
	reg.UpsertComponent(other)
	cfg := newCfg("")
	if err := AddPolicy(cfg, reg, "a@other/one", "", ""); err != nil {
		t.Fatal(err)
	}
	if !equal(cfg.RepositoryResolution.Locators, []string{"a@other/one"}) {
		t.Fatalf("locators = %v", cfg.RepositoryResolution.Locators)
	}
	wantToken(t, AddPolicy(newCfg(""), reg, "one", "", ""), diag.Usage)
}

func TestAddPolicyPositioningBeforeAfter(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "one"), locatorComponent("b", "two"), locatorComponent("c", "three"))
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one"), ref("b", "two")}
	if err := AddPolicy(cfg, reg, ref("c", "three"), ref("b", "two"), ""); err != nil {
		t.Fatalf("AddPolicy --before: %v", err)
	}
	want := []string{ref("a", "one"), ref("c", "three"), ref("b", "two")}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestAddPolicyBothBeforeAndAfterIsUsageError(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "one"))
	wantToken(t, AddPolicy(newCfg(""), reg, ref("a", "one"), "x", "y"), diag.Usage)
}

func TestRemovePolicyDropsAndKeepsComponentInstalled(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one"), ref("b", "two")}
	if err := RemovePolicy(cfg, []string{ref("a", "one")}); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}
	if !equal(cfg.RepositoryResolution.Locators, []string{ref("b", "two")}) {
		t.Errorf("locators = %v", cfg.RepositoryResolution.Locators)
	}
}

func TestRemovePolicyAbsentIsNoOp(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one")}
	if err := RemovePolicy(cfg, []string{ref("z", "nope")}); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}
	if !equal(cfg.RepositoryResolution.Locators, []string{ref("a", "one")}) {
		t.Errorf("locators = %v, want unchanged", cfg.RepositoryResolution.Locators)
	}
}

func TestMovePolicyRequiresExactlyOneDirection(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one"), ref("b", "two")}
	wantToken(t, MovePolicy(cfg, ref("a", "one"), "", ""), diag.Usage)
	wantToken(t, MovePolicy(cfg, ref("a", "one"), ref("b", "two"), ref("b", "two")), diag.Usage)
}

func TestMovePolicyReorders(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one"), ref("b", "two"), ref("c", "three")}
	if err := MovePolicy(cfg, ref("c", "three"), ref("a", "one"), ""); err != nil {
		t.Fatalf("MovePolicy: %v", err)
	}
	want := []string{ref("c", "three"), ref("a", "one"), ref("b", "two")}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestMovePolicyUnknownRefIsUsageError(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one")}
	wantToken(t, MovePolicy(cfg, ref("z", "nope"), ref("a", "one"), ""), diag.Usage)
	wantToken(t, MovePolicy(cfg, ref("a", "one"), ref("z", "nope"), ""), diag.Usage)
}

func TestReplacePolicyRejectsDuplicates(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "one"), locatorComponent("b", "two"))
	cfg := newCfg("")
	wantToken(t, ReplacePolicy(cfg, reg, []string{ref("a", "one"), ref("b", "two"), ref("a", "one")}), diag.Usage)
	if err := ReplacePolicy(cfg, reg, []string{ref("a", "one"), ref("b", "two")}); err != nil {
		t.Fatal(err)
	}
	want := []string{ref("a", "one"), ref("b", "two")}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatalf("locators = %v, want %v", cfg.RepositoryResolution.Locators, want)
	}
}

func TestReplacePolicyRejectsUnknownLeavesCfgUntouched(t *testing.T) {
	reg := regOf(nil, locatorComponent("a", "one"))
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("a", "one")}
	wantToken(t, ReplacePolicy(cfg, reg, []string{ref("a", "one"), ref("z", "nope")}), diag.Usage)
	if !equal(cfg.RepositoryResolution.Locators, []string{ref("a", "one")}) {
		t.Errorf("cfg mutated on rejected replace: %v", cfg.RepositoryResolution.Locators)
	}
}

func TestListLocatorsProjectsAcceptsAndInPolicy(t *testing.T) {
	reg := regOf(nil, registry.Component{
		Provider: prov("work-reference"), Name: "filesystem-repository-locator", Role: registry.RoleRepositoryLocator, Entrypoint: "locator",
		Accepts: []string{"name", "git_fetch_urls", "query"}, DisplayName: "Filesystem repositories",
		Description: "Finds local Git repositories in configured search roots",
	})
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("work-reference", "filesystem-repository-locator")}
	got := ListLocators(cfg, reg)
	if len(got) != 1 {
		t.Fatalf("ListLocators = %+v", got)
	}
	l := got[0]
	if l.Ref != ref("work-reference", "filesystem-repository-locator") || !l.InPolicy || !l.Enabled {
		t.Errorf("locator = %+v", l)
	}
	if len(l.Accepts) != 3 {
		t.Errorf("Accepts = %v", l.Accepts)
	}
}

func TestListLocatorsNotInPolicy(t *testing.T) {
	reg := regOf(nil, locatorComponent("acme", "corp-index"))
	got := ListLocators(newCfg(""), reg)
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

func TestDisabledLocatorPolicyRetained(t *testing.T) {
	r := regOf([]string{"a"}, locatorComponent("a", "loc"))
	cfg := config.Default()
	cfg.RepositoryResolution.Locators = []string{ref("a", "loc")}
	entries := ListPolicy(cfg, r)
	if len(entries) != 1 || entries[0].Available || cfg.RepositoryResolution.Locators[0] != ref("a", "loc") {
		t.Fatalf("%+v", entries)
	}
	if err := AddPolicy(cfg, r, ref("a", "loc"), "", ""); err != nil {
		t.Fatal(err)
	}
	if len(ListLocators(cfg, r)) != 1 {
		t.Fatal("installed inventory lost")
	}
}

func TestPolicyDisabledAndRetainedReplacement(t *testing.T) {
	reg := regOf([]string{"a"}, locatorComponent("a", "clone"), registry.Component{Provider: prov("b"), Name: "old", Role: registry.RoleStarter, Entrypoint: "old"})
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{ref("missing", "gone"), ref("b", "old")}
	if err := AddPolicy(cfg, reg, "clone", ref("missing", "gone"), ""); err != nil {
		t.Fatal(err)
	}
	if err := ReplacePolicy(cfg, reg, []string{ref("b", "old"), "clone", "gone"}); err != nil {
		t.Fatal(err)
	}
	want := []string{ref("b", "old"), ref("a", "clone"), ref("missing", "gone")}
	if !equal(cfg.RepositoryResolution.Locators, want) {
		t.Fatal(cfg.RepositoryResolution.Locators)
	}
	for _, refs := range [][]string{{ref("a", "clone"), "clone"}, {ref("a", "clone"), "unknown"}} {
		wantToken(t, ReplacePolicy(cfg, reg, refs), diag.Usage)
		if !equal(cfg.RepositoryResolution.Locators, want) {
			t.Fatal("partial replace")
		}
	}
	cfg.RepositoryResolution.Locators = append(cfg.RepositoryResolution.Locators, ref("missing", "clone"))
	wantToken(t, ReplacePolicy(cfg, reg, []string{"clone"}), diag.Usage)
}

func TestAdminRootLateRemovalInvalidPreservesSlice(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	cfg := newCfg("", a, b)
	original := cfg.RepositoryRoots
	wantToken(t, RemoveRoots(cfg, []string{a, "\x00"}), diag.Usage)
	if !equal(cfg.RepositoryRoots, []string{a, b}) || !equal(original, []string{a, b}) {
		t.Fatal("partial removal")
	}
}
