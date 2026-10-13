package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAdminRecoveryReadAndStatus(t *testing.T) {
	h := newF6Harness(t)
	home := filepath.Join(h.root, "home")
	path := filepath.Join(home, "state", "administration.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"epoch":99,"status":"pending","transaction_id":"missing-evidence"}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, home)
	for _, args := range [][]string{{"plugin", "list", "--json"}, {"repository", "policy", "list", "--json"}} {
		out, _ := h.run(t, h.root, 54, args...)
		if out != "" {
			t.Fatal("partial mixed output")
		}
	}
	h.run(t, h.dir, 0, "status", "--json")
	// Invalid option grammar must not enter recovery at all.
	h.run(t, h.root, 2, "plugin", "install", "https://example.test/plugin", "--link")
	h.run(t, h.root, 31, "plugin", "install", "missing-source", "--as", "../escape")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("read recovered or changed pending state")
	}
	// A valid mutating invocation reaches recovery before source lookup.
	h.run(t, h.root, 54, "plugin", "install", filepath.Join(h.root, "missing-source"))
	// A damaged intermediate config cannot hide recovery from a fully
	// specified start invocation.
	if err := os.WriteFile(filepath.Join(home, "config", "work.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 54, "start", filepath.Join(h.root, "demo"), "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "recovery", "--prefix", "{slug}", "--yes")
}
