package pluginadmin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminUninstallInterrupt(t *testing.T) {
	if root := os.Getenv("WORK_TEST_UNINSTALL_HOME"); root != "" {
		h := workhome.At(root)
		plan, err := PlanUninstall(context.Background(), h, []string{"a", "b"})
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
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "move-backup:b", "move-publish:b", "json:config", "json:registry", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := uninstallFixture(t)
			if err := adminstate.WithWrite(ctx, h, "memory", func(s *adminstate.Session) error { s.Memory.Set("identity", "team"); return nil }); err != nil {
				t.Fatal(err)
			}
			oldReg, _ := os.ReadFile(h.RegistryFile())
			oldCfg, _ := os.ReadFile(h.ConfigFile())
			oldMemory, _ := os.ReadFile(h.BranchConventionsFile())
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminUninstallInterrupt$")
			cmd.Env = append(os.Environ(), "WORK_TEST_UNINSTALL_HOME="+h.Root(), "WORK_TEST_UNINSTALL_POINT="+point)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 77 {
				t.Fatal(err)
			}
			if point != "prepared" && point != "stable" && point != "cleanup" {
				if _, err := adminstate.Read(ctx, h); diag.ExitCode(err) != 54 || !strings.Contains(err.Error(), "plugin uninstall") {
					t.Fatal(err)
				}
			}
			if _, err := adminstate.Preflight(ctx, h); err != nil {
				t.Fatal(err)
			}
			committed := point == "committed" || point == "stable" || point == "cleanup"
			s, err := adminstate.Read(ctx, h)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range []string{"a", "b"} {
				_, registered := s.Registry.PackageByAlias(a)
				if registered == committed {
					t.Fatal(a, committed, registered)
				}
				_, err := os.Lstat(filepath.Join(h.PluginsDir(), a))
				if committed && !os.IsNotExist(err) || !committed && err != nil {
					t.Fatal(a, err)
				}
			}
			if !committed {
				reg, _ := os.ReadFile(h.RegistryFile())
				cfg, _ := os.ReadFile(h.ConfigFile())
				if string(reg) != string(oldReg) || string(cfg) != string(oldCfg) {
					t.Fatal("interruption lost before-images")
				}
			}
			memory, _ := os.ReadFile(h.BranchConventionsFile())
			if string(memory) != string(oldMemory) {
				t.Fatal("memory changed")
			}
			entries, err := os.ReadDir(h.PluginsDir())
			want := 3
			if committed {
				want = 1
			}
			if err != nil || len(entries) != want {
				t.Fatal("owned staging/backup leaked", entries, err)
			}
		})
	}
}
