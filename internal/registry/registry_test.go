package registry

import (
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
}
