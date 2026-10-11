package plugininstall

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func invalidManifestFixture(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures", "plugins", "invalid-manifest")
}

// localGitRemote initializes a git repository at dir containing src's tree
// plus one commit, and returns a "file://" URL to it — a source Install
// classifies as remote (it contains "://") and can clone with no network.
func localGitRemote(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, rerr := os.ReadFile(filepath.Join(src, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gc"),
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return "file://" + filepath.ToSlash(dir)
}

func wantToken(t *testing.T, err error, c diag.Category) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with token %q, got nil", c.Token)
	}
	if got := diag.Token(err); got != c.Token {
		t.Fatalf("token = %q, want %q (%v)", got, c.Token, err)
	}
}

func TestInstallLocalPinnedCopies(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginLocalPinned {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginLocalPinned)
	}
	if res.Package.Alias != "specific-starter" {
		t.Errorf("Alias = %q, want default manifest name", res.Package.Alias)
	}

	sourceLink := filepath.Join(plugins, "specific-starter", "source")
	info, err := os.Lstat(sourceLink)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("pinned install should copy, not symlink")
	}
	if _, err := os.Stat(filepath.Join(sourceLink, "plugin.json")); err != nil {
		t.Errorf("copied tree missing plugin.json: %v", err)
	}

	if len(reg.Components) != 1 || reg.Components[0].Alias != "specific-starter" {
		t.Errorf("components = %+v", reg.Components)
	}
	if len(reg.Conventions) != 1 || reg.Conventions[0].Name != "gitflow" {
		t.Errorf("conventions = %+v", reg.Conventions)
	}
}

