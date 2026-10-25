package integration

import (
	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"strings"
	"testing"
)

func TestAdminCredentialsUpdateAndGitFailure(t *testing.T) {
	h := newAdminHarness(t)
	source := "file://private-user:private-password@invalid-host/nonexistent-plugin?token=private-token#private-fragment"
	home := workhome.At(adminHome(h))
	if err := adminstate.WithWrite(context.Background(), home, "fixture", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(registry.Package{Alias: "a", Origin: registry.OriginRemotePinned, Version: "blue\nline", Source: source, Reference: source + "@old"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plugin", "list", "--json"}, {"plugin", "update", "--check", "--json"}, {"plugin", "update", "--check"}, {"plugin", "update", "a", "--yes"}, {"plugin", "install", source, "--as", "new"}} {
		code := 50
		if args[1] == "list" {
			code = 0
		}
		if args[1] == "install" {
			code = 34
		}
		out, ui := h.run(t, h.root, code, args...)
		for _, secret := range []string{"private-user", "private-password", "private-token", "private-fragment"} {
			if strings.Contains(out+ui, secret) {
				t.Fatalf("%v leaked %s: %s %s", args, secret, out, ui)
			}
		}
		if strings.Contains(out, "blue\nline") {
			t.Fatal("unescaped author text")
		}
	}
}
