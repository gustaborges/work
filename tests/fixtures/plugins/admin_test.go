package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/plugin"
)

func TestAdminNativeVariants(t *testing.T) {
	dir := AdminVariant(t, "starter", "linker", "importer")
	b, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "release-blue" || len(m.Components) != 4 || len(m.Conventions) != 1 {
		t.Fatal("wrong fixture")
	}
	for _, c := range m.Components {
		if c.IsFallbackStarter() {
			t.Fatal("admin fixture must not add fallback")
		}
	}
	Revise(t, dir, "release-green", nil)
	b, err = os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err = plugin.Parse(b)
	if err != nil || m.Version != "release-green" {
		t.Fatal(err)
	}
}
