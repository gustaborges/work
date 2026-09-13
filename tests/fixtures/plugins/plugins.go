// Package plugins builds the fake plugin packages in this directory
// (specific-starter, colliding-starter, invalid-manifest, fallback-starter-a,
// fallback-starter-b) into installable directories, so tests/plugininstall,
// tests/contract, and tests/integration can exercise `work plugin install`
// and internal/starter.Match without a real third-party plugin.
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
// invalid-manifest has no main.go (it is never meant to build or run — its
// install always fails manifest validation first) and must not be passed
// here; reference it by its checked-in path instead.
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
