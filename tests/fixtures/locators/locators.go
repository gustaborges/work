// Package locators builds and installs the fake Repository Locator
// entrypoints in this directory (ok, empty, two, dupe, invalid, boom) into a
// test plugins directory and registry, so internal/locator, internal/repoconfig,
// and tests/contract can exercise policy traversal without a real plugin
// install.
package locators

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

// Build compiles the fixture directory <repoRoot>/tests/fixtures/locators/<fixture>
// (one of ok, empty, two, dupe, invalid, boom) and returns the path to the
// built binary. It skips the test if the fixture cannot be built.
func Build(t *testing.T, fixture string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Join(filepath.Dir(thisFile), fixture)

	bin := filepath.Join(t.TempDir(), fixture)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, pkgDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build locator fixture %s: %v\n%s", fixture, err, out)
	}
	return bin
}

// Install builds the named fixture and registers it in reg as a
// repository-locator component identified by (provider, name), with entrypoint
// laid out under pluginsDir exactly where registry.Component.EntrypointPath
// expects it. accepts is the component's declared `accepts` vocabulary. A
// dummy package for the provider is recorded when none exists.
func Install(t *testing.T, reg *registry.Registry, pluginsDir string, provider pluginid.Provider, name, fixture string, accepts []string) registry.Component {
	t.Helper()
	bin := Build(t, fixture)

	entrypoint := fixture
	src := filepath.Join(pluginsDir, provider.Marketplace, provider.Plugin, "source")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", src, err)
	}
	destName := entrypoint
	if runtime.GOOS == "windows" {
		destName += ".exe"
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read built fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, destName), data, 0o755); err != nil {
		t.Fatalf("install fixture binary: %v", err)
	}

	installfx.EnsurePackage(reg, provider)
	c := registry.Component{
		Provider:    provider,
		Name:        name,
		Role:        registry.RoleRepositoryLocator,
		Entrypoint:  entrypoint,
		Accepts:     accepts,
		DisplayName: name,
		Description: "fixture repository locator (" + fixture + ")",
	}
	reg.UpsertComponent(c)
	return c
}
