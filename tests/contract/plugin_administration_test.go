package contract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/registry"
)

func TestAdminInventoryShape(t *testing.T) {
	s := &adminstate.Snapshot{Registry: &registry.Registry{Packages: []registry.Package{{Alias: "z", Reference: "https://alice:secret@example.test/pkg?token=hidden@sha", Origin: registry.OriginRemotePinned}, {Alias: "a"}}}}
	rows := pluginadmin.Inventory(s)
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var array []map[string]any
	if err = json.Unmarshal(b, &array); err != nil || len(array) != 2 || array[0]["alias"] != "a" {
		t.Fatal(string(b), err)
	}
	for _, r := range array {
		for _, key := range []string{"alias", "origin", "reference", "components", "conventions", "enabled", "version", "content_digest", "inspection_status"} {
			if _, ok := r[key]; !ok {
				t.Errorf("missing %s", key)
			}
		}
		for _, key := range []string{"components", "conventions"} {
			if r[key] == nil {
				t.Errorf("null %s", key)
			}
		}
	}
	for _, secret := range []string{"alice", "secret", "hidden"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("leaked %s", secret)
		}
	}
}
