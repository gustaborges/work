package pluginadmin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func uninstallFixture(t *testing.T) workhome.Home {
	t.Helper()
	h := workhome.At(t.TempDir())
	src := updateSource(t, "one")
	for _, a := range []string{"a", "b", "keep"} {
		installUpdateFixture(t, h, src, a)
	}
	if err := adminstate.WithWrite(context.Background(), h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = []string{"keep/clone", "a/stale", "ghost/old", "b/clone", "a/clone", "ghost/last"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return h
}
func TestAdminUninstallWholeSetAndSharedOwners(t *testing.T) {
	ctx := context.Background()
	h := uninstallFixture(t)
	before, _ := os.ReadFile(h.RegistryFile())
	if _, err := PlanUninstall(ctx, h, []string{"a", "unknown"}); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(h.RegistryFile()); string(b) != string(before) {
		t.Fatal("unknown set changed registry")
	}
	plan, err := PlanUninstall(ctx, h, []string{"b", "a", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Aliases(), []string{"a", "b"}) || !reflect.DeepEqual(plan.PolicyReferences(), []string{"a/stale", "b/clone", "a/clone"}) {
		t.Fatal(plan)
	}
	if rows, err := Uninstall(ctx, h, plan, false); rows != nil || diag.ExitCode(err) != 52 {
		t.Fatal(rows, err)
	}
	// Unrelated latest state is merged, including policy order and roots.
	if err := adminstate.WithWrite(ctx, h, "unrelated", func(s *adminstate.Session) error { s.Config.RepositoryRoots = []string{h.Root()}; return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := Uninstall(ctx, h, plan, true)
	if err != nil || !reflect.DeepEqual(rows, []string{"a", "b"}) {
		t.Fatal(rows, err)
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, []string{"keep/clone", "ghost/old", "ghost/last"}) || !reflect.DeepEqual(s.Config.RepositoryRoots, []string{h.Root()}) {
		t.Fatal(s.Config)
	}
	if _, ok := s.Registry.Effective().ConventionByName("team"); !ok {
		t.Fatal("shared convention removed")
	}
	for _, a := range rows {
		if _, err := os.Lstat(filepath.Join(h.PluginsDir(), a)); !os.IsNotExist(err) {
			t.Fatal(a, err)
		}
	}
}
func TestAdminUninstallRejectsReviewedDrift(t *testing.T) {
	for _, kind := range []string{"dependency", "provider", "content", "enablement"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			plan, err := PlanUninstall(ctx, h, []string{"a"})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "dependency":
				err = adminstate.WithWrite(ctx, h, "drift", func(s *adminstate.Session) error {
					s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, "a/new-stale")
					return nil
				})
			case "provider":
				_, err = SetEnabled(ctx, h, []string{"keep"}, false)
			case "enablement":
				_, err = SetEnabled(ctx, h, []string{"a"}, false)
			case "content":
				err = os.WriteFile(filepath.Join(h.PluginsDir(), "a", "source", "locator"), []byte("drift"), 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(h.RegistryFile())
			rows, err := Uninstall(ctx, h, plan, true)
			if rows != nil || diag.ExitCode(err) != 53 {
				t.Fatal(rows, err)
			}
			if b, _ := os.ReadFile(h.RegistryFile()); string(b) != string(before) {
				t.Fatal("conflict changed state")
			}
		})
	}
}
func TestAdminUninstallRollbackAndCommittedRecovery(t *testing.T) {
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "move-backup:b", "move-publish:b", "json:config", "json:registry", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			beforeReg, _ := os.ReadFile(h.RegistryFile())
			beforeCfg, _ := os.ReadFile(h.ConfigFile())
			content := map[string]string{}
			for _, a := range []string{"a", "b"} {
				v, _ := os.ReadFile(filepath.Join(h.PluginsDir(), a, "source", "plugin.json"))
				content[a] = string(v)
			}
			plan, err := PlanUninstall(ctx, h, []string{"a", "b"})
			if err != nil {
				t.Fatal(err)
			}
			hit := false
			rows, err := UninstallWithOptions(ctx, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
				if p == point {
					hit = true
					return errors.New("injected")
				}
				return nil
			}})
			if !hit || err == nil {
				t.Fatal(point, hit, err)
			}
			if point == "committed" || point == "stable" || point == "cleanup" {
				if diag.ExitCode(err) != 55 || len(rows) != 2 {
					t.Fatal(rows, err)
				}
				if _, err = adminstate.Preflight(ctx, h); err != nil {
					t.Fatal(err)
				}
				s, err := adminstate.Read(ctx, h)
				if err != nil {
					t.Fatal(err)
				}
				for _, a := range []string{"a", "b"} {
					if _, ok := s.Registry.PackageByAlias(a); ok {
						t.Fatal("resurrected", a)
					}
					if _, err := os.Lstat(filepath.Join(h.PluginsDir(), a)); !os.IsNotExist(err) {
						t.Fatal(a, err)
					}
				}
			} else {
				if rows != nil || diag.ExitCode(err) != 51 {
					t.Fatal(rows, err)
				}
				reg, _ := os.ReadFile(h.RegistryFile())
				cfg, _ := os.ReadFile(h.ConfigFile())
				if string(reg) != string(beforeReg) || string(cfg) != string(beforeCfg) {
					t.Fatal("partial JSON removal")
				}
				for a, want := range content {
					got, _ := os.ReadFile(filepath.Join(h.PluginsDir(), a, "source", "plugin.json"))
					if string(got) != want {
						t.Fatal("partial package removal", a)
					}
				}
			}
		})
	}
}

