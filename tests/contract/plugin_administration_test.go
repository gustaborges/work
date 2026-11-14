package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func TestInventoryShape(t *testing.T) {
	z, a := installfx.P("z"), installfx.P("a")
	reg := registry.New()
	pz := installfx.Package(z, true)
	pz.Provenance.Source = "https://alice:secret@example.test/pkg?token=hidden"
	reg.UpsertPackage(pz)
	reg.UpsertPackage(installfx.Package(a, false))
	reg.UpsertComponent(registry.Component{Provider: a, Name: "clone", Role: registry.RoleRepositoryLocator, Entrypoint: "c"})
	reg.UpsertConvention(registry.Convention{Provider: a, Name: "team", Prefixes: []string{"{slug}"}})
	s := &adminstate.Snapshot{Registry: reg, Inspection: map[string]string{"a@m": "ready"}}
	rows := pluginadmin.Inventory(s)
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var array []map[string]any
	if err = json.Unmarshal(b, &array); err != nil || len(array) != 2 || array[0]["id"] != "a@m" || array[1]["id"] != "z@m" {
		t.Fatal(string(b), err)
	}
	for _, r := range array {
		for _, key := range []string{"id", "name", "marketplace", "enabled", "version", "content_digest", "inspection_status", "source_kind", "source", "revision", "components", "conventions"} {
			if _, ok := r[key]; !ok {
				t.Errorf("missing %s", key)
			}
		}
		for _, key := range []string{"components", "conventions"} {
			if r[key] == nil {
				t.Errorf("null %s", key)
			}
		}
		for _, retired := range []string{"alias", "origin", "reference"} {
			if _, ok := r[retired]; ok {
				t.Errorf("retired field %q present", retired)
			}
		}
	}
	if first := array[0]["components"].([]any)[0].(map[string]any); first["id"] != "a@m/clone" {
		t.Errorf("components must carry full provider references: %v", first)
	}
	if first := array[0]["conventions"].([]any)[0]; first != "a@m/team" {
		t.Errorf("conventions must carry full provider references: %v", first)
	}
	for _, secret := range []string{"alice", "secret", "hidden"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("leaked %s", secret)
		}
	}
}
