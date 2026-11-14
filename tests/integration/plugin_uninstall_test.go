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
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminUninstallHomonymsAndWorkRetention(t *testing.T) {
	h := newF6Harness(t)
	src := fixtures.Prepare(t, "admin-suite")
	h.install(t, "one", src)
	h.install(t, "two", src)
	h.run(t, h.root, 0, "plugin", "disable", "admin-suite@one")
	home := h.home()
	if err := adminstate.WithWrite(context.Background(), home, "stale policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, "admin-suite@one/stale", "admin-suite@two/clone")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", "admin-suite@two/admin-team")
	memory, _ := os.ReadFile(home.BranchConventionsFile())
	workBefore := adminTree(t, h.dir)
	before := adminTree(t, home.Root())
	// Removing only "one" never touches its homonym "two".
	for _, args := range [][]string{{"admin-suite@one"}, {"admin-suite@one", "missing@one", "--yes"}, {"one", "--yes"}, {"admin-suite@one", "admin-suite@two", "--yes"}} {
		code := 2
		if len(args) == 3 && args[1] == "admin-suite@two" {
			code = 52
		}
		out, _ := h.run(t, h.root, code, append([]string{"plugin", "remove"}, args...)...)
		if out != "" {
			t.Fatal("premature receipt", out)
		}
		if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
			t.Fatal("refusal changed home")
		}
	}
	out, _ := h.run(t, h.root, 0, "plugin", "remove", "admin-suite@two", "admin-suite@one", "admin-suite@one", "--yes", "--remove-policy-references")
	if out != "work: removed admin-suite@one\nwork: removed admin-suite@two\n" {
		t.Fatal(out)
	}
	if !reflect.DeepEqual(workBefore, adminTree(t, h.dir)) {
		t.Fatal("uninstall changed a Work")
	}
	if b, _ := os.ReadFile(home.BranchConventionsFile()); string(b) != string(memory) {
		t.Fatal("uninstall cleared convention memory")
	}
	s, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range s.Config.RepositoryResolution.Locators {
		if strings.HasPrefix(ref, "admin-suite@") {
			t.Fatal("owned dependency retained", ref)
		}
	}
	for _, mp := range []string{"one", "two"} {
		if _, err := os.Lstat(filepath.Join(home.PluginsDir(), mp)); !os.IsNotExist(err) {
			t.Fatalf("emptied namespace %q was not pruned: %v", mp, err)
		}
	}
	// The reference package and its namespace are untouched.
	if _, err := os.Stat(filepath.Join(home.PluginsDir(), "builtin", "work-reference", "source", "plugin.json")); err != nil {
		t.Fatal(err)
	}
}

var removeProviders = []pluginid.Provider{{Marketplace: "a", Plugin: "admin-suite"}, {Marketplace: "b", Plugin: "admin-suite"}}

func TestAdminUninstallNativeRecovery(t *testing.T) {
	if root := os.Getenv("WORK_TEST_REMOVE_HOME"); root != "" {
		home := workhome.At(root)
		ctx := context.Background()
		plan, err := pluginadmin.PlanUninstall(ctx, home, removeProviders)
		if err != nil {
			os.Exit(79)
		}
		_, _ = pluginadmin.UninstallWithOptions(ctx, home, plan, true, adminstate.WriteOptions{Checkpoint: func(point string) error {
			if point == os.Getenv("WORK_TEST_REMOVE_POINT") {
				os.Exit(77)
			}
			return nil
		}})
		os.Exit(78)
	}
	for _, point := range []string{"json:registry", "committed"} {
		t.Run(point, func(t *testing.T) {
			h := newAdminHarness(t)
			home := h.home()
			source := fixtures.Prepare(t, "admin-suite")
			for _, p := range removeProviders {
				h.install(t, p.Marketplace, source)
				h.run(t, h.root, 0, "repository", "policy", "add", p.String()+"/clone")
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminUninstallNativeRecovery$")
			cmd.Env = append(os.Environ(), "WORK_TEST_REMOVE_HOME="+home.Root(), "WORK_TEST_REMOVE_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 77 {
				t.Fatal(err)
			}
			before := adminTree(t, home.Root())
			out, ui := h.run(t, h.root, 54, "plugin", "list", "--json")
			if out != "" || !strings.Contains(ui, "plugin remove") {
				t.Fatal(out, ui)
			}
			h.run(t, h.root, 2, "plugin", "remove", "admin-suite@a", "admin-suite")
			if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
				t.Fatal("pure read or invalid grammar recovered state")
			}
			want := 0
			if point == "committed" {
				want = 2
			}
			out, ui = h.run(t, h.root, want, "plugin", "remove", "admin-suite@a", "admin-suite@b", "--yes", "--remove-policy-references")
			if !strings.Contains(ui, "recovered interrupted administrative operation") {
				t.Fatal(ui)
			}
			if want == 0 && out != "work: removed admin-suite@a\nwork: removed admin-suite@b\n" || want == 2 && out != "" {
				t.Fatal(out)
			}
			s, err := adminstate.Read(context.Background(), home)
			if err != nil || len(s.Registry.Packages) != 0 || len(s.Config.RepositoryResolution.Locators) != 0 {
				t.Fatal(s, err)
			}
		})
	}
}