func TestAdminUninstallCancellationAndMissingRoot(t *testing.T) {
	ctx := context.Background()
	h := uninstallFixture(t)
	if err := os.RemoveAll(filepath.Join(h.PluginsDir(), "b")); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanUninstall(ctx, h, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.RegistryFile())
	cancelled, cancel := context.WithCancel(ctx)
	rows, err := UninstallWithOptions(cancelled, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
		if p == "move-backup:b" {
			cancel()
		}
		return nil
	}})
	if rows != nil || diag.ExitCode(err) != 20 {
		t.Fatal(rows, err)
	}
	if b, _ := os.ReadFile(h.RegistryFile()); string(b) != string(before) {
		t.Fatal("cancelled set changed registry")
	}
	rows, err = Uninstall(ctx, h, plan, true)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
}

func TestAdminUninstallSeedHistoryAtomic(t *testing.T) {
	for _, point := range []string{"json:config", "json:registry", "json:lifecycle", "committed"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := workhome.At(t.TempDir())
			if err := bootstrap.EnsureSeed(h); err != nil {
				t.Fatal(err)
			}
			beforeLife, _ := os.ReadFile(h.ReferencePackageFile())
			beforeReg, _ := os.ReadFile(h.RegistryFile())
			beforeCfg, _ := os.ReadFile(h.ConfigFile())
			plan, err := PlanUninstall(ctx, h, []string{bootstrap.Alias})
			if err != nil {
				t.Fatal(err)
			}
			rows, err := UninstallWithOptions(ctx, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
				if p == point {
					return errors.New("injected")
				}
				return nil
			}})
			if point == "committed" {
				if len(rows) != 1 || diag.ExitCode(err) != 55 {
					t.Fatal(rows, err)
				}
				if _, err := adminstate.Preflight(ctx, h); err != nil {
					t.Fatal(err)
				}
				if err := bootstrap.EnsureSeed(h); err != nil {
					t.Fatal(err)
				}
				s, err := adminstate.Read(ctx, h)
				if err != nil || !s.Lifecycle.Removed || !s.Lifecycle.PolicySeeded || !s.Lifecycle.Initialized {
					t.Fatal(s, err)
				}
				if _, err := os.Lstat(filepath.Join(h.PluginsDir(), bootstrap.Alias)); !os.IsNotExist(err) {
					t.Fatal("committed seed resurrected", err)
				}
			} else {
				if rows != nil || diag.ExitCode(err) != 51 {
					t.Fatal(rows, err)
				}
				life, _ := os.ReadFile(h.ReferencePackageFile())
				reg, _ := os.ReadFile(h.RegistryFile())
				cfg, _ := os.ReadFile(h.ConfigFile())
				if string(life) != string(beforeLife) || string(reg) != string(beforeReg) || string(cfg) != string(beforeCfg) {
					t.Fatal("seed history/policy/registry split")
				}
				if _, err := os.Lstat(filepath.Join(h.PluginsDir(), bootstrap.Alias)); err != nil {
					t.Fatal("seed rollback failed", err)
				}
			}
		})
	}
}

func TestAdminUninstallPreventsStaleInvocation(t *testing.T) {
	ctx := context.Background()
	h := uninstallFixture(t)
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	var selected registry.Component
	for _, c := range s.Registry.Components {
		if c.Alias == "a" {
			selected = c
		}
	}
	plan, err := PlanUninstall(ctx, h, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(ctx, h, plan, true); err != nil {
		t.Fatal(err)
	}
	ran := false
	err = adminstate.WithInvocation(ctx, h, selected, func() error { ran = true; return nil })
	if ran || diag.ExitCode(err) != 53 {
		t.Fatal(ran, err)
	}
}
