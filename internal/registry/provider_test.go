package registry_test

import (
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

const (
	instOld = "11111111111111111111111111111111"
	instNew = "22222222222222222222222222222222"
)

func withInstance(p pluginid.Provider, instance string, enabled bool) registry.Package {
	pkg := installfx.Package(p, enabled)
	pkg.Provenance.RegistrationID = instance
	return pkg
}

// Two providers publishing the same plugin, component and convention names are
// four independent records with disjoint managed paths.
func TestHomonymsOwnTheirComponentsConventionsAndPaths(t *testing.T) {
	a := pluginid.Provider{Marketplace: "alpha", Plugin: "sample"}
	b := pluginid.Provider{Marketplace: "beta", Plugin: "sample"}
	r := registry.New()
	for i, p := range []pluginid.Provider{a, b} {
		r.UpsertPackage(withInstance(p, []string{instOld, instNew}[i], true))
		r.UpsertComponent(registry.Component{Provider: p, Name: "run", Role: "linker", Entrypoint: "bin/tool"})
		r.UpsertConvention(registry.Convention{Provider: p, Name: "team", Prefixes: []string{p.Marketplace + "/{slug}"}})
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(r.Components) != 2 || len(r.Conventions) != 2 || len(r.ConventionsNamed("team")) != 2 {
		t.Fatalf("homonymous records merged: %d components, %d conventions", len(r.Components), len(r.Conventions))
	}
	pa := r.Components[0].EntrypointPath("/plugins")
	pb := r.Components[1].EntrypointPath("/plugins")
	if pa == pb || filepath.Dir(pa) == filepath.Dir(pb) {
		t.Fatalf("homonyms share a managed path: %q %q", pa, pb)
	}
	cv, _ := r.ConventionByRef(pluginid.Ref{Provider: b, Name: "team"})
	if cv.Prefixes[0] != "beta/{slug}" {
		t.Fatalf("beta's convention = %+v", cv)
	}

	// Removing or disabling one owner leaves the other's records untouched.
	pkg, _ := r.PackageByProvider(a)
	pkg.Enabled = false
	r.UpsertPackage(pkg)
	eff := r.Effective()
	if len(eff.Conventions) != 1 || eff.Conventions[0].Provider != b || len(eff.Components) != 1 {
		t.Fatalf("effective = %+v", eff)
	}
	r.RemoveProvider(a)
	if len(r.Components) != 1 || len(r.Conventions) != 1 || r.Components[0].Provider != b {
		t.Fatalf("removing alpha disturbed beta: %+v", r)
	}
}

// A remembered convention choice names its provider and source instance. After
// the source is removed and registered again, equal public names with a new
// instance must not satisfy it, while the original instance still does.
func TestRemovedAndReRegisteredSourceDoesNotRebindRetainedChoice(t *testing.T) {
	p := pluginid.Provider{Marketplace: "team", Plugin: "tools"}
	build := func(instance string) *registry.Registry {
		r := registry.New()
		r.UpsertPackage(withInstance(p, instance, true))
		r.UpsertConvention(registry.Convention{Provider: p, Name: "flow", Prefixes: []string{"{slug}"}})
		return r
	}
	mem := repoconv.New()
	mem.Set("repo", repoconv.NewReference(instOld, pluginid.Ref{Provider: p, Name: "flow"}))

	if !repoconv.Available(mustGet(t, mem), build(instOld)) {
		t.Fatal("the original instance lost its choice")
	}
	removed := build(instOld)
	removed.RemoveProvider(p)
	if repoconv.Available(mustGet(t, mem), removed) {
		t.Fatal("a removed provider satisfied the choice")
	}
	if v := mem.View("repo", removed); v.Convention == nil || v.Available {
		t.Fatalf("a removed source must leave the choice remembered but unavailable: %+v", v)
	}
	if repoconv.Available(mustGet(t, mem), build(instNew)) {
		t.Fatal("a re-registered source with equal names claimed the retained choice")
	}
	// An explicit new choice against the new instance is a separate, valid memory.
	mem.Set("repo", repoconv.NewReference(instNew, pluginid.Ref{Provider: p, Name: "flow"}))
	if !repoconv.Available(mustGet(t, mem), build(instNew)) {
		t.Fatal("an explicit new choice did not take effect")
	}
}

func mustGet(t *testing.T, s *repoconv.Store) repoconv.Reference {
	t.Helper()
	ref, ok := s.Get("repo")
	if !ok {
		t.Fatal("no remembered choice")
	}
	return ref
}
