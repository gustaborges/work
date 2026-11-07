package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/tests/fixtures/installfx"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

type sourceRow struct {
	Name           string  `json:"name"`
	DisplayName    *string `json:"display_name"`
	SourceKind     string  `json:"source_kind"`
	Source         string  `json:"source"`
	Branch         *string `json:"branch"`
	Revision       *string `json:"revision"`
	PluginCount    *int    `json:"plugin_count"`
	CatalogStatus  string  `json:"catalog_status"`
	InstalledCount int     `json:"installed_count"`
	EnabledCount   int     `json:"enabled_count"`
}

func listSources(t *testing.T, h *f6Harness) map[string]sourceRow {
	t.Helper()
	out, _ := h.run(t, h.root, 0, "plugin", "marketplace", "list", "--json")
	var rows []sourceRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err, out)
	}
	byName := map[string]sourceRow{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	return byName
}

func gitMarketplace(t *testing.T, h *f6Harness, fixture, branch string) (dir, url string) {
	t.Helper()
	dir = marketplaces.Copy(t, fixture)
	gittest.Git(t, dir, "init", "-q", "-b", branch)
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "publish")
	return dir, "file://" + filepath.ToSlash(dir)
}

// Registration is zero-install: a local non-Git source and a file:// Git source
// are browsed after both origins disappear, with no plugin or policy change.
func TestRegisterLocalAndGitThenBrowseOffline(t *testing.T) {
	h := newAdminHarness(t)
	home := adminHome(h)
	local := marketplaces.Copy(t, "multiple")
	gitDir, gitURL := gitMarketplace(t, h, "single", "main")

	out, _ := h.run(t, h.root, 0, "plugin", "marketplace", "add", local)
	if !strings.HasPrefix(out, "work: registered demo (sha256:") {
		t.Fatal(out)
	}
	out, _ = h.run(t, h.root, 0, "plugin", "marketplace", "add", gitURL, "--json")
	var added struct {
		Name, Result, Revision string
		Committed              bool
	}
	if err := json.Unmarshal([]byte(out), &added); err != nil || added.Name != "solo" || added.Result != "registered" || !added.Committed || added.Revision != gittest.Git(t, gitDir, "rev-parse", "HEAD") {
		t.Fatal(out, err, added)
	}
	reg, _ := os.ReadFile(filepath.Join(home, "state", "registry.json"))
	cfgBefore, _ := os.ReadFile(filepath.Join(home, "config", "work.json"))
	if len(reg) != 0 && strings.Contains(string(reg), "packages\": [\n") {
		t.Fatal("registration installed something")
	}

	if err := os.RemoveAll(local); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gitDir); err != nil {
		t.Fatal(err)
	}
	rows := listSources(t, h)
	demo, solo, builtin := rows["demo"], rows["solo"], rows["builtin"]
	if demo.CatalogStatus != "ready" || demo.PluginCount == nil || *demo.PluginCount != 3 || demo.SourceKind != "local" || demo.InstalledCount != 0 {
		t.Fatalf("demo = %+v", demo)
	}
	if demo.DisplayName == nil || *demo.DisplayName != "Demo Plugins" || demo.Branch != nil {
		t.Fatalf("display name / branch = %+v", demo)
	}
	if solo.CatalogStatus != "ready" || solo.SourceKind != "git" || solo.Branch == nil || *solo.Branch != "main" || solo.PluginCount == nil || *solo.PluginCount != 1 {
		t.Fatalf("solo = %+v", solo)
	}
	if builtin.SourceKind != "builtin" || builtin.CatalogStatus != "ready" {
		t.Fatalf("builtin = %+v", builtin)
	}
	if after, _ := os.ReadFile(filepath.Join(home, "config", "work.json")); string(after) != string(cfgBefore) {
		t.Fatal("browsing changed configuration")
	}
	if _, err := os.Stat(filepath.Join(home, "state", "work.db")); !os.IsNotExist(err) {
		t.Fatal("browsing opened the Work database")
	}
	// Policy stays unhandled: registration never seeds or handles it.
	if life, err := os.ReadFile(filepath.Join(home, "state", "lifecycle.json")); err == nil && strings.Contains(string(life), `"policy_handled": true`) {
		t.Fatal("registration handled the default policy")
	}
}

