package adminstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// prov builds the provider "plugin@marketplace" for tests.
func prov(plugin, marketplace string) pluginid.Provider {
	return pluginid.Provider{Marketplace: marketplace, Plugin: plugin}
}

// testPackage builds a valid installed package record for p.
func testPackage(p pluginid.Provider) registry.Package {
	return registry.Package{
		Provider: p, InstallMode: registry.InstallModeCopy, Enabled: true, Version: "1",
		ContentDigest: "sha256:aa", DigestAlgorithm: registry.DigestPackageV1,
		Provenance: registry.Provenance{
			RegistrationID: "0123456789abcdef0123456789abcdef", Kind: pluginid.KindLocal, Source: "/src",
			MarketplaceName: p.Marketplace, PluginName: p.Plugin, Path: "plugins/" + p.Plugin,
			Revision: "sha256:rev", CatalogDigest: "sha256:cat",
		},
	}
}

// seedRoot creates a live package root with one file so moves have a before image.
func seedRoot(t *testing.T, h workhome.Home, p pluginid.Provider, name, content string) string {
	t.Helper()
	root, err := h.PackageRoot(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// stageWith creates a stage through the session and fills it.
func stageWith(t *testing.T, s *Session, name, content string) string {
	t.Helper()
	stage, err := s.NewStage()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return stage
}
