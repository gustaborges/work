package contract

import (
	"context"
	"encoding/json"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	"strings"
	"testing"
)

func TestAdminUpdateResultContract(t *testing.T) {
	rows, err := pluginadmin.Check(context.Background(), workhome.At(t.TempDir()), nil)
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	row := pluginadmin.UpdateResult{Alias: "a", Origin: "local-pinned", Status: "failed", PolicyReferencesRemoved: []string{}, Error: &pluginadmin.UpdateError{Token: "plugin-invalid", Message: "invalid"}}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	for _, k := range []string{"alias", "origin", "status", "current_version", "candidate_version", "current_reference", "candidate_reference", "current_digest", "candidate_digest", "policy_references_removed", "error"} {
		if _, ok := v[k]; !ok {
			t.Error(k)
		}
	}
	if v["current_version"] != nil || v["policy_references_removed"] == nil || !strings.Contains(string(b), `"token":"plugin-invalid"`) {
		t.Fatal(string(b))
	}
}
