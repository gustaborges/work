package convention

import (
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
)

func freeformCatalog() Catalog {
	return Load(&registry.Registry{
		Conventions: []registry.Convention{{Name: Freeform, Prefixes: []string{"{slug}"}}},
	})
}

func TestInterpolate(t *testing.T) {
	if got := Interpolate("{slug}", "add-x"); got != "add-x" {
		t.Errorf("got %q", got)
	}
	if got := Interpolate("feat/{slug}", "add-x"); got != "feat/add-x" {
		t.Errorf("got %q", got)
	}
}

func TestDeriveNameFreeform(t *testing.T) {
	c := freeformCatalog()
	got, err := c.DeriveName(Freeform, "{slug}", "add-retry")
	if err != nil {
		t.Fatal(err)
	}
	if got != "add-retry" {
		t.Errorf("got %q, want add-retry", got)
	}
}

func TestDeriveNameRejectsUnknownPrefix(t *testing.T) {
	c := freeformCatalog()
	_, err := c.DeriveName(Freeform, "feat/{slug}", "x")
	if diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDeriveNameRejectsUnknownConvention(t *testing.T) {
	c := freeformCatalog()
	_, err := c.DeriveName("gitflow", "{slug}", "x")
	if diag.Token(err) != diag.BootstrapFailed.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultPrefix(t *testing.T) {
	got, err := freeformCatalog().DefaultPrefix(Freeform)
	if err != nil || got != "{slug}" {
		t.Fatalf("got %q err %v", got, err)
	}
}
