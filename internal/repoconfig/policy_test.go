package repoconfig

import (
	"testing"

	"github.com/gustaborges/work/internal/registry"
)

func TestLocatorReasonPrecedence(t *testing.T) {
	reg := regOf([]string{"disabled"},
		registry.Component{Provider: prov("disabled"), Name: "wrong", Role: "starter", Entrypoint: "w"},
		locatorComponent("disabled", "clone"),
		locatorComponent("enabled", "clone"),
	)
	for ref, want := range map[string]string{
		ref("missing", "clone"):  "package-missing",
		ref("disabled", "none"):  "component-missing",
		ref("disabled", "wrong"): "role-changed",
		ref("disabled", "clone"): "package-disabled",
		ref("enabled", "clone"):  "",
		"disabled/clone":         "reference-invalid", // the retired alias form is not a reference
		"clone":                  "reference-invalid",
	} {
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
