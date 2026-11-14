package plugininstall

import (
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

const manifestJSON = `{"name":"github","version":"1.2.0","conventions":[{"name":"team","prefixes":["{slug}"]}],"components":[
  {"name":"gh-starter","role":"starter","entrypoint":"starter"},
  {"name":"gh-linker","role":"linker","key":"github.pull_request","entrypoint":"linker","discover":{"automatic":true,"on":[{"event":"start:finalized"}],"inputs":["work:slug"]},"manual":{"display_name":"GH"}}]}`

func spec(t *testing.T, marketplace string) Spec {
	t.Helper()
	m, err := plugin.Parse([]byte(manifestJSON))
	if err != nil {
		t.Fatal(err)
	}
	p := pluginid.Provider{Marketplace: marketplace, Plugin: "github"}
	return Spec{Provider: p, Manifest: m, Enabled: true, ContentDigest: "sha256:aa", Provenance: registry.Provenance{
		RegistrationID: "0123456789abcdef0123456789abcdef", Kind: pluginid.KindLocal, Source: "/src",
		MarketplaceName: marketplace, PluginName: "github", Path: "plugins/github", Revision: "sha256:r", CatalogDigest: "sha256:c",
	}}
}

func TestRegisterRecordsDeclarationsUnderTheProvider(t *testing.T) {
	reg := registry.New()
	res, err := Register(reg, spec(t, "team"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Package.ID() != "github@team" || res.Package.Version != "1.2.0" || res.Package.InstallMode != registry.InstallModeCopy {
		t.Errorf("package = %+v", res.Package)
	}
	if err := reg.Validate(); err != nil {
		t.Fatalf("registered registry is invalid: %v", err)
	}
	if len(reg.Components) != 2 || reg.Components[0].StarterLayer != registry.LayerFallback {
		t.Errorf("components = %+v", reg.Components)
	}
	if l := reg.Components[1]; len(l.Inputs) != 1 || l.Inputs[0] != "work:slug" || l.Discover == nil {
		t.Errorf("linker discover inputs not copied: %+v", l)
	}
	if len(reg.Conventions) != 1 || reg.Conventions[0].Provider.Marketplace != "team" {
		t.Errorf("conventions = %+v", reg.Conventions)
	}
}

func TestRegisterReplacesOnlyTheSameProvider(t *testing.T) {
	reg := registry.New()
	for _, mp := range []string{"alpha", "beta"} {
		if _, err := Register(reg, spec(t, mp)); err != nil {
			t.Fatal(err)
		}
	}
	// A homonym in another marketplace must survive reinstalling the first.
	s := spec(t, "alpha")
	s.Enabled = false
	if _, err := Register(reg, s); err != nil {
		t.Fatal(err)
	}
	if len(reg.Packages) != 2 || len(reg.Components) != 4 || len(reg.Conventions) != 2 {
		t.Fatalf("registry = %+v", reg)
	}
	if reg.PackageEnabled(pluginid.Provider{Marketplace: "alpha", Plugin: "github"}) || !reg.PackageEnabled(pluginid.Provider{Marketplace: "beta", Plugin: "github"}) {
		t.Error("enablement leaked across providers")
	}
}

func TestRegisterRejectsIdentityMismatch(t *testing.T) {
	s := spec(t, "team")
	s.Provider.Plugin = "other"
	if _, err := Register(registry.New(), s); diag.ExitCode(err) != diag.PluginInvalid.Code {
		t.Errorf("name mismatch = %v", err)
	}
	s = spec(t, "team")
	s.Provider.Marketplace = "a/b"
	if _, err := Register(registry.New(), s); err == nil {
		t.Error("unsafe marketplace accepted")
	}
}

func TestPreflightRuntimes(t *testing.T) {
	m, _ := plugin.Parse([]byte(`{"name":"p","version":"1","conventions":[],"components":[{"name":"s","role":"starter","entrypoint":"s","runtime":"definitely-not-installed-xyz"}]}`))
	err := PreflightRuntimes(m)
	if diag.ExitCode(err) != diag.PluginInstallFailed.Code {
		t.Fatalf("missing runtime = %v", err)
	}
	if PreflightRuntimes(&plugin.Manifest{}) != nil {
		t.Error("a manifest without runtimes must pass")
	}
}
