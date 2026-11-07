package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func TestReviewedConventionMergesAndRejectsDrift(t *testing.T) {
	ctx := context.Background()
	h := workhome.At(filepath.Join(t.TempDir(), "home"))
	provider := installfx.P("a")
	team := "a@m/team"
	err := adminstate.WithWrite(ctx, h, "fixture", func(s *adminstate.Session) error {
		p := installfx.Package(provider, true)
		p.Version = "blue"
		s.Registry.UpsertPackage(p)
		s.Registry.UpsertConvention(registry.Convention{Provider: provider, Name: "team", Prefixes: []string{"team/{slug}"}})
		s.Memory.Set("repo", repoconv.NewReference(installfx.RegistrationID("m"), pluginid.Ref{Provider: installfx.P("gone"), Name: "old"}))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	other := repoconv.NewReference(installfx.RegistrationID("m"), pluginid.Ref{Provider: provider, Name: "team"})
	err = adminstate.WithWrite(ctx, h, "other", func(s *adminstate.Session) error { s.Memory.Set("other", other); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = setReviewedConvention(ctx, h, "repo", team, reviewed); err != nil {
		t.Fatal(err)
	}
	current, err := adminstate.Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := current.Memory.Get("other"); got != other {
		t.Fatal("lost unrelated memory")
	}
	if got, _ := current.Memory.Get("repo"); got.String() != team || got.RegistrationID != installfx.RegistrationID("m") {
		t.Fatalf("remembered %+v, want the qualified reference bound to its source instance", got)
	}
	if err = setReviewedConvention(ctx, h, "repo", team, reviewed); diag.ExitCode(err) != 53 {
		t.Fatalf("memory drift: %v", err)
	}
	reviewed = current
	err = adminstate.WithWrite(ctx, h, "change", func(s *adminstate.Session) error {
		p, _ := s.Registry.PackageByProvider(provider)
		p.Version = "green"
		s.Registry.UpsertPackage(p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = setReviewedConvention(ctx, h, "repo", team, reviewed); diag.ExitCode(err) != 53 {
		t.Fatalf("provider drift: %v", err)
	}
}

func TestResolveConventionRef(t *testing.T) {
	reg := registry.New()
	for _, p := range []pluginid.Provider{{Marketplace: "x", Plugin: "style"}, {Marketplace: "y", Plugin: "style"}, {Marketplace: "x", Plugin: "solo"}} {
		reg.UpsertPackage(installfx.Package(p, true))
		reg.UpsertConvention(registry.Convention{Provider: p, Name: "freeform", Prefixes: []string{"{slug}"}})
	}
	reg.UpsertConvention(registry.Convention{Provider: pluginid.Provider{Marketplace: "x", Plugin: "solo"}, Name: "only-here", Prefixes: []string{"{slug}"}})
	for input, want := range map[string]string{"style@x/freeform": "style@x/freeform", "only-here": "solo@x/only-here"} {
		ref, err := resolveConventionRef(reg, input)
		if err != nil || ref.String() != want {
			t.Errorf("%q = %v, %v; want %s (stored fully qualified)", input, ref, err, want)
		}
	}
	if _, err := resolveConventionRef(reg, "freeform"); diag.ExitCode(err) != 2 {
		t.Errorf("ambiguous bare name = %v, want usage", err)
	}
	for _, input := range []string{"missing", "style@z/freeform", "style@x/none", "style/freeform"} {
		if _, err := resolveConventionRef(reg, input); diag.ExitCode(err) != diag.ConventionUnknown.Code {
			t.Errorf("%q = %v, want convention-unknown", input, err)
		}
	}
	off := installfx.Package(pluginid.Provider{Marketplace: "x", Plugin: "style"}, false)
	reg.UpsertPackage(off)
	if _, err := resolveConventionRef(reg, "style@x/freeform"); diag.ExitCode(err) != diag.ConventionUnknown.Code {
		t.Errorf("a disabled provider's convention resolved: %v", err)
	}
}

func TestSafeAdministrativeEquivalent(t *testing.T) {
	got := safeCommand([]string{"plugin", "marketplace", "add", "https://private-user:private-password@example.test/plugin?token=private-token#private-fragment"})
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
