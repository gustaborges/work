package marketplaceadmin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func bytesOf(t *testing.T, h interface {
	ConfigFile() string
	CatalogsFile() string
	AdministrationFile() string
}) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range []string{h.ConfigFile(), h.CatalogsFile(), h.AdministrationFile()} {
		b, _ := os.ReadFile(f)
		out[f] = string(b)
	}
	return out
}

func row(t *testing.T, rows []UpgradeResult, name string) UpgradeResult {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no row for %q in %+v", name, rows)
	return UpgradeResult{}
}

func TestRefreshOfAnIdenticalOfferPreservesBytesAndEpoch(t *testing.T) {
	h := home(t)
	src := marketplaces.Copy(t, "multiple")
	mustAdd(t, h, local(src))
	before := bytesOf(t, h)
	rows, err := Upgrade(context.Background(), h, []string{"demo"})
	if err != nil || len(rows) != 1 || rows[0].Status != StatusUnchanged || rows[0].Revision == nil || rows[0].Committed {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	for f, want := range before {
		if got, _ := os.ReadFile(f); string(got) != want {
			t.Errorf("%s changed on a no-op refresh", filepath.Base(f))
		}
	}
	if entries, _ := os.ReadDir(h.SnapshotsDir()); len(entries) != 1 {
		t.Errorf("a no-op refresh left extra captures: %v", entries)
	}
}

func TestRefreshPublishesAChangedOfferWithoutTouchingInstallations(t *testing.T) {
	h := home(t)
	src := marketplaces.Copy(t, "multiple")
	mustAdd(t, h, local(src))
	s := read(t, h)
	oldRevision := s.Catalogs.Catalogs[0].Revision
	// An installed copy and its enablement are untouched by discovery.
	err := adminstate.WithWrite(context.Background(), h, "install fixture", func(sess *adminstate.Session) error {
		p := fixturePackage(s.Config.Marketplaces[0].RegistrationID)
		sess.Registry.UpsertPackage(p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	registryBefore, _ := os.ReadFile(h.RegistryFile())
	policyBefore, _ := os.ReadFile(h.LifecycleFile())

	if err := os.WriteFile(filepath.Join(src, "plugins", "alpha", "extra.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := Upgrade(context.Background(), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	demo := row(t, rows, "demo")
	if demo.Status != StatusUpgraded || !demo.Committed || demo.Revision == nil || *demo.Revision == oldRevision {
		t.Fatalf("demo = %+v", demo)
	}
	if b := row(t, rows, pluginid.Builtin); b.Status != StatusUnchanged {
		t.Fatalf("the included offer must always be current: %+v", b)
	}
	after := read(t, h)
	if after.Catalogs.Catalogs[0].Revision != *demo.Revision {
		t.Fatal("the refreshed offer was not published")
	}
	if got, _ := os.ReadFile(h.RegistryFile()); string(got) != string(registryBefore) {
		t.Fatal("refresh changed installed state")
	}
	if got, _ := os.ReadFile(h.LifecycleFile()); string(got) != string(policyBefore) {
		t.Fatal("refresh changed lifecycle/policy")
	}
	// Exactly one capture remains: the superseded one is collected.
	if err := adminstate.WithWrite(context.Background(), h, "noop", func(*adminstate.Session) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(h.SnapshotsDir()); len(entries) != 1 {
		t.Fatalf("captures = %v", entries)
	}
}

func TestRefreshFailureKeepsTheStoredOfferAndOtherSourcesProceed(t *testing.T) {
	h := home(t)
	good := marketplaces.Copy(t, "multiple")
	bad := marketplaces.Copy(t, "single")
	mustAdd(t, h, local(bad))
	mustAdd(t, h, local(good))
	before := read(t, h)
	if err := os.WriteFile(filepath.Join(good, "plugins", "beta", "extra.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// "bad" now publishes an invalid catalog.
	if err := os.WriteFile(filepath.Join(bad, "marketplace.json"), []byte(`{"schema":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := Upgrade(context.Background(), h, nil)
	if code(err) != diag.MarketplaceUpgradeFailed.Code {
		t.Fatalf("aggregate = %v", err)
	}
	if r := row(t, rows, "solo"); r.Status != StatusFailed || r.Error == nil || r.Error.Token != diag.MarketplaceInvalid.Token || r.Revision != nil {
		t.Fatalf("solo = %+v", r)
	}
	if r := row(t, rows, "demo"); r.Status != StatusUpgraded {
		t.Fatalf("an independent success was lost: %+v", r)
	}
	after := read(t, h)
	solo := func(s *adminstate.Snapshot) string {
		for _, c := range s.Catalogs.Catalogs {
			if c.Namespace == "solo" {
				return c.Revision
			}
		}
		return ""
	}
	if solo(before) == "" || solo(before) != solo(after) {
		t.Fatal("a failed refresh must preserve the earlier valid offer")
	}
}

func TestRefreshRejectsAChangedNamespace(t *testing.T) {
	h := home(t)
	src := marketplaces.Copy(t, "single")
	mustAdd(t, h, local(src))
	if err := os.WriteFile(filepath.Join(src, "marketplace.json"), []byte(`{"schema":1,"name":"renamed","plugins":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := bytesOf(t, h)
	rows, err := Upgrade(context.Background(), h, []string{"solo"})
	if code(err) != diag.MarketplaceUpgradeFailed.Code || row(t, rows, "solo").Error.Token != diag.MarketplaceNameConflict.Token {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	for f, want := range before {
		if got, _ := os.ReadFile(f); string(got) != want {
			t.Errorf("%s changed after a rejected rename", filepath.Base(f))
		}
	}
}

func TestRefreshOfAGitSourceKeepsTheRevisionForASemanticNoop(t *testing.T) {
	h := home(t)
	dir, url := gitCatalog(t, "multiple", "main")
	mustAdd(t, h, Spec{Kind: pluginid.KindGit, Source: url})
	first := read(t, h).Catalogs.Catalogs[0].Revision
	// A new commit with an identical offer keeps the earlier published revision.
	gittest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "same offer")
	before := bytesOf(t, h)
	rows, err := Upgrade(context.Background(), h, []string{"demo"})
	if err != nil || rows[0].Status != StatusUnchanged || rows[0].Revision == nil || *rows[0].Revision != first {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	for f, want := range before {
		if got, _ := os.ReadFile(f); string(got) != want {
			t.Errorf("%s changed", filepath.Base(f))
		}
	}
	// A real change advances to the new exact commit.
	if err := os.WriteFile(filepath.Join(dir, "plugins", "gamma", "extra"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "change")
	tip := gittest.Git(t, dir, "rev-parse", "HEAD")
	rows, err = Upgrade(context.Background(), h, []string{"demo"})
	if err != nil || rows[0].Status != StatusUpgraded || *rows[0].Revision != tip {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	if read(t, h).Catalogs.Catalogs[0].CaptureDigest != nil {
		t.Fatal("a Git catalog records a commit, not a capture")
	}
}

func TestUpgradeTargetsAndCancellation(t *testing.T) {
	h := home(t)
	mustAdd(t, h, local(marketplaces.Copy(t, "single")))
	if _, err := Upgrade(context.Background(), h, []string{"nope"}); code(err) != 2 {
		t.Fatalf("unknown target = %v", err)
	}
	rows, err := Upgrade(context.Background(), h, []string{"solo", "solo", "builtin"})
	if err != nil || len(rows) != 2 || rows[0].Name != "solo" || rows[1].Name != "builtin" {
		t.Fatalf("rows = %+v, %v (explicit targets are deduplicated in the order given)", rows, err)
	}
	rows, err = Upgrade(context.Background(), h, nil)
	if err != nil || len(rows) != 2 || rows[0].Name != "builtin" || rows[1].Name != "solo" {
		t.Fatalf("rows = %+v, %v (every source is refreshed in name order)", rows, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows, err = Upgrade(ctx, h, nil)
	if code(err) != diag.Cancelled.Code {
		t.Fatalf("cancelled = %v", err)
	}
	for _, r := range rows {
		if r.Status == StatusUpgraded {
			t.Fatalf("cancellation published work: %+v", r)
		}
	}
}
