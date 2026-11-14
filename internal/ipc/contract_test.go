package ipc

import (
	"encoding/json"
	"testing"
)

func TestContractDetached(t *testing.T) {
	a := Contract()
	original, _ := json.Marshal(a)
	a.Types["ImporterInput"][0].Name = "bad"
	b := Contract()
	fresh, _ := json.Marshal(b)
	if string(original) != string(fresh) {
		t.Fatal("descriptor aliases")
	}
	found := false
	for _, f := range b.Types["ImporterInput"] {
		if f.Name == "output_dir" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing actual output_dir")
	}
}
