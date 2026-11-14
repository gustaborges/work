package pluginadmin_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func newHome(t *testing.T) workhome.Home { return workhome.At(filepath.Join(t.TempDir(), "home")) }

func register(t *testing.T, h workhome.Home, src string) {
	t.Helper()
	spec, err := marketplaceadmin.Classify(src, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = marketplaceadmin.Add(context.Background(), h, spec); err != nil {
		t.Fatalf("registering %s: %v", src, err)
	}
}

func upgrade(t *testing.T, h workhome.Home, names ...string) {
	t.Helper()
	if _, err := marketplaceadmin.Upgrade(context.Background(), h, names); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, h workhome.Home) *adminstate.Snapshot {
	t.Helper()
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func add(t *testing.T, h workhome.Home, plugin, mkt string) pluginadmin.InstallResult {
	t.Helper()
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, plugin, mkt)
	if err != nil {
		t.Fatalf("prepare %s@%s: %v", plugin, mkt, err)
	}
	defer plan.Close()
	res, err := plan.Apply(context.Background(), false)
	if err != nil {
		t.Fatalf("apply %s@%s: %v", plugin, mkt, err)
	}
	return res
}

func code(err error) int { return diag.ExitCode(err) }

// writeCatalog creates a local marketplace whose plugins are given as name →
// plugin.json, each with one executable entrypoint "bin/tool".
func writeCatalog(t *testing.T, namespace string, plugins map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	type listing struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	var list []listing
	for name, manifest := range plugins {
		root := filepath.Join(dir, "plugins", name)
		if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := marketplaces.WriteEntrypoint(filepath.Join(root, "bin", "tool"), "#!/bin/sh\nexit 0\n"); err != nil {
			t.Fatal(err)
		}
		list = append(list, listing{Name: name, Path: "plugins/" + name})
	}
	raw, _ := json.Marshal(map[string]any{"schema": 1, "name": namespace, "plugins": list})
	if err := os.WriteFile(filepath.Join(dir, "marketplace.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func manifest(name, version string, extra string) string {
	return `{"name":"` + name + `","version":"` + version + `","conventions":[],"components":[` + extra + `]}`
}

const linker = `{"name":"run","role":"linker","key":"fixture.run","entrypoint":"bin/tool","manual":{"display_name":"run"}}`

func TestOnlyTheSelectedEntryIsCopied(t *testing.T) {
	h := newHome(t)
	src := marketplaces.Copy(t, "multiple")
	register(t, h, src)
	if len(snapshot(t, h).Registry.Packages) != 0 {
		t.Fatal("registration installed something")
	}

	res := add(t, h, "alpha", "demo")
	if res.Result != pluginadmin.ResultInstalled || res.Provider.String() != "alpha@demo" {
		t.Fatalf("result = %+v", res)
	}
	s := snapshot(t, h)
	if len(s.Registry.Packages) != 1 {
		t.Fatalf("packages = %+v", s.Registry.Packages)
	}
	pkg := s.Registry.Packages[0]
	if !pkg.Enabled || pkg.Provenance.Kind != pluginid.KindLocal || pkg.Provenance.PluginName != "alpha" || pkg.Provenance.MarketplaceName != "demo" {
		t.Fatalf("package = %+v", pkg)
	}
	cfg, _ := s.Config.Marketplace("demo")
	if pkg.Provenance.RegistrationID != cfg.RegistrationID || pkg.ContentDigest == "" || pkg.Provenance.Revision == "" || pkg.Provenance.Source != cfg.Source {
		t.Fatalf("provenance is not the actual source facts: %+v vs %+v", pkg.Provenance, cfg)
	}
	for _, sibling := range []string{"beta", "gamma"} {
		root, _ := h.PackageRoot(pluginid.Provider{Marketplace: "demo", Plugin: sibling})
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("sibling %s was copied: %v", sibling, err)
		}
	}
	root, _ := h.PackageRoot(res.Provider)
	for _, want := range []string{"source/plugin.json", "source/bin/tool", ".install-meta.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(want))); err != nil {
			t.Errorf("managed copy lacks %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "source", "marketplace.json")); err == nil {
		t.Error("the catalog was copied into the package")
	}
	if len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Errorf("add changed policy: %v", s.Config.RepositoryResolution.Locators)
	}
}

func TestInstalledCopyDoesNotDependOnTheAuthorSource(t *testing.T) {
	h := newHome(t)
	src := marketplaces.Copy(t, "single")
	register(t, h, src)

	// The author edits, then deletes the live source after registration. The
	// copy comes from the captured offer, never from either.
	tool := filepath.Join(src, "plugins", "sample", "bin", "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho tampered\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := add(t, h, "sample", "solo")
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	root, _ := h.PackageRoot(res.Provider)
	got, err := os.ReadFile(filepath.Join(root, "source", "bin", "tool"))
	if err != nil || strings.Contains(string(got), "tampered") {
		t.Fatalf("copy followed the live source: %q, %v", got, err)
	}
	s := snapshot(t, h)
	if s.Inspection["sample@solo"] != "ready" {
		t.Fatalf("inspection = %v", s.Inspection)
	}
}

func TestRepeatedAddIsAByteAndEpochPreservingNoOp(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "single"))
	add(t, h, "sample", "solo")
	before, _ := os.ReadFile(h.RegistryFile())
	epoch := snapshot(t, h).Epoch

	res := add(t, h, "sample", "solo")
	if res.Result != pluginadmin.ResultUnchanged {
		t.Fatalf("result = %q", res.Result)
	}
	after, _ := os.ReadFile(h.RegistryFile())
	if string(before) != string(after) || snapshot(t, h).Epoch != epoch {
		t.Fatal("an unchanged add rewrote state or advanced the epoch")
	}
}

func TestChangedContentIsAnExplicitReinstallThatKeepsEnablement(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{{Marketplace: "team", Plugin: "tool"}}, false); err != nil {
		t.Fatal(err)
	}

	// Equal opaque version, different bytes: only an explicit reinstall changes it.
	if err := os.WriteFile(filepath.Join(src, "plugins", "tool", "bin", "tool"), []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if res := add(t, h, "tool", "team"); res.Result != pluginadmin.ResultUnchanged {
		t.Fatalf("an unrefreshed offer must not change an installed copy: %q", res.Result)
	}
	upgrade(t, h, "team")
	if p, _ := snapshot(t, h).Registry.PackageByProvider(pluginid.Provider{Marketplace: "team", Plugin: "tool"}); p.Version != "1" {
		t.Fatalf("refresh changed the installed copy: %+v", p)
	}
	res := add(t, h, "tool", "team")
	if res.Result != pluginadmin.ResultReinstalled {
		t.Fatalf("result = %q", res.Result)
	}
	p, _ := snapshot(t, h).Registry.PackageByProvider(res.Provider)
	if p.Enabled {
		t.Fatal("reinstall re-enabled a disabled plugin")
	}
	root, _ := h.PackageRoot(res.Provider)
	if b, _ := os.ReadFile(filepath.Join(root, "source", "bin", "tool")); !strings.Contains(string(b), "exit 3") {
		t.Fatalf("replacement bytes = %q", b)
	}
}

func TestNonSelectorsAndUnknownTargetsAreUsageErrors(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "single"))
	before, _ := os.ReadFile(h.ConfigFile())
	ctx := context.Background()
	for _, c := range []struct{ plugin, mkt string }{
		{"../evil", "solo"}, {"plugins/sample", "solo"}, {"./sample", "solo"}, {"https://x.example/p.git", "solo"},
		{"plugin.json", "solo"}, {"sample", ""}, {"", "solo"}, {"sample", "nowhere"}, {"absent", "solo"}, {"sample", "SOLO"},
	} {
		plan, err := pluginadmin.PrepareInstall(ctx, h, c.plugin, c.mkt)
		if plan != nil {
			plan.Close()
		}
		if code(err) != 2 {
			t.Errorf("%q@%q: exit %d (%v)", c.plugin, c.mkt, code(err), err)
		}
	}
	if after, _ := os.ReadFile(h.ConfigFile()); string(before) != string(after) {
		t.Fatal("a rejected target changed configuration")
	}
	if len(snapshot(t, h).Registry.Packages) != 0 {
		t.Fatal("a rejected target installed something")
	}
}

