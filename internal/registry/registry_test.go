package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
)

var (
	refProvider  = pluginid.Provider{Marketplace: "builtin", Plugin: "work-reference"}
	teamProvider = pluginid.Provider{Marketplace: "team", Plugin: "github"}
	homonymA     = pluginid.Provider{Marketplace: "alpha", Plugin: "sample"}
	homonymB     = pluginid.Provider{Marketplace: "beta", Plugin: "sample"}
)

// pkg builds a valid installed package for a provider.
func pkg(p pluginid.Provider, enabled bool) Package {
	prov := Provenance{
		RegistrationID: "0123456789abcdef0123456789abcdef", Kind: pluginid.KindGit,
		Source: "https://example.org/" + p.Marketplace + ".git", Branch: "main",
		MarketplaceName: p.Marketplace, PluginName: p.Plugin, Path: "plugins/" + p.Plugin,
		Revision: "0123456789abcdef", CatalogDigest: "sha256:abc",
	}
	if p.Marketplace == pluginid.Builtin {
		prov.RegistrationID, prov.Kind, prov.Source, prov.Branch = pluginid.BuiltinRegistrationID, pluginid.KindBuiltin, "builtin", ""
	}
	return Package{Provider: p, InstallMode: InstallModeCopy, Enabled: enabled, Version: "1", ContentDigest: "sha256:def", DigestAlgorithm: DigestPackageV1, Provenance: prov}
}

func comp(p pluginid.Provider, name, role string) Component {
	return Component{Provider: p, Name: name, Role: role, Entrypoint: name}
}

func TestUpsertComponentIdempotentByProvider(t *testing.T) {
	r := New()
	c := Component{Provider: refProvider, Name: "local-path-starter", Role: RoleStarter, Entrypoint: "starter", StarterLayer: LayerFallback}
	r.UpsertComponent(c)
	r.UpsertComponent(c)
	if len(r.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(r.Components))
	}
	c.Entrypoint = "starter.exe"
	r.UpsertComponent(c)
	if len(r.Components) != 1 || r.Components[0].Entrypoint != "starter.exe" {
		t.Errorf("upsert did not replace: %+v", r.Components)
	}
	// The same bare name from another provider is a different component.
	other := c
	other.Provider = teamProvider
	r.UpsertComponent(other)
	if len(r.Components) != 2 {
		t.Errorf("homonymous component overwrote another provider: %+v", r.Components)
	}
}

func TestHomonymousConventionsStayIndependent(t *testing.T) {
	r := New()
	r.UpsertConvention(Convention{Provider: homonymA, Name: "freeform", Prefixes: []string{"{slug}"}})
	r.UpsertConvention(Convention{Provider: homonymB, Name: "freeform", Prefixes: []string{"x/{slug}"}})
	r.UpsertConvention(Convention{Provider: homonymA, Name: "freeform", Prefixes: []string{"{slug}", "y"}})
	if len(r.Conventions) != 2 {
		t.Fatalf("conventions = %d, want one per provider", len(r.Conventions))
	}
	a, _ := r.ConventionByRef(pluginid.Ref{Provider: homonymA, Name: "freeform"})
	b, _ := r.ConventionByRef(pluginid.Ref{Provider: homonymB, Name: "freeform"})
	if len(a.Prefixes) != 2 || len(b.Prefixes) != 1 || b.Prefixes[0] != "x/{slug}" {
		t.Errorf("ownership mixed: a=%+v b=%+v", a, b)
	}
	if got := r.ConventionsNamed("freeform"); len(got) != 2 {
		t.Errorf("ConventionsNamed = %+v", got)
	}
}

func TestQueries(t *testing.T) {
	r := New()
	r.UpsertComponent(Component{Provider: refProvider, Name: "s", Role: RoleStarter, Entrypoint: "s", StarterLayer: LayerFallback})
	r.UpsertComponent(Component{Provider: refProvider, Name: "l", Role: RoleRepositoryLocator, Entrypoint: "l", Accepts: []string{"name"}})
	if got := r.ByRole(RoleStarter); len(got) != 1 || got[0].Name != "s" {
		t.Errorf("ByRole(starter) = %+v", got)
	}
	if sf, ok := r.StarterFallback(); !ok || sf.Name != "s" {
		t.Errorf("StarterFallback = %+v, %v", sf, ok)
	}
	if !r.HasComponent(pluginid.Ref{Provider: refProvider, Name: "l"}) || r.HasComponent(pluginid.Ref{Provider: refProvider, Name: "nope"}) || r.HasComponent(pluginid.Ref{Provider: teamProvider, Name: "l"}) {
		t.Errorf("HasComponent wrong")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "registry.json")
	if r, err := Load(path); err != nil || len(r.Components) != 0 || r.Schema != Schema {
		t.Fatalf("Load(missing) = %+v, %v", r, err)
	}
	in := New()
	in.UpsertPackage(pkg(refProvider, true))
	in.UpsertComponent(Component{Provider: refProvider, Name: "local-path-starter", Role: RoleStarter, Entrypoint: "starter", StarterLayer: LayerFallback})
	in.UpsertConvention(Convention{Provider: refProvider, Name: "freeform", Prefixes: []string{"{slug}"}})
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip changed the registry:\n in: %+v\nout: %+v", in, out)
	}
}

