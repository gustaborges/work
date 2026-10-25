package contract

import (
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconfig"
	"testing"
)

func TestAdminRepositoryStateViews(t *testing.T) {
	cfg := config.Default()
	reg := &registry.Registry{}
	if repoconfig.ListPolicy(cfg, reg) == nil || repoconfig.ListLocators(cfg, reg) == nil || repoconfig.ListRoots(cfg) == nil {
		t.Fatal("null collection")
	}
	off := false
	reg.Packages = []registry.Package{{Alias: "a", Enabled: &off}}
	reg.Components = []registry.Component{{Alias: "a", Name: "clone", Role: registry.RoleRepositoryLocator}}
	cfg.RepositoryResolution.Locators = []string{"ghost/clone", "a/clone"}
	rows := repoconfig.ListPolicy(cfg, reg)
	if len(rows) != 2 || rows[0].Position != 1 || rows[0].Installed || rows[1].Position != 2 || !rows[1].Installed || rows[1].Enabled || rows[1].Available || *rows[1].UnavailableReason != "package-disabled" {
		t.Fatal(rows)
	}
}