func TestSourceCollisionsAndOfficialNameSpoofing(t *testing.T) {
	h := newAdminHarness(t)
	first := marketplaces.Copy(t, "single")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", first)
	before := adminTree(t, adminHome(h))

	// A different source claiming the same namespace fails without overwrite.
	h.run(t, h.root, 58, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"))
	// A catalog may not claim the included provider's namespace, in any case.
	h.run(t, h.root, 58, "plugin", "marketplace", "add", marketplaces.Copy(t, "invalid-reserved-namespace"))
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("a rejected registration changed state")
	}

	// A catalog merely named "work" proves no official ownership: it is an ordinary source.
	impostor := marketplaces.Copy(t, "single")
	if err := os.WriteFile(filepath.Join(impostor, "marketplace.json"), []byte(`{"schema":1,"name":"work","plugins":[{"name":"sample","path":"plugins/sample"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 0, "plugin", "marketplace", "add", impostor)
	rows := listSources(t, h)
	if rows["work"].SourceKind != "local" || rows["work"].DisplayName != nil {
		t.Fatalf("work = %+v; the name alone grants no official attribution", rows["work"])
	}
	// No official online source is ever registered implicitly.
	fresh := newAdminHarness(t)
	if _, ok := listSources(t, fresh)["work"]; ok {
		t.Fatal("an official online source appeared without registration")
	}
}

func TestInvalidMissingAndOfflineCatalogsFailWithoutPartialState(t *testing.T) {
	h := newAdminHarness(t)
	for fixture, want := range map[string]int{
		"no-catalog": 57, "invalid-schema": 57, "invalid-escaping-path": 57, "invalid-name-mismatch": 57, "invalid-duplicate-key": 57,
	} {
		h.run(t, h.root, want, "plugin", "marketplace", "add", marketplaces.Copy(t, fixture))
	}
	h.run(t, h.root, 59, "plugin", "marketplace", "add", filepath.Join(h.root, "does-not-exist"))
	h.run(t, h.root, 59, "plugin", "marketplace", "add", "file://"+filepath.ToSlash(filepath.Join(h.root, "no-such-remote")))
	if _, err := os.Stat(adminHome(h)); !os.IsNotExist(err) {
		entries := adminTree(t, adminHome(h))
		for name := range entries {
			if strings.Contains(name, "marketplace-snapshots") && name != "state/marketplace-snapshots" {
				t.Fatalf("partial capture state: %s", name)
			}
		}
	}
	// Usage errors are reported before any retrieval or state.
	h.run(t, h.root, 2, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"), "--branch", "main")
	h.run(t, h.root, 2, "plugin", "marketplace", "add")
	h.run(t, h.root, 2, "plugin", "marketplace", "add", "a", "b")
}

func TestRefreshIndependentOutcomesDisplayNameChangesAndTotals(t *testing.T) {
	h := newAdminHarness(t)
	good := marketplaces.Copy(t, "multiple")
	bad := marketplaces.Copy(t, "single")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", good)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", bad)

	// Install one plugin from "demo" so totals are non-zero and independent of catalog health.
	provider := installfx.InstallAs(t, h.home(), "demo", filepath.Join(good, "plugins", "alpha"), true)
	bindToRegisteredSource(t, h, "demo", provider)
	reg, _ := os.ReadFile(filepath.Join(adminHome(h), "state", "registry.json"))
	if !strings.Contains(string(reg), "alpha") {
		t.Fatal("fixture install failed")
	}

	// A display-name change is a meaningful offer change; the canonical namespace is unchanged.
	if err := os.WriteFile(filepath.Join(good, "marketplace.json"), []byte(strings.Replace(mustRead(t, filepath.Join(good, "marketplace.json")), "Demo Plugins", "Renamed Demo", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	// "solo" becomes invalid: its earlier offer must survive while "demo" still refreshes.
	if err := os.WriteFile(filepath.Join(bad, "marketplace.json"), []byte(`{"schema":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, ui := h.run(t, h.root, 61, "plugin", "marketplace", "upgrade", "--json")
	var res struct {
		Results []struct {
			Name, Status string
			Revision     *string
			Error        *struct{ Token, Message string }
		}
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err, out, ui)
	}
	got := map[string]string{}
	for _, r := range res.Results {
		got[r.Name] = r.Status
		if r.Name == "solo" && (r.Error == nil || r.Error.Token != "marketplace-invalid" || r.Revision != nil) {
			t.Fatalf("solo row = %+v", r)
		}
	}
	if got["demo"] != "upgraded" || got["solo"] != "failed" || got["builtin"] != "unchanged" {
		t.Fatalf("results = %v", got)
	}
	if strings.Contains(out+ui, bad) || strings.Contains(out+ui, "fatal:") {
		t.Fatalf("output leaks local paths or Git text: %s %s", out, ui)
	}

	rows := listSources(t, h)
	if rows["demo"].DisplayName == nil || *rows["demo"].DisplayName != "Renamed Demo" || rows["demo"].Name != "demo" {
		t.Fatalf("demo = %+v", rows["demo"])
	}
	if rows["demo"].InstalledCount != 1 || rows["demo"].EnabledCount != 1 {
		t.Fatalf("totals = %+v", rows["demo"])
	}
	if rows["solo"].CatalogStatus != "ready" || rows["solo"].PluginCount == nil || *rows["solo"].PluginCount != 1 {
		t.Fatalf("the earlier valid offer must survive a failed refresh: %+v", rows["solo"])
	}
	// Refresh never changes installed plugins or enablement.
	if after, _ := os.ReadFile(filepath.Join(adminHome(h), "state", "registry.json")); string(after) != string(reg) {
		t.Fatal("refresh changed installed state")
	}

	// An unknown target is a usage error before any retrieval. Recovery preflight
	// may still collect an unreferenced superseded capture, but no published
	// state (configuration, catalogs, installations, lifecycle) changes.
	published := func() map[string]string {
		out := map[string]string{}
		for _, f := range []string{"config/work.json", "state/marketplace-catalogs.json", "state/registry.json", "state/lifecycle.json", "state/administration.json"} {
			b, _ := os.ReadFile(filepath.Join(adminHome(h), filepath.FromSlash(f)))
			out[f] = string(b)
		}
		return out
	}
	before := published()
	h.run(t, h.root, 2, "plugin", "marketplace", "upgrade", "nope")
	if !reflect.DeepEqual(before, published()) {
		t.Fatal("a usage error changed published state")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// bindToRegisteredSource makes the installed fixture package claim the real
// registration instance of the named source, as a marketplace install would.
func bindToRegisteredSource(t *testing.T, h *f6Harness, source string, provider pluginid.Provider) {
	t.Helper()
	ctx := context.Background()
	snap, err := adminstate.Read(ctx, h.home())
	if err != nil {
		t.Fatal(err)
	}
	m, ok := snap.Config.Marketplace(source)
	if !ok {
		t.Fatalf("source %q is not registered", source)
	}
	err = adminstate.WithWrite(ctx, h.home(), "bind fixture", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByProvider(provider)
		p.Provenance.RegistrationID = m.RegistrationID
		s.Registry.UpsertPackage(p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
