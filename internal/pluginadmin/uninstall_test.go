package pluginadmin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

func uninstallFixture(t *testing.T) workhome.Home {
	t.Helper()
	h := installedHome(t)
	setPolicy(t, h, "keep@m1/clone", "a@m1/stale", "ghost@m1/old", "b@m1/clone", "a@m1/clone", "ghost@m1/last", "a@m2/clone")
	return h
}

func ab() []pluginid.Provider { return []pluginid.Provider{prov("a", "m1"), prov("b", "m1")} }

func TestUninstallWholeSetLeavesHomonymsAndSiblings(t *testing.T) {
	ctx := context.Background()
	h := uninstallFixture(t)
	before, _ := os.ReadFile(h.RegistryFile())
	if _, err := PlanUninstall(ctx, h, []pluginid.Provider{prov("a", "m1"), prov("unknown", "m1")}); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(h.RegistryFile()); string(b) != string(before) {
		t.Fatal("an unknown target changed the registry")
	}
	plan, err := PlanUninstall(ctx, h, []pluginid.Provider{prov("b", "m1"), prov("a", "m1"), prov("a", "m1")})
	if err != nil {
		t.Fatal(err)
	}
	if got := ProviderNames(plan.Providers()); !reflect.DeepEqual(got, []string{"a@m1", "b@m1"}) {
		t.Fatalf("providers = %v", got)
	}
	// The homonym a@m2/clone is not a dependency of removing a@m1.
	if want := []string{"a@m1/stale", "b@m1/clone", "a@m1/clone"}; !reflect.DeepEqual(plan.PolicyReferences(), want) {
		t.Fatalf("references = %v", plan.PolicyReferences())
	}
	if rows, err := Uninstall(ctx, h, plan, false); rows != nil || diag.ExitCode(err) != 52 {
		t.Fatalf("without approval: %v %v", rows, err)
	}
	// Unrelated latest state is merged, including policy order and roots.
	if err := adminstate.WithWrite(ctx, h, "unrelated", func(s *adminstate.Session) error { s.Config.RepositoryRoots = []string{h.Root()}; return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := Uninstall(ctx, h, plan, true)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"keep@m1/clone", "ghost@m1/old", "ghost@m1/last", "a@m2/clone"}; !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, want) {
		t.Fatalf("policy = %v", s.Config.RepositoryResolution.Locators)
	}
	if !reflect.DeepEqual(s.Config.RepositoryRoots, []string{h.Root()}) {
		t.Fatal("unrelated roots lost")
	}
	for _, survivor := range []pluginid.Provider{prov("keep", "m1"), prov("a", "m2")} {
		if _, ok := s.Registry.PackageByProvider(survivor); !ok {
			t.Fatalf("%s was removed", survivor)
		}
		if _, ok := s.Registry.Effective().ConventionByRef(pluginid.Ref{Provider: survivor, Name: "shared"}); !ok {
			t.Fatalf("%s lost its convention", survivor)
		}
		if root, _ := h.PackageRoot(survivor); func() bool { _, err := os.Stat(root); return err != nil }() {
			t.Fatalf("%s lost its content", survivor)
		}
	}
	for _, p := range ab() {
		root, _ := h.PackageRoot(p)
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatal(p, err)
		}
	}
}

func TestUninstallReportsConventionEffectsPerProvider(t *testing.T) {
	h := uninstallFixture(t)
	plan, err := PlanUninstall(context.Background(), h, []pluginid.Provider{prov("a", "m1")})
	if err != nil {
		t.Fatal(err)
	}
	effects := plan.ConventionEffects()
	if len(effects) != 1 || effects[0].Name != "a@m1/shared" || !effects[0].AvailableBefore || effects[0].AvailableAfter {
		t.Fatalf("effects = %+v (the homonym's convention must not appear)", effects)
	}
}

func TestUninstallRejectsReviewedDrift(t *testing.T) {
	for _, kind := range []string{"dependency", "content", "enablement"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			plan, err := PlanUninstall(ctx, h, []pluginid.Provider{prov("a", "m1")})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "dependency":
				err = adminstate.WithWrite(ctx, h, "drift", func(s *adminstate.Session) error {
					s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, "a@m1/new-stale")
					return nil
				})
			case "enablement":
				_, err = SetEnabled(ctx, h, []pluginid.Provider{prov("a", "m1")}, false)
			case "content":
				src, _ := h.PackageSource(prov("a", "m1"))
				err = os.WriteFile(filepath.Join(src, "locator"), []byte("drift"), 0o700)
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
				t.Fatal("a conflict changed state")
			}
		})
	}
}

