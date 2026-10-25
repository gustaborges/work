package pluginadmin

import (
	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/workhome"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAdminUpdateInterrupt(t *testing.T) {
	if root := os.Getenv("WORK_TEST_UPDATE_HOME"); root != "" {
		h := workhome.At(root)
		plan, err := PrepareUpdates(context.Background(), h, []string{"a"}, false)
		if err != nil {
			os.Exit(79)
		}
		_, _ = ApplyWithOptions(context.Background(), h, plan, true, adminstate.WriteOptions{Checkpoint: func(point string) error {
			if point == os.Getenv("WORK_TEST_UPDATE_POINT") {
				os.Exit(77)
			}
			return nil
		}})
		os.Exit(78)
	}
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "json:registry", "json:config", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			h := workhome.At(t.TempDir())
			src := updateSource(t, "one")
			installUpdateFixture(t, h, src, "a")
			if err := adminstate.WithWrite(ctx, h, "policy", func(s *adminstate.Session) error {
				s.Config.RepositoryResolution.Locators = []string{"a/clone"}
				s.Memory.Set("identity", "team")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			oldRegistry, _ := os.ReadFile(h.RegistryFile())
			oldConfig, _ := os.ReadFile(h.ConfigFile())
			oldMemory, _ := os.ReadFile(h.BranchConventionsFile())
			oldContent, _ := os.ReadFile(filepath.Join(h.PluginsDir(), "a", "source", "plugin.json"))
			writeVersion(t, src, "two", true)
			temp := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestAdminUpdateInterrupt$")
			cmd.Env = append(os.Environ(), "WORK_TEST_UPDATE_HOME="+h.Root(), "WORK_TEST_UPDATE_POINT="+point, "TMPDIR="+temp, "TMP="+temp, "TEMP="+temp)
			if err := cmd.Run(); err == nil {
				t.Fatal("helper survived")
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 77 {
				t.Fatal(err)
			}
			if _, err := adminstate.Preflight(ctx, h); err != nil {
				t.Fatal(err)
			}
			committed := point == "committed" || point == "stable" || point == "cleanup"
			s, err := adminstate.Read(ctx, h)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := s.Registry.PackageByAlias("a")
			if committed {
				if p.Version != "two" || len(s.Config.RepositoryResolution.Locators) != 0 {
					t.Fatal(p, s.Config)
				}
			} else {
				reg, _ := os.ReadFile(h.RegistryFile())
				cfg, _ := os.ReadFile(h.ConfigFile())
				content, _ := os.ReadFile(filepath.Join(h.PluginsDir(), "a", "source", "plugin.json"))
				if string(reg) != string(oldRegistry) || string(cfg) != string(oldConfig) || string(content) != string(oldContent) {
					t.Fatal("interruption lost before-state")
				}
			}
			mem, _ := os.ReadFile(h.BranchConventionsFile())
			if string(mem) != string(oldMemory) {
				t.Fatal("update rewrote memory")
			}
			entries, err := os.ReadDir(h.PluginsDir())
			if err != nil || len(entries) != 1 {
				t.Fatal("stage or backup leaked", entries, err)
			}
		})
	}
}
