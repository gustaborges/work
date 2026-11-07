package adminstate

import (
	"context"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminInvocationBlocksMaintenance(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	if err := WithWrite(ctx, h, "install", func(s *Session) error {
		p := testPackage(prov("a", "m"))
		p.ContentDigest = "blue"
		s.Registry.UpsertPackage(p)
		s.Registry.UpsertComponent(registry.Component{Provider: prov("a", "m"), Name: "run", Role: "starter", Entrypoint: "run"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s, err := Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Registry.Components[0]
	entered, finish := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- WithInvocation(ctx, h, c, func() error { close(entered); <-finish; return nil }) }()
	<-entered
	// Injected timeout proves exclusive ownership without a 30-second sleep.
	if release, err := acquire(ctx, h, time.Millisecond); diag.ExitCode(err) != 56 {
		if release != nil {
			release()
		}
		t.Fatalf("maintenance entered: %v", err)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := WithWrite(ctx, h, "replace", func(s *Session) error {
		p, _ := s.Registry.PackageByProvider(prov("a", "m"))
		p.ContentDigest = "green"
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ran := false
	err = WithInvocation(ctx, h, c, func() error { ran = true; return nil })
	if diag.ExitCode(err) != 53 || ran {
		t.Fatalf("stale invocation: %v %v", err, ran)
	}
}

// The execution baseline is the selected provider's record, provenance and
// declaration, never a bare name: replacing a homonym leaves the other's guard
// valid, and re-registering a source (same names, new instance) invalidates it.
func TestInvocationGuardIsPerProviderAndSourceInstance(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	a, b := prov("sample", "alpha"), prov("sample", "beta")
	if err := WithWrite(ctx, h, "install", func(s *Session) error {
		for _, p := range []pluginid.Provider{a, b} {
			s.Registry.UpsertPackage(testPackage(p))
			s.Registry.UpsertComponent(registry.Component{Provider: p, Name: "run", Role: "starter", Entrypoint: "run"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s, err := Read(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	var ca, cb registry.Component
	for _, c := range s.Registry.Components {
		if c.Provider == a {
			ca = c
		} else {
			cb = c
		}
	}
	// Replace only alpha's package content.
	if err = WithWrite(ctx, h, "replace alpha", func(s *Session) error {
		p, _ := s.Registry.PackageByProvider(a)
		p.ContentDigest = "sha256:changed"
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = WithInvocation(ctx, h, cb, func() error { return nil }); err != nil {
		t.Fatalf("an unrelated homonym's replacement invalidated beta: %v", err)
	}
	if err = WithInvocation(ctx, h, ca, func() error { t.Fatal("stale alpha ran"); return nil }); diag.ExitCode(err) != 53 {
		t.Fatalf("alpha's guard = %v", err)
	}

	// A re-registered beta has the same public names but another instance.
	s, _ = Read(ctx, h)
	var cb2 registry.Component
	for _, c := range s.Registry.Components {
		if c.Provider == b {
			cb2 = c
		}
	}
	if err = WithWrite(ctx, h, "re-register beta", func(s *Session) error {
		p, _ := s.Registry.PackageByProvider(b)
		p.Provenance.RegistrationID = "ffffffffffffffffffffffffffffffff"
		s.Registry.UpsertPackage(p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = WithInvocation(ctx, h, cb2, func() error { t.Fatal("a replaced source instance ran under the old baseline"); return nil }); diag.ExitCode(err) != 53 {
		t.Fatalf("beta's guard after re-registration = %v", err)
	}
}
