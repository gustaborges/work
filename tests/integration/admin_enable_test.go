package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginid"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminDisableAllRolesAndRetention(t *testing.T) {
	h := newF6Harness(t)
	home := h.home()
	source := fixtures.Prepare(t, "context-suite")
	h.install(t, "automatic", source)
	beforeConfig, err := os.ReadFile(home.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	beforeWork := adminTree(t, h.dir)
	h.run(t, h.root, 0, "plugin", "disable", "context-suite@automatic", "manual-suite@manual")
	// No disabled manual component may be selected, and disabled specific
	// Starters fall through without invocation to the normal locator failure.
	h.run(t, h.dir, 39, "link", "manual-suite@manual/linker", "https://example.test/value")
	h.run(t, h.dir, 39, "import", "manual-suite@manual/importer")
	h.run(t, h.root, 26, "start", "demo-ctx-15", "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "disabled", "--prefix", "{slug}", "--yes")
	// Explicit policy retains a disabled native Locator, never invoking it.
	adminSource := fixtures.Prepare(t, "admin-suite")
	h.install(t, "catalog", adminSource)
	h.install(t, "catalog-shared", adminSource)
	const catalog, shared = "admin-suite@catalog", "admin-suite@catalog-shared"
	// Equal convention names from two providers are distinct: a bare name is ambiguous.
	h.run(t, filepath.Join(h.root, "demo"), 2, "convention", "set", "admin-team")
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", catalog+"/admin-team")
	beforeMemory, _ := os.ReadFile(home.BranchConventionsFile())
	h.run(t, h.root, 0, "repository", "policy", "add", catalog+"/clone", "--before", "work-reference@builtin/filesystem-repository-locator")
	cfgBeforeToggle, _ := os.ReadFile(home.ConfigFile())
	h.env = append(h.env, "WORK_ADMIN_LOG="+filepath.Join(h.root, "admin-log"))
	h.run(t, h.root, 0, "plugin", "disable", catalog)
	snapshot, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	sharedRef := pluginid.Ref{Provider: pluginid.Provider{Marketplace: "catalog-shared", Plugin: "admin-suite"}, Name: "admin-team"}
	if _, ok := snapshot.Registry.Effective().ConventionByRef(sharedRef); !ok {
		t.Fatal("the enabled homonym's convention was lost")
	}
	// The remembered choice names the disabled provider, so it is unavailable
	// and the enabled homonym cannot silently satisfy it.
	if view := snapshot.Memory.View(mustIdentity(t, filepath.Join(h.root, "demo")), snapshot.Registry); view.Available {
		t.Fatalf("a homonym satisfied the disabled provider's remembered choice: %+v", view)
	}
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", shared+"/admin-team")
	h.run(t, h.root, 26, "start", "missing-admin-repo", "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "disabled-locator", "--prefix", "{slug}", "--yes")
	if _, err = os.Stat(filepath.Join(h.root, "admin-log")); !os.IsNotExist(err) {
		t.Fatal("disabled Locator invoked")
	}
	if b, _ := os.ReadFile(h.log); len(b) > 0 {
		t.Fatalf("disabled executable ran: %s", b)
	}
	h.run(t, h.root, 0, "plugin", "enable", catalog)
	if after, _ := os.ReadFile(home.ConfigFile()); string(after) != string(cfgBeforeToggle) {
		t.Fatal("toggle rewrote policy")
	}
	if after, _ := os.ReadFile(home.BranchConventionsFile()); len(beforeMemory) == 0 || len(after) == 0 {
		t.Fatal("convention memory missing")
	}
	if !reflect.DeepEqual(beforeWork, adminTree(t, h.dir)) {
		t.Fatal("toggle changed Work")
	}
	if len(beforeConfig) == 0 {
		t.Fatal("missing baseline")
	}
	h.run(t, h.root, 0, "plugin", "enable", "manual-suite@manual", "context-suite@automatic")
	h.run(t, h.dir, 0, "link", "manual-suite@manual/linker", "https://example.test/value")
	h.run(t, h.dir, 0, "import", "manual-suite@manual/importer")
}

func TestAdminInventoryRepeatedReads(t *testing.T) {
	h := newAdminHarness(t)
	home := h.home()
	provider := h.install(t, "managed", fixtures.Prepare(t, "admin-suite"))
	for _, kind := range []string{"ready", "damaged"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "damaged" {
				src, _ := home.PackageSource(provider)
				if err := os.Remove(filepath.Join(src, "plugin.json")); err != nil {
					t.Fatal(err)
				}
			}
			before := adminTree(t, home.Root())
			for range 20 {
				out, _ := h.run(t, h.root, 0, "plugin", "list", "--json")
				var env struct {
					Installed []map[string]any `json:"installed"`
				}
				if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Installed) != 1 {
					t.Fatal(err, out)
				}
				want := "ready"
				if kind == "damaged" {
					want = "content-unavailable"
				}
				if row := env.Installed[0]; row["inspection_status"] != want || row["id"] != "admin-suite@managed" {
					t.Fatal(out)
				}
			}
			if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
				t.Fatal("inventory mutated installed state")
			}
		})
	}
}
