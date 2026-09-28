package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func repoFile(t *testing.T, rel ...string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(append([]string{filepath.Dir(thisFile), "..", ".."}, rel...)...)
}

// definedPublicKeys reads the keys of the §5 table of the published Semantic
// Conventions: rows shaped "| `key` | section | … |".
func definedPublicKeys(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(repoFile(t, "specs", "006-automatic-start-context", "contracts", "semantic-conventions.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(data), "## 5.")
	if !ok {
		t.Fatal("semantic-conventions.md has no §5")
	}
	section, _, _ = strings.Cut(section, "## 6.")
	row := regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")
	keys := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		keys[m[1]] = true
	}
	if len(keys) == 0 {
		t.Fatal("no keys parsed from §5")
	}
	return keys
}

// TestEveryPublicKeyInShippedFixturesIsDefined: a key that looks
// public and appears in a valid fixture manifest, or in the keys a fixture
// Starter publishes, must be defined by the published conventions; fixture-only
// data uses private keys.
func TestEveryPublicKeyInShippedFixturesIsDefined(t *testing.T) {
	defined := definedPublicKeys(t)
	dirs, err := os.ReadDir(repoFile(t, "tests", "fixtures", "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]string{} // key -> where
	note := func(key, where string) {
		if semconv.ValidKey(key) == nil && !strings.HasPrefix(key, "plugin.") {
			used[key] = where
		}
	}
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), "invalid-") {
			continue
		}
		raw, err := os.ReadFile(repoFile(t, "tests", "fixtures", "plugins", d.Name(), "plugin.json"))
		if err != nil {
			continue
		}
		var m struct {
			Components []struct {
				Key      string   `json:"key"`
				Inputs   []string `json:"inputs"`
				Discover struct {
					Inputs []string `json:"inputs"`
				} `json:"discover"`
			} `json:"components"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		for _, c := range m.Components {
			note(c.Key, d.Name())
			for _, in := range append(c.Inputs, c.Discover.Inputs...) {
				if parsed, err := plugin.ParseInput(in); err == nil && parsed.Source != plugin.SourceWork {
					note(parsed.Key, d.Name())
				}
			}
		}
	}
	// The context-suite Starter publishes these; they are literals in its source.
	note("github.pull_request", "context-suite starter")
	note("github.pull_request.number", "context-suite starter")

	for key, where := range used {
		if !defined[key] {
			t.Errorf("public key %q (used by %s) is not defined in semantic-conventions.md §5", key, where)
		}
	}
}

// TestEveryManifestRuleHasAViolatingFixture: each rule of the
// manifest contract is exercised by at least one fixture that violates it and
// is refused as plugin-invalid, and the runtime rule as plugin-install-failed.
func TestEveryManifestRuleHasAViolatingFixture(t *testing.T) {
	rules := []struct {
		rule    string
		fixture string
	}{
		{"M1 event must be a core event", "invalid-event"},
		{"M2 starters entries", "invalid-starters"},
		{"M3 malformed input / unknown work fact", "invalid-input"},
		{"M4 duplicate input key", "invalid-dup-input"},
		{"M5 linker key ownership", "invalid-key-owner"},
		{"M6 manual display_name", "invalid-manual"},
		{"unknown fields inside on/manual/discover", "invalid-unknown-field"},
	}
	for _, r := range rules {
		t.Run(r.rule, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtures.Prepare(t, r.fixture), "plugin.json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plugin.Parse(raw); err == nil {
				t.Fatalf("%s parsed; it must violate %q", r.fixture, r.rule)
			}
			// Through the install pipeline the same refusal is plugin-invalid (31).
			_, err = plugininstall.Install(t.TempDir(), &registry.Registry{}, fixtures.Prepare(t, r.fixture), plugininstall.Options{})
			if diag.Token(err) != diag.PluginInvalid.Token {
				t.Errorf("install token = %q, want %q (%v)", diag.Token(err), diag.PluginInvalid.Token, err)
			}
		})
	}

	t.Run("runtime not on PATH", func(t *testing.T) {
		_, err := plugininstall.Install(t.TempDir(), &registry.Registry{}, fixtures.Prepare(t, "invalid-runtime"), plugininstall.Options{})
		if diag.Token(err) != diag.PluginInstallFailed.Token {
			t.Errorf("token = %q, want %q (%v)", diag.Token(err), diag.PluginInstallFailed.Token, err)
		}
	})

	t.Run("M7 automatic without on is accepted", func(t *testing.T) {
		const manifest = `{"name":"m7","version":"1","components":[
			{"name":"l","role":"linker","key":"github.pull_request","entrypoint":"l","discover":{"automatic":true}}],
			"conventions":[]}`
		if _, err := plugin.Parse([]byte(manifest)); err != nil {
			t.Errorf("Parse: %v", err)
		}
	})
}
