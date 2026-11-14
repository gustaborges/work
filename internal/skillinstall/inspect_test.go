package skillinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnership(t *testing.T) {
	root := t.TempDir()
	ds, err := testTargets(t, root)
	if err != nil {
		t.Fatal(err)
	}
	d := ds[0]
	if got := Inspect(d, "embedded"); got.State != "absent" {
		t.Fatal(got)
	}
	if err = os.MkdirAll(d.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(d, "embedded"); got.State != "foreign" {
		t.Fatal(got)
	}
	if err = os.WriteFile(filepath.Join(d.Path, ".work-skill.json"), []byte(`{"schema":99}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(d, "embedded"); got.State != "foreign" {
		t.Fatal(got)
	}
}