func TestMissingStoredOfferIsEntryUnavailable(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "single"))
	if err := os.Remove(h.CatalogsFile()); err != nil {
		t.Fatal(err)
	}
	_, err := pluginadmin.PrepareInstall(context.Background(), h, "sample", "solo")
	if code(err) != diag.MarketplaceEntryUnavailable.Code {
		t.Fatalf("err = %v (exit %d)", err, code(err))
	}
}

func TestInstallationFailsForMissingRuntimeOrEntrypointButDiscoveryDoesNot(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{
		"norun":  manifest("norun", "1", `{"name":"run","role":"linker","key":"fixture.run","entrypoint":"bin/tool","runtime":"definitely-not-a-runtime-xyz","manual":{"display_name":"r"}}`),
		"nofile": manifest("nofile", "1", `{"name":"run","role":"linker","key":"fixture.run","entrypoint":"bin/missing","manual":{"display_name":"r"}}`),
		"fine":   manifest("fine", "1", linker),
	})
	register(t, h, src) // discovery tolerates both
	for _, name := range []string{"norun", "nofile"} {
		plan, err := pluginadmin.PrepareInstall(context.Background(), h, name, "team")
		if plan != nil {
			plan.Close()
		}
		if code(err) != diag.PluginInstallFailed.Code {
			t.Errorf("%s: exit %d (%v)", name, code(err), err)
		}
	}
	if entries, _ := os.ReadDir(h.PluginsDir()); len(entries) != 0 {
		t.Fatalf("failed installs left staged content: %v", entries)
	}
	add(t, h, "fine", "team")
}

