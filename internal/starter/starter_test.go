package starter

import (
	"encoding/json"
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
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
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

// buildPluginFixtureStarter builds one of tests/fixtures/plugins/{specific-
// starter,colliding-starter} and installs it as a registered Starter
// component under a fresh plugins directory, returning the plugins dir and
// the component — mirrors buildTestdataStarter but sources from the F4
// fixture tree, whose "specific-starter" carries a real pattern/base_branch/
// start_modes response (contracts/starter-protocol.md).
func buildPluginFixtureStarter(t *testing.T, fixture, alias string) (string, registry.Component) {
	t.Helper()
	dir := fixtures.Prepare(t, fixture)
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Components []struct {
			Name       string `json:"name"`
			Entrypoint string `json:"entrypoint"`
			Pattern    string `json:"pattern"`
		} `json:"components"`
	}
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}
	c0 := m.Components[0]

	plugins := t.TempDir()
	src := filepath.Join(plugins, alias, "source")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	name := c0.Entrypoint
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, name), data, 0o755); err != nil {
		t.Fatal(err)
	}

	c := registry.Component{Alias: alias, Name: c0.Name, Role: registry.RoleStarter, Entrypoint: c0.Entrypoint, Pattern: c0.Pattern}
	return plugins, c
}

func TestMatchZeroSpecificMatchesFallsBackToFallback(t *testing.T) {
	_, reg := seededHome(t)
	_, specific := buildPluginFixtureStarter(t, "specific-starter", "demo")
	reg.UpsertComponent(specific)

	got, out, err := Match(reg, "not-a-match")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if out.Ambiguous != nil {
		t.Errorf("Ambiguous = %v, want nil", out.Ambiguous)
	}
	if got.Name != LogicalName {
		t.Errorf("Matched %q, want the fallback %q", got.Name, LogicalName)
	}
}

func TestMatchOneSpecificMatchInvokedDirectly(t *testing.T) {
	_, reg := seededHome(t)
	_, specific := buildPluginFixtureStarter(t, "specific-starter", "demo")
	reg.UpsertComponent(specific)

	got, out, err := Match(reg, "demo-pr-1")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if out.Ambiguous != nil {
		t.Errorf("Ambiguous = %v, want nil", out.Ambiguous)
	}
	if got.Alias != "demo" || got.Name != specific.Name {
		t.Errorf("Matched %+v, want the specific Starter", got)
	}
}

func TestMatchNoMatchAndNoFallbackIsStarterNotMatched(t *testing.T) {
	reg := &registry.Registry{}
	_, specific := buildPluginFixtureStarter(t, "specific-starter", "demo")
	reg.UpsertComponent(specific)

	_, _, err := Match(reg, "not-a-match")
	if diag.Token(err) != diag.StarterNotMatched.Token {
		t.Fatalf("err = %v, want token %q", err, diag.StarterNotMatched.Token)
	}
}

func TestMatchTwoSpecificMatchesIsAmbiguous(t *testing.T) {
	_, reg := seededHome(t)
	_, specific := buildPluginFixtureStarter(t, "specific-starter", "demo")
	_, colliding := buildPluginFixtureStarter(t, "colliding-starter", "other")
	reg.UpsertComponent(specific)
	reg.UpsertComponent(colliding)

	got, out, err := Match(reg, "demo-pr-1")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Name != "" || got.Alias != "" {
		t.Errorf("Matched = %+v, want zero value on an ambiguous outcome", got)
	}
	if len(out.Ambiguous) != 2 {
		t.Fatalf("Ambiguous = %+v, want 2 candidates", out.Ambiguous)
	}
}

// TestInvokeParsesBaseBranchAndStartModes asserts BaseBranch/StartModes are
// read off the wire (F4) — not yet consumed by any caller in this phase
// (consumption lands in Phase 5, research R8).
func TestInvokeParsesBaseBranchAndStartModes(t *testing.T) {
	plugins, c := buildPluginFixtureStarter(t, "specific-starter", "demo")
	ref, err := Invoke(plugins, c, "demo-pr-1")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if ref.BaseBranch != "feature/source-branch" {
		t.Errorf("BaseBranch = %q", ref.BaseBranch)
	}
	if len(ref.StartModes) != 2 || ref.StartModes[0] != "contribution" || ref.StartModes[1] != "fork" {
		t.Errorf("StartModes = %v", ref.StartModes)
	}
}
