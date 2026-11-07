package marketplaceadmin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func home(t *testing.T) workhome.Home { return workhome.At(filepath.Join(t.TempDir(), "home")) }

func local(path string) Spec { return Spec{Kind: pluginid.KindLocal, Source: path} }

func mustAdd(t *testing.T, h workhome.Home, spec Spec) AddResult {
	t.Helper()
	res, err := Add(context.Background(), h, spec)
	if err != nil {
		t.Fatalf("Add(%+v): %v", spec, err)
	}
	return res
}

func read(t *testing.T, h workhome.Home) *adminstate.Snapshot {
	t.Helper()
	s, err := adminstate.Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func code(err error) int { return diag.ExitCode(err) }

func TestClassify(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		source, branch, kind string
		wantErr              bool
	}{
		"https url":    {"https://example.org/team/plugins.git", "", pluginid.KindGit, false},
		"ssh url":      {"ssh://git@example.org/team/plugins.git", "dev", pluginid.KindGit, false},
		"file url":     {"file:///srv/plugins.git", "", pluginid.KindGit, false},
		"scp style":    {"git@example.org:team/plugins.git", "", pluginid.KindGit, false},
		"local dir":    {dir, "", pluginid.KindLocal, false},
		"relative":     {"./catalog", "", pluginid.KindLocal, false},
		"local+branch": {dir, "main", "", true},
		"empty":        {"  ", "", "", true},
		"bad branch":   {"https://example.org/x.git", "-oops", "", true},
	}
	for name, c := range cases {
		spec, err := Classify(c.source, c.branch)
		if (err != nil) != c.wantErr || (err != nil && code(err) != 2) {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if err == nil && spec.Kind != c.kind {
			t.Errorf("%s: kind = %q, want %q", name, spec.Kind, c.kind)
		}
	}
	// A local path is made absolute and cleaned; a checkout is never treated as Git.
	spec, _ := Classify("./a/../catalog", "")
	if !filepath.IsAbs(spec.Source) || strings.Contains(spec.Source, "..") {
		t.Errorf("local source not normalized: %q", spec.Source)
	}
	// Windows drive paths are local, not scp-style.
	if spec, err := Classify(`C:\work\catalog`, ""); err != nil || spec.Kind != pluginid.KindLocal {
		t.Errorf("drive path = %+v, %v", spec, err)
	}
}

func TestRegisterLocalSourceInstallsNothingAndLeavesPolicyUnhandled(t *testing.T) {
	h := home(t)
	src := marketplaces.Copy(t, "multiple")
	res := mustAdd(t, h, local(src))
	if res.Name != "demo" || res.Result != "registered" || !res.Committed || !strings.HasPrefix(res.Revision, "sha256:") {
		t.Fatalf("result = %+v", res)
	}
	s := read(t, h)
	if len(s.Config.Marketplaces) != 1 || len(s.Registry.Packages) != 0 || len(s.Registry.Components) != 0 {
		t.Fatalf("registration installed or ran something: %+v %+v", s.Config.Marketplaces, s.Registry)
	}
	m := s.Config.Marketplaces[0]
	if m.Name != "demo" || m.Kind != pluginid.KindLocal || m.Source != src || m.Branch != "" || pluginid.ValidateRegistrationID(m.RegistrationID) != nil || m.RegistrationID == pluginid.BuiltinRegistrationID {
		t.Fatalf("registration = %+v", m)
	}
	c, ok := s.Catalogs.ByRegistration(m.RegistrationID)
	if !ok || len(c.Entries) != 3 || c.DisplayName != "Demo Plugins" || c.CaptureDigest == nil || c.Revision != *c.CaptureDigest {
		t.Fatalf("stored catalog = %+v", c)
	}
	if s.Lifecycle.PolicyHandled || s.Lifecycle.Initialized || len(s.Config.RepositoryResolution.Locators) != 0 {
		t.Fatalf("registration touched policy or lifecycle: %+v / %v", s.Lifecycle, s.Config.RepositoryResolution.Locators)
	}
	// The capture is a hex-only directory and survives the author's source.
	hexDigest, _ := marketplace.HexPayload(*c.CaptureDigest)
	dir, err := h.SnapshotDir(hexDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marketplace.json")); err != nil {
		t.Fatalf("capture missing: %v", err)
	}
	// Browsing is offline: the projection needs neither the source nor the network.
	rows := List(read(t, h))
	var demo Source
	for _, r := range rows {
		if r.Name == "demo" {
			demo = r
		}
	}
	if demo.CatalogStatus != CatalogReady || demo.PluginCount == nil || *demo.PluginCount != 3 || demo.InstalledCount != 0 {
		t.Fatalf("row = %+v", demo)
	}
}

func TestIdenticalRegistrationIsUnchangedAndKeepsTheInstance(t *testing.T) {
	h := home(t)
	src := marketplaces.Copy(t, "single")
	first := mustAdd(t, h, local(src))
	before := read(t, h)
	id := before.Config.Marketplaces[0].RegistrationID
	files := map[string]string{}
	for _, f := range []string{h.ConfigFile(), h.CatalogsFile(), h.AdministrationFile()} {
		b, _ := os.ReadFile(f)
		files[f] = string(b)
	}
	// Unchanged means no retrieval at all: it works even after the source vanished.
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	again, err := Add(context.Background(), h, local(src))
	if err != nil || again.Result != "unchanged" || again.Name != first.Name {
		t.Fatalf("second add = %+v, %v", again, err)
	}
	for f, want := range files {
		if b, _ := os.ReadFile(f); string(b) != want {
			t.Errorf("%s changed on an identical registration", filepath.Base(f))
		}
	}
	if read(t, h).Config.Marketplaces[0].RegistrationID != id {
		t.Fatal("registration instance changed")
	}
}

func TestNamespaceCollisionsAndReservedNames(t *testing.T) {
	h := home(t)
	mustAdd(t, h, local(marketplaces.Copy(t, "single")))
	before := read(t, h)
	// Another source claiming an existing namespace fails without overwrite or rename.
	other := marketplaces.Copy(t, "single")
	if err := os.WriteFile(filepath.Join(other, "marketplace.json"), []byte(`{"schema":1,"name":"SOLO","plugins":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, spec := range map[string]Spec{"case-equivalent namespace": local(other), "same namespace, different source": local(marketplaces.Copy(t, "single"))} {
		if _, err := Add(context.Background(), h, spec); code(err) != diag.MarketplaceNameConflict.Code {
			t.Errorf("%s = %v, want marketplace-name-conflict", name, err)
		}
	}
	// The reserved namespace belongs to the included provider.
	reserved := marketplaces.Copy(t, "invalid-reserved-namespace")
	if _, err := Add(context.Background(), h, local(reserved)); code(err) != diag.MarketplaceNameConflict.Code {
		t.Errorf("reserved = %v", err)
	}
	after := read(t, h)
	if len(after.Config.Marketplaces) != len(before.Config.Marketplaces) || len(after.Catalogs.Catalogs) != 1 {
		t.Fatal("a rejected registration published partial state")
	}
	// No capture was published for a rejected source.
	entries, _ := os.ReadDir(h.SnapshotsDir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".stage-") {
			t.Errorf("stage left behind: %s", e.Name())
		}
	}
}

func TestInvalidAndUnavailableSourcesPublishNothing(t *testing.T) {
	for name, c := range map[string]struct {
		spec func(t *testing.T) Spec
		code int
	}{
		"no catalog":     {func(t *testing.T) Spec { return local(marketplaces.Copy(t, "no-catalog")) }, diag.MarketplaceInvalid.Code},
		"bad schema":     {func(t *testing.T) Spec { return local(marketplaces.Copy(t, "invalid-schema")) }, diag.MarketplaceInvalid.Code},
		"escaping path":  {func(t *testing.T) Spec { return local(marketplaces.Copy(t, "invalid-escaping-path")) }, diag.MarketplaceInvalid.Code},
		"name mismatch":  {func(t *testing.T) Spec { return local(marketplaces.Copy(t, "invalid-name-mismatch")) }, diag.MarketplaceInvalid.Code},
		"missing source": {func(t *testing.T) Spec { return local(filepath.Join(t.TempDir(), "gone")) }, diag.MarketplaceSourceUnavailable.Code},
		"missing git": {func(t *testing.T) Spec {
			return Spec{Kind: pluginid.KindGit, Source: "file://" + filepath.ToSlash(t.TempDir())}
		}, diag.MarketplaceSourceUnavailable.Code},
	} {
		t.Run(name, func(t *testing.T) {
			h := home(t)
			_, err := Add(context.Background(), h, c.spec(t))
			if code(err) != c.code {
				t.Fatalf("err = %v (%d), want %d", err, code(err), c.code)
			}
			if _, statErr := os.Stat(h.ConfigFile()); !os.IsNotExist(statErr) {
				t.Error("a failed registration wrote configuration")
			}
			if entries, _ := os.ReadDir(h.SnapshotsDir()); len(entries) != 0 {
				t.Errorf("leftover capture state: %v", entries)
			}
		})
	}
}

func TestSourceErrorsNeverLeakTransportDetail(t *testing.T) {
	h := home(t)
	_, err := Add(context.Background(), h, Spec{Kind: pluginid.KindGit, Source: "https://user:s3cret@127.0.0.1:1/x.git", Branch: "main"})
	if code(err) != diag.MarketplaceSourceUnavailable.Code {
		t.Fatalf("err = %v", err)
	}
	for _, leak := range []string{"s3cret", "127.0.0.1", "user"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error leaks %q: %v", leak, err)
		}
	}
}

func gitCatalog(t *testing.T, fixture, branch string) (dir, url string) {
	t.Helper()
	dir = marketplaces.Copy(t, fixture)
	gittest.Git(t, dir, "init", "-q", "-b", branch)
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "publish")
	return dir, "file://" + filepath.ToSlash(dir)
}

func TestRegisterGitSourceRecordsTheDiscoveredBranch(t *testing.T) {
	h := home(t)
	dir, url := gitCatalog(t, "multiple", "trunk")
	res := mustAdd(t, h, Spec{Kind: pluginid.KindGit, Source: url})
	if res.Name != "demo" || res.Result != "registered" {
		t.Fatalf("result = %+v", res)
	}
	s := read(t, h)
	m := s.Config.Marketplaces[0]
	if m.Kind != pluginid.KindGit || m.Branch != "trunk" || m.Source != url {
		t.Fatalf("registration = %+v", m)
	}
	c, _ := s.Catalogs.ByRegistration(m.RegistrationID)
	if c.Revision != gittest.Git(t, dir, "rev-parse", "HEAD") || c.CaptureDigest != nil {
		t.Fatalf("git catalog = %+v", c)
	}
	// Re-adding without a branch is unchanged: the recorded branch wins and no default is re-discovered.
	gittest.Git(t, dir, "checkout", "-q", "-b", "other")
	if res := mustAdd(t, h, Spec{Kind: pluginid.KindGit, Source: url}); res.Result != "unchanged" {
		t.Fatalf("re-add = %+v", res)
	}
	// An explicit different branch of the same source is a different source for that namespace.
	if _, err := Add(context.Background(), h, Spec{Kind: pluginid.KindGit, Source: url, Branch: "other"}); code(err) != diag.MarketplaceNameConflict.Code {
		t.Errorf("same namespace from another branch = %v", err)
	}
}

func TestPrepareDoesNotPersistAndCloseReleasesEverything(t *testing.T) {
	h := home(t)
	plan, err := PrepareAdd(context.Background(), h, local(marketplaces.Copy(t, "multiple")))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Name() != "demo" || plan.Offer() == nil || len(plan.Offer().Entries) != 3 {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := os.Stat(h.ConfigFile()); !os.IsNotExist(err) {
		t.Fatal("preparation persisted configuration")
	}
	if leases, _ := os.ReadDir(h.LeasesDir()); len(leases) == 0 {
		t.Fatal("preparation holds no lease for its private capture")
	}
	plan.Close()
	plan.Close() // idempotent
	if leases, _ := os.ReadDir(h.LeasesDir()); len(leases) != 0 {
		t.Fatalf("lease artifacts remain after Close: %v", leases)
	}
	if entries, _ := os.ReadDir(h.SnapshotsDir()); len(entries) != 0 {
		t.Fatalf("capture stage remains after Close: %v", entries)
	}
}

func TestApplyConflictsWhenTheNamespaceWasTakenMeanwhile(t *testing.T) {
	h := home(t)
	ctx := context.Background()
	plan, err := PrepareAdd(ctx, h, local(marketplaces.Copy(t, "single")))
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	mustAdd(t, h, local(marketplaces.Copy(t, "single"))) // same namespace, another source instance
	if _, err := plan.Apply(ctx); code(err) != diag.MarketplaceNameConflict.Code && code(err) != diag.AdministrationConflict.Code {
		t.Fatalf("apply after a competing registration = %v", err)
	}
	if got := read(t, h); len(got.Config.Marketplaces) != 1 {
		t.Fatalf("registrations = %+v", got.Config.Marketplaces)
	}
}

func TestCancellationPublishesNothing(t *testing.T) {
	h := home(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Add(ctx, h, local(marketplaces.Copy(t, "single")))
	if code(err) != diag.Cancelled.Code && !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(h.ConfigFile()); !os.IsNotExist(statErr) {
		t.Fatal("cancelled registration wrote configuration")
	}
}

func TestInterruptedRegistrationRecoversWithoutPartialState(t *testing.T) {
	// A source registration is one journal: config, catalog image and the capture
	// reference commit together or not at all.
	for _, point := range []string{"prepared", "pending", "json:config", "json:catalogs", "before-commit", "committed", "stable"} {
		t.Run(point, func(t *testing.T) {
			h := home(t)
			ctx := context.Background()
			plan, err := PrepareAdd(ctx, h, local(marketplaces.Copy(t, "single")))
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			committed := point == "committed" || point == "stable"
			func() {
				defer func() {
					if r := recover(); r == nil {
						t.Fatal("operation was not interrupted")
					}
				}()
				_ = adminstate.WithWriteOptions(ctx, h, opAdd+" x", func(s *adminstate.Session) error {
					id, _ := newRegistrationID()
					if err := s.PublishCapture(plan.stage, plan.snap.Offer.CaptureDigest()); err != nil {
						return err
					}
					plan.snap.Detach()
					s.Config.Marketplaces = append(s.Config.Marketplaces, configEntry(id, plan))
					s.Catalogs.Upsert(marketplace.StoredFrom(id, plan.snap))
					return nil
				}, adminstate.WriteOptions{Checkpoint: func(p string) error {
					if p == point {
						panic("simulated process death")
					}
					return nil
				}})
			}()
			if _, err := adminstate.Preflight(ctx, h); err != nil {
				t.Fatal(err)
			}
			s := read(t, h)
			if (len(s.Config.Marketplaces) == 1) != committed || (len(s.Catalogs.Catalogs) == 1) != committed {
				t.Fatalf("partial registration at %q: %d configured, %d catalogs", point, len(s.Config.Marketplaces), len(s.Catalogs.Catalogs))
			}
			if !committed {
				// Another mutation collects the now-unreferenced capture.
				if err := adminstate.WithWrite(ctx, h, "noop", func(*adminstate.Session) error { return nil }); err != nil {
					t.Fatal(err)
				}
				if entries, _ := os.ReadDir(h.SnapshotsDir()); len(entries) != 0 {
					t.Fatalf("unreferenced capture survived: %v", entries)
				}
			}
		})
	}
}

func configEntry(id string, p *AddPlan) config.Marketplace {
	return config.Marketplace{RegistrationID: id, Name: p.snap.Offer.Name, Kind: p.spec.Kind, Source: p.spec.Source, Branch: p.spec.Branch}
}
