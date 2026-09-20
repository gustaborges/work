// Core-side Starter pattern-match contract (F4, US2/US4): pins
// specs/005-plugin-origins/contracts/starter-protocol.md §Selection against
// the real fixture Starter binaries in tests/fixtures/plugins, mirroring how
// locator_resolution_test.go pins internal/locator against
// tests/fixtures/locators. Ambiguous/collision consumption by `work start`
// itself lands in Phase 6; this file pins internal/starter.Match's own
// zero/one/many outcome shape end to end, including a real subprocess
// invocation of the matched Starter.
package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/starter"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// registerFixtureStarter builds fixture (one of tests/fixtures/plugins'
// starter packages) and registers its first component under alias in a
// fresh registry backed by pluginsDir, exactly the layout
// registry.Component.EntrypointPath expects.
func registerFixtureStarter(t *testing.T, pluginsDir, alias, fixture string) *registry.Registry {
	t.Helper()
	reg := &registry.Registry{}
	registerFixtureStarterInto(t, reg, pluginsDir, alias, fixture)
	return reg
}

func registerFixtureStarterInto(t *testing.T, reg *registry.Registry, pluginsDir, alias, fixture string) {
	t.Helper()
	fixtureDir := fixtures.Prepare(t, fixture)

	manifestBytes, err := os.ReadFile(filepath.Join(fixtureDir, "plugin.json"))
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
	if len(m.Components) == 0 {
		t.Fatalf("fixture %s declares no components", fixture)
	}
	c0 := m.Components[0]

	src := filepath.Join(pluginsDir, alias, "source")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	binName := c0.Entrypoint
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	data, err := os.ReadFile(filepath.Join(fixtureDir, binName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, binName), data, 0o755); err != nil {
		t.Fatal(err)
	}

	reg.UpsertComponent(registry.Component{
		Alias: alias, Name: c0.Name, Role: registry.RoleStarter,
		Entrypoint: c0.Entrypoint, Pattern: c0.Pattern,
	})
}

// seededPluginsHome bootstraps the embedded reference package (its fallback
// Starter) into a fresh Work home and returns its plugins directory and
// registry, for the "falls back to the reference Starter" case.
func seededPluginsHome(t *testing.T) (string, *registry.Registry) {
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
	return h.PluginsDir(), reg
}

func TestStarterMatchContract(t *testing.T) {
	t.Run("specific match invoked directly, no selection step", func(t *testing.T) {
		plugins := t.TempDir()
		reg := registerFixtureStarter(t, plugins, "demo", "specific-starter")

		comp, out, err := starter.Match(reg, "demo-pr-1")
		if err != nil {
			t.Fatalf("Match: %v", err)
		}
		if out.Ambiguous != nil {
			t.Fatalf("Ambiguous = %v, want nil (exactly one match)", out.Ambiguous)
		}
		if comp.Alias != "demo" {
			t.Fatalf("Matched %+v, want the specific Starter", comp)
		}

		ref, err := starter.Invoke(plugins, comp, "demo-pr-1")
		if err != nil {
			t.Fatalf("Invoke: %v", err)
		}
		if ref.Name != "demo-pr-1" {
			t.Errorf("Repository.Name = %q, want the arg echoed back", ref.Name)
		}
		if ref.BaseBranch != "feature/source-branch" {
			t.Errorf("BaseBranch = %q", ref.BaseBranch)
		}
	})

	t.Run("no specific match falls back to the reference Starter", func(t *testing.T) {
		plugins, reg := seededPluginsHome(t)
		registerFixtureStarterInto(t, reg, plugins, "demo", "specific-starter")

		comp, out, err := starter.Match(reg, "not-a-match-at-all")
		if err != nil {
			t.Fatalf("Match: %v", err)
		}
		if out.Ambiguous != nil {
			t.Fatalf("Ambiguous = %v, want nil", out.Ambiguous)
		}
		if comp.Name != starter.LogicalName {
			t.Fatalf("Matched %q, want the reference fallback %q", comp.Name, starter.LogicalName)
		}
	})

	t.Run("collision returns an Ambiguous outcome naming both, never memoized", func(t *testing.T) {
		plugins := t.TempDir()
		reg := registerFixtureStarter(t, plugins, "demo", "specific-starter")
		registerFixtureStarterInto(t, reg, plugins, "collide", "colliding-starter")

		comp, out, err := starter.Match(reg, "demo-pr-1")
		if err != nil {
			t.Fatalf("Match: %v", err)
		}
		if out.Ambiguous == nil {
			t.Fatalf("Ambiguous = nil, want both colliding components")
		}
		if len(out.Ambiguous) != 2 {
			t.Fatalf("Ambiguous = %+v, want exactly 2 colliding components", out.Ambiguous)
		}
		if comp.Alias != "" || comp.Name != "" {
			t.Fatalf("Component = %+v, want the zero value on an ambiguous outcome — neither invoked yet", comp)
		}
		seen := map[string]bool{}
		for _, c := range out.Ambiguous {
			seen[c.Alias] = true
		}
		if !seen["demo"] || !seen["collide"] {
			t.Fatalf("Ambiguous = %+v, want both demo and collide named", out.Ambiguous)
		}

		// Match is a pure function with no state: the identical collision run
		// again returns the same Ambiguous outcome rather than remembering
		// whichever component a caller previously chose (FR-012, SC-006).
		_, out2, err := starter.Match(reg, "demo-pr-1")
		if err != nil {
			t.Fatalf("Match (second run): %v", err)
		}
		if len(out2.Ambiguous) != 2 {
			t.Fatalf("second Match Ambiguous = %+v, want 2 again (not memoized)", out2.Ambiguous)
		}
	})

	t.Run("no match and no fallback is starter-not-matched (35)", func(t *testing.T) {
		plugins := t.TempDir()
		reg := registerFixtureStarter(t, plugins, "demo", "specific-starter")

		_, _, err := starter.Match(reg, "not-a-match-at-all")
		if diag.Token(err) != diag.StarterNotMatched.Token {
			t.Fatalf("err = %v, want token %q", err, diag.StarterNotMatched.Token)
		}
		if diag.ExitCode(err) != 35 {
			t.Fatalf("exit code = %d, want 35", diag.ExitCode(err))
		}
	})
}

