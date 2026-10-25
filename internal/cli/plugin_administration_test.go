package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminToggleReceiptsAndGrammar(t *testing.T) {
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	err := adminstate.WithWrite(context.Background(), h, "fixture", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(registry.Package{Alias: "z"})
		s.Registry.UpsertPackage(registry.Package{Alias: "a"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plugin", "disable", "z", "a", "a"}, {"plugin", "disable", "a", "z"}} {
		out, ui, code := runWorkHome(t, h.Root(), args...)
		if code != 0 || ui != "" || out != "work: disabled a\nwork: disabled z\n" {
			t.Fatalf("%s %s %d", out, ui, code)
		}
	}
	before, _ := os.ReadFile(h.RegistryFile())
	for _, args := range [][]string{{"plugin", "enable"}, {"plugin", "enable", "a", "missing"}, {"plugin", "disable", "a", "--json"}} {
		out, ui, code := runWorkHome(t, h.Root(), args...)
		if code != 2 || out != "" || !strings.Contains(ui, "usage") {
			t.Fatalf("%s %s %d", out, ui, code)
		}
		after, _ := os.ReadFile(h.RegistryFile())
		if string(before) != string(after) {
			t.Fatal("invalid request changed packages")
		}
	}
}
