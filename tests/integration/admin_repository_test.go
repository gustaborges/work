package integration

import (
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdminPolicyAndRoots(t *testing.T) {
	h := newAdminHarness(t)
	source := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "catalog")
	h.run(t, h.root, 0, "plugin", "disable", "catalog")
	h.run(t, h.root, 0, "repository", "policy", "add", "clone")
	h.run(t, h.root, 0, "repository", "policy", "replace", "catalog/clone")
	out, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if !strings.Contains(out, `"installed": true`) || !strings.Contains(out, "package-disabled") {
		t.Fatal(out)
	}
	baseline := adminTree(t, adminHome(h))
	for _, args := range [][]string{{"repository", "policy", "replace", "catalog/clone", "clone"}, {"repository", "policy", "move", "clone", "--before", "clone"}, {"repository", "root", "add", h.root, filepath.Join(h.root, "missing")}} {
		h.run(t, h.root, 2, args...)
		if !reflect.DeepEqual(baseline, adminTree(t, adminHome(h))) {
			t.Fatal("rejected edit wrote state")
		}
	}
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "catalog")
	out, _ = h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if !strings.Contains(out, `"position": 1`) {
		t.Fatal(out)
	}
	dir := filepath.Join(h.root, "repos")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 0, "repository", "root", "add", dir, dir)
	h.run(t, h.root, 0, "repository", "root", "remove", dir)
	out, _ = h.run(t, h.root, 0, "repository", "root", "list", "--json")
	if !strings.Contains(out, `"roots": []`) {
		t.Fatal(out)
	}
	baseline = adminTree(t, adminHome(h))
	for range 20 {
		h.run(t, h.root, 0, "repository", "locator", "list", "--json")
		h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	}
	if !reflect.DeepEqual(baseline, adminTree(t, adminHome(h))) {
		t.Fatal("read wrote state")
	}
}
