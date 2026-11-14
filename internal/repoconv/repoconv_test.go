package repoconv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

const (
	instanceA = "0123456789abcdef0123456789abcdef"
	instanceB = "fedcba9876543210fedcba9876543210"
)

func ref(instance, plugin, marketplace, name string) Reference {
	return NewReference(instance, pluginid.Ref{Provider: pluginid.Provider{Marketplace: marketplace, Plugin: plugin}, Name: name})
}

func TestGetUnknownIdentity(t *testing.T) {
	s := New()
	if got, ok := s.Get("nope"); ok || got != (Reference{}) {
		t.Errorf("Get(unknown) = %+v, %v, want zero, false", got, ok)
	}
}

func TestSetThenGetAndOverwrite(t *testing.T) {
	s := New()
	s.Set("repo-a", ref(instanceA, "p", "m", "gitflow"))
	s.Set("repo-a", ref(instanceA, "p", "m", "freeform"))
	if len(s.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 (Set must upsert)", len(s.Entries))
	}
	got, ok := s.Get("repo-a")
	if !ok || got.Name != "freeform" || got.String() != "p@m/freeform" {
		t.Errorf("Get(repo-a) = %+v, %v", got, ok)
	}
}

func TestLoadMissingFileYieldsEmptyCurrentStore(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "state", "branch_conventions.json"))
	if err != nil || len(s.Entries) != 0 || s.Schema != Schema {
		t.Fatalf("Load(missing) = %+v, %v", s, err)
	}
}

func TestSaveLoadRoundTripIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "branch_conventions.json")
	in := New()
	in.Set("repo-a", ref(instanceA, "p", "m", "gitflow"))
	in.Set("repo-b", ref(pluginid.BuiltinRegistrationID, "work-reference", "builtin", "freeform"))
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := out.Get("repo-b"); got.RegistrationID != pluginid.BuiltinRegistrationID {
		t.Errorf("round trip lost builtin instance: %+v", got)
	}
	if err := Save(path, out); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Error("reload and save changed the bytes")
	}
}

func TestStrictCurrentSchema(t *testing.T) {
	good := `{"schema":1,"entries":[{"identity":"r","reference":{"registration_id":"` + instanceA + `","marketplace":"m","plugin":"p","name":"c"}}]}`
	if _, err := Decode([]byte(good)); err != nil {
		t.Fatalf("valid memory rejected: %v", err)
	}
	for name, doc := range map[string]string{
		"bare string shape": `{"entries":[{"identity":"r","convention":"freeform"}]}`,
		"missing schema":    `{"entries":[]}`,
		"future schema":     `{"schema":2,"entries":[]}`,
		"unknown field":     `{"schema":1,"entries":[],"x":1}`,
		"stray brace":       `{"schema":1,"entries":[]}}`,
		"stray bracket":     `{"schema":1,"entries":[]}]`,
		"bad instance":      `{"schema":1,"entries":[{"identity":"r","reference":{"registration_id":"zz","marketplace":"m","plugin":"p","name":"c"}}]}`,
		"bad name":          `{"schema":1,"entries":[{"identity":"r","reference":{"registration_id":"` + instanceA + `","marketplace":"m","plugin":"p","name":"a/b"}}]}`,
		"duplicate":         `{"schema":1,"entries":[{"identity":"r","reference":{"registration_id":"` + instanceA + `","marketplace":"m","plugin":"p","name":"c"}},{"identity":"r","reference":{"registration_id":"` + instanceA + `","marketplace":"m","plugin":"p","name":"d"}}]}`,
	} {
		if _, err := Decode([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func registryWith(instance string, enabled bool) *registry.Registry {
	r := registry.New()
	p := pluginid.Provider{Marketplace: "m", Plugin: "p"}
	r.UpsertPackage(registry.Package{
		Provider: p, InstallMode: registry.InstallModeCopy, Enabled: enabled, Version: "1", ContentDigest: "sha256:x", DigestAlgorithm: registry.DigestPackageV1,
		Provenance: registry.Provenance{RegistrationID: instance, Kind: pluginid.KindLocal, Source: "/src", MarketplaceName: "m", PluginName: "p", Path: "p", Revision: "r", CatalogDigest: "sha256:c"},
	})
	r.UpsertConvention(registry.Convention{Provider: p, Name: "c", Prefixes: []string{"{slug}"}})
	return r
}

func TestViewBindsTheSourceInstance(t *testing.T) {
	s := New()
	s.Set("repo", ref(instanceA, "p", "m", "c"))

	if v := s.View("repo", registryWith(instanceA, true)); !v.Available || v.Convention == nil || *v.Convention != "p@m/c" {
		t.Errorf("same instance should be available: %+v", v)
	}
	// A removed-and-re-registered source reuses every public name but has a new
	// registration instance: it must not satisfy the retained choice.
	v := s.View("repo", registryWith(instanceB, true))
	if v.Available || v.UnavailableReason == nil || *v.UnavailableReason != "provider-unavailable" {
		t.Errorf("replacement instance rebound the choice: %+v", v)
	}
	if v := s.View("repo", registryWith(instanceA, false)); v.Available {
		t.Errorf("a disabled provider satisfied availability: %+v", v)
	}
	if v := s.View("repo", registry.New()); v.Available || v.Convention == nil {
		t.Errorf("uninstalled provider: %+v", v)
	}
	if v := s.View("none", registryWith(instanceA, true)); v.Convention != nil || v.UnavailableReason == nil || *v.UnavailableReason != "not-set" {
		t.Errorf("unset identity: %+v", v)
	}
}
