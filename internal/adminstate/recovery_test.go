package adminstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func testPackage(alias string) registry.Package {
	return registry.Package{Alias: alias, Origin: registry.OriginLocalPinned, Reference: "fixture"}
}

// A real process termination bypasses compensation and defers.
func TestAdminInterrupt(t *testing.T) {
	if root := os.Getenv("WORK_TEST_ADMIN_HOME"); root != "" {
		point := os.Getenv("WORK_TEST_ADMIN_POINT")
		h := workhome.At(root)
		_ = withWrite(context.Background(), h, "work plugin enable a", func(s *Session) error { s.Config.Workspace = "after"; return nil }, func(p string) error {
			if p == point {
				os.Exit(77)
			}
			return nil
		})
		os.Exit(78)
	}
	for _, point := range []string{"prepared", "pending", "json:config", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(filepath.Join(t.TempDir(), "home"))
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminInterrupt$")
			cmd.Env = append(os.Environ(), "WORK_TEST_ADMIN_HOME="+h.Root(), "WORK_TEST_ADMIN_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			}
			_, err := Read(context.Background(), h)
			if point == "pending" || point == "json:config" || point == "committed" {
				if diag.ExitCode(err) != 54 {
					t.Fatalf("pending read: %v", err)
				}
			}
			if _, err := Preflight(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			s, err := Read(context.Background(), h)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if point == "committed" || point == "stable" || point == "cleanup" {
				want = "after"
			}
			if s.Config.Workspace != want {
				t.Fatalf("got %q want %q", s.Config.Workspace, want)
			}
		})
	}
}
func TestAdminUnsafeRecovery(t *testing.T) {
	h := workhome.At(t.TempDir())
	if err := os.MkdirAll(h.StateDir(), 0700); err != nil {
		t.Fatal(err)
	}
	b := []byte(`{"schema":1,"epoch":1,"status":"pending","transaction_id":"missing"}`)
	if err := os.WriteFile(h.AdministrationFile(), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Preflight(context.Background(), h); diag.ExitCode(err) != 54 {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(h.AdministrationFile())
	if string(after) != string(b) {
		t.Fatal("unsafe evidence rewritten")
	}
}

func TestAdminInterruptPackageMoves(t *testing.T) {
	if root := os.Getenv("WORK_TEST_ADMIN_MOVE_HOME"); root != "" {
		h := workhome.At(root)
		point := os.Getenv("WORK_TEST_ADMIN_POINT")
		_ = withWrite(context.Background(), h, "work plugin disable a b", func(s *Session) error {
			for _, alias := range []string{"a", "b"} {
				stage, err := os.MkdirTemp(h.PluginsDir(), ".admin-stage-")
				if err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(stage, "new"), []byte(alias+"after"), 0600); err != nil {
					return err
				}
				if err = s.Replace(alias, stage); err != nil {
					return err
				}
				s.Registry.UpsertPackage(testPackage(alias))
			}
			s.Config.Workspace = "after"
			s.Memory.Set("identity", "after")
			s.Lifecycle.Initialized = true
			return nil
		}, func(p string) error {
			if p == point {
				os.Exit(77)
			}
			return nil
		})
		os.Exit(78)
	}
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "move-backup:b", "move-publish:b", "json:config", "json:registry", "json:memory", "json:lifecycle", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			if err := h.EnsureLayout(); err != nil {
				t.Fatal(err)
			}
			for _, alias := range []string{"a", "b"} {
				dest := filepath.Join(h.PluginsDir(), alias)
				if err := os.MkdirAll(dest, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dest, "old"), []byte(alias+"before"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminInterruptPackageMoves$")
			cmd.Env = append(os.Environ(), "WORK_TEST_ADMIN_MOVE_HOME="+h.Root(), "WORK_TEST_ADMIN_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			}
			if _, err := Preflight(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			committed := point == "committed" || point == "stable" || point == "cleanup"
			for _, alias := range []string{"a", "b"} {
				name, want := "old", alias+"before"
				if committed {
					name, want = "new", alias+"after"
				}
				b, err := os.ReadFile(filepath.Join(h.PluginsDir(), alias, name))
				if err != nil || string(b) != want {
					t.Fatalf("package %s: %q %v", alias, b, err)
				}
			}
			s, err := Read(context.Background(), h)
			if err != nil {
				t.Fatal(err)
			}
			if (s.Config.Workspace == "after") != committed || (len(s.Registry.Packages) == 2) != committed || s.Lifecycle.Initialized != committed {
				t.Fatal("partial set")
			}
			entries, err := os.ReadDir(h.PluginsDir())
			if err != nil || len(entries) != 2 {
				t.Fatalf("uncollected package evidence: %v %v", entries, err)
			}
		})
	}
}
