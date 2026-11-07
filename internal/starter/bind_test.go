package starter

import (
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func TestBindStoresPrivateKeysAgainstThePublishingSourceInstance(t *testing.T) {
	alpha := pluginid.Provider{Marketplace: "alpha", Plugin: "sample"}
	beta := pluginid.Provider{Marketplace: "beta", Plugin: "sample"}
	reg := registry.New()
	for _, p := range []pluginid.Provider{alpha, beta} {
		reg.UpsertPackage(installfx.Package(p, true))
	}
	ref := Reference{
		Meta:  map[string]any{"plugin.sample.build_id": "42", "github.pull_request.number": 7},
		Links: map[string]string{"plugin.sample.url": "https://example.test/1", "github.pull_request": "https://example.test/pr"},
	}
	component := func(p pluginid.Provider) registry.Component {
		return registry.Component{Provider: p, Name: "s", Role: registry.RoleStarter, Entrypoint: "s"}
	}
	if err := ValidateResponse(ref, "sample"); err != nil {
		t.Fatal(err)
	}
	a := Bind(ref, component(alpha), reg)
	b := Bind(ref, component(beta), reg)

	idA, idB := installfx.RegistrationID("alpha"), installfx.RegistrationID("beta")
	if a.Meta[semconv.StorageKey(alpha, idA, "build_id")] != "42" || b.Meta[semconv.StorageKey(beta, idB, "build_id")] != "42" {
		t.Fatalf("private meta not stored per provider: %v / %v", a.Meta, b.Meta)
	}
	if _, ok := a.Meta["plugin.sample.build_id"]; ok {
		t.Fatal("the relative private spelling must not be stored")
	}
	if a.Meta["github.pull_request.number"] != 7 || a.Links["github.pull_request"] != "https://example.test/pr" {
		t.Fatalf("public keys are shared semantic keys and must stay unchanged: %v %v", a.Meta, a.Links)
	}
	if a.Links[semconv.StorageKey(alpha, idA, "url")] == "" {
		t.Fatalf("private link not stored per provider: %v", a.Links)
	}
	if ref.Meta["plugin.sample.build_id"] != "42" {
		t.Fatal("Bind must not mutate the response it was given")
	}
}

func TestStarterHintsNameCurrentCommands(t *testing.T) {
	_, err := Select(registry.New())
	if err == nil {
		t.Fatal("an empty registry has no Starter")
	}
	for _, retired := range []string{"plugin install", "--embedded"} {
		if strings.Contains(err.Error(), retired) {
			t.Errorf("hint still names retired syntax %q: %v", retired, err)
		}
	}
}
