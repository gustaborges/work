package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdminUpdateGrammarAndEmpty(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{{"--check", "--all"}, {"--check", "--yes"}, {"--check", "--remove-policy-references"}, {"--all", "a", "--yes"}, {"--yes"}, {"a"}, {"a", "--yes", "--json"}} {
		out, ui, code := runWorkHome(t, home, append([]string{"plugin", "update"}, args...)...)
		if code != 2 || out != "" || !strings.Contains(ui, "usage") {
			t.Fatalf("%v: %d %s %s", args, code, out, ui)
		}
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatal("invalid grammar created home")
		}
	}
	out, ui, code := runWorkHome(t, home, "plugin", "update", "--check", "--json")
	var v map[string][]any
	if code != 0 || ui != "" || json.Unmarshal([]byte(out), &v) != nil || !reflect.DeepEqual(v["updates"], []any{}) {
		t.Fatal(out, ui, code)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("empty check created home")
	}
}
