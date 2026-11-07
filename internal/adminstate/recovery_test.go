package adminstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
)

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
			for _, p := range []pluginid.Provider{prov("a", "m"), prov("b", "n")} {
				stage, err := s.NewStage()
				if err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(stage, "new"), []byte(p.Plugin+"after"), 0600); err != nil {
					return err
				}
				if err = s.Replace(p, stage); err != nil {
					return err
				}
				s.Registry.UpsertPackage(testPackage(p))
			}
			s.Config.Workspace = "after"
			s.Memory.Set("identity", repoconv.NewReference(pluginid.BuiltinRegistrationID, pluginid.Ref{Provider: prov("a", "builtin"), Name: "after"}))
			s.Lifecycle.Initialized = true
			s.Catalogs.Catalogs = s.Catalogs.Catalogs[:0]
			s.Persist("catalogs")
			return nil
		}, func(p string) error {
			if p == point {
				os.Exit(77)
			}
			return nil
		})
		os.Exit(78)
	}
	for _, point := range []string{"prepared", "pending", "move-backup:a@m", "move-publish:a@m", "move-backup:b@n", "move-publish:b@n", "json:config", "json:registry", "json:memory", "json:lifecycle", "json:catalogs", "before-commit", "committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			if err := h.EnsureLayout(); err != nil {
				t.Fatal(err)
			}
			for _, p := range []pluginid.Provider{prov("a", "m"), prov("b", "n")} {
				seedRoot(t, h, p, "old", p.Plugin+"before")
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
			for _, p := range []pluginid.Provider{prov("a", "m"), prov("b", "n")} {
				name, want := "old", p.Plugin+"before"
				if committed {
					name, want = "new", p.Plugin+"after"
				}
				root, _ := h.PackageRoot(p)
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(b) != want {
					t.Fatalf("package %s: %q %v", p, b, err)
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
			for _, e := range entries {
				if e.Name() != "m" && e.Name() != "n" {
					t.Fatalf("stray entry %q", e.Name())
				}
			}
		})
	}
}

func TestAdminRecoveryUnsafeRemovalJournal(t *testing.T) {
	good := prov("a", "m")
	for name, m := range map[string]Move{
		"path-shaped plugin":      {Provider: pluginid.Provider{Marketplace: "m", Plugin: "../outside"}, Remove: true},
		"path-shaped marketplace": {Provider: pluginid.Provider{Marketplace: "../x", Plugin: "a"}, Remove: true},
		"empty address":           {Remove: true},
		"stage on removal":        {Provider: good, Remove: true, Stage: ".admin-stage-owned"},
		"after digest on removal": {Provider: good, Remove: true, AfterDigest: "unexpected"},
		"exists without digest":   {Provider: good, Remove: true, BeforeExists: true},
		"digest without exists":   {Provider: good, Remove: true, BeforeDigest: "unexpected"},
		"foreign stage":           {Provider: good, Stage: "../escape", AfterDigest: "d"},
		"unowned stage name":      {Provider: good, Stage: "plain", AfterDigest: "d"},
		"replacement no digest":   {Provider: good, Stage: ".admin-stage-x"},
	} {
		t.Run(name, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			id := "0123456789abcdef0123456789abcdef"
			dir, err := h.TransactionDir(id)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			j := Journal{Schema: 1, ID: id, Operation: "work plugin remove a@m --yes", Phase: "prepared", Moves: []Move{m}}
			if err := saveJSON(filepath.Join(dir, "journal.json"), j); err != nil {
				t.Fatal(err)
			}
			if _, err := loadJournal(h, id); diag.ExitCode(err) != 54 {
				t.Fatal(m, err)
			}
		})
	}
}
