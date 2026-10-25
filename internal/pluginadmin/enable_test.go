package pluginadmin

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func installedHome(t *testing.T) workhome.Home {
	t.Helper()
	h := workhome.At(t.TempDir())
	err := adminstate.WithWrite(context.Background(), h, "fixture", func(s *adminstate.Session) error {
		for _, alias := range []string{"a", "b"} {
			s.Registry.UpsertPackage(registry.Package{Alias: alias, Conventions: []string{"shared"}})
		}
		s.Registry.UpsertConvention(registry.Convention{Name: "shared"})
		s.Config.RepositoryResolution.Locators = []string{"b/clone", "a/clone"}
		s.Memory.Set("repo", "shared")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func TestAdminEnableAtomicNoop(t *testing.T) {
	h := installedHome(t)
	ctx := context.Background()
	cfg, _ := os.ReadFile(h.ConfigFile())
	mem, _ := os.ReadFile(h.BranchConventionsFile())
	reg, _ := os.ReadFile(h.RegistryFile())
	if _, err := SetEnabled(ctx, h, []string{"a", "unknown"}, false); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(h.RegistryFile())
	if string(after) != string(reg) {
		t.Fatal("partial batch")
	}
	aliases, err := SetEnabled(ctx, h, []string{"b", "a", "a"}, false)
	if err != nil || !reflect.DeepEqual(aliases, []string{"a", "b"}) {
		t.Fatal(aliases, err)
	}
	s, _ := adminstate.Read(ctx, h)
	if s.Registry.PackageEnabled("a") || len(s.Registry.Effective().Conventions) != 0 || len(s.Registry.Packages) != 2 {
		t.Fatal("toggle lost raw ownership")
	}
	epoch := s.Epoch
	reg, _ = os.ReadFile(h.RegistryFile())
	if _, err = SetEnabled(ctx, h, aliases, false); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(h.RegistryFile())
	s, _ = adminstate.Read(ctx, h)
	if s.Epoch != epoch || string(after) != string(reg) {
		t.Fatal("noop rewrote state")
	}
	if _, err = SetEnabled(ctx, h, []string{"a"}, true); err != nil {
		t.Fatal(err)
	}
	s, _ = adminstate.Read(ctx, h)
	if len(s.Registry.Effective().Conventions) != 1 {
		t.Fatal("shared convention not restored")
	}
	after, _ = os.ReadFile(h.ConfigFile())
	if string(after) != string(cfg) {
		t.Fatal("policy changed")
	}
	after, _ = os.ReadFile(h.BranchConventionsFile())
	if string(after) != string(mem) {
		t.Fatal("memory changed")
	}
}
func TestAdminEnableConcurrentLatest(t *testing.T) {
	h := installedHome(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Go(func() { _, err := SetEnabled(ctx, h, []string{"a"}, false); errs <- err })
	wg.Go(func() {
		errs <- adminstate.WithWrite(ctx, h, "remember", func(s *adminstate.Session) error { s.Memory.Set("another", "shared"); return nil })
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
	if s.Registry.PackageEnabled("a") {
		t.Fatal("toggle lost")
	}
	if v, ok := s.Memory.Get("another"); !ok || v != "shared" {
		t.Fatal("unrelated edit lost")
	}
}
