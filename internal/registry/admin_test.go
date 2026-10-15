package registry

import (
	"encoding/json"
	"testing"
)

func TestAdminMetadataDefaults(t *testing.T) {
	var r Registry
	if err := json.Unmarshal([]byte(`{"packages":[{"alias":"a"}],"components":[{"alias":"a","name":"run"},{"alias":"legacy","name":"old"}]}`), &r); err != nil {
		t.Fatal(err)
	}
	if !r.PackageEnabled("a") || !r.PackageEnabled("legacy") || len(r.Effective().Components) != 2 {
		t.Fatal("legacy enablement")
	}
	b := false
	r.Packages[0].Enabled = &b
	if len(r.Effective().Components) != 1 {
		t.Fatal("disabled activation")
	}
}
func TestAdminSharedConvention(t *testing.T) {
	f := false
	r := Registry{Packages: []Package{{Alias: "a", Enabled: &f, Conventions: []string{"team"}}, {Alias: "b", Conventions: []string{"team"}}}, Conventions: []Convention{{Name: "team"}}}
	if len(r.Effective().Conventions) != 1 {
		t.Fatal("enabled shared owner lost")
	}
	r.Packages[1].Enabled = &f
	if len(r.Effective().Conventions) != 0 || len(r.Conventions) != 1 {
		t.Fatal("raw ownership lost")
	}
}
