package registry

import (
	"os"
	"reflect"
	"runtime"

	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"path/filepath"
	"testing"
)

func TestUpsertComponentIdempotent(t *testing.T) {
	r := &Registry{}
	c := Component{Alias: "work-reference", Name: "local-path-starter", Role: RoleStarter, Entrypoint: "starter", StarterLayer: LayerFallback}
	r.UpsertComponent(c)
	r.UpsertComponent(c)
	if len(r.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(r.Components))
	}

	// Same key, changed payload -> replace, not append.
	c.Entrypoint = "starter.exe"
	r.UpsertComponent(c)
	if len(r.Components) != 1 || r.Components[0].Entrypoint != "starter.exe" {
		t.Errorf("upsert did not replace: %+v", r.Components)
	}

	// Different name -> new entry.
	r.UpsertComponent(Component{Alias: "work-reference", Name: "filesystem-repository-locator", Role: RoleRepositoryLocator, Entrypoint: "locator", Accepts: []string{"name"}})
	if len(r.Components) != 2 {
		t.Errorf("components = %d, want 2", len(r.Components))
	}
}

func TestUpsertConventionIdempotent(t *testing.T) {
	r := &Registry{}
	r.UpsertConvention(Convention{Name: "freeform", Prefixes: []string{"{slug}"}})
	r.UpsertConvention(Convention{Name: "freeform", Prefixes: []string{"{slug}", "x"}})
	if len(r.Conventions) != 1 {
		t.Fatalf("conventions = %d, want 1", len(r.Conventions))
	}
	if len(r.Conventions[0].Prefixes) != 2 {
		t.Errorf("convention not replaced: %+v", r.Conventions[0])
	}
}

func TestQueries(t *testing.T) {
	r := &Registry{}
	r.UpsertComponent(Component{Alias: "a", Name: "s", Role: RoleStarter, Entrypoint: "s", StarterLayer: LayerFallback})
	r.UpsertComponent(Component{Alias: "a", Name: "l", Role: RoleRepositoryLocator, Entrypoint: "l", Accepts: []string{"name"}})
	r.UpsertConvention(Convention{Name: "freeform", Prefixes: []string{"{slug}"}})

	if got := r.ByRole(RoleStarter); len(got) != 1 || got[0].Name != "s" {
		t.Errorf("ByRole(starter) = %+v", got)
	}
	sf, ok := r.StarterFallback()
	if !ok || sf.Name != "s" {
		t.Errorf("StarterFallback = %+v, %v", sf, ok)
	}
	if _, ok := r.ConventionByName("freeform"); !ok {
		t.Errorf("ConventionByName(freeform) not found")
	}
	if !r.HasComponent("a", "l") || r.HasComponent("a", "nope") {
		t.Errorf("HasComponent wrong")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "registry.json")

	if r, err := Load(path); err != nil || len(r.Components) != 0 {
		t.Fatalf("Load(missing) = %+v, %v", r, err)
	}

	in := &Registry{}
	in.UpsertComponent(Component{Alias: "work-reference", Name: "local-path-starter", Role: RoleStarter, Entrypoint: "starter", StarterLayer: LayerFallback})
	in.UpsertConvention(Convention{Name: "freeform", Prefixes: []string{"{slug}"}})
	in.UpsertPackage(Package{Alias: "work-reference", Origin: OriginLocalPinned, Reference: "/opt/work-reference"})
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Components) != 1 || out.Components[0].Name != "local-path-starter" {
		t.Errorf("round trip components = %+v", out.Components)
	}
	if len(out.Conventions) != 1 {
		t.Errorf("round trip conventions = %+v", out.Conventions)
	}
	if len(out.Packages) != 1 || out.Packages[0].Alias != "work-reference" || out.Packages[0].Origin != OriginLocalPinned {
		t.Errorf("round trip packages = %+v", out.Packages)
	}
}

