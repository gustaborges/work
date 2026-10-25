package contract

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminUninstallResultAndDiagnosticsContract(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	if err := adminstate.WithWrite(ctx, h, "fixture", func(s *adminstate.Session) error {
		for _, a := range []string{"b", "a"} {
			s.Registry.UpsertPackage(registry.Package{Alias: a, Origin: registry.OriginLocalLinked})
		}
		s.Config.RepositoryResolution.Locators = []string{"a/stale", "keep/first", "b/stale", "keep/last"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pluginadmin.PlanUninstall(ctx, h, []string{"a/clone"}); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	plan, err := pluginadmin.PlanUninstall(ctx, h, []string{"b", "a", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Aliases(), []string{"a", "b"}) || !reflect.DeepEqual(plan.PolicyReferences(), []string{"a/stale", "b/stale"}) {
		t.Fatal(plan)
	}
	// Returned review views cannot authorize additional dependencies or targets.
	plan.Aliases()[0] = "keep"
	plan.PolicyReferences()[0] = "keep/first"
	if rows, err := pluginadmin.Uninstall(ctx, h, plan, false); rows != nil || diag.ExitCode(err) != 52 {
		t.Fatal(rows, err)
	}
	rows, err := pluginadmin.UninstallWithOptions(ctx, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
		if p == "json:registry" {
			return errors.New("private source credential")
		}
		return nil
	}})
	if rows != nil || diag.ExitCode(err) != 51 || diag.Token(err) != "plugin-uninstall-failed" {
		t.Fatal(rows, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if rows, err := pluginadmin.Uninstall(cancelled, h, plan, true); rows != nil || diag.ExitCode(err) != 20 {
		t.Fatal(rows, err)
	}
	rows, err = pluginadmin.UninstallWithOptions(ctx, h, plan, true, adminstate.WriteOptions{Checkpoint: func(p string) error {
		if p == "committed" {
			return errors.New("cleanup")
		}
		return nil
	}})
	if !reflect.DeepEqual(rows, []string{"a", "b"}) || diag.ExitCode(err) != 55 {
		t.Fatal(rows, err)
	}
	if _, err := adminstate.Read(ctx, h); diag.ExitCode(err) != 54 {
		t.Fatal(err)
	}
	if _, err := adminstate.Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil || len(s.Registry.Packages) != 0 || !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, []string{"keep/first", "keep/last"}) {
		t.Fatal(s, err)
	}
	for _, c := range []diag.Category{diag.PluginUninstallFailed, diag.PluginPolicyDependency, diag.AdministrationConflict, diag.AdministrationRecoveryRequired, diag.AdministrationCleanupFailed, diag.AdministrationBusy} {
		if diag.ExitCode(diag.New(c, "detail")) != c.Code {
			t.Fatal(c)
		}
	}
}
