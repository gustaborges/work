package convention

import (
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

var (
	alpha = pluginid.Provider{Marketplace: "alpha", Plugin: "style"}
	beta  = pluginid.Provider{Marketplace: "beta", Plugin: "style"}
)

const freeform = "style@alpha/freeform"

func pkg(p pluginid.Provider, enabled bool) registry.Package {
	return registry.Package{Provider: p, InstallMode: registry.InstallModeCopy, Enabled: enabled, Version: "1", ContentDigest: "sha256:x", DigestAlgorithm: registry.DigestPackageV1,
		Provenance: registry.Provenance{RegistrationID: "0123456789abcdef0123456789abcdef", Kind: pluginid.KindLocal, Source: "/s", MarketplaceName: p.Marketplace, PluginName: p.Plugin, Path: "p", Revision: "r", CatalogDigest: "c"}}
}

func freeformCatalog() Catalog {
	r := registry.New()
	r.UpsertPackage(pkg(alpha, true))
	r.UpsertConvention(registry.Convention{Provider: alpha, Name: "freeform", Prefixes: []string{"{slug}"}})
	return Load(r)
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
	got, err := freeformCatalog().DeriveName(freeform, "{slug}", "add-retry")
	if err != nil || got != "add-retry" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDeriveNameRejectsUnknownPrefix(t *testing.T) {
	_, err := freeformCatalog().DeriveName(freeform, "feat/{slug}", "x")
	if diag.Token(err) != diag.Usage.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestDeriveNameRejectsUnknownAndBareConventions(t *testing.T) {
	c := freeformCatalog()
	for _, name := range []string{"style@alpha/gitflow", "freeform", "style@beta/freeform"} {
		if _, err := c.DeriveName(name, "{slug}", "x"); diag.Token(err) != diag.BootstrapFailed.Token {
			t.Errorf("%q: err = %v", name, err)
		}
	}
}

func TestDefaultPrefix(t *testing.T) {
	got, err := freeformCatalog().DefaultPrefix(freeform)
	if err != nil || got != "{slug}" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestHomonymousConventionsKeepTheirOwnPrefixes(t *testing.T) {
	r := registry.New()
	r.UpsertPackage(pkg(alpha, true))
	r.UpsertPackage(pkg(beta, true))
	r.UpsertConvention(registry.Convention{Provider: alpha, Name: "freeform", Prefixes: []string{"{slug}"}})
	r.UpsertConvention(registry.Convention{Provider: beta, Name: "freeform", Prefixes: []string{"team/{slug}"}})
	c := Load(r)
	if got, err := c.DeriveName("style@alpha/freeform", "{slug}", "x"); err != nil || got != "x" {
		t.Errorf("alpha = %q, %v", got, err)
	}
	if got, err := c.DeriveName("style@beta/freeform", "team/{slug}", "x"); err != nil || got != "team/x" {
		t.Errorf("beta = %q, %v", got, err)
	}
	if _, err := c.DeriveName("style@beta/freeform", "{slug}", "x"); diag.Token(err) != diag.Usage.Token {
		t.Errorf("alpha's prefix must not satisfy beta's convention: %v", err)
	}
}

func TestDisabledConventionUnavailable(t *testing.T) {
	r := registry.New()
	r.UpsertPackage(pkg(alpha, false))
	r.UpsertPackage(pkg(beta, true))
	r.UpsertConvention(registry.Convention{Provider: alpha, Name: "freeform", Prefixes: []string{"{slug}"}})
	if _, err := Load(r).DefaultPrefix(freeform); err == nil {
		t.Fatal("disabled convention available")
	}
	// A different enabled provider with the same bare name does not make it available.
	r.UpsertConvention(registry.Convention{Provider: beta, Name: "freeform", Prefixes: []string{"{slug}"}})
	if _, err := Load(r).DefaultPrefix(freeform); err == nil {
		t.Fatal("a homonym satisfied a disabled provider's convention")
	}
	pk, _ := r.PackageByProvider(alpha)
	pk.Enabled = true
	r.UpsertPackage(pk)
	if _, err := Load(r).DefaultPrefix(freeform); err != nil {
		t.Fatal(err)
	}
}
