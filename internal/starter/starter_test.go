package starter

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
)

func seededHome(t *testing.T) (workhome.Home, *registry.Registry) {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Skipf("no embedded seed; run `make seed` (%v)", err)
	}
	h := workhome.At(filepath.Join(t.TempDir(), "dotwork"))
	if err := bootstrap.EnsureSeed(h); err != nil {
		t.Fatalf("EnsureSeed: %v", err)
	}
	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		t.Fatal(err)
	}
	return h, reg
}

func TestSelectReturnsFallbackStarter(t *testing.T) {
	_, reg := seededHome(t)
	c, err := Select(reg)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if c.Name != LogicalName {
		t.Errorf("selected %q, want %q", c.Name, LogicalName)
	}
}

func TestSelectErrorsWithoutRegistration(t *testing.T) {
	_, err := Select(&registry.Registry{})
	if diag.Token(err) != diag.BootstrapFailed.Token {
		t.Fatalf("err = %v", err)
	}
}

func TestInvokeResolvesRelativePathToAbsolute(t *testing.T) {
	h, reg := seededHome(t)
	c, _ := Select(reg)
	ref, err := Invoke(h.PluginsDir(), c, "./somewhere")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !filepath.IsAbs(ref.Path) {
		t.Errorf("path %q is not absolute", ref.Path)
	}
}

func TestInvokeEmptyArgIsUnusableRepo(t *testing.T) {
	h, reg := seededHome(t)
	c, _ := Select(reg)
	_, err := Invoke(h.PluginsDir(), c, "   ")
	if diag.Token(err) != diag.UnusableRepo.Token {
		t.Fatalf("err = %v", err)
	}
}

// buildTestdataStarter compiles tests/testdata/<pkg> and installs it as a
// registered Starter component under a fresh plugins directory, returning the
// plugins dir and the component.
func buildTestdataStarter(t *testing.T, pkg string) (string, registry.Component) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), pkg)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/"+pkg).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, out)
	}

	plugins := t.TempDir()
	src := filepath.Join(plugins, pkg+"-pkg", "source")
	os.MkdirAll(src, 0o755)
	name := pkg
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, _ := os.ReadFile(bin)
	os.WriteFile(filepath.Join(src, name), data, 0o755)

	c := registry.Component{Alias: pkg + "-pkg", Name: pkg, Role: registry.RoleStarter, Entrypoint: pkg}
	return plugins, c
}

func TestInvokeIgnoresSmuggledFields(t *testing.T) {
	plugins, c := buildTestdataStarter(t, "rogue")
	ref, err := Invoke(plugins, c, "/x")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if ref.Path != "/rogue/wants/this" {
		t.Errorf("path = %q", ref.Path)
	}
	// Reference has no field that could carry id/status/branch/starter/work.*,
	// so the smuggled keys are structurally unreachable.
	if ref.GitFetchURLs != nil || ref.Name != "" || ref.Query != "" {
		t.Errorf("Reference carries more than Path: %+v", ref)
	}
}

func TestInvokeReturnsNameOnlyReference(t *testing.T) {
	plugins, c := buildTestdataStarter(t, "nameonly")
	ref, err := Invoke(plugins, c, "payments")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if ref.Path != "" {
		t.Errorf("Path = %q, want empty", ref.Path)
	}
	if ref.Name != "payments" {
		t.Errorf("Name = %q, want payments", ref.Name)
	}
}

func TestInvokeReturnsMixedReference(t *testing.T) {
	plugins, c := buildTestdataStarter(t, "mixedref")
	ref, err := Invoke(plugins, c, "anything")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if ref.Name != "acme" || ref.Query != "billing" || len(ref.GitFetchURLs) != 1 || ref.GitFetchURLs[0] != "https://example.com/acme.git" {
		t.Errorf("ref = %+v", ref)
	}
}

// TestInvokeDoesNotRequireAPath asserts the F1 "no repository path" guard was
// removed from starter.Invoke: a well-formed response with no path succeeds
// here; internal/locator is the layer that classifies a fieldless reference
// (research R8).
func TestInvokeDoesNotRequireAPath(t *testing.T) {
	plugins, c := buildTestdataStarter(t, "nameonly")
	if _, err := Invoke(plugins, c, "x"); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
}
