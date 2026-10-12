package extension

import (
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"slices"
	"testing"
)

func manualLink(alias string) registry.Component {
	return registry.Component{Alias: alias, Name: "linker", Role: plugin.RoleLinker, Key: "github.pull_request", Runtime: "missing-runtime", Inputs: []string{"meta:missing.value"}, Manual: &plugin.Manual{DisplayName: "PR relation", Description: "Associate a PR"}}
}
func TestManualPolicyAndResolution(t *testing.T) {
	a, b := manualLink("a"), manualLink("b")
	reg := &registry.Registry{Components: []registry.Component{b, a, importer("automatic", nil)}}
	ds := ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))
	if !slices.Equal(names(ds), []string{"a/linker", "b/linker"}) || ds[0].Component.Manual.Description != "Associate a PR" {
		t.Fatal(ds)
	}
	if _, err := ResolveManual(reg, plugin.RoleLinker, "linker"); diag.ExitCode(err) != 40 {
		t.Fatal(err)
	}
	if c, err := ResolveManual(reg, plugin.RoleLinker, "a/linker"); err != nil || c.Key != a.Key {
		t.Fatal(c, err)
	}
	disabled := false
	reg.Packages = []registry.Package{{Alias: "a", PluginName: "owner", Enabled: &disabled}, {Alias: "b", PluginName: "owner"}}
	if c, err := ResolveManual(reg, plugin.RoleLinker, "linker"); err != nil || c.Alias != "b" {
		t.Fatal(c, err)
	}
	for _, ref := range []string{"a/linker", "unknown", "acme/automatic"} {
		if _, err := ResolveManual(reg, plugin.RoleLinker, ref); diag.ExitCode(err) != 39 {
			t.Fatal(ref, err)
		}
	}
	reg.Components[0].Key = "plugin.owner.relation"
	if len(ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))) != 1 {
		t.Fatal("manifest owner ignored")
	}
	reg.Components[0].Key = "plugin.b.relation"
	if len(ManualCandidates(reg, plugin.RoleLinker, subject("new", nil, nil))) != 0 {
		t.Fatal("alias granted private ownership")
	}
}
func TestManualImporterAmbiguityBeforeInputs(t *testing.T) {
	a, b := manualLink("a"), manualLink("b")
	a.Role = plugin.RoleImporter
	b.Role = plugin.RoleImporter
	a.Inputs = nil
	b.Inputs = []string{"link:github.issue"}
	reg := &registry.Registry{Components: []registry.Component{b, a}}
	if len(ManualCandidates(reg, plugin.RoleImporter, subject("new", nil, nil))) != 1 {
		t.Fatal("input filtering")
	}
	if _, err := ResolveManual(reg, plugin.RoleImporter, "linker"); diag.ExitCode(err) != 40 {
		t.Fatal(err)
	}
}