// TestValidateResponseContract pins starter-protocol.md's structural rules
// (research R9): checked before any Work materialization, on the typed
// Reference the core actually consumes — so this needs no subprocess fixture,
// only the shapes ipc decoding could hand back from any Starter, well-behaved
// or not.
func TestValidateResponseContract(t *testing.T) {
	t.Run("unrecognized start mode is starter-response-invalid (37)", func(t *testing.T) {
		err := starter.ValidateResponse(starter.Reference{
			BaseBranch: "main",
			StartModes: []string{"rebase"},
		}, "acme")
		if diag.Token(err) != diag.StarterResponseInvalid.Token {
			t.Fatalf("err = %v, want token %q", err, diag.StarterResponseInvalid.Token)
		}
		if diag.ExitCode(err) != 37 {
			t.Fatalf("exit code = %d, want 37", diag.ExitCode(err))
		}
	})

	t.Run("contribution without base_branch is starter-response-invalid (37)", func(t *testing.T) {
		err := starter.ValidateResponse(starter.Reference{
			StartModes: []string{"contribution"},
		}, "acme")
		if diag.Token(err) != diag.StarterResponseInvalid.Token {
			t.Fatalf("err = %v, want token %q", err, diag.StarterResponseInvalid.Token)
		}
		if diag.ExitCode(err) != 37 {
			t.Fatalf("exit code = %d, want 37", diag.ExitCode(err))
		}
	})

	t.Run("contribution with a base_branch is valid", func(t *testing.T) {
		err := starter.ValidateResponse(starter.Reference{
			BaseBranch: "feature/source-branch",
			StartModes: []string{"contribution", "fork"},
		}, "acme")
		if err != nil {
			t.Fatalf("ValidateResponse: %v", err)
		}
	})

	t.Run("absent start_modes is valid", func(t *testing.T) {
		if err := starter.ValidateResponse(starter.Reference{}, "acme"); err != nil {
			t.Fatalf("ValidateResponse: %v", err)
		}
	})

	t.Run("fork alone with no base_branch is valid (FR-023 prompts for one)", func(t *testing.T) {
		if err := starter.ValidateResponse(starter.Reference{StartModes: []string{"fork"}}, "acme"); err != nil {
			t.Fatalf("ValidateResponse: %v", err)
		}
	})
}
