package contract

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The CLI/domain surface is the only publication path: results and inventory
// rows carry real composed identity, and the stable stdout carries no human
// receipt, frame, Git text or path of the managed copy.
func TestAddAndInventoryContracts(t *testing.T) {
	c := newWorkCLI(t)
	c.run(0, "plugin", "marketplace", "add", marketplaces.Copy(t, "multiple"))

	out, ui := c.run(0, "plugin", "add", "alpha@demo", "--json")
	if ui != "" {
		t.Errorf("stderr = %q", ui)
	}
	add := keys(t, out)
	want := []string{"committed", "content_digest", "id", "marketplace", "name", "result", "revision"}
	if got := sortedKeys(add); !reflect.DeepEqual(got, want) {
		t.Fatalf("add keys = %v, want %v", got, want)
	}
	if add["id"] != "alpha@demo" || add["name"] != "alpha" || add["marketplace"] != "demo" || add["result"] != "installed" || add["committed"] != true {
		t.Errorf("add = %s", out)
	}
	if strings.Contains(out, c.home) || strings.Contains(out, "work:") {
		t.Errorf("JSON leaks a managed path or a human receipt: %s", out)
	}

	out, _ = c.run(0, "plugin", "list", "--available", "--json")
	inv := keys(t, out)
	if got := sortedKeys(inv); !reflect.DeepEqual(got, []string{"available", "catalog_errors", "installed"}) {
		t.Fatalf("inventory keys = %v", got)
	}
	for _, k := range []string{"available", "catalog_errors", "installed"} {
		if rows, ok := inv[k].([]any); !ok || rows == nil {
			t.Errorf("%s must be a non-null array: %s", k, out)
		}
	}
	installed := inv["installed"].([]any)
	var row map[string]any
	for _, r := range installed {
		if m := r.(map[string]any); m["id"] == "alpha@demo" {
			row = m
		}
	}
	if row == nil {
		t.Fatalf("alpha@demo missing from %s", out)
	}
	for _, k := range []string{"id", "name", "marketplace", "enabled", "version", "content_digest", "inspection_status", "source_kind", "source", "revision", "components", "conventions"} {
		if _, ok := row[k]; !ok {
			t.Errorf("installed row lacks %q", k)
		}
	}
	if row["marketplace"] != "demo" || row["source_kind"] != "local" {
		t.Errorf("installed row has no real provenance: %v", row)
	}
	offered := map[string]any{}
	for _, r := range inv["available"].([]any) {
		m := r.(map[string]any)
		offered[m["id"].(string)] = m
	}
	if alpha := offered["alpha@demo"].(map[string]any); alpha["installed"] != true || alpha["enabled"] != true {
		t.Errorf("alpha offer = %v", alpha)
	}
	if beta := offered["beta@demo"].(map[string]any); beta["installed"] != false || beta["enabled"] != nil {
		t.Errorf("beta offer must be not installed with enabled null: %v", beta)
	}
	for _, k := range []string{"id", "name", "marketplace", "version", "revision", "description", "components", "conventions", "installed", "enabled"} {
		if _, ok := offered["beta@demo"].(map[string]any)[k]; !ok {
			t.Errorf("available row lacks %q", k)
		}
	}

	out, _ = c.run(0, "plugin", "remove", "alpha@demo", "--yes", "--json")
	rm := keys(t, out)
	if rm["result"] != "removed" || rm["committed"] != true || !reflect.DeepEqual(rm["removed"], []any{"alpha@demo"}) {
		t.Errorf("remove = %s", out)
	}
}

func TestReceiptsNameExactQualifiedIdentities(t *testing.T) {
	c := newWorkCLI(t)
	c.run(0, "plugin", "marketplace", "add", marketplaces.Copy(t, "multiple"))
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"plugin", "add", "alpha@demo"}, "work: installed alpha@demo\n"},
		{[]string{"plugin", "add", "alpha@demo"}, "work: unchanged alpha@demo\n"},
		{[]string{"plugin", "disable", "alpha@demo"}, "work: disabled alpha@demo\n"},
		{[]string{"plugin", "enable", "alpha", "-m", "demo"}, "work: enabled alpha@demo\n"},
		{[]string{"plugin", "remove", "alpha@demo", "--yes"}, "work: removed alpha@demo\n"},
	} {
		if out, _ := c.run(0, step.args...); out != step.want {
			t.Errorf("%v printed %q, want %q", step.args, out, step.want)
		}
	}
}

func TestOnlyCurrentPluginCommandsExist(t *testing.T) {
	c := newWorkCLI(t)
	out, _ := c.run(0, "plugin", "--help")
	for _, command := range []string{"add", "list", "remove", "update", "enable", "disable", "marketplace"} {
		if !strings.Contains(out, "\n  "+command+" ") {
			t.Errorf("plugin --help lacks %q:\n%s", command, out)
		}
	}
	for _, retired := range []string{"install", "uninstall"} {
		if strings.Contains(out, "\n  "+retired+" ") {
			t.Errorf("plugin --help advertises the retired %q", retired)
		}
	}
	before, _ := os.ReadDir(filepath.Dir(c.home))
	for _, args := range [][]string{{"plugin", "install", "x"}, {"plugin", "uninstall", "x@y"}, {"plugin", "add", "./dir"}, {"marketplace"}} {
		c.run(2, args...)
	}
	after, _ := os.ReadDir(filepath.Dir(c.home))
	if !slices.EqualFunc(before, after, func(a, b os.DirEntry) bool { return a.Name() == b.Name() }) {
		t.Error("a rejected command created state")
	}
}
