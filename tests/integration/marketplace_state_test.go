package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

// Unsupported or malformed state that is present is rejected by the one strict
// current validator: the command fails with an ordinary diagnostic and the
// file is left byte-for-byte untouched, never converted, repaired or deleted.
func TestStrictRejectionOfMalformedAndUnsupportedState(t *testing.T) {
	cases := map[string]struct {
		file, data string
		wantCode   int
	}{
		"unversioned config":       {"config/work.json", `{"workspace":"/x","repository_roots":[],"repository_resolution":{"locators":[]}}`, 16},
		"future config schema":     {"config/work.json", `{"schema":2}`, 16},
		"alias-shaped registry":    {"state/registry.json", `{"components":[],"conventions":[],"packages":[{"alias":"p","origin":"local-linked","reference":"/x"}]}`, 16},
		"bare-string memory":       {"state/branch_conventions.json", `{"entries":[{"identity":"r","convention":"freeform"}]}`, 16},
		"old lifecycle":            {"state/lifecycle.json", `{"schema":1,"initialized":true,"policy_seeded":true,"removed":false}`, 54},
		"unsupported catalogs":     {"state/marketplace-catalogs.json", `{"schema":2,"catalogs":[]}`, 16},
		"tampered stored catalog":  {"state/marketplace-catalogs.json", `{"schema":1,"catalogs":[{"registration_id":"0123456789abcdef0123456789abcdef","namespace":"x","source":{"kind":"local","source":"/s"},"revision":"sha256:00","catalog_digest":"sha256:00","digest_algorithm":"work-catalog-v1","capture_digest":"sha256:00","entries":[]}]}`, 16},
		"future administration":    {"state/administration.json", `{"schema":2,"epoch":1,"status":"stable","transaction_id":null}`, 54},
		"pending without evidence": {"state/administration.json", `{"schema":1,"epoch":1,"status":"pending","transaction_id":"nothing-here"}`, 54},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := newAdminHarness(t)
			home := adminHome(h)
			path := filepath.Join(home, filepath.FromSlash(c.file))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(c.data), 0o600); err != nil {
				t.Fatal(err)
			}
			before := adminTree(t, home)
			// Pure reads report and refuse; a mutation preflight does not convert either.
			h.run(t, h.root, c.wantCode, "plugin", "list", "--json")
			h.run(t, h.root, c.wantCode, "plugin", "marketplace", "list")
			if got, _ := os.ReadFile(path); string(got) != c.data {
				t.Fatal("the unsupported file was rewritten or removed")
			}
			if !reflect.DeepEqual(before, adminTree(t, home)) {
				t.Fatal("a read changed the home")
			}
			h.run(t, h.root, c.wantCode, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"))
			if got, _ := os.ReadFile(path); string(got) != c.data {
				t.Fatal("a mutation converted or deleted unsupported state")
			}
		})
	}
}

func TestFreshHomeInspectionAndNoopsPreserveBytesAndEpoch(t *testing.T) {
	h := newAdminHarness(t)
	home := adminHome(h)
	// Fresh home: inspection works and creates nothing.
	out, _ := h.run(t, h.root, 0, "plugin", "marketplace", "list", "--json")
	if !strings.Contains(out, `"name": "builtin"`) || !strings.Contains(out, `"catalog_status": "ready"`) {
		t.Fatal(out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("inspection created the Work home")
	}
	src := marketplaces.Copy(t, "multiple")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	snapshot := adminTree(t, home)
	// Identical registration and identical refresh are byte- and epoch-preserving no-ops.
	for _, args := range [][]string{{"plugin", "marketplace", "add", src}, {"plugin", "marketplace", "upgrade", "demo"}, {"plugin", "marketplace", "upgrade"}} {
		h.run(t, h.root, 0, args...)
		if !reflect.DeepEqual(snapshot, adminTree(t, home)) {
			t.Fatalf("%v changed bytes or epoch", args)
		}
	}
}
