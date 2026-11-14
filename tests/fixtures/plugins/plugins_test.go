package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func parseFixture(t *testing.T, name string) error {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(name, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = plugin.Parse(data)
	return err
}

func TestValidFixturesParse(t *testing.T) {
	for _, name := range []string{"context-suite", "restricted-suite", "runtime-sh", "manual-suite"} {
		if err := parseFixture(t, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Each invalid-* fixture must fail for the reason it was written to exercise,
// not for an incidental one.
func TestInvalidFixturesFailForTheirReason(t *testing.T) {
	want := map[string]string{
		"invalid-event":     "not a core event",
		"invalid-input":     "inputs entry",
		"invalid-dup-input": "same key",
		"invalid-key-owner": "private to plugin",
		"invalid-manual":    "",
	}
	for name, sub := range want {
		err := parseFixture(t, name)
		if err == nil || !strings.Contains(err.Error(), sub) {
			t.Errorf("%s: err = %v, want it to contain %q", name, err, sub)
		}
	}
}

// invalid-runtime is a well-formed manifest: only the install-time PATH check
// rejects it.
func TestInvalidRuntimeFixtureParses(t *testing.T) {
	if err := parseFixture(t, "invalid-runtime"); err != nil {
		t.Errorf("invalid-runtime: %v", err)
	}
}

func TestPrepareStagesScriptsAndBinaries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runtime-sh needs a POSIX sh")
	}
	dir := Prepare(t, "runtime-sh")
	info, err := os.Stat(filepath.Join(dir, "starter.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 != 0 {
		t.Error("the script fixture must not be executable")
	}

	suite := Prepare(t, "context-suite")
	for _, e := range []string{"starter", "linker", "linker2", "importer", "importer2", "plugin.json"} {
		bin := filepath.Join(suite, e)
		if _, err := os.Stat(bin); err != nil {
			t.Errorf("context-suite is missing %s: %v", e, err)
		}
	}
}

func TestManualFixtureRetainedEnablement(t *testing.T) {
	dir := Prepare(t, "manual-suite")
	for _, name := range []string{"linker", "importer", "needs-input", "automatic-only"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "registry.json")
	manual, alternate := installfx.P("manual"), installfx.P("alternate")
	r := registry.New()
	r.UpsertPackage(installfx.Package(manual, true))
	r.UpsertPackage(installfx.Package(alternate, true))
	if err := registry.Save(path, r); err != nil {
		t.Fatal(err)
	}
	if err := SetPackageEnabled(path, manual, false); err != nil {
		t.Fatal(err)
	}
	r, err := registry.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.PackageEnabled(manual) || !r.PackageEnabled(alternate) {
		t.Fatalf("state: %+v", r.Packages)
	}
}
