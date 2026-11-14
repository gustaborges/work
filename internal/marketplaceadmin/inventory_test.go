package marketplaceadmin

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func fixturePackage(registrationID string) registry.Package {
	p := pluginid.Provider{Marketplace: "demo", Plugin: "alpha"}
	return registry.Package{
		Provider: p, InstallMode: registry.InstallModeCopy, Enabled: true, Version: "1.0.0", ContentDigest: "sha256:x", DigestAlgorithm: registry.DigestPackageV1,
		Provenance: registry.Provenance{RegistrationID: registrationID, Kind: pluginid.KindLocal, Source: "/s", MarketplaceName: "demo", PluginName: "alpha", Path: "plugins/alpha", Revision: "r", CatalogDigest: "c"},
	}
}

func TestListProjectsTheIncludedProviderWithoutCreatingAHome(t *testing.T) {
	h := workhome.At(t.TempDir() + "/absent")
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	rows := List(s)
	if len(rows) != 1 || rows[0].Name != pluginid.Builtin || rows[0].SourceKind != pluginid.KindBuiltin || rows[0].CatalogStatus != CatalogReady ||
		rows[0].PluginCount == nil || *rows[0].PluginCount != 1 || rows[0].Revision == nil || rows[0].InstalledCount != 0 || rows[0].EnabledCount != 0 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].DisplayName == nil || *rows[0].DisplayName != "Included" || rows[0].Branch != nil {
		t.Fatalf("display/branch = %+v", rows[0])
	}
	if _, err := os.Stat(h.Root()); !os.IsNotExist(err) {
		t.Fatal("a pure projection created the home")
	}
	// The online `work` marketplace appears only after explicit registration.
	for _, r := range rows {
		if r.Name == "work" {
			t.Fatal("an official online source was registered implicitly")
		}
	}
}

func TestTotalsAreLocalCompleteAndInstanceBound(t *testing.T) {
	h := home(t)
	ctx := context.Background()
	mustAdd(t, h, local(marketplaces.Copy(t, "multiple")))
	s := read(t, h)
	id := s.Config.Marketplaces[0].RegistrationID
	other := "fedcba9876543210fedcba9876543210"
	if err := adminstate.WithWrite(ctx, h, "fixture", func(sess *adminstate.Session) error {
		// alpha enabled, beta disabled, gamma withdrawn from the offer but still installed.
		for name, enabled := range map[string]bool{"alpha": true, "beta": false, "gamma": true, "stale": true} {
			p := fixturePackage(id)
			p.Provider.Plugin, p.Provenance.PluginName, p.Enabled = name, name, enabled
			if name == "stale" {
				p.Provenance.RegistrationID = other // a previous registration reusing the namespace
			}
			sess.Registry.UpsertPackage(p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var demo Source
	for _, r := range List(read(t, h)) {
		if r.Name == "demo" {
			demo = r
		}
	}
	if demo.InstalledCount != 3 || demo.EnabledCount != 2 || demo.EnabledCount > demo.InstalledCount {
		t.Fatalf("totals = %d/%d; a re-registered namespace must not inherit the earlier instance's packages", demo.EnabledCount, demo.InstalledCount)
	}
}

func TestMissingOrMismatchedCatalogLeavesTotalsKnown(t *testing.T) {
	h := home(t)
	ctx := context.Background()
	src := marketplaces.Copy(t, "single")
	mustAdd(t, h, local(src))
	s := read(t, h)
	id := s.Config.Marketplaces[0].RegistrationID
	if err := adminstate.WithWrite(ctx, h, "fixture", func(sess *adminstate.Session) error {
		p := fixturePackage(id)
		p.Provider.Marketplace, p.Provenance.MarketplaceName = "solo", "solo"
		sess.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	find := func() Source {
		for _, r := range List(read(t, h)) {
			if r.Name == "solo" {
				return r
			}
		}
		t.Fatal("solo missing")
		return Source{}
	}
	if r := find(); r.CatalogStatus != CatalogReady {
		t.Fatalf("ready = %+v", r)
	}
	// Mismatch: the stored offer no longer belongs to the registered descriptor.
	if err := adminstate.WithWrite(ctx, h, "edit", func(sess *adminstate.Session) error {
		sess.Config.Marketplaces[0].Source = "/another/place"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := find()
	if r.CatalogStatus != CatalogMissing || r.PluginCount != nil || r.Revision != nil || r.DisplayName != nil || r.InstalledCount != 1 || r.EnabledCount != 1 {
		t.Fatalf("mismatched catalog = %+v", r)
	}
	// Removed catalog state behaves the same.
	if err := adminstate.WithWrite(ctx, h, "drop", func(sess *adminstate.Session) error {
		sess.Catalogs.Remove(id)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if r := find(); r.CatalogStatus != CatalogMissing || r.InstalledCount != 1 {
		t.Fatalf("missing catalog = %+v", r)
	}
}

func TestListIsSafeSortedAndSerializesNulls(t *testing.T) {
	h := home(t)
	if err := adminstate.WithWrite(context.Background(), h, "fixture", func(s *adminstate.Session) error {
		s.Config.Marketplaces = []config.Marketplace{
			{RegistrationID: "0123456789abcdef0123456789abcdef", Name: "zeta", Kind: config.KindGit, Source: "https://user:secret@example.org/z.git?token=t", Branch: "main"},
			{RegistrationID: "fedcba9876543210fedcba9876543210", Name: "alpha", Kind: config.KindLocal, Source: "/src/alpha"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows := List(read(t, h))
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "alpha,builtin,zeta" {
		t.Fatalf("order = %v", names)
	}
	b, _ := json.Marshal(rows)
	for _, secret := range []string{"secret", "token=t", "user:"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("projection leaks %q: %s", secret, b)
		}
	}
	var generic []map[string]any
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "display_name", "source_kind", "source", "branch", "revision", "plugin_count", "catalog_status", "installed_count", "enabled_count"} {
		if _, ok := generic[0][key]; !ok {
			t.Errorf("missing %q", key)
		}
	}
	if generic[0]["plugin_count"] != nil || generic[0]["revision"] != nil || generic[0]["catalog_status"] != "missing" {
		t.Errorf("an unfetched source must report null facts: %v", generic[0])
	}
	_ = marketplace.StateSchema
}