func TestInstallLocalLinkedSymlinksContentNeverDuplicated(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{Link: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginLocalLinked {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginLocalLinked)
	}

	sourceLink := filepath.Join(plugins, "specific-starter", "source")
	info, err := os.Lstat(sourceLink)
	if err != nil {
		t.Fatalf("lstat source: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("--link install should create a symlink, got mode %v", info.Mode())
	}
	target, err := os.Readlink(sourceLink)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	absSrc, _ := filepath.Abs(src)
	if target != absSrc {
		t.Errorf("symlink target = %q, want %q", target, absSrc)
	}
}

func TestInstallLinkWithRemoteSourceRejectedBeforeIO(t *testing.T) {
	plugins := t.TempDir()
	reg := &registry.Registry{}
	_, err := Install(plugins, reg, "https://example.com/plugin.git", Options{Link: true})
	wantToken(t, err, diag.Usage)
	if len(reg.Packages) != 0 {
		t.Errorf("nothing should be registered: %+v", reg.Packages)
	}
	entries, _ := os.ReadDir(plugins)
	if len(entries) != 0 {
		t.Errorf("no I/O should have run: %v", entries)
	}
}

func TestInstallRemotePinnedRecordsClonedHEAD(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	remote := localGitRemote(t, src)
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, remote, Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginRemotePinned {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginRemotePinned)
	}
	if !strings.HasPrefix(res.Package.Reference, remote+"@") {
		t.Errorf("Reference = %q, want prefix %q", res.Package.Reference, remote+"@")
	}
	if strings.Contains(res.Package.Reference, "@@") || strings.HasSuffix(res.Package.Reference, "@") {
		t.Errorf("Reference = %q, malformed pin", res.Package.Reference)
	}

	sourceDirPath := filepath.Join(plugins, "specific-starter", "source")
	if _, err := os.Stat(filepath.Join(sourceDirPath, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git should not be staged: err=%v", err)
	}
}

func TestInstallDefaultAliasVsAs(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{Alias: "custom-alias"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Alias != "custom-alias" {
		t.Errorf("Alias = %q, want custom-alias", res.Package.Alias)
	}
	if _, ok := reg.PackageByAlias("custom-alias"); !ok {
		t.Errorf("custom-alias not registered")
	}
}

func TestInstallAliasConflictDifferentOriginRegistersNothing(t *testing.T) {
	srcA := fixtures.Prepare(t, "specific-starter")
	srcB := fixtures.Prepare(t, "colliding-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, srcA, Options{Alias: "x"}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	before := registrySnapshot(reg)

	_, err := Install(plugins, reg, srcB, Options{Alias: "x"})
	wantToken(t, err, diag.PluginAliasConflict)

	after := registrySnapshot(reg)
	if before != after {
		t.Errorf("registry mutated by a failed install:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestInstallSameOriginSameAliasIsIdempotent(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if len(reg.Packages) != 1 {
		t.Errorf("packages = %+v, want exactly one entry", reg.Packages)
	}
}

func TestInstallFallbackConflictAtInstallTime(t *testing.T) {
	srcA := fixtures.Prepare(t, "fallback-starter-a")
	srcB := fixtures.Prepare(t, "fallback-starter-b")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, srcA, Options{}); err != nil {
		t.Fatalf("install fallback-starter-a: %v", err)
	}
	before := registrySnapshot(reg)

	_, err := Install(plugins, reg, srcB, Options{})
	wantToken(t, err, diag.PluginFallbackConflict)

	after := registrySnapshot(reg)
	if before != after {
		t.Errorf("registry mutated by a failed install:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestInstallReinstallingOwnFallbackIsNotAConflict(t *testing.T) {
	src := fixtures.Prepare(t, "fallback-starter-a")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstalling own fallback: %v", err)
	}
}

func TestInstallInvalidManifestRegistersNothing(t *testing.T) {
	plugins := t.TempDir()
	reg := &registry.Registry{}

	_, err := Install(plugins, reg, invalidManifestFixture(t), Options{})
	wantToken(t, err, diag.PluginInvalid)
	if len(reg.Packages) != 0 || len(reg.Components) != 0 {
		t.Errorf("nothing should be registered: packages=%+v components=%+v", reg.Packages, reg.Components)
	}
	if _, err := os.Stat(filepath.Join(plugins, "invalid-manifest")); !os.IsNotExist(err) {
		t.Errorf("nothing should be staged: err=%v", err)
	}
}

func registrySnapshot(reg *registry.Registry) string {
	var b strings.Builder
	for _, p := range reg.ListPackages() {
		fmt.Fprintf(&b, "%s|%s|%s\n", p.Alias, p.Origin, p.Reference)
	}
	for _, c := range reg.Components {
		fmt.Fprintf(&b, "%s/%s\n", c.Alias, c.Name)
	}
	return b.String()
}

func conflictError(t *testing.T, err error) *diag.Error {
	t.Helper()
	wantToken(t, err, diag.PluginAliasConflict)
	var de *diag.Error
	if !errors.As(err, &de) {
		t.Fatalf("not a *diag.Error: %v", err)
	}
	return de
}

func TestInstallAliasConflictFromPluginNameMessage(t *testing.T) {
	srcA := fixtures.Prepare(t, "specific-starter")
	srcB := fixtures.Prepare(t, "colliding-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}
	if _, err := Install(plugins, reg, srcA, Options{Alias: "colliding-starter"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	_, err := Install(plugins, reg, srcB, Options{})
	de := conflictError(t, err)

	for _, want := range []string{`"colliding-starter"`, "name collides", "not installed"} {
		if !strings.Contains(de.Summary, want) {
			t.Errorf("summary %q missing %q", de.Summary, want)
		}
	}
	for _, want := range []string{"--as <alias>", "work plugin uninstall colliding-starter"} {
		if !strings.Contains(de.Hint, want) {
			t.Errorf("hint %q missing %q", de.Hint, want)
		}
	}
	if strings.Contains(de.Summary+de.Hint, srcA) {
		t.Errorf("message leaks the existing plugin's path: %s / %s", de.Summary, de.Hint)
	}
}

func TestInstallAliasConflictFromExplicitAliasMessage(t *testing.T) {
	srcA := fixtures.Prepare(t, "specific-starter")
	srcB := fixtures.Prepare(t, "colliding-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}
	if _, err := Install(plugins, reg, srcA, Options{Alias: "x"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	_, err := Install(plugins, reg, srcB, Options{Alias: "x"})
	de := conflictError(t, err)

	for _, want := range []string{`"colliding-starter"`, `alias "x"`, "--as"} {
		if !strings.Contains(de.Summary, want) {
			t.Errorf("summary %q missing %q", de.Summary, want)
		}
	}
	if strings.Contains(de.Hint, "uninstall") {
		t.Errorf("explicit-alias hint should only offer another alias: %q", de.Hint)
	}
	if strings.Contains(de.Summary+de.Hint, srcA) {
		t.Errorf("message leaks the existing plugin's path: %s / %s", de.Summary, de.Hint)
	}
}

// writeManifestDir returns a local plugin directory holding only manifest.
func writeManifestDir(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstallInvalidStarterPatternRegistersNothing(t *testing.T) {
	src := writeManifestDir(t, `{"name":"p","version":"1","conventions":[],"components":[
		{"name":"s","role":"starter","entrypoint":"s","pattern":"(unclosed"}]}`)
	plugins := t.TempDir()
	reg := &registry.Registry{}

	_, err := Install(plugins, reg, src, Options{})
	wantToken(t, err, diag.PluginInvalid)
	if len(reg.Packages) != 0 || len(reg.Components) != 0 {
		t.Errorf("nothing should be registered: %+v %+v", reg.Packages, reg.Components)
	}
}

func TestInstallInvalidAliasIsRejectedBeforePluginStorageIsTouched(t *testing.T) {
	valid := writeManifestDir(t, `{"name":"p","version":"1","conventions":[],"components":[]}`)
	for _, alias := range []string{"..", ".", "a/b", "x.old", ".hidden"} {
		t.Run("as="+alias, func(t *testing.T) {
			plugins := t.TempDir()
			reg := &registry.Registry{}
			_, err := Install(plugins, reg, valid, Options{Alias: alias})
			wantToken(t, err, diag.PluginInvalid)
			assertPluginStorageUntouched(t, plugins, reg)
		})
	}

	t.Run("manifest name", func(t *testing.T) {
		src := writeManifestDir(t, `{"name":"..","version":"1","conventions":[],"components":[]}`)
		plugins := t.TempDir()
		reg := &registry.Registry{}
		_, err := Install(plugins, reg, src, Options{})
		wantToken(t, err, diag.PluginInvalid)
		assertPluginStorageUntouched(t, plugins, reg)
	})
}

func assertPluginStorageUntouched(t *testing.T, plugins string, reg *registry.Registry) {
	t.Helper()
	if entries, _ := os.ReadDir(plugins); len(entries) != 0 {
		t.Errorf("plugin storage was touched: %v", entries)
	}
	if len(reg.Packages) != 0 || len(reg.Components) != 0 {
		t.Errorf("nothing should be registered: %+v %+v", reg.Packages, reg.Components)
	}
}

// seedRegistered mimics bootstrap's registration of the reference package: its
// components and convention are in the registry, but no Package record is.
func seedRegistered() *registry.Registry {
	reg := &registry.Registry{}
	reg.UpsertComponent(registry.Component{Alias: "work-reference", Name: "local-path-starter", Role: registry.RoleStarter, Entrypoint: "starter", StarterLayer: registry.LayerFallback})
	reg.UpsertConvention(registry.Convention{Name: "freeform", Prefixes: []string{"{slug}"}})
	return reg
}

func TestInstallReferencePackageAliasIsReserved(t *testing.T) {
	// Reserved whether or not bootstrap has registered the seed yet.
	valid := writeManifestDir(t, `{"name":"p","version":"1","conventions":[],"components":[]}`)
	named := writeManifestDir(t, `{"name":"work-reference","version":"1","conventions":[],"components":[]}`)

	for name, tc := range map[string]struct {
		src  string
		opts Options
	}{
		"--as":          {valid, Options{Alias: "work-reference"}},
		"manifest name": {named, Options{}},
	} {
		for _, seeded := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/seeded=%v", name, seeded), func(t *testing.T) {
				plugins := t.TempDir()
				reg := &registry.Registry{}
				if seeded {
					reg = seedRegistered()
				}
				before := registrySnapshot(reg)

				_, err := Install(plugins, reg, tc.src, tc.opts)
				wantToken(t, err, diag.PluginAliasConflict)
				if after := registrySnapshot(reg); after != before {
					t.Errorf("registry mutated by a rejected install:\nbefore: %s\nafter:  %s", before, after)
				}
				if entries, _ := os.ReadDir(plugins); len(entries) != 0 {
					t.Errorf("plugin storage was touched: %v", entries)
				}
			})
		}
	}
}

func componentNames(reg *registry.Registry, alias string) []string {
	var names []string
	for _, c := range reg.Components {
		if c.Alias == alias {
			names = append(names, c.Name)
		}
	}
	return names
}

func TestInstallReinstallReplacesWhatTheAliasRegistered(t *testing.T) {
	src := t.TempDir()
	write := func(manifest string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plugins := t.TempDir()
	reg := &registry.Registry{}

	write(`{"name":"p","version":"1","components":[
		{"name":"a","role":"starter","entrypoint":"a","pattern":"^a"},
		{"name":"b","role":"starter","entrypoint":"b","pattern":"^b"}],
		"conventions":[{"name":"gitflow","prefixes":["feature/{slug}"]}]}`)
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	write(`{"name":"p","version":"2","components":[
		{"name":"a","role":"starter","entrypoint":"a","pattern":"^a"}],
		"conventions":[]}`)
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	if got := componentNames(reg, "p"); len(got) != 1 || got[0] != "a" {
		t.Errorf("components after reinstall = %v, want only [a]", got)
	}
	if _, ok := reg.ConventionByName("gitflow"); ok {
		t.Errorf("convention dropped by the new manifest is still in the catalog")
	}
	if pkg, _ := reg.PackageByAlias("p"); len(pkg.Conventions) != 0 {
		t.Errorf("package conventions = %v, want none", pkg.Conventions)
	}
}

func TestInstallReinstallKeepsConventionAnotherPackageDeclares(t *testing.T) {
	shared := `"conventions":[{"name":"gitflow","prefixes":["feature/{slug}"]}]`
	other := writeManifestDir(t, `{"name":"other","version":"1","components":[],`+shared+`}`)
	src := writeManifestDir(t, `{"name":"p","version":"1","components":[],`+shared+`}`)
	plugins := t.TempDir()
	reg := &registry.Registry{}
	if _, err := Install(plugins, reg, other, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(src, "plugin.json"),
		[]byte(`{"name":"p","version":"2","components":[],"conventions":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if _, ok := reg.ConventionByName("gitflow"); !ok {
		t.Errorf("a convention another package still declares was removed from the catalog")
	}
}

func TestComponentEntryRecordsActivationData(t *testing.T) {
	src := fixtures.Prepare(t, "context-suite")
	reg := &registry.Registry{}
	if _, err := Install(t.TempDir(), reg, src, Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	byName := map[string]registry.Component{}
	for _, c := range reg.Components {
		byName[c.Name] = c
	}
	linker := byName["linker"]
	if linker.Key != "github.pull_request" || linker.Discover == nil || !linker.Discover.Automatic {
		t.Errorf("linker key/discover = %q / %+v", linker.Key, linker.Discover)
	}
	if len(linker.Inputs) != 1 || linker.Inputs[0] != "work:worktree_path" {
		t.Errorf("linker Inputs = %v, want discover.inputs copied", linker.Inputs)
	}
	importer := byName["importer"]
	if len(importer.On) != 1 || importer.On[0].Event != "start:finalized" || importer.On[0].Starters[0] != "starter" {
		t.Errorf("importer On = %+v", importer.On)
	}
	if importer.Manual == nil || importer.Manual.DisplayName != "Context Importer" {
		t.Errorf("importer Manual = %+v", importer.Manual)
	}
	if len(importer.Inputs) != 2 {
		t.Errorf("importer Inputs = %v", importer.Inputs)
	}
	if pkg, _ := reg.PackageByAlias("context-suite"); pkg.PluginName != "context-suite" {
		t.Errorf("PluginName = %q", pkg.PluginName)
	}
}

func TestInstallRuntimeMissingLeavesNothing(t *testing.T) {
	src := fixtures.Prepare(t, "invalid-runtime")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	_, err := Install(plugins, reg, src, Options{})
	wantToken(t, err, diag.PluginInstallFailed)
	if !strings.Contains(err.Error(), "no-such-interpreter") || !strings.Contains(err.Error(), "PATH") {
		t.Errorf("message = %q, want the runtime and PATH named", err)
	}
	if len(reg.Components) != 0 || len(reg.Packages) != 0 {
		t.Errorf("registry touched: %+v", reg)
	}
	if entries, _ := os.ReadDir(plugins); len(entries) != 0 {
		t.Errorf("plugins dir not empty: %v", entries)
	}
}

func TestInstallRestrictionToUninstalledStarterStillInstalls(t *testing.T) {
	src := fixtures.Prepare(t, "restricted-suite")
	reg := &registry.Registry{}
	if _, err := Install(t.TempDir(), reg, src, Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
}

func TestReinstallReplacesDeclarations(t *testing.T) {
	src := fixtures.Prepare(t, "context-suite")
	plugins := t.TempDir()
	reg := &registry.Registry{}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	m, err := os.ReadFile(filepath.Join(src, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(m, &doc); err != nil {
		t.Fatal(err)
	}
	var kept []any
	for _, c := range doc["components"].([]any) {
		if c.(map[string]any)["name"] != "importer2" {
			kept = append(kept, c)
		}
	}
	doc["components"] = kept
	out, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if reg.HasComponent("context-suite", "importer2") {
		t.Errorf("dropped importer survived reinstall")
	}
	if !reg.HasComponent("context-suite", "importer") {
		t.Errorf("kept importer missing after reinstall")
	}
}
