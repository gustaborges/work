package marketplace

import (
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
)

func TestBuiltinProjectionIsPureAndCodeOwned(t *testing.T) {
	a, err := BuiltinSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := BuiltinSnapshot()
	if a.Offer.Name != pluginid.Builtin || a.Source != BuiltinSource || a.Dir != "" {
		t.Errorf("builtin snapshot = %+v", a)
	}
	if a.Revision != b.Revision || a.Revision != a.Offer.CaptureDigest() {
		t.Error("projection must be deterministic")
	}
	e, ok := a.Offer.Entry("work-reference")
	if !ok || e.Version == "" || len(e.Components) != 2 || len(e.Conventions) != 1 {
		t.Fatalf("entry = %+v", e)
	}
	a.Close() // no scratch content to release
}
