package adminstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminRollbackBoundaries(t *testing.T) {
	for _, point := range []string{"prepared", "pending", "move-backup:a", "move-publish:a", "json:config", "json:registry", "before-commit"} {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			if err := h.EnsureLayout(); err != nil {
				t.Fatal(err)
			}
			cfg := config.Default()
			cfg.Workspace = "old"
			if err := config.Save(h.ConfigFile(), cfg); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(h.ConfigFile())
			dest := filepath.Join(h.PluginsDir(), "a")
			if err := os.MkdirAll(dest, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dest, "old"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			err := withWrite(context.Background(), h, "work plugin install fixture", func(s *Session) error {
				stage, e := os.MkdirTemp(h.PluginsDir(), ".admin-stage-")
				if e != nil {
					return e
				}
				if e = os.WriteFile(filepath.Join(stage, "new"), []byte("new"), 0600); e != nil {
					return e
				}
				if e = s.Replace("a", stage); e != nil {
					return e
				}
				s.Config.Workspace = "new"
				s.Registry.UpsertPackage(testPackage("a"))
				return nil
			}, func(p string) error {
				if p == point {
					return errors.New("injected")
				}
				return nil
			})
			if err == nil {
				t.Fatal("missing fault")
			}
			after, _ := os.ReadFile(h.ConfigFile())
			if !reflect.DeepEqual(before, after) {
				t.Fatal("config changed")
			}
			if b, e := os.ReadFile(filepath.Join(dest, "old")); e != nil || string(b) != "old" {
				t.Fatalf("old lost: %s %v", b, e)
			}
			if _, e := os.Stat(h.RegistryFile()); !os.IsNotExist(e) {
				t.Fatal("absent before image lost")
			}
			if _, e := Read(context.Background(), h); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestAdminLatestNoopAndCancel(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	if err := WithWrite(ctx, h, "setup", func(s *Session) error { s.Config.Workspace = "one"; return nil }); err != nil {
		t.Fatal(err)
	}
	a, _ := Read(ctx, h)
	if err := WithWrite(ctx, h, "noop", func(s *Session) error { return nil }); err != nil {
		t.Fatal(err)
	}
	b, _ := Read(ctx, h)
	if b.Epoch != a.Epoch {
		t.Fatal("noop published")
	}
	c, cancel := context.WithCancel(ctx)
	err := withWrite(c, h, "cancel", func(s *Session) error { s.Config.Workspace = "two"; return nil }, func(p string) error {
		if p == "json:config" {
			cancel()
		}
		return nil
	})
	if diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
	b, _ = Read(ctx, h)
	if b.Config.Workspace != "one" {
		t.Fatal("cancel changed config")
	}
}

func TestAdminCommittedCleanupAndCancellation(t *testing.T) {
	for _, point := range []string{"committed", "stable", "cleanup"} {
		t.Run(point, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := withWrite(ctx, h, "work plugin enable a", func(s *Session) error { s.Config.Workspace = "committed"; return nil }, func(p string) error {
				if p == point {
					cancel()
					return errors.New("cleanup fault")
				}
				return nil
			})
			if diag.ExitCode(err) != 55 {
				t.Fatal(err)
			}
			if _, err := Preflight(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			s, err := Read(context.Background(), h)
			if err != nil || s.Config.Workspace != "committed" {
				t.Fatal(s, err)
			}
		})
	}
}
func TestAdminFailedCompensationRetainsEvidence(t *testing.T) {
	h := workhome.At(t.TempDir())
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(h.PluginsDir(), "a")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "old"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	err := withWrite(context.Background(), h, "work plugin enable a", func(s *Session) error {
		stage, err := os.MkdirTemp(h.PluginsDir(), ".admin-stage-")
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(stage, "new"), []byte("new"), 0600); err != nil {
			return err
		}
		return s.Replace("a", stage)
	}, func(p string) error {
		if p != "move-backup:a" {
			return nil
		}
		c, err := loadControl(h)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(backupPath(h, *c.TransactionID, "a"), "old"), []byte("corrupt evidence"), 0600); err != nil {
			return err
		}
		return errors.New("injected")
	})
	if diag.ExitCode(err) != 54 {
		t.Fatal(err)
	}
	c, err := loadControl(h)
	if err != nil || c.Status != "pending" {
		t.Fatal(c, err)
	}
	j, err := loadJournal(h, *c.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(h.PluginsDir(), j.Moves[0].Stage)) {
		t.Fatal("discarded unresolved staged evidence")
	}
	if _, err = Preflight(context.Background(), h); diag.ExitCode(err) != 54 {
		t.Fatal(err)
	}
}
func TestAdminStableOldJournalCannotOverwriteNewer(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	var saved []byte
	var id string
	err := withWrite(ctx, h, "work plugin enable a", func(s *Session) error { s.Config.Workspace = "old-commit"; return nil }, func(p string) error {
		if p != "stable" {
			return nil
		}
		entries, err := os.ReadDir(h.AdministrationDir())
		if err != nil {
			return err
		}
		id = entries[0].Name()
		dir, _ := h.TransactionDir(id)
		saved, err = os.ReadFile(filepath.Join(dir, "journal.json"))
		if err != nil {
			return err
		}
		return errors.New("leave committed cleanup")
	})
	if diag.ExitCode(err) != 55 {
		t.Fatal(err)
	}
	if err = WithWrite(ctx, h, "newer", func(s *Session) error { s.Config.Workspace = "newer"; return nil }); err != nil {
		t.Fatal(err)
	}
	dir, _ := h.TransactionDir(id)
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "journal.json"), saved, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Preflight(ctx, h); err != nil {
		t.Fatal(err)
	}
	s, err := Read(ctx, h)
	if err != nil || s.Config.Workspace != "newer" {
		t.Fatal(s, err)
	}
}
