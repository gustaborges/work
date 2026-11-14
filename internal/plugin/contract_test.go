package plugin

import "testing"

func TestContractDetached(t *testing.T) {
	a := Contract()
	a.TopLevel[0] = "bad"
	r := a.Rules[RoleImporter]
	r.AnyOf[0][0] = "bad"
	b := Contract()
	if b.TopLevel[0] == "bad" || b.Rules[RoleImporter].AnyOf[0][0] == "bad" {
		t.Fatal("descriptor mutates validator")
	}
}
