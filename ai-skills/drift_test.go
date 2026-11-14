package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func TestIndependentContractPerturbations(t *testing.T) {
	original := ContractBytes()
	for _, domain := range []string{"Plugin", "Catalog", "IPC"} {
		t.Run(domain, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal(original, &d); err != nil {
				t.Fatal(err)
			}
			part := d[domain].(map[string]any)
			switch domain {
			case "Plugin":
				part["Rules"].(map[string]any)["starter"].(map[string]any)["Required"] = []string{"unexpected"}
			case "Catalog":
				part["CatalogKeys"] = []string{"unexpected"}
			case "IPC":
				part["Types"].(map[string]any)["ImporterInput"] = []any{map[string]any{"Name": "unexpected", "Type": "string"}}
			}
			changed, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(original, changed) || sha256.Sum256(original) == sha256.Sum256(changed) {
				t.Fatal("drift not represented")
			}
		})
	}
}
