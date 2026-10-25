package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestReviewedConventionMergesAndRejectsDrift(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	err := adminstate.WithWrite(ctx, h, "fixture", func(s *adminstate.Session) error {
		s.Registry.UpsertPackage(registry.Package{Alias: "a", Version: "blue", Conventions: []string{"team"}})
		s.Registry.Conventions = []registry.Convention{{Name: "team", Prefixes: []string{"team/{slug}"}}}
		s.Memory.Set("repo", "gone")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	err = adminstate.WithWrite(ctx, h, "other", func(s *adminstate.Session) error { s.Memory.Set("other", "team"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = setReviewedConvention(ctx, h, "repo", "team", reviewed); err != nil {
		t.Fatal(err)
	}
	current, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := current.Memory.Get("other"); got != "team" {
		t.Fatal("lost unrelated memory")
	}
	if err = setReviewedConvention(ctx, h, "repo", "team", reviewed); diag.ExitCode(err) != 53 {
		t.Fatalf("memory drift: %v", err)
	}
	reviewed = current
	err = adminstate.WithWrite(ctx, h, "change", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByAlias("a")
		p.Version = "green"
		s.Registry.UpsertPackage(p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = setReviewedConvention(ctx, h, "repo", "team", reviewed); diag.ExitCode(err) != 53 {
		t.Fatalf("provider drift: %v", err)
	}
}

func TestSafeAdministrativeEquivalent(t *testing.T) {
	got := safeCommand([]string{"plugin", "install", "https://private-user:private-password@example.test/plugin?token=private-token#private-fragment", "--as", "safe"})
	for _, secret := range []string{"private-user", "private-password", "private-token", "private-fragment"} {
		if strings.Contains(got, secret) {
			t.Fatal(got)
		}
	}
	got = safeCommand([]string{"repository", "root", "add", "/path with spaces/'quote/$(command)"})
	if !strings.Contains(got, "'\\''") || !strings.Contains(got, "'/path with spaces/") {
		t.Fatal("unsafe command quoting", got)
	}
}

func TestEquivalentPreservesLocalRootText(t *testing.T) {
	path := "/tmp/local@directory:part?query=literal"
	got := safeCommand([]string{"repository", "root", "add", path})
	if !strings.Contains(got, path) {
		t.Fatal("equivalent changed root operand", got)
	}
}