func TestStrictValidation(t *testing.T) {
	good := New()
	good.UpsertPackage(pkg(teamProvider, true))
	good.UpsertComponent(comp(teamProvider, "run", plugin.RoleLinker))
	if err := good.Validate(); err != nil {
		t.Fatalf("valid registry rejected: %v", err)
	}

	mutate := map[string]func(r *Registry){
		"future schema":          func(r *Registry) { r.Schema = 2 },
		"missing schema":         func(r *Registry) { r.Schema = 0 },
		"orphan component":       func(r *Registry) { r.Components[0].Provider = homonymA },
		"orphan convention":      func(r *Registry) { r.Conventions = append(r.Conventions, Convention{Provider: homonymA, Name: "x"}) },
		"duplicate package":      func(r *Registry) { r.Packages = append(r.Packages, r.Packages[0]) },
		"duplicate component":    func(r *Registry) { r.Components = append(r.Components, r.Components[0]) },
		"bad provider":           func(r *Registry) { r.Packages[0].Provider.Plugin = "a/b"; r.Components[0].Provider.Plugin = "a/b" },
		"bad install mode":       func(r *Registry) { r.Packages[0].InstallMode = "link" },
		"missing mode":           func(r *Registry) { r.Packages[0].InstallMode = "" },
		"missing digest":         func(r *Registry) { r.Packages[0].ContentDigest = "" },
		"wrong algorithm":        func(r *Registry) { r.Packages[0].DigestAlgorithm = "md5" },
		"provenance name":        func(r *Registry) { r.Packages[0].Provenance.PluginName = "other" },
		"provenance marketplace": func(r *Registry) { r.Packages[0].Provenance.MarketplaceName = "other" },
		"bad registration":       func(r *Registry) { r.Packages[0].Provenance.RegistrationID = "nope" },
		"git without branch":     func(r *Registry) { r.Packages[0].Provenance.Branch = "" },
		"unknown kind":           func(r *Registry) { r.Packages[0].Provenance.Kind = "svn" },
		"builtin spoof":          func(r *Registry) { r.Packages[0].Provenance.Kind = pluginid.KindBuiltin },
		"empty revision":         func(r *Registry) { r.Packages[0].Provenance.Revision = "" },
	}
	for name, fn := range mutate {
		r := New()
		r.UpsertPackage(pkg(teamProvider, true))
		r.UpsertComponent(comp(teamProvider, "run", plugin.RoleLinker))
		fn(r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestDecodeRejectsPriorShapes(t *testing.T) {
	for name, doc := range map[string]string{
		"unversioned alias shape": `{"components":[{"alias":"p","name":"i","role":"importer","entrypoint":"i"}],"conventions":[],"packages":[{"alias":"p","origin":"local-linked","reference":"/x"}]}`,
		"unknown field":           `{"schema":1,"packages":[],"components":[],"conventions":[],"extra":1}`,
		"trailing document":       `{"schema":1,"packages":[],"components":[],"conventions":[]}{}`,
		"not json":                `{`,
	} {
		if _, err := Decode([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Decode([]byte(`{"schema":1}`)); err != nil {
		t.Errorf("minimal current document rejected: %v", err)
	}
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(`{"schema":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "registry.json") {
		t.Errorf("Load error must name the file: %v", err)
	}
}

func TestUpsertPackageIdempotentAndSorted(t *testing.T) {
	r := New()
	r.UpsertPackage(pkg(pluginid.Provider{Marketplace: "zeta", Plugin: "a"}, true))
	r.UpsertPackage(pkg(pluginid.Provider{Marketplace: "alpha", Plugin: "b"}, true))
	r.UpsertPackage(pkg(pluginid.Provider{Marketplace: "alpha", Plugin: "a"}, true))
	changed := pkg(pluginid.Provider{Marketplace: "zeta", Plugin: "a"}, false)
	changed.Version = "2"
	r.UpsertPackage(changed)
	if len(r.Packages) != 3 {
		t.Fatalf("packages = %d, want 3 (upsert replaces)", len(r.Packages))
	}
	list := r.ListPackages()
	got := []string{list[0].ID(), list[1].ID(), list[2].ID()}
	if !reflect.DeepEqual(got, []string{"a@alpha", "b@alpha", "a@zeta"}) {
		t.Errorf("ListPackages order = %v", got)
	}
	if p, ok := r.PackageByProvider(pluginid.Provider{Marketplace: "zeta", Plugin: "a"}); !ok || p.Version != "2" || p.Enabled {
		t.Errorf("replacement lost: %+v", p)
	}
	if _, ok := r.PackageByProvider(pluginid.Provider{Marketplace: "zeta", Plugin: "nope"}); ok {
		t.Error("unknown provider found")
	}
}

func TestRemoveProviderRetractsOnlyItsRecords(t *testing.T) {
	r := New()
	for _, p := range []pluginid.Provider{homonymA, homonymB} {
		r.UpsertPackage(pkg(p, true))
		r.UpsertComponent(comp(p, "run", plugin.RoleLinker))
		r.UpsertConvention(Convention{Provider: p, Name: "freeform", Prefixes: []string{"{slug}"}})
	}
	r.RemoveProvider(homonymA)
	if len(r.Packages) != 1 || len(r.Components) != 1 || len(r.Conventions) != 1 {
		t.Fatalf("remove touched the wrong records: %+v", r)
	}
	if r.Packages[0].Provider != homonymB || r.Components[0].Provider != homonymB || r.Conventions[0].Provider != homonymB {
		t.Errorf("survivor is not the sibling provider: %+v", r)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("registry invalid after removal: %v", err)
	}
}

func TestActivationFieldsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	in := New()
	in.UpsertPackage(pkg(teamProvider, true))
	c := comp(teamProvider, "imp", plugin.RoleImporter)
	c.On = []plugin.Subscription{{Event: "started"}}
	c.Inputs = []string{"work:id"}
	c.Key = "k"
	in.UpsertComponent(c)
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Components, out.Components) {
		t.Errorf("activation data changed: %+v vs %+v", in.Components, out.Components)
	}
}

func TestTargetAndEntrypointPath(t *testing.T) {
	dir := filepath.Join("plugins")
	native := Component{Provider: teamProvider, Name: "n", Entrypoint: "starter"}
	script := Component{Provider: teamProvider, Name: "n", Entrypoint: "starter.py", Runtime: "python3"}

	wantNative := filepath.Join(dir, "team", "github", "source", "starter")
	if runtime.GOOS == "windows" {
		wantNative += ".exe"
	}
	if got := native.EntrypointPath(dir); got != wantNative {
		t.Errorf("native EntrypointPath = %q, want %q", got, wantNative)
	}
	if got := native.Target(dir); got != (ipc.Target{Path: wantNative}) {
		t.Errorf("native Target = %+v", got)
	}
	wantScript := filepath.Join(dir, "team", "github", "source", "starter.py")
	if got := script.EntrypointPath(dir); got != wantScript {
		t.Errorf("script EntrypointPath = %q, want %q", got, wantScript)
	}
	if got := script.Target(dir); got != (ipc.Target{Runtime: "python3", Path: wantScript}) {
		t.Errorf("script Target = %+v", got)
	}
}

func TestQualifiedName(t *testing.T) {
	if got := (Component{Provider: teamProvider, Name: "linker"}).QualifiedName(); got != "github@team/linker" {
		t.Errorf("QualifiedName = %q", got)
	}
	if got := (Convention{Provider: refProvider, Name: "freeform"}).Ref().String(); got != "work-reference@builtin/freeform" {
		t.Errorf("Convention ref = %q", got)
	}
}

func TestExtensionsReturnsOnlyImportersAndLinkers(t *testing.T) {
	r := New()
	r.Components = []Component{
		comp(teamProvider, "s", RoleStarter), comp(teamProvider, "l", plugin.RoleLinker),
		comp(teamProvider, "i", plugin.RoleImporter), comp(teamProvider, "loc", RoleRepositoryLocator),
	}
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

func TestEffectiveUsesActualOwnerEnablement(t *testing.T) {
	r := New()
	r.UpsertPackage(pkg(homonymA, false))
	r.UpsertPackage(pkg(homonymB, true))
	for _, p := range []pluginid.Provider{homonymA, homonymB} {
		r.UpsertComponent(comp(p, "run", plugin.RoleLinker))
		r.UpsertConvention(Convention{Provider: p, Name: "team", Prefixes: []string{"{slug}"}})
	}
	if r.PackageEnabled(homonymA) || !r.PackageEnabled(homonymB) || r.PackageEnabled(teamProvider) {
		t.Fatal("PackageEnabled must reflect each exact provider and treat unknown as disabled")
	}
	e := r.Effective()
	if len(e.Packages) != 1 || len(e.Components) != 1 || len(e.Conventions) != 1 {
		t.Fatalf("effective = %+v", e)
	}
	if e.Components[0].Provider != homonymB || e.Conventions[0].Provider != homonymB {
		t.Errorf("a disabled homonym's convention satisfied availability: %+v", e)
	}
	if len(r.Conventions) != 2 {
		t.Error("raw ownership lost")
	}
}