func TestHomonymsAreIndependentCopies(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "homonym-alpha"))
	register(t, h, marketplaces.Copy(t, "homonym-beta"))
	add(t, h, "sample", "alpha-market")
	add(t, h, "sample", "beta-market")

	s := snapshot(t, h)
	if len(s.Registry.Packages) != 2 {
		t.Fatalf("packages = %+v", s.Registry.Packages)
	}
	a, _ := s.Registry.PackageByProvider(pluginid.Provider{Marketplace: "alpha-market", Plugin: "sample"})
	b, _ := s.Registry.PackageByProvider(pluginid.Provider{Marketplace: "beta-market", Plugin: "sample"})
	if a.Provenance.RegistrationID == b.Provenance.RegistrationID {
		t.Fatal("homonyms share a source instance")
	}
	ra, _ := h.PackageRoot(a.Provider)
	rb, _ := h.PackageRoot(b.Provider)
	if ra == rb {
		t.Fatal("homonyms share a managed root")
	}
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{a.Provider}, false); err != nil {
		t.Fatal(err)
	}
	s = snapshot(t, h)
	if s.Registry.PackageEnabled(a.Provider) || !s.Registry.PackageEnabled(b.Provider) {
		t.Fatal("toggling one homonym affected the other")
	}
}

func TestFallbackStarterIsExclusive(t *testing.T) {
	h := newHome(t)
	starter := func(name string) string {
		return manifest(name, "1", `{"name":"fallback","role":"starter","entrypoint":"bin/tool"}`)
	}
	register(t, h, writeCatalog(t, "one", map[string]string{"first": starter("first")}))
	register(t, h, writeCatalog(t, "two", map[string]string{"second": starter("second")}))
	add(t, h, "first", "one")
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "second", "two")
	if plan != nil {
		plan.Close()
	}
	if code(err) != diag.PluginFallbackConflict.Code {
		t.Fatalf("err = %v (exit %d)", err, code(err))
	}
	if !strings.Contains(err.Error(), "first@one") || !strings.Contains(err.Error(), "second@two") {
		t.Fatalf("the conflict must name both complete providers: %v", err)
	}
}

func TestApplyConflictsWhenTheTargetChangedAfterPreparation(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "single"))
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "sample", "solo")
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	add(t, h, "sample", "solo") // a competing installation wins the race
	if _, err := plan.Apply(context.Background(), false); code(err) != diag.AdministrationConflict.Code {
		t.Fatalf("apply after a competing install = %v (exit %d)", err, code(err))
	}
}

func TestUnrelatedConcurrentChangesMerge(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "multiple"))
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "alpha", "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	add(t, h, "beta", "demo") // unrelated sibling lands first
	if _, err := plan.Apply(context.Background(), false); err != nil {
		t.Fatalf("an unrelated change blocked the reviewed install: %v", err)
	}
	if n := len(snapshot(t, h).Registry.Packages); n != 2 {
		t.Fatalf("packages = %d", n)
	}
}

func TestClosingWithoutApplyRemovesTheStageAndLeases(t *testing.T) {
	h := newHome(t)
	register(t, h, marketplaces.Copy(t, "single"))
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "sample", "solo")
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(h.PluginsDir()); len(entries) == 0 {
		t.Fatal("preparation staged nothing")
	}
	plan.Close()
	plan.Close()
	if entries, _ := os.ReadDir(h.PluginsDir()); len(entries) != 0 {
		t.Fatalf("stage remains: %v", entries)
	}
	if leases, _ := os.ReadDir(h.LeasesDir()); len(leases) != 0 {
		t.Fatalf("leases remain: %v", leases)
	}
	if len(snapshot(t, h).Registry.Packages) != 0 {
		t.Fatal("a closed plan installed something")
	}
}

