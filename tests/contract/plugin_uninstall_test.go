package contract

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func TestUninstallResultAndDiagnosticsContract(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(t.TempDir())
	if err := adminstate.WithWrite(ctx, h, "fixture", func(s *adminstate.Session) error {
		for _, a := range []string{"b", "a"} {
			s.Registry.UpsertPackage(installfx.Package(installfx.P(a), true))
		}
		s.Config.RepositoryResolution.Locators = []string{"a@m/stale", "keep@m/first", "b@m/stale", "keep@m/last"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a, b := installfx.P("a"), installfx.P("b")
	if _, err := pluginadmin.PlanUninstall(ctx, h, []pluginid.Provider{{Plugin: "a/clone", Marketplace: "m"}}); diag.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	plan, err := pluginadmin.PlanUninstall(ctx, h, []pluginid.Provider{b, a, a})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Providers(), []pluginid.Provider{a, b}) || !reflect.DeepEqual(plan.PolicyReferences(), []string{"a@m/stale", "b@m/stale"}) {
		t.Fatal(plan)
	}
	// Returned review views cannot authorize additional dependencies or targets.
	plan.Providers()[0] = installfx.P("keep")
	plan.PolicyReferences()[0] = "keep@m/first"
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
	if !reflect.DeepEqual(rows, []pluginid.Provider{a, b}) || diag.ExitCode(err) != 55 {
		t.Fatal(rows, err)
	}
	if _, err := adminstate.Read(ctx, h); diag.ExitCode(err) != 54 {
		t.Fatal(err)
	}
	if _, err := adminstate.Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil || len(s.Registry.Packages) != 0 || !reflect.DeepEqual(s.Config.RepositoryResolution.Locators, []string{"keep@m/first", "keep@m/last"}) {
		t.Fatal(s, err)
	}
	for _, c := range []diag.Category{diag.PluginUninstallFailed, diag.PluginPolicyDependency, diag.AdministrationConflict, diag.AdministrationRecoveryRequired, diag.AdministrationCleanupFailed, diag.AdministrationBusy} {
		if diag.ExitCode(diag.New(c, "detail")) != c.Code {
			t.Fatal(c)
		}
	}
}
