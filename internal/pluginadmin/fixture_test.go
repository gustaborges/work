package pluginadmin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func prov(plugin, marketplace string) pluginid.Provider {
	return pluginid.Provider{Marketplace: marketplace, Plugin: plugin}
}

// pluginDir writes a plugin with one repository-locator component ("clone") and
// one convention ("shared"), so every provider contributes policy and
// convention state.
func pluginDir(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"name":"` + name + `","version":"1","components":[{"name":"clone","role":"repository-locator","entrypoint":"locator","accepts":["name"]}],"conventions":[{"name":"shared","prefixes":["{slug}"]}]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locator"), []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// installedHome installs a@m1, b@m1, keep@m1 and the homonym a@m2.
func installedHome(t *testing.T) workhome.Home {
	t.Helper()
	h := workhome.At(t.TempDir())
	for _, p := range []pluginid.Provider{prov("a", "m1"), prov("b", "m1"), prov("keep", "m1"), prov("a", "m2")} {
		installfx.Install(t, h, p, pluginDir(t, p.Plugin, "binary-"+p.String()), true)
	}
	return h
}

func setPolicy(t *testing.T, h workhome.Home, refs ...string) {
	t.Helper()
	if err := adminstate.WithWrite(context.Background(), h, "policy", func(s *adminstate.Session) error {
		s.Config.RepositoryResolution.Locators = refs
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// installReference installs a stand-in for the included reference provider.
func installReference(t *testing.T, h workhome.Home) {
	t.Helper()
	installfx.Install(t, h, adminstate.ReferenceProvider, pluginDir(t, adminstate.ReferencePlugin, "ref"), true)
}