func TestUpsertPackageIdempotentAndSorted(t *testing.T) {
	r := &Registry{}
	r.UpsertPackage(Package{Alias: "zeta", Origin: OriginLocalPinned, Reference: "/z"})
	r.UpsertPackage(Package{Alias: "alpha", Origin: OriginRemotePinned, Reference: "https://example.com/a.git@abc"})
	r.UpsertPackage(Package{Alias: "zeta", Origin: OriginLocalLinked, Reference: "/z2"})
	if len(r.Packages) != 2 {
		t.Fatalf("packages = %d, want 2 (upsert must replace, not append)", len(r.Packages))
	}

	list := r.ListPackages()
	if len(list) != 2 || list[0].Alias != "alpha" || list[1].Alias != "zeta" {
		t.Errorf("ListPackages not sorted by alias: %+v", list)
	}
	if list[1].Origin != OriginLocalLinked || list[1].Reference != "/z2" {
		t.Errorf("zeta not replaced by second upsert: %+v", list[1])
	}

	p, ok := r.PackageByAlias("alpha")
	if !ok || p.Reference != "https://example.com/a.git@abc" {
		t.Errorf("PackageByAlias(alpha) = %+v, %v", p, ok)
	}
	if _, ok := r.PackageByAlias("nope"); ok {
		t.Errorf("PackageByAlias(nope) should not be found")
	}
}

func TestExistingComponentsAndConventionsUnaffectedByPackages(t *testing.T) {
	r := &Registry{}
	r.UpsertComponent(Component{Alias: "a", Name: "s", Role: RoleStarter, Entrypoint: "s", StarterLayer: LayerFallback})
	r.UpsertConvention(Convention{Name: "freeform", Prefixes: []string{"{slug}"}})
	r.UpsertPackage(Package{Alias: "a", Origin: OriginLocalPinned, Reference: "/a"})

	if !r.HasComponent("a", "s") {
		t.Error("HasComponent regressed")
	}
	if _, ok := r.ConventionByName("freeform"); !ok {
		t.Error("ConventionByName regressed")
	}
	if got := r.ByRole(RoleStarter); len(got) != 1 {
		t.Error("ByRole regressed")
	}
}

func TestRemoveAliasRetractsComponentsPackageAndUnsharedConventions(t *testing.T) {
	r := &Registry{}
	r.UpsertComponent(Component{Alias: "p", Name: "a", Role: RoleStarter})
	r.UpsertComponent(Component{Alias: "q", Name: "a", Role: RoleStarter})
	r.UpsertConvention(Convention{Name: "own", Prefixes: []string{"{slug}"}})
	r.UpsertConvention(Convention{Name: "shared", Prefixes: []string{"{slug}"}})
	r.UpsertConvention(Convention{Name: "seed", Prefixes: []string{"{slug}"}})
	r.UpsertPackage(Package{Alias: "p", Conventions: []string{"own", "shared"}})
	r.UpsertPackage(Package{Alias: "q", Conventions: []string{"shared"}})

	r.RemoveAlias("p")

	if r.HasComponent("p", "a") || !r.HasComponent("q", "a") {
		t.Errorf("components after RemoveAlias(p): %+v", r.Components)
	}
	if _, ok := r.PackageByAlias("p"); ok {
		t.Errorf("package p survived")
	}
	if _, ok := r.ConventionByName("own"); ok {
		t.Errorf("convention only p declared survived")
	}
	for _, keep := range []string{"shared", "seed"} {
		if _, ok := r.ConventionByName(keep); !ok {
			t.Errorf("convention %q was removed but p did not exclusively declare it", keep)
		}
	}
}

func TestActivationFieldsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	in := &Registry{
		Components: []Component{
			{
				Alias: "p", Name: "lnk", Role: "linker", Entrypoint: "l.py", Runtime: "python3", Key: "github.pull_request",
				Discover: &plugin.Discover{
					Automatic: true,
					On:        []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"s"}}},
					Inputs:    []string{"work:worktree_path"},
				},
				Manual: &plugin.Manual{DisplayName: "L", Description: "d"},
				Inputs: []string{"work:worktree_path"},
			},
			{
				Alias: "p", Name: "imp", Role: "importer", Entrypoint: "i.py",
				On:     []plugin.Subscription{{Event: plugin.EventStartFinalized}},
				Inputs: []string{"link:github.pull_request", "work:start_mode:optional"},
			},
		},
		Packages: []Package{{Alias: "p", Origin: OriginLocalLinked, Reference: "/x", PluginName: "plugin-p"}},
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip changed the registry:\n in: %+v\nout: %+v", in, out)
	}
}

