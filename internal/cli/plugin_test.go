package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPluginExplicitHelpExitZero(t *testing.T) {
	out, _, code := runWork(t, "plugin", "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"list", "marketplace", "enable", "disable", "uninstall"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
	for _, retired := range []string{"  install ", "  update "} {
		if strings.Contains(out, retired) {
			t.Errorf("help advertises a retired command %q:\n%s", retired, out)
		}
	}
}

func TestPluginJSONOnParentIsUsage(t *testing.T) {
	_, _, code := runWork(t, "plugin", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestPluginRetiredCommandsAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"plugin", "install", "./x"}, {"plugin", "install", "--embedded"}, {"plugin", "update", "--check"}, {"plugin", "update", "--all"},
		{"marketplace", "add", "x"},
	} {
		home := t.TempDir()
		_, errb, code := runWorkHome(t, home, args...)
		if code != 2 || !strings.Contains(errb, "usage") {
			t.Errorf("%v: exit = %d, stderr %q", args, code, errb)
		}
	}
}

func TestPluginListEmptyHomeIsPureAndUsesTheEnvelope(t *testing.T) {
	out, _, code := runWork(t, "plugin", "list")
	if code != 0 || out != "" {
		t.Fatalf("plugin list on an empty home: exit %d, %q", code, out)
	}
	out, _, code = runWork(t, "plugin", "list", "--json")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	for _, k := range []string{"installed", "available", "catalog_errors"} {
		if v, ok := env[k].([]any); !ok || len(v) != 0 {
			t.Errorf("%s must be present, non-null and empty: %v", k, env[k])
		}
	}
}
