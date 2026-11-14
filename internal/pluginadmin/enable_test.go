package pluginadmin

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoconv"
)

func TestSetEnabledIsAtomicDeterministicAndIdempotent(t *testing.T) {
	h := installedHome(t)
	ctx := context.Background()
	setPolicy(t, h, "b@m1/clone", "a@m1/clone")
	if err := adminstate.WithWrite(ctx, h, "remember", func(s *adminstate.Session) error {
		s.Memory.Set("repo", repoconv.NewReference("0123456789abcdef0123456789abcdef", pluginid.Ref{Provider: prov("a", "m1"), Name: "shared"}))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := os.ReadFile(h.ConfigFile())
	mem, _ := os.ReadFile(h.BranchConventionsFile())
	reg, _ := os.ReadFile(h.RegistryFile())

	if _, err := SetEnabled(ctx, h, []pluginid.Provider{prov("a", "m1"), prov("ghost", "m1")}, false); diag.ExitCode(err) != 2 {
		t.Fatalf("unknown target = %v", err)
	}
	if after, _ := os.ReadFile(h.RegistryFile()); string(after) != string(reg) {
		t.Fatal("a partial batch was applied")
	}

	done, err := SetEnabled(ctx, h, []pluginid.Provider{prov("b", "m1"), prov("a", "m1"), prov("a", "m1")}, false)
	if err != nil || !reflect.DeepEqual(ProviderNames(done), []string{"a@m1", "b@m1"}) {
		t.Fatalf("done = %v, %v", done, err)
	}
	s, _ := adminstate.Read(ctx, h)
	if s.Registry.PackageEnabled(prov("a", "m1")) || !s.Registry.PackageEnabled(prov("a", "m2")) {
		t.Fatal("toggling one provider must not touch its homonym")
	}
	// The homonym's convention stays available; the disabled providers' do not.
	if _, ok := s.Registry.Effective().ConventionByRef(pluginid.Ref{Provider: prov("a", "m2"), Name: "shared"}); !ok {
		t.Fatal("homonym convention lost")
	}
	if _, ok := s.Registry.Effective().ConventionByRef(pluginid.Ref{Provider: prov("a", "m1"), Name: "shared"}); ok {
		t.Fatal("disabled provider's convention still available")
	}
	if len(s.Registry.Packages) != 4 {
		t.Fatal("toggle lost raw ownership")
	}

	epoch := s.Epoch
	reg, _ = os.ReadFile(h.RegistryFile())
	if _, err = SetEnabled(ctx, h, done, false); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(h.RegistryFile())
	s, _ = adminstate.Read(ctx, h)
	if s.Epoch != epoch || string(after) != string(reg) {
		t.Fatal("a satisfied request rewrote state")
	}

	if _, err = SetEnabled(ctx, h, []pluginid.Provider{prov("a", "m1")}, true); err != nil {
		t.Fatal(err)
	}
	if after, _ = os.ReadFile(h.ConfigFile()); string(after) != string(cfg) {
		t.Fatal("policy changed")
	}
	if after, _ = os.ReadFile(h.BranchConventionsFile()); string(after) != string(mem) {
		t.Fatal("memory changed")
	}
}

func TestSetEnabledRejectsInvalidAndEmptyTargets(t *testing.T) {
	h := installedHome(t)
	for _, targets := range [][]pluginid.Provider{nil, {{Plugin: "a"}}, {{Marketplace: "m1", Plugin: "a/b"}}, {{Marketplace: "..", Plugin: "a"}}} {
		if _, err := SetEnabled(context.Background(), h, targets, false); diag.ExitCode(err) != 2 {
			t.Errorf("%v = %v", targets, err)
		}
	}
}

func TestSetEnabledMergesConcurrentUnrelatedEdits(t *testing.T) {
	h := installedHome(t)
	ctx := context.Background()
	ref := repoconv.NewReference("0123456789abcdef0123456789abcdef", pluginid.Ref{Provider: prov("keep", "m1"), Name: "shared"})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Go(func() { _, err := SetEnabled(ctx, h, []pluginid.Provider{prov("a", "m1")}, false); errs <- err })
	wg.Go(func() {
		errs <- adminstate.WithWrite(ctx, h, "remember", func(s *adminstate.Session) error { s.Memory.Set("another", ref); return nil })
	})
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if s.Registry.PackageEnabled(prov("a", "m1")) {
		t.Fatal("toggle lost")
	}
	if v, ok := s.Memory.Get("another"); !ok || v != ref {
		t.Fatal("unrelated edit lost")
	}
}
