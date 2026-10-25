package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminUninstallLinkedAndWorkRetention(t *testing.T) {
	h := newF6Harness(t)
	src := fixtures.Prepare(t, "admin-suite")
	if runtime.GOOS == "windows" {
		if err := os.Symlink(src, filepath.Join(t.TempDir(), "capability")); err != nil {
			t.Skipf("Windows symlink capability unavailable: %v", err)
		}
	}
	h.run(t, h.root, 0, "plugin", "install", src, "--link", "--as", "linked")
	h.run(t, h.root, 0, "plugin", "install", src, "--as", "pinned")
	h.run(t, h.root, 0, "plugin", "disable", "linked")
	home := workhome.At(filepath.Join(h.root, "home"))
	if err := adminstate.WithWrite(context.Background(), home, "stale policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, "linked/stale", "pinned/clone")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.run(t, filepath.Join(h.root, "demo"), 0, "convention", "set", "admin-team")
	memory, _ := os.ReadFile(home.BranchConventionsFile())
	workBefore := adminTree(t, h.dir)
	sourceBefore := adminTree(t, src)
	before := adminTree(t, home.Root())
	for _, args := range [][]string{{"linked", "pinned"}, {"linked", "missing", "--yes"}, {"linked", "pinned", "--yes"}} {
		code := 2
		if len(args) == 3 && args[1] == "pinned" {
			code = 52
		}
		out, _ := h.run(t, h.root, code, append([]string{"plugin", "uninstall"}, args...)...)
		if out != "" {
			t.Fatal("premature receipt", out)
		}
		if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
			t.Fatal("refusal changed home")
		}
	}
	out, _ := h.run(t, h.root, 0, "plugin", "uninstall", "pinned", "linked", "linked", "--yes", "--remove-policy-references")
	if out != "work: uninstalled linked\nwork: uninstalled pinned\n" {
		t.Fatal(out)
	}
	if !reflect.DeepEqual(workBefore, adminTree(t, h.dir)) || !reflect.DeepEqual(sourceBefore, adminTree(t, src)) {
		t.Fatal("uninstall changed Work or linked source")
	}
	if b, _ := os.ReadFile(home.BranchConventionsFile()); string(b) != string(memory) {
		t.Fatal("uninstall cleared convention memory")
	}
	s, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range s.Config.RepositoryResolution.Locators {
		if strings.HasPrefix(ref, "linked/") || strings.HasPrefix(ref, "pinned/") {
			t.Fatal("owned stale dependency retained", ref)
		}
	}
	for _, a := range []string{"linked", "pinned"} {
		if _, err := os.Lstat(filepath.Join(home.PluginsDir(), a)); !os.IsNotExist(err) {
			t.Fatal(a, err)
		}
	}
}

func TestAdminLinkedUninstallAliasSymlink(t *testing.T) {
	h := newAdminHarness(t)
	src := fixtures.Prepare(t, "admin-suite")
	if runtime.GOOS == "windows" {
		if err := os.Symlink(src, filepath.Join(t.TempDir(), "capability")); err != nil {
			t.Skipf("Windows symlink capability unavailable: %v", err)
		}
	}
	h.run(t, h.root, 0, "plugin", "install", src, "--link", "--as", "linked")
	home := workhome.At(adminHome(h))
	root := filepath.Join(home.PluginsDir(), "linked")
	// Legacy linked ownership can be a root symlink rather than a wrapper
	// directory containing a source link. Removal must unlink either shape.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, root); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, src)
	h.run(t, h.root, 0, "plugin", "uninstall", "linked", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, src)) {
		t.Fatal("followed linked alias root")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestAdminUninstallNativeRecovery(t *testing.T) {
	if root := os.Getenv("WORK_TEST_REMOVE_HOME"); root != "" {
		home := workhome.At(root)
		ctx := context.Background()
		plan, err := pluginadmin.PlanUninstall(ctx, home, []string{"a", "b"})
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
			home := workhome.At(adminHome(h))
			source := fixtures.Prepare(t, "admin-suite")
			for _, a := range []string{"a", "b"} {
				h.run(t, h.root, 0, "plugin", "install", source, "--as", a)
				h.run(t, h.root, 0, "repository", "policy", "add", a+"/clone")
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
			if out != "" || !strings.Contains(ui, "plugin uninstall") {
				t.Fatal(out, ui)
			}
			h.run(t, h.root, 2, "plugin", "uninstall", "a", "b")
			if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
				t.Fatal("pure read or invalid grammar recovered state")
			}
			want := 0
			if point == "committed" {
				want = 2
			}
			out, ui = h.run(t, h.root, want, "plugin", "uninstall", "a", "b", "--yes", "--remove-policy-references")
			if !strings.Contains(ui, "recovered interrupted administrative operation") {
				t.Fatal(ui)
			}
			if want == 0 && out != "work: uninstalled a\nwork: uninstalled b\n" || want == 2 && out != "" {
				t.Fatal(out)
			}
			s, err := adminstate.Read(context.Background(), home)
			if err != nil || len(s.Registry.Packages) != 0 || len(s.Config.RepositoryResolution.Locators) != 0 {
				t.Fatal(s, err)
			}
		})
	}
}
