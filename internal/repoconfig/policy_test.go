package repoconfig

import (
	"github.com/gustaborges/work/internal/registry"
	"testing"
)

func TestAdminLocatorReasonPrecedence(t *testing.T) {
	off := false
	reg := &registry.Registry{Packages: []registry.Package{{Alias: "disabled", Enabled: &off}}, Components: []registry.Component{{Alias: "disabled", Name: "wrong", Role: "starter"}, {Alias: "disabled", Name: "clone", Role: registry.RoleRepositoryLocator}, {Alias: "legacy", Name: "clone", Role: registry.RoleRepositoryLocator}}}
	for ref, want := range map[string]string{"missing/clone": "package-missing", "disabled/missing": "component-missing", "disabled/wrong": "role-changed", "disabled/clone": "package-disabled", "legacy/clone": ""} {
		state, reason := LocatorState(reg, ref)
		got := ""
		if reason != nil {
			got = *reason
		}
		if got != want || (state == "available") != (want == "") {
			t.Fatalf("%s: %s %v", ref, state, reason)
		}
	}
}
