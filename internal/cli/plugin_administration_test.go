package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func TestToggleReceiptsAndGrammar(t *testing.T) {
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	err := adminstate.WithWrite(context.Background(), h, "fixture", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(installfx.Package(installfx.P("z"), true))
		s.Registry.UpsertPackage(installfx.Package(installfx.P("a"), true))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plugin", "disable", "z@m", "a@m", "a@m"}, {"plugin", "disable", "a@m", "z@m"}} {
		out, ui, code := runWorkHome(t, h.Root(), args...)
		if code != 0 || ui != "" || out != "work: disabled a@m\nwork: disabled z@m\n" {
			t.Fatalf("%s %s %d", out, ui, code)
		}
	}
	before, _ := os.ReadFile(h.RegistryFile())
	for _, args := range [][]string{{"plugin", "enable"}, {"plugin", "enable", "a@m", "missing@m"}, {"plugin", "enable", "a"}, {"plugin", "disable", "a@m", "--json"}} {
		out, ui, code := runWorkHome(t, h.Root(), args...)
		if code != 2 || out != "" || !strings.Contains(ui, "usage") {
			t.Fatalf("%v: %s %s %d", args, out, ui, code)
		}
		after, _ := os.ReadFile(h.RegistryFile())
		if string(before) != string(after) {
			t.Fatal("invalid request changed packages")
		}
	}
}
