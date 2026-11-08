package marketplaceadmin

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
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func installFrom(t *testing.T, h workhome.Home, plugin, mkt string) {
	t.Helper()
	plan, err := pluginadmin.PrepareInstall(context.Background(), h, plugin, mkt)
	if err != nil {
		t.Fatalf("prepare %s@%s: %v", plugin, mkt, err)
	}
	defer plan.Close()
	if _, err = plan.Apply(context.Background(), false); err != nil {
		t.Fatalf("apply %s@%s: %v", plugin, mkt, err)
	}
}

func p(plugin, mkt string) pluginid.Provider {
	return pluginid.Provider{Marketplace: mkt, Plugin: plugin}
}

func setPolicy(t *testing.T, h workhome.Home, refs ...string) {
	t.Helper()
	if err := adminstate.WithWrite(context.Background(), h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = refs
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func prepareRemove(t *testing.T, h workhome.Home, name string) *RemovePlan {
	t.Helper()
	plan, err := PrepareRemove(context.Background(), h, name)
	if err != nil {
		t.Fatalf("PrepareRemove(%s): %v", name, err)
	}
	return plan
}

// demoHome registers the three-plugin "demo" source and the "solo" source and
// installs alpha (enabled), beta (disabled), gamma (later withdrawn) from demo
// and sample from solo.
func demoHome(t *testing.T) (workhome.Home, string) {
	t.Helper()
	h := home(t)
	demo := marketplaces.Copy(t, "multiple")
	mustAdd(t, h, local(demo))
	mustAdd(t, h, local(marketplaces.Copy(t, "single")))
	for _, name := range []string{"alpha", "beta", "gamma"} {
		installFrom(t, h, name, "demo")
	}
	installFrom(t, h, "sample", "solo")
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{p("beta", "demo")}, false); err != nil {
		t.Fatal(err)
	}
	// The author withdraws gamma and the source refreshes: the copy survives.
	raw, _ := json.Marshal(map[string]any{"schema": 1, "name": "demo", "plugins": []map[string]string{
		{"name": "alpha", "path": "plugins/alpha"}, {"name": "beta", "path": "plugins/beta"},
	}})
	if err := os.WriteFile(filepath.Join(demo, "marketplace.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Upgrade(context.Background(), h, []string{"demo"}); err != nil {
		t.Fatal(err)
	}
	return h, demo
}

func TestRemoveEnumeratesTheExactSetIncludingDisabledAndWithdrawnOffline(t *testing.T) {
	h, demo := demoHome(t)
	if err := os.RemoveAll(demo); err != nil { // the origin is gone: removal is local-only
		t.Fatal(err)
	}
	plan := prepareRemove(t, h, "demo")
	want := []pluginid.Provider{p("alpha", "demo"), p("beta", "demo"), p("gamma", "demo")}
	if got := plan.Providers(); !slices.Equal(got, want) {
		t.Fatalf("providers = %v, want %v", got, want)
	}
	res, err := plan.Apply(context.Background(), false)
	if err != nil || !res.Committed || !slices.Equal(res.Removed, want) {
		t.Fatalf("result = %+v, err = %v", res, err)
	}

	s := read(t, h)
	if _, ok := s.Config.Marketplace("demo"); ok {
		t.Fatal("the registration survived")
	}
	if len(s.Catalogs.Catalogs) != 1 || s.Catalogs.Catalogs[0].Namespace != "solo" {
		t.Fatalf("catalogs = %+v", s.Catalogs.Catalogs)
	}
	if len(s.Registry.Packages) != 1 || s.Registry.Packages[0].Provider != p("sample", "solo") {
		t.Fatalf("registry = %+v", s.Registry.Packages)
	}
	if _, err := os.Stat(filepath.Join(h.PluginsDir(), "demo")); !os.IsNotExist(err) {
		t.Fatalf("the namespace directory remains: %v", err)
	}
	if root, _ := h.PackageRoot(p("sample", "solo")); root == "" {
		t.Fatal("no root for the unrelated source")
	} else if _, err := os.Stat(root); err != nil {
		t.Fatalf("an unrelated source lost its copy: %v", err)
	}
}

func TestRemovalNeedsSeparateAuthorizationForPolicyReferences(t *testing.T) {
	h, _ := demoHome(t)
	setPolicy(t, h, "alpha@demo/find", "sample@solo/keep")
	plan := prepareRemove(t, h, "demo")
	if got := plan.PolicyReferences(); !slices.Equal(got, []string{"alpha@demo/find"}) {
		t.Fatalf("references = %v", got)
	}
	before := read(t, h)

	res, err := plan.Apply(context.Background(), false)
	if code(err) != diag.PluginPolicyDependency.Code || res.Committed {
		t.Fatalf("refused authorization = %+v, %v (exit %d)", res, err, code(err))
	}
	after := read(t, h)
	if after.Epoch != before.Epoch || len(after.Registry.Packages) != len(before.Registry.Packages) {
		t.Fatal("a refused removal changed state")
	}

	if _, err = plan.Apply(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, h).Config.RepositoryResolution.Locators; !slices.Equal(got, []string{"sample@solo/keep"}) {
		t.Fatalf("policy = %v", got)
	}
}

func TestTheIncludedProviderAndUnknownSourcesAreRefusedByTheDomain(t *testing.T) {
	h := home(t)
	for name, wantReason := range map[string]string{"builtin": ReasonDistributionProvided, "nowhere": ""} {
		_, err := PrepareRemove(context.Background(), h, name)
		if code(err) != 2 || (wantReason != "" && !strings.Contains(err.Error(), wantReason)) {
			t.Errorf("%s: %v (exit %d)", name, err, code(err))
		}
	}
	if _, err := os.Stat(h.Root()); !os.IsNotExist(err) {
		t.Fatal("a refused removal created the home")
	}
}

func TestRemovalRespectsTheSourceInstanceNotTheNamespaceText(t *testing.T) {
	h, _ := demoHome(t)
	// A different registration now owns the public name "demo"; the installed
	// copies still belong to the previous instance.
	if err := adminstate.WithWrite(context.Background(), h, "test", func(s *adminstate.Session) error {
		s.Config.Marketplaces[0].RegistrationID = strings.Repeat("b", 32)
		for i := range s.Config.Marketplaces {
			if s.Config.Marketplaces[i].Name == "solo" {
				s.Config.Marketplaces[i].RegistrationID = strings.Repeat("c", 32)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	plan := prepareRemove(t, h, "demo")
	if got := plan.Providers(); len(got) != 0 {
		t.Fatalf("a namespace reuse swept in copies of another instance: %v", got)
	}
}

func TestConcurrentTargetDriftConflictsWithoutWideningTheSet(t *testing.T) {
	for name, drift := range map[string]func(t *testing.T, h workhome.Home){
		"target toggled": func(t *testing.T, h workhome.Home) {
			if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{p("alpha", "demo")}, false); err != nil {
				t.Fatal(err)
			}
		},
		"policy dependency appears": func(t *testing.T, h workhome.Home) { setPolicy(t, h, "alpha@demo/find") },
		"target removed":            func(t *testing.T, h workhome.Home) { removePlugin(t, h, p("beta", "demo")) },
	} {
		t.Run(name, func(t *testing.T) {
			h, _ := demoHome(t)
			plan := prepareRemove(t, h, "demo")
			drift(t, h)
			before := read(t, h)
			_, err := plan.Apply(context.Background(), true)
			if code(err) != diag.AdministrationConflict.Code {
				t.Fatalf("err = %v (exit %d)", err, code(err))
			}
			after := read(t, h)
			if after.Epoch != before.Epoch {
				t.Fatal("a conflicting removal changed state")
			}
			if _, ok := after.Config.Marketplace("demo"); !ok {
				t.Fatal("the registration was removed despite the conflict")
			}
		})
	}
}

func removePlugin(t *testing.T, h workhome.Home, target pluginid.Provider) {
	t.Helper()
	plan, err := pluginadmin.PlanUninstall(context.Background(), h, []pluginid.Provider{target})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pluginadmin.Uninstall(context.Background(), h, plan, true); err != nil {
		t.Fatal(err)
	}
}

func TestPluginInstalledFromTheSourceAfterReviewConflicts(t *testing.T) {
	h := home(t)
	mustAdd(t, h, local(marketplaces.Copy(t, "multiple")))
	installFrom(t, h, "alpha", "demo")
	plan := prepareRemove(t, h, "demo")
	installFrom(t, h, "beta", "demo")
	if _, err := plan.Apply(context.Background(), true); code(err) != diag.AdministrationConflict.Code {
		t.Fatalf("err = %v (exit %d)", err, code(err))
	}
	if _, ok := read(t, h).Registry.PackageByProvider(p("beta", "demo")); !ok {
		t.Fatal("the late installation was swept in")
	}
}

func TestUnrelatedConcurrentChangesMergeAndCancellationPreservesTheSet(t *testing.T) {
	h, _ := demoHome(t)
	plan := prepareRemove(t, h, "demo")
	if _, err := pluginadmin.SetEnabled(context.Background(), h, []pluginid.Provider{p("sample", "solo")}, false); err != nil {
		t.Fatal(err)
	}
	before := read(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res, err := plan.ApplyWithOptions(ctx, true, adminstate.WriteOptions{}); code(err) != diag.Cancelled.Code || res.Committed {
		t.Fatalf("cancelled apply = %+v, %v", res, err)
	}
	if read(t, h).Epoch != before.Epoch {
		t.Fatal("a cancelled removal changed state")
	}
	if _, err := plan.Apply(context.Background(), true); err != nil {
		t.Fatalf("an unrelated change blocked the reviewed removal: %v", err)
	}
	sample, _ := read(t, h).Registry.PackageByProvider(p("sample", "solo"))
	if sample.Enabled {
		t.Fatal("the unrelated change was lost")
	}
}

func TestRemovalPreservesRememberedConventionsAsUnavailable(t *testing.T) {
	h, _ := demoHome(t)
	var regID string
	if err := adminstate.WithWrite(context.Background(), h, "memory", func(s *adminstate.Session) error {
		m, _ := s.Config.Marketplace("demo")
		regID = m.RegistrationID
		s.Memory.Set("repo-identity", repoconv.NewReference(regID, pluginid.Ref{Provider: p("alpha", "demo"), Name: "freeform"}))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareRemove(t, h, "demo").Apply(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	s := read(t, h)
	ref, ok := s.Memory.Get("repo-identity")
	if !ok || ref.RegistrationID != regID {
		t.Fatalf("memory = %+v, %v", ref, ok)
	}
	if repoconv.Available(ref, s.Registry) {
		t.Fatal("a convention of a removed source is still available")
	}
	// Registering the same origin again is a new instance and cannot claim it.
	mustAdd(t, h, local(marketplaces.Copy(t, "multiple")))
	installFrom(t, h, "alpha", "demo")
	s = read(t, h)
	again, _ := s.Config.Marketplace("demo")
	if again.RegistrationID == regID || repoconv.Available(ref, s.Registry) {
		t.Fatal("a re-registration claimed retained choices")
	}
}
