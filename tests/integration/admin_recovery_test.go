package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

// recoveryProviders sit in different marketplaces so a journal exercises two
// typed addresses and two namespace directories.
var recoveryProviders = []pluginid.Provider{{Marketplace: "m", Plugin: "a"}, {Marketplace: "n", Plugin: "b"}}

func recoveryReference(name string) repoconv.Reference {
	return repoconv.NewReference(installfx.RegistrationID("m"), pluginid.Ref{Provider: recoveryProviders[0], Name: name})
}

func cumulativeRecoveryWrite(home workhome.Home, checkpoint func(string) error) error {
	return adminstate.WithWriteOptions(context.Background(), home, "work plugin disable a b", func(s *adminstate.Session) error {
		for _, p := range recoveryProviders {
			stage, err := s.NewStage()
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(stage, "after"), []byte(p.Plugin+" after"), 0600); err != nil {
				return err
			}
			if err := s.Replace(p, stage); err != nil {
				return err
			}
			pkg, _ := s.Registry.PackageByProvider(p)
			pkg.Version = "after"
			s.Registry.UpsertPackage(pkg)
		}
		s.Config.Workspace = "after"
		s.Memory.Set("identity", recoveryReference("after"))
		s.Lifecycle.Initialized = true
		s.Lifecycle.PolicyHandled = true
		s.Persist("catalogs")
		return nil
	}, adminstate.WriteOptions{Checkpoint: checkpoint})
}

func cumulativeRecoverySetup(t *testing.T, home workhome.Home) {
	t.Helper()
	if err := home.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	for _, p := range recoveryProviders {
		path, err := home.PackageRoot(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "before"), []byte(p.Plugin+" before"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := adminstate.WithWrite(context.Background(), home, "fixture", func(s *adminstate.Session) error {
		s.Config.Workspace = "before"
		s.Memory.Set("identity", recoveryReference("before"))
		for _, p := range recoveryProviders {
			pkg := installfx.Package(p, true)
			pkg.Version = "before"
			s.Registry.UpsertPackage(pkg)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func cumulativeRecoveryState(t *testing.T, home workhome.Home) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{"plugins": adminTree(t, home.PluginsDir())}
	for name, path := range map[string]string{"config": home.ConfigFile(), "registry": home.RegistryFile(), "memory": home.BranchConventionsFile(), "lifecycle": home.LifecycleFile(), "catalogs": home.CatalogsFile()} {
		out[name] = adminTree(t, path)
	}
	return out
}

// Every package move and all four JSON publications are interrupted by a real
// process exit. Recovery is reached through valid public mutating grammar,
// before the deliberately unknown target is resolved.
func TestAdminRecoveryCumulativeInterruptions(t *testing.T) {
	if root := os.Getenv("WORK_TEST_CUMULATIVE_HOME"); root != "" {
		_ = cumulativeRecoveryWrite(workhome.At(root), func(point string) error {
			if point == os.Getenv("WORK_TEST_CUMULATIVE_POINT") {
				os.Exit(77)
			}
			return nil
		})
		os.Exit(78)
	}
	h := newAdminHarness(t)
	expectedHome := workhome.At(filepath.Join(t.TempDir(), "expected"))
	cumulativeRecoverySetup(t, expectedHome)
	before := cumulativeRecoveryState(t, expectedHome)
	if err := cumulativeRecoveryWrite(expectedHome, nil); err != nil {
		t.Fatal(err)
	}
	after := cumulativeRecoveryState(t, expectedHome)
	for _, point := range []string{"prepared", "pending", "move-backup:a@m", "move-publish:a@m", "move-backup:b@n", "move-publish:b@n", "json:config", "json:registry", "json:memory", "json:lifecycle", "json:catalogs", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			home := workhome.At(adminHome(h))
			if err := os.RemoveAll(home.Root()); err != nil {
				t.Fatal(err)
			}
			cumulativeRecoverySetup(t, home)
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminRecoveryCumulativeInterruptions$")
			cmd.Env = append(os.Environ(), "WORK_TEST_CUMULATIVE_HOME="+home.Root(), "WORK_TEST_CUMULATIVE_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 77 {
				t.Fatal(err)
			}
			interrupted := adminTree(t, home.Root())
			if point != "prepared" && point != "stable" && point != "cleanup" {
				out, ui := h.run(t, h.root, 54, "plugin", "list", "--json")
				if out != "" || !strings.Contains(ui, "work plugin disable") {
					t.Fatal(out, ui)
				}
			}
			h.run(t, h.root, 2, "plugin", "remove", "a")
			if !reflect.DeepEqual(interrupted, adminTree(t, home.Root())) {
				t.Fatal("read or invalid confirmation recovered state")
			}
			out, ui := h.run(t, h.root, 2, "plugin", "enable", "missing@m")
			if out != "" || !strings.Contains(ui, "recovered interrupted administrative operation") {
				t.Fatal(out, ui)
			}
			want := before
			if point == "committed" || point == "stable" || point == "cleanup" {
				want = after
			}
			if got := cumulativeRecoveryState(t, home); !reflect.DeepEqual(got, want) {
				t.Fatalf("recovered bytes/existence differ at %s\ngot %#v\nwant %#v", point, got, want)
			}
			entries, err := os.ReadDir(home.AdministrationDir())
			if err != nil || len(entries) != 0 {
				t.Fatal("recovery retained owned journal", entries, err)
			}
		})
	}
}

func TestAdminRecoveryReadAndStatus(t *testing.T) {
	h := newF6Harness(t)
	home := filepath.Join(h.root, "home")
	path := filepath.Join(home, "state", "administration.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"epoch":99,"status":"pending","transaction_id":"missing-evidence"}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, home)
	for _, args := range [][]string{{"plugin", "list", "--json"}, {"repository", "policy", "list", "--json"}} {
		out, _ := h.run(t, h.root, 54, args...)
		if out != "" {
			t.Fatal("partial mixed output")
		}
	}
	h.run(t, h.dir, 0, "status", "--json")
	// Invalid option grammar must not enter recovery at all.
	h.run(t, h.root, 2, "plugin", "marketplace", "add", filepath.Join(h.root, "dir"), "--branch", "main")
	h.run(t, h.root, 2, "plugin", "marketplace", "add")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("read recovered or changed pending state")
	}
	// A valid mutating invocation reaches recovery before source lookup.
	h.run(t, h.root, 54, "plugin", "marketplace", "add", filepath.Join(h.root, "missing-source"))
	// A damaged intermediate config cannot hide recovery from a fully
	// specified start invocation.
	if err := os.WriteFile(filepath.Join(home, "config", "work.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 54, "start", filepath.Join(h.root, "demo"), "--workspace", filepath.Join(h.root, "ws"), "--base", "main", "--slug", "recovery", "--prefix", "{slug}", "--yes")
}