func TestReinstallThatDropsALocatorNeedsPolicyAuthorization(t *testing.T) {
	h := newHome(t)
	locator := `{"name":"find","role":"repository-locator","entrypoint":"bin/tool","accepts":["name"]}`
	src := writeCatalog(t, "team", map[string]string{"loc": manifest("loc", "1", locator)})
	register(t, h, src)
	add(t, h, "loc", "team")
	if err := adminstate.WithWrite(context.Background(), h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = []string{"loc@team/find"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	next := writeCatalog(t, "team", map[string]string{"loc": manifest("loc", "2", linker)})
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, src); err != nil {
		t.Fatal(err)
	}
	upgrade(t, h, "team")

	plan, err := pluginadmin.PrepareInstall(context.Background(), h, "loc", "team")
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if plan.Result() != pluginadmin.ResultReinstalled || !slices.Equal(plan.PolicyReferences(), []string{"loc@team/find"}) {
		t.Fatalf("plan = %q %v", plan.Result(), plan.PolicyReferences())
	}
	if _, err := plan.Apply(context.Background(), false); code(err) != diag.PluginPolicyDependency.Code {
		t.Fatalf("apply without authorization = %v (exit %d)", err, code(err))
	}
	if refs := snapshot(t, h).Config.RepositoryResolution.Locators; len(refs) != 1 {
		t.Fatalf("a refused reinstall changed policy: %v", refs)
	}
	if _, err := plan.Apply(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if refs := snapshot(t, h).Config.RepositoryResolution.Locators; len(refs) != 0 {
		t.Fatalf("authorized reinstall left %v", refs)
	}
}

func TestGitOfferInstallsTheExactRegisteredCommit(t *testing.T) {
	h := newHome(t)
	dir := marketplaces.Copy(t, "single")
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "publish")
	url := "file://" + filepath.ToSlash(dir)
	register(t, h, url)

	// The tip moves on, but nothing refreshed the offer: install the registered commit.
	tool := filepath.Join(dir, "plugins", "sample", "bin", "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho newer\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "newer")

	res := add(t, h, "sample", "solo")
	root, _ := h.PackageRoot(res.Provider)
	if b, _ := os.ReadFile(filepath.Join(root, "source", "bin", "tool")); strings.Contains(string(b), "newer") {
		t.Fatalf("install fell forward to the branch tip: %q", b)
	}
	pkg, _ := snapshot(t, h).Registry.PackageByProvider(res.Provider)
	if pkg.Provenance.Kind != pluginid.KindGit || pkg.Provenance.Branch != "main" || len(pkg.Provenance.Revision) < 40 {
		t.Fatalf("provenance = %+v", pkg.Provenance)
	}
}

func TestBuiltinEntryInstallsOfflineThroughTheCommonPipeline(t *testing.T) {
	h := newHome(t)
	res := add(t, h, adminstate.ReferencePlugin, pluginid.Builtin)
	if res.Result != pluginadmin.ResultInstalled {
		t.Fatalf("result = %q", res.Result)
	}
	pkg, _ := snapshot(t, h).Registry.PackageByProvider(adminstate.ReferenceProvider)
	if pkg.Provenance.Kind != pluginid.KindBuiltin || pkg.Provenance.RegistrationID != pluginid.BuiltinRegistrationID {
		t.Fatalf("provenance = %+v", pkg.Provenance)
	}
	if again := add(t, h, adminstate.ReferencePlugin, pluginid.Builtin); again.Result != pluginadmin.ResultUnchanged {
		t.Fatalf("second add = %q", again.Result)
	}
	s := snapshot(t, h)
	if len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Fatalf("explicit add inserted policy: %v", s.Config.RepositoryResolution.Locators)
	}
	if !s.Lifecycle.Initialized || !s.Lifecycle.PolicyHandled || s.Lifecycle.Removed {
		t.Fatalf("an explicit reference add must settle first-use setup and policy: %+v", *s.Lifecycle)
	}
}

// The installer package may expose registration helpers but never a function
// that publishes a package from an arbitrary directory, URL or manifest.
func TestInstallerExposesNoStandalonePublisher(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join("..", "plugininstall", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no installer sources found: %v", err)
	}
	allowed := []string{"ComponentEntry", "PreflightRuntimes", "ProvenanceOf", "Register", "Prepare", "Validate", "WriteMeta"}
	var exported []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.IsExported() {
				exported = append(exported, fn.Name.Name)
			}
		}
	}
	for _, name := range exported {
		if !slices.Contains(allowed, name) {
			t.Errorf("plugininstall exports %s; new publishers must go through a resolved marketplace entry", name)
		}
	}
	_ = plugininstall.MetaFile
}
