package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every offered mutation uses shared direct handlers or the reviewed service;
// hubs must never become an additional storage writer or plugin executor.
func TestAdminHubServiceBoundary(t *testing.T) {
	for _, name := range []string{"plugin_hub.go", "repository_hub.go", "convention.go"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "cli", name))
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		for _, forbidden := range []string{".Save(", "exec.Command(", "os.WriteFile(", "os.RemoveAll("} {
			if strings.Contains(s, forbidden) {
				t.Errorf("%s bypasses application services: %s", name, forbidden)
			}
		}
		if !strings.Contains(s, "hubChoice(") {
			t.Errorf("%s bypasses shared wizard navigation", name)
		}
	}
}