func TestUninstallRollbackAndCommittedRecovery(t *testing.T) {
	for _, point := range []string{"prepared", "pending", "move-backup:a@m1", "move-publish:a@m1", "move-backup:b@m1", "move-publish:b@m1", "json:config", "json:registry", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			beforeReg, _ := os.ReadFile(h.RegistryFile())
			beforeCfg, _ := os.ReadFile(h.ConfigFile())
			content := map[string]string{}
			for _, p := range ab() {
				src, _ := h.PackageSource(p)
				v, _ := os.ReadFile(filepath.Join(src, "plugin.json"))
				content[p.String()] = string(v)
			}
			plan, err := PlanUninstall(ctx, h, ab())
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
				for _, p := range ab() {
					if _, ok := s.Registry.PackageByProvider(p); ok {
						t.Fatal("resurrected", p)
					}
					if root, _ := h.PackageRoot(p); func() bool { _, err := os.Lstat(root); return !os.IsNotExist(err) }() {
						t.Fatal("content survived a committed removal", p)
					}
				}
				return
			}
			if rows != nil || diag.ExitCode(err) != 51 {
				t.Fatal(rows, err)
			}
			reg, _ := os.ReadFile(h.RegistryFile())
			cfg, _ := os.ReadFile(h.ConfigFile())
			if string(reg) != string(beforeReg) || string(cfg) != string(beforeCfg) {
				t.Fatal("partial JSON removal")
			}
			for _, p := range ab() {
				src, _ := h.PackageSource(p)
				if got, _ := os.ReadFile(filepath.Join(src, "plugin.json")); string(got) != content[p.String()] {
					t.Fatal("partial package removal", p)
				}
			}
		})
	}
}

// A killed process bypasses compensation entirely; recovery must restore or
// complete the exact set from the journal alone.
func TestUninstallInterruptedProcessRecovers(t *testing.T) {
	if root := os.Getenv("WORK_TEST_UNINSTALL_HOME"); root != "" {
		h := workhome.At(root)
		plan, err := PlanUninstall(context.Background(), h, ab())
		if err != nil {
			os.Exit(79)
		}
		_, _ = UninstallWithOptions(context.Background(), h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
			if p == os.Getenv("WORK_TEST_UNINSTALL_POINT") {
				os.Exit(77)
			}
			return nil
		}})
		os.Exit(78)
	}
	for _, point := range []string{"pending", "move-backup:a@m1", "move-publish:b@m1", "json:registry", "before-commit", "committed", "stable"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestUninstallInterruptedProcessRecovers$")
			cmd.Env = append(os.Environ(), "WORK_TEST_UNINSTALL_HOME="+h.Root(), "WORK_TEST_UNINSTALL_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 77 {
				t.Fatal(err)
			}
			if point != "stable" {
				if _, err := adminstate.Read(ctx, h); diag.ExitCode(err) != 54 || !strings.Contains(err.Error(), "plugin uninstall") {
					t.Fatalf("pending read = %v", err)
				}
			}
			if _, err := adminstate.Preflight(ctx, h); err != nil {
				t.Fatal(err)
			}
			committed := point == "committed" || point == "stable"
			s, err := adminstate.Read(ctx, h)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range ab() {
				_, ok := s.Registry.PackageByProvider(p)
				root, _ := h.PackageRoot(p)
				_, statErr := os.Stat(root)
				if ok == committed || (statErr == nil) == committed {
					t.Fatalf("%s: registry=%v content=%v committed=%v", p, ok, statErr == nil, committed)
				}
			}
		})
	}
}

func TestUninstallCancellationAndMissingRoot(t *testing.T) {
	ctx := context.Background()
	h := uninstallFixture(t)
	rootB, _ := h.PackageRoot(prov("b", "m1"))
	if err := os.RemoveAll(rootB); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanUninstall(ctx, h, ab())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.RegistryFile())
	cancelled, cancel := context.WithCancel(ctx)
	rows, err := UninstallWithOptions(cancelled, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
		if p == "move-backup:b@m1" {
			cancel()
		}
		return nil
	}})
	if rows != nil || diag.ExitCode(err) != 20 {
		t.Fatal(rows, err)
	}
	if b, _ := os.ReadFile(h.RegistryFile()); string(b) != string(before) {
		t.Fatal("a cancelled set changed the registry")
	}
	rows, err = Uninstall(ctx, h, plan, true)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
}

func TestUninstallingTheIncludedReferenceRecordsDeliberateRemoval(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	if err := adminstate.WithWrite(ctx, h, "seed lifecycle", func(s *adminstate.Session) error {
		s.Lifecycle.Initialized = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	installReference(t, h)
	plan, err := PlanUninstall(ctx, h, []pluginid.Provider{adminstate.ReferenceProvider})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(ctx, h, plan, true); err != nil {
		t.Fatal(err)
	}
	s, _ := adminstate.Read(ctx, h)
	if !s.Lifecycle.Removed || !s.Lifecycle.Initialized {
		t.Fatalf("lifecycle = %+v", s.Lifecycle)
	}
}