func TestLegacyEntryLoadsInert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	legacy := `{"components":[{"alias":"p","name":"i","role":"importer","entrypoint":"i"}],"conventions":[],"packages":[{"alias":"p","origin":"local-linked","reference":"/x"}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c := r.Components[0]
	if c.On != nil || c.Manual != nil || c.Inputs != nil || c.Discover != nil || c.Key != "" {
		t.Errorf("legacy entry gained activation data: %+v", c)
	}
	if got := r.PluginNameOf("p"); got != "p" {
		t.Errorf("PluginNameOf = %q, want the alias fallback", got)
	}
}

func TestTargetAndEntrypointPath(t *testing.T) {
	dir := filepath.Join("plugins")
	native := Component{Alias: "a", Name: "n", Entrypoint: "starter"}
	script := Component{Alias: "a", Name: "n", Entrypoint: "starter.py", Runtime: "python3"}

	wantNative := filepath.Join(dir, "a", "source", "starter")
	if runtime.GOOS == "windows" {
		wantNative += ".exe"
	}
	if got := native.EntrypointPath(dir); got != wantNative {
		t.Errorf("native EntrypointPath = %q, want %q", got, wantNative)
	}
	if got := native.Target(dir); got != (ipc.Target{Path: wantNative}) {
		t.Errorf("native Target = %+v", got)
	}

	wantScript := filepath.Join(dir, "a", "source", "starter.py") // never ".exe": a script is not executable
	if got := script.EntrypointPath(dir); got != wantScript {
		t.Errorf("script EntrypointPath = %q, want %q", got, wantScript)
	}
	if got := script.Target(dir); got != (ipc.Target{Runtime: "python3", Path: wantScript}) {
		t.Errorf("script Target = %+v", got)
	}
}

func TestQualifiedNameAndPluginNameOf(t *testing.T) {
	if got := (Component{Alias: "gh", Name: "linker"}).QualifiedName(); got != "gh/linker" {
		t.Errorf("QualifiedName = %q", got)
	}
	r := &Registry{Packages: []Package{
		{Alias: "gh", PluginName: "github-plugin"},
		{Alias: "old"},
	}}
	if got := r.PluginNameOf("gh"); got != "github-plugin" {
		t.Errorf("PluginNameOf(gh) = %q", got)
	}
	if got := r.PluginNameOf("old"); got != "old" {
		t.Errorf("PluginNameOf(old) = %q, want alias fallback", got)
	}
	if got := r.PluginNameOf("missing"); got != "missing" {
		t.Errorf("PluginNameOf(missing) = %q, want alias fallback", got)
	}
}

func TestExtensionsReturnsOnlyImportersAndLinkers(t *testing.T) {
	r := &Registry{Components: []Component{
		{Alias: "a", Name: "s", Role: RoleStarter},
		{Alias: "a", Name: "l", Role: plugin.RoleLinker},
		{Alias: "a", Name: "i", Role: plugin.RoleImporter},
		{Alias: "a", Name: "loc", Role: RoleRepositoryLocator},
	}}
	if got := r.Extensions(plugin.RoleLinker); len(got) != 1 || got[0].Name != "l" {
		t.Errorf("Extensions(linker) = %+v", got)
	}
	if got := r.Extensions(plugin.RoleImporter); len(got) != 1 || got[0].Name != "i" {
		t.Errorf("Extensions(importer) = %+v", got)
	}
	for _, role := range []string{RoleStarter, RoleRepositoryLocator, ""} {
		if got := r.Extensions(role); got != nil {
			t.Errorf("Extensions(%q) = %+v, want nothing", role, got)
		}
	}
}
