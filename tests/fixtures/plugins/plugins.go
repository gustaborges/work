// Package plugins builds the fake plugin packages in this directory
// (specific-starter, colliding-starter, invalid-manifest, fallback-starter-a,
// fallback-starter-b, context-suite, restricted-suite, runtime-sh and the
// manifest-only invalid-* packages) into installable directories, so
// tests/plugininstall, tests/contract, and tests/integration can exercise
// `work plugin install`, internal/starter.Match and the automatic-context
// extensions without a real third-party plugin.
package plugins

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Prepare builds the fixture package at tests/fixtures/plugins/<fixture> and
// returns a fresh directory containing its plugin.json plus one compiled
// entrypoint binary per component the manifest declares (named exactly as
// the manifest's "entrypoint" field, ".exe"-suffixed on Windows) — ready to
// hand to `work plugin install <dir>` (or as --link's SOURCE). It mirrors
// tests/fixtures/locators/locators.go's Build pattern but stages a whole
// installable directory rather than registering a component directly.
//
// A component that declares a "runtime" is a script: its entrypoint file is
// copied verbatim (never marked executable, never suffixed) so the runtime
// invocation path is what gets exercised. A fixture without a main.go and
// without such scripts (invalid-manifest and the other invalid-* packages,
// which fail manifest validation before any entrypoint is touched) yields just
// its plugin.json.
func Prepare(t *testing.T, fixture string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Join(filepath.Dir(thisFile), fixture)

	manifest, err := os.ReadFile(filepath.Join(pkgDir, "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json for fixture %s: %v", fixture, err)
	}
	var m struct {
		Components []struct {
			Entrypoint string `json:"entrypoint"`
			Runtime    string `json:"runtime"`
		} `json:"components"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatalf("parse plugin.json for fixture %s: %v", fixture, err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), manifest, 0o644); err != nil {
		t.Fatalf("write plugin.json for fixture %s: %v", fixture, err)
	}

	built := map[string]bool{}
	for _, c := range m.Components {
		if c.Entrypoint == "" || built[c.Entrypoint] {
			continue
		}
		built[c.Entrypoint] = true
		if c.Runtime != "" {
			script, err := os.ReadFile(filepath.Join(pkgDir, c.Entrypoint))
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, c.Entrypoint), script, 0o644); err != nil {
				t.Fatalf("copy fixture %s script %s: %v", fixture, c.Entrypoint, err)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(pkgDir, "main.go")); err != nil {
			continue
		}
		bin := filepath.Join(dir, c.Entrypoint)
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", bin, pkgDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build fixture %s entrypoint %s: %v\n%s", fixture, c.Entrypoint, err, out)
		}
	}
	return dir
}
