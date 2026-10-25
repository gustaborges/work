package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminDisableAllRolesAndRetention(t *testing.T) {
	h := newF6Harness(t)
	source := fixtures.Prepare(t, "context-suite")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "automatic")
	home := workhome.At(filepath.Join(h.root, "home"))
	beforeConfig, err := os.ReadFile(home.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	beforeWork := adminTree(t, h.dir)
	h.run(t, h.root, 0, "plugin", "disable", "automatic", "manual")
	// No disabled manual component may be selected, and disabled specific
	// Starters fall through without invocation to the normal locator failure.
	h.run(t, h.dir, 39, "link", "manual/linker", "https://example.test/value")
	h.run(t, h.dir, 39, "import", "manual/importer")
	h.run(t, h.root, 26, "start", "demo-ctx-15", "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "disabled", "--prefix", "{slug}", "--yes")
	// Explicit policy retains a disabled native Locator, never invoking it.
	adminSource := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", adminSource, "--as", "catalog")
	h.run(t, h.root, 0, "plugin", "install", adminSource, "--as", "catalog-shared")
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", "admin-team")
	beforeMemory, _ := os.ReadFile(home.BranchConventionsFile())
	h.run(t, h.root, 0, "repository", "policy", "add", "catalog/clone", "--before", "work-reference/filesystem-repository-locator")
	cfgBeforeToggle, _ := os.ReadFile(home.ConfigFile())
	h.env = append(h.env, "WORK_ADMIN_LOG="+filepath.Join(h.root, "admin-log"))
	h.run(t, h.root, 0, "plugin", "disable", "catalog")
	snapshot, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Registry.Effective().ConventionByName("admin-team"); !ok {
		t.Fatal("enabled shared convention owner lost")
	}
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", "admin-team")
	h.run(t, h.root, 26, "start", "missing-admin-repo", "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "disabled-locator", "--prefix", "{slug}", "--yes")
	if _, err = os.Stat(filepath.Join(h.root, "admin-log")); !os.IsNotExist(err) {
		t.Fatal("disabled Locator invoked")
	}
	if b, _ := os.ReadFile(h.log); len(b) > 0 {
		t.Fatalf("disabled executable ran: %s", b)
	}
	h.run(t, h.root, 0, "plugin", "enable", "catalog")
	if after, _ := os.ReadFile(home.ConfigFile()); string(after) != string(cfgBeforeToggle) {
		t.Fatal("toggle rewrote policy")
	}
	if after, _ := os.ReadFile(home.BranchConventionsFile()); string(after) != string(beforeMemory) {
		t.Fatal("toggle rewrote memory")
	}
	if !reflect.DeepEqual(beforeWork, adminTree(t, h.dir)) {
		t.Fatal("toggle changed Work")
	}
	if len(beforeConfig) == 0 {
		t.Fatal("missing baseline")
	}
	h.run(t, h.root, 0, "plugin", "enable", "manual", "automatic")
	h.run(t, h.dir, 0, "link", "manual/linker", "https://example.test/value")
	h.run(t, h.dir, 0, "import", "manual/importer")
}
func TestAdminInventoryRepeatedReads(t *testing.T) {
	h := newAdminHarness(t)
	source := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "managed")
	home := workhome.At(adminHome(h))
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		t.Fatal(err)
	}
	p, _ := reg.PackageByAlias("managed")
	p.Version = ""
	p.ContentDigest = ""
	reg.UpsertPackage(p)
	if err = registry.Save(home.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"legacy", "damaged"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "damaged" {
				if err := os.Remove(filepath.Join(home.PluginsDir(), "managed", "source", "plugin.json")); err != nil {
					t.Fatal(err)
				}
			}
			before := adminTree(t, home.Root())
			for range 20 {
				out, _ := h.run(t, h.root, 0, "plugin", "list", "--json")
				var rows []map[string]any
				if err := json.Unmarshal([]byte(out), &rows); err != nil {
					t.Fatal(err)
				}
				want := "metadata-unknown"
				if kind == "damaged" {
					want = "content-unavailable"
				}
				if rows[0]["inspection_status"] != want {
					t.Fatal(out)
				}
			}
			if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
				t.Fatal("inventory mutated installed state")
			}
		})
	}
}
func TestAdminLinkedBaselineNeverRefreshes(t *testing.T) {
	h := newAdminHarness(t)
	source := fixtures.Prepare(t, "admin-suite")
	if runtime.GOOS == "windows" {
		if err := os.Symlink(source, filepath.Join(t.TempDir(), "capability")); err != nil {
			t.Skipf("Windows symlink capability unavailable: %v", err)
		}
	}
	// Windows platforms without symlink capability are handled by the existing
	// linked-install capability check; this test reports that limitation.
	cmd := []string{"plugin", "install", source, "--link", "--as", "linked"}
	h.run(t, h.root, 0, cmd...)
	fixtures.Revise(t, source, "live-drift", nil)
	before := adminTree(t, adminHome(h))
	external := adminTree(t, source)
	for range 20 {
		out, _ := h.run(t, h.root, 0, "plugin", "list", "--json")
		if strings.Contains(out, "live-drift") || !strings.Contains(out, "release-blue") {
			t.Fatal(out)
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) || !reflect.DeepEqual(external, adminTree(t, source)) {
		t.Fatal("linked inspection wrote content")
	}
}
