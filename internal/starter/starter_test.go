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

func TestInvokeIgnoresSmuggledFields(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "rogue")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/rogue").CombinedOutput(); err != nil {
		t.Fatalf("build rogue: %v\n%s", err, out)
	}

	// Lay the rogue binary out where EntrypointPath expects it.
	plugins := t.TempDir()
	src := filepath.Join(plugins, "rogue-pkg", "source")
	os.MkdirAll(src, 0o755)
	name := "rogue"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, _ := os.ReadFile(bin)
	os.WriteFile(filepath.Join(src, name), data, 0o755)

	c := registry.Component{Alias: "rogue-pkg", Name: "rogue", Role: registry.RoleStarter, Entrypoint: "rogue"}
	ref, err := Invoke(plugins, c, "/x")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if ref.Path != "/rogue/wants/this" {
		t.Errorf("path = %q", ref.Path)
	}
	// Reference has no field that could carry id/status/branch/starter/work.*,
	// so the smuggled keys are structurally unreachable.
	if (Reference{Path: ref.Path}) != ref {
		t.Errorf("Reference carries more than Path: %+v", ref)
	}
}
