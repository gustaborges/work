package skillinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTargets(t *testing.T) {
	root := t.TempDir()
	targets, err := Resolve(Options{Dir: root})
	if err != nil || len(targets) != 2 {
		t.Fatalf("%v %v", targets, err)
	}
	if _, err = Resolve(Options{Dir: root, Global: true}); err == nil {
		t.Fatal("accepted both scopes")
	}
	if err = os.Symlink(t.TempDir(), filepath.Join(root, ".claude")); err != nil {
		t.Skip("symlink capability unavailable")
	}
	ds, err := Resolve(Options{Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	if r := Inspect(ds[0], "embedded"); r.State != "conflict" {
		t.Fatal("followed descendant link", r)
	}
}
