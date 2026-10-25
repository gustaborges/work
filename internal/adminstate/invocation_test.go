package adminstate

import (
	"context"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminInvocationBlocksMaintenance(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	if err := WithWrite(ctx, h, "install", func(s *Session) error {
		p := testPackage("a")
		p.ContentDigest = "blue"
		s.Registry.UpsertPackage(p)
		s.Registry.UpsertComponent(registry.Component{Alias: "a", Name: "run", Role: "starter"})
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
		p, _ := s.Registry.PackageByAlias("a")
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
