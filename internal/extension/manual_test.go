package extension

import (
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/tests/fixtures/installfx"
	"slices"
	"testing"
)

func manualLink(owner string) registry.Component {
	return registry.Component{Provider: installfx.P(owner), Entrypoint: "e", Name: "linker", Role: plugin.RoleLinker, Key: "github.pull_request", Runtime: "missing-runtime", Inputs: []string{"meta:missing.value"}, Manual: &plugin.Manual{DisplayName: "PR relation", Description: "Associate a PR"}}
}
func TestManualPolicyAndResolution(t *testing.T) {
	a, b := manualLink("a"), manualLink("b")
	reg := installfx.Reg(nil, b, a, importer("automatic", nil))
	ds := ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))
	if !slices.Equal(names(ds), []string{"a@m/linker", "b@m/linker"}) || ds[0].Component.Manual.Description != "Associate a PR" {
		t.Fatal(ds)
	}
	if _, err := ResolveManual(reg, plugin.RoleLinker, "linker"); diag.ExitCode(err) != 40 {
		t.Fatal(err)
	}
	if c, err := ResolveManual(reg, plugin.RoleLinker, "a@m/linker"); err != nil || c.Key != a.Key {
		t.Fatal(c, err)
	}
	installfx.SetPackageEnabled(reg, "a", false)
	if c, err := ResolveManual(reg, plugin.RoleLinker, "linker"); err != nil || c.Provider.Plugin != "b" {
		t.Fatal(c, err)
	}
	for _, ref := range []string{"a@m/linker", "unknown", "acme@m/automatic", "a/linker"} {
		if _, err := ResolveManual(reg, plugin.RoleLinker, ref); diag.ExitCode(err) != 39 {
			t.Fatal(ref, err)
		}
	}
	reg.Components[0].Key = "plugin.b.relation"
	if len(ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))) != 1 {
		t.Fatal("the provider's own plugin name must own its private keys")
	}
	reg.Components[0].Key = "plugin.a.relation"
	if len(ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))) != 0 {
		t.Fatal("another plugin's private key was accepted")
	}
}
func TestManualImporterAmbiguityBeforeInputs(t *testing.T) {
	a, b := manualLink("a"), manualLink("b")
	a.Role = plugin.RoleImporter
	b.Role = plugin.RoleImporter
	a.Inputs = nil
	b.Inputs = []string{"link:github.issue"}
	reg := installfx.Reg(nil, b, a)
	if len(ManualCandidates(reg, plugin.RoleImporter, subject("new", nil, nil))) != 1 {
		t.Fatal("input filtering")
	}
	if _, err := ResolveManual(reg, plugin.RoleImporter, "linker"); diag.ExitCode(err) != 40 {
		t.Fatal(err)
	}
}
