package skills

import (
	"bytes"
	"io/fs"
	"testing"
)

func TestSkillContractPinned(t *testing.T) {
	tree, err := Tree("work-plugin-builder")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := fs.ReadFile(tree, "CONTRACT")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pin, []byte(ContractPin())) {
		t.Fatal("the Work contract changed; update ai-skills/work-plugin-builder (references/ and templates/), then run make skill-lock")
	}
	for name, want := range Generated() {
		got, err := fs.ReadFile(tree, "references/generated/"+name)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("stale generated reference %s; run go generate ./ai-skills/...", name)
		}
	}
}
