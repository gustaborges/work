package pluginadmin_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

func prov2(plugin, mkt string) pluginid.Provider {
	return pluginid.Provider{Marketplace: mkt, Plugin: plugin}
}

// writeOffer rewrites a local catalog so it lists exactly entries (name → path).
func writeOffer(t *testing.T, src, namespace string, entries map[string]string) {
	t.Helper()
	type listing struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	var list []listing
	for name, path := range entries {
		list = append(list, listing{Name: name, Path: path})
	}
	slices.SortFunc(list, func(a, b listing) int { return strings.Compare(a.Name, b.Name) })
	raw, _ := json.Marshal(map[string]any{"schema": 1, "name": namespace, "plugins": list})
	if err := os.WriteFile(filepath.Join(src, "marketplace.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// setManifest replaces a plugin's manifest and tool bytes in a local catalog.
func setPlugin(t *testing.T, src, path, manifestJSON, tool string) {
	t.Helper()
	root := filepath.Join(src, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "tool"), []byte(tool), 0o755); err != nil {
		t.Fatal(err)
	}
}

const toolV1, toolV2 = "#!/bin/sh\nexit 0\n", "#!/bin/sh\nexit 2\n"

func check(t *testing.T, h workhome.Home, ps ...pluginid.Provider) ([]pluginadmin.UpdateResult, error) {
	t.Helper()
	return pluginadmin.Check(context.Background(), h, ps)
}

func row(t *testing.T, rows []pluginadmin.UpdateResult, id string) pluginadmin.UpdateResult {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row %s in %+v", id, rows)
	return pluginadmin.UpdateResult{}
}

// homeFiles lists every path under the Work home, to prove a read-only
// operation wrote nothing.
func homeFiles(t *testing.T, h workhome.Home) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(h.Root(), func(p string, d os.DirEntry, err error) error {
		if err == nil {
			info, _ := d.Info()
			out = append(out, p+"|"+info.ModTime().String())
		}
		return nil
	})
	return out
}

func TestCheckJudgesTheRecordedSourceNotDiscoveryAndWritesNothing(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")

	// The author publishes version 2 but nobody refreshed discovery.
	setPlugin(t, src, "plugins/tool", manifest("tool", "2", linker), toolV2)
	stored := snapshot(t, h).Catalogs.Catalogs[0].Revision
	before := homeFiles(t, h)

	rows, err := check(t, h, prov2("tool", "team"))
	if err != nil {
		t.Fatal(err)
	}
	r := row(t, rows, "tool@team")
	if r.Status != pluginadmin.UpdateAvailable || *r.CurrentVersion != "1" || *r.CandidateVersion != "2" {
		t.Fatalf("row = %+v", r)
	}
	if after := homeFiles(t, h); !slices.Equal(before, after) {
		t.Fatalf("a check wrote to the Work home:\nbefore %v\nafter  %v", before, after)
	}
	if got := snapshot(t, h).Catalogs.Catalogs[0].Revision; got != stored {
		t.Fatal("a check republished discovery")
	}
}

func TestEqualVersionWithChangedBytesIsUnchangedAndInvalidOnesFail(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")

	setPlugin(t, src, "plugins/tool", manifest("tool", "1", linker), toolV2)
	rows, err := check(t, h, prov2("tool", "team"))
	if err != nil || row(t, rows, "tool@team").Status != pluginadmin.UpdateUnchanged {
		t.Fatalf("valid changed bytes at an equal version: %+v, %v", rows, err)
	}

	// An equal version is still validated: a candidate that could never start fails.
	broken := manifest("tool", "1", `{"name":"run","role":"linker","key":"fixture.run","entrypoint":"bin/gone","manual":{"display_name":"run"}}`)
	setPlugin(t, src, "plugins/tool", broken, toolV1)
	rows, err = check(t, h, prov2("tool", "team"))
	r := row(t, rows, "tool@team")
	if r.Status != pluginadmin.UpdateFailed || r.Error == nil || r.Error.Token != diag.PluginInstallFailed.Token {
		t.Fatalf("row = %+v", r)
	}
	if code(err) != diag.PluginUpdateFailed.Code {
		t.Fatalf("aggregate = %v (exit %d)", err, code(err))
	}
	if p, _ := snapshot(t, h).Registry.PackageByProvider(prov2("tool", "team")); p.Version != "1" {
		t.Fatal("a failed check changed the installed copy")
	}
}

func TestWithdrawnEntryKeepsTheInstalledCopy(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker), "other": manifest("other", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")

	writeOffer(t, src, "team", map[string]string{"other": "plugins/other"})
	upgrade(t, h, "team")
	rows, err := check(t, h, prov2("tool", "team"))
	if err != nil || row(t, rows, "tool@team").Status != pluginadmin.UpdateWithdrawn {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	if _, ok := snapshot(t, h).Registry.PackageByProvider(prov2("tool", "team")); !ok {
		t.Fatal("withdrawal removed the installed copy")
	}
}

func TestMovedPathKeepsIdentityOfTheSameEntry(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")

	setPlugin(t, src, "relocated/tool", manifest("tool", "2", linker), toolV2)
	if err := os.RemoveAll(filepath.Join(src, "plugins")); err != nil {
		t.Fatal(err)
	}
	writeOffer(t, src, "team", map[string]string{"tool": "relocated/tool"})

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("tool", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	rows, err := pluginadmin.ApplyUpdates(context.Background(), plan, false)
	if err != nil || row(t, rows, "tool@team").Status != pluginadmin.UpdateUpdated {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	p, _ := snapshot(t, h).Registry.PackageByProvider(prov2("tool", "team"))
	if p.Version != "2" || p.Provenance.Path != "relocated/tool" || p.Provenance.PluginName != "tool" {
		t.Fatalf("package = %+v", p)
	}
}

func TestMaintenanceNeedsTheRecordedRegistrationButNotDiscovery(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")

	// Missing discovery does not prevent a check.
	if err := os.Remove(h.CatalogsFile()); err != nil {
		t.Fatal(err)
	}
	if rows, err := check(t, h, prov2("tool", "team")); err != nil || row(t, rows, "tool@team").Status != pluginadmin.UpdateUnchanged {
		t.Fatalf("without discovery: %+v, %v", rows, err)
	}

	// A different registration of the same public name cannot take over maintenance.
	if err := adminstate.WithWrite(context.Background(), h, "test", func(s *adminstate.Session) error {
		s.Config.Marketplaces[0].RegistrationID = strings.Repeat("a", 32)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := check(t, h, prov2("tool", "team"))
	r := row(t, rows, "tool@team")
	if r.Status != pluginadmin.UpdateFailed || r.Error.Token != diag.MarketplaceEntryUnavailable.Token || code(err) != diag.PluginUpdateFailed.Code {
		t.Fatalf("row = %+v, err = %v", r, err)
	}
}

func TestSiblingsOfOneSourceShareOneCandidateRevision(t *testing.T) {
	h := newHome(t)
	dir := writeCatalog(t, "team", map[string]string{"one": manifest("one", "1", linker), "two": manifest("two", "1", linker)})
	gittest.Git(t, dir, "init", "-q", "-b", "main")
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "publish")
	register(t, h, "file://"+filepath.ToSlash(dir))
	add(t, h, "one", "team")
	add(t, h, "two", "team")

	setPlugin(t, dir, "plugins/one", manifest("one", "2", linker), toolV2)
	setPlugin(t, dir, "plugins/two", manifest("two", "2", linker), toolV2)
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "both")

	rows, err := check(t, h, prov2("one", "team"), prov2("two", "team"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := row(t, rows, "one@team"), row(t, rows, "two@team")
	if a.Status != pluginadmin.UpdateAvailable || b.Status != pluginadmin.UpdateAvailable ||
		a.CandidateRevision == nil || *a.CandidateRevision != *b.CandidateRevision || len(*a.CandidateRevision) < 40 {
		t.Fatalf("rows = %+v / %+v", a, b)
	}
}

func TestApplyPublishesTheExactReviewedCandidateWithoutRefetch(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")
	setPlugin(t, src, "plugins/tool", manifest("tool", "2", linker), toolV2)

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("tool", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()

	// The author moves on after review. The reviewed bytes are what get applied.
	setPlugin(t, src, "plugins/tool", manifest("tool", "3", linker), "#!/bin/sh\nexit 3\n")
	rows, err := pluginadmin.ApplyUpdates(context.Background(), plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if r := row(t, rows, "tool@team"); r.Status != pluginadmin.UpdateUpdated || !r.Committed {
		t.Fatalf("row = %+v", r)
	}
	p, _ := snapshot(t, h).Registry.PackageByProvider(prov2("tool", "team"))
	root, _ := h.PackageRoot(p.Provider)
	if b, _ := os.ReadFile(filepath.Join(root, "source", "bin", "tool")); string(b) != toolV2 || p.Version != "2" {
		t.Fatalf("applied %q at version %q", b, p.Version)
	}
}

func TestInvalidCandidatePreservesTheOldCopy(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")
	setPlugin(t, src, "plugins/tool", manifest("tool", "2", `{"name":"run","role":"linker","key":"fixture.run","entrypoint":"bin/gone","manual":{"display_name":"run"}}`), toolV2)

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("tool", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	rows, err := pluginadmin.ApplyUpdates(context.Background(), plan, false)
	if row(t, rows, "tool@team").Status != pluginadmin.UpdateFailed || code(err) != diag.PluginUpdateFailed.Code {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	p, _ := snapshot(t, h).Registry.PackageByProvider(prov2("tool", "team"))
	root, _ := h.PackageRoot(p.Provider)
	if b, _ := os.ReadFile(filepath.Join(root, "source", "bin", "tool")); string(b) != toolV1 || p.Version != "1" {
		t.Fatalf("old copy changed: %q at %q", b, p.Version)
	}
	if entries, _ := os.ReadDir(h.PluginsDir()); len(entries) != 1 {
		t.Fatalf("stage left behind: %v", entries)
	}
}

func TestDriftedTargetFailsAloneAndSiblingsStillUpdate(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"one": manifest("one", "1", linker), "two": manifest("two", "1", linker)})
	register(t, h, src)
	add(t, h, "one", "team")
	add(t, h, "two", "team")
	setPlugin(t, src, "plugins/one", manifest("one", "2", linker), toolV2)
	setPlugin(t, src, "plugins/two", manifest("two", "2", linker), toolV2)

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("one", "team"), prov2("two", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{prov2("one", "team")}, false); err != nil {
		t.Fatal(err)
	}
	rows, err := pluginadmin.ApplyUpdates(context.Background(), plan, false)
	if row(t, rows, "one@team").Status != pluginadmin.UpdateFailed || row(t, rows, "one@team").Error.Token != diag.AdministrationConflict.Token {
		t.Fatalf("rows = %+v", rows)
	}
	if row(t, rows, "two@team").Status != pluginadmin.UpdateUpdated {
		t.Fatalf("an independent target did not survive: %+v", rows)
	}
	if code(err) != diag.AdministrationConflict.Code {
		t.Fatalf("the conflict must outrank the generic failure: %v (exit %d)", err, code(err))
	}
	s := snapshot(t, h)
	one, _ := s.Registry.PackageByProvider(prov2("one", "team"))
	two, _ := s.Registry.PackageByProvider(prov2("two", "team"))
	if one.Version != "1" || two.Version != "2" {
		t.Fatalf("versions = %q %q", one.Version, two.Version)
	}
}

func TestUpdatePreservesEnablementAndWithdrawnLocatorsNeedAuthorization(t *testing.T) {
	h := newHome(t)
	locator := `{"name":"find","role":"repository-locator","entrypoint":"bin/tool","accepts":["name"]}`
	src := writeCatalog(t, "team", map[string]string{"loc": manifest("loc", "1", locator)})
	register(t, h, src)
	add(t, h, "loc", "team")
	if err := adminstate.WithWrite(context.Background(), h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = []string{"loc@team/find", "gone@elsewhere/stale"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{prov2("loc", "team")}, false); err != nil {
		t.Fatal(err)
	}
	setPlugin(t, src, "plugins/loc", manifest("loc", "2", linker), toolV2)

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("loc", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	r := row(t, plan.Results, "loc@team")
	if !slices.Equal(r.PolicyReferencesRemoved, []string{"loc@team/find"}) {
		t.Fatalf("withdrawn references = %v", r.PolicyReferencesRemoved)
	}
	if _, err := pluginadmin.ApplyUpdates(context.Background(), plan, false); code(err) != diag.PluginPolicyDependency.Code {
		t.Fatalf("unauthorized dependency removal = %v (exit %d)", err, code(err))
	}
	if p, _ := snapshot(t, h).Registry.PackageByProvider(prov2("loc", "team")); p.Version != "1" {
		t.Fatal("a refused batch applied something")
	}
	if _, err := pluginadmin.ApplyUpdates(context.Background(), plan, true); err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, h)
	p, _ := s.Registry.PackageByProvider(prov2("loc", "team"))
	if p.Version != "2" || p.Enabled {
		t.Fatalf("package = %+v", p)
	}
	if got := s.Config.RepositoryResolution.Locators; !slices.Equal(got, []string{"gone@elsewhere/stale"}) {
		t.Fatalf("unrelated stale references must stay; got %v", got)
	}
}

func TestUnknownOrUninstalledTargetsAreUsageErrorsBeforeRetrieval(t *testing.T) {
	h := newHome(t)
	register(t, h, writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)}))
	for _, ps := range [][]pluginid.Provider{{prov2("tool", "team")}, {}} {
		plan, err := pluginadmin.PrepareUpdates(context.Background(), h, ps, false)
		if plan != nil {
			plan.Close()
		}
		if code(err) != 2 {
			t.Errorf("%v: exit %d (%v)", ps, code(err), err)
		}
	}
}

func TestCancellationOutranksTheBatchAndStagesAreReleased(t *testing.T) {
	h := newHome(t)
	src := writeCatalog(t, "team", map[string]string{"tool": manifest("tool", "1", linker)})
	register(t, h, src)
	add(t, h, "tool", "team")
	setPlugin(t, src, "plugins/tool", manifest("tool", "2", linker), toolV2)

	plan, err := pluginadmin.PrepareUpdates(context.Background(), h, []pluginid.Provider{prov2("tool", "team")}, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows, err := pluginadmin.ApplyUpdates(ctx, plan, false)
	if code(err) != diag.Cancelled.Code || row(t, rows, "tool@team").Status != pluginadmin.UpdateNotApplied {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	plan.Close()
	plan.Close()
	if entries, _ := os.ReadDir(h.PluginsDir()); len(entries) != 1 {
		t.Fatalf("stage remains: %v", entries)
	}
	if leases, _ := os.ReadDir(h.LeasesDir()); len(leases) != 0 {
		t.Fatalf("leases remain: %v", leases)
	}
}
