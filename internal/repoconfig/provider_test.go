package repoconfig

import (
	"slices"
	"testing"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

// otherMarketplace returns c re-homed to a homonymous provider in marketplace "other".
func otherMarketplace(c registry.Component) registry.Component {
	c.Provider = pluginid.Provider{Marketplace: "other", Plugin: c.Provider.Plugin}
	return c
}

func policyOf(refs ...string) *config.Config {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = refs
	return cfg
}

func reasonOf(t *testing.T, reg *registry.Registry, ref string) string {
	t.Helper()
	for _, e := range ListPolicy(policyOf(ref), reg) {
		if e.Ref == ref {
			if e.UnavailableReason == nil {
				return ""
			}
			return *e.UnavailableReason
		}
	}
	t.Fatalf("%s is not listed", ref)
	return ""
}

// Policy entries are fully qualified, so a removed provider's reference stays
// listed and unavailable; a homonym in another marketplace never claims it.
func TestRetainedPolicyReferenceIsNotClaimedByAHomonym(t *testing.T) {
	const ref = "a@m/one"
	owner := locatorComponent("a", "one")
	if got := reasonOf(t, regOf(nil, owner), ref); got != "" {
		t.Fatalf("installed owner: reason %q", got)
	}
	homonym := otherMarketplace(owner)
	if got := reasonOf(t, regOf(nil, homonym), ref); got != "package-missing" {
		t.Fatalf("a homonym in another marketplace satisfied the reference: %q", got)
	}
	if got := reasonOf(t, regOf(nil, homonym, owner), ref); got != "" {
		t.Fatalf("owner alongside a homonym: %q", got)
	}
}

func TestDisablingOneHomonymLeavesTheOthersReferenceAvailable(t *testing.T) {
	a, b := locatorComponent("a", "one"), otherMarketplace(locatorComponent("a", "one"))
	reg := regOf(nil, a, b)
	pkg, _ := reg.PackageByProvider(b.Provider)
	pkg.Enabled = false
	reg.UpsertPackage(pkg)
	if got := reasonOf(t, reg, "a@m/one"); got != "" {
		t.Fatalf("enabled owner became unavailable: %q", got)
	}
	if got := reasonOf(t, reg, "a@other/one"); got != "package-disabled" {
		t.Fatalf("disabled owner reason = %q", got)
	}
}

func TestReplacingPolicyKeepsQualifiedRetainedEntriesOfRemovedProviders(t *testing.T) {
	cfg := newCfg("")
	cfg.RepositoryResolution.Locators = []string{"gone@old/find"}
	reg := regOf(nil, locatorComponent("a", "one"))
	if err := ReplacePolicy(cfg, reg, []string{"gone@old/find", "a@m/one"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.RepositoryResolution.Locators, []string{"gone@old/find", "a@m/one"}) {
		t.Fatalf("locators = %v", cfg.RepositoryResolution.Locators)
	}
	// A bare name never resolves to a retained entry of a provider that is not installed here.
	wantToken(t, ReplacePolicy(cfg, reg, []string{"nosuch"}), diag.Usage)
}

func TestBareLocatorNameNeverPicksAmongHomonyms(t *testing.T) {
	a := locatorComponent("a", "one")
	reg := regOf(nil, a, otherMarketplace(a))
	cfg := newCfg("")
	wantToken(t, AddPolicy(cfg, reg, "one", "", ""), diag.Usage)
	if len(cfg.RepositoryResolution.Locators) != 0 {
		t.Fatalf("an ambiguous add changed policy: %v", cfg.RepositoryResolution.Locators)
	}
	if err := AddPolicy(cfg, reg, "a@other/one", "", ""); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.RepositoryResolution.Locators, []string{"a@other/one"}) {
		t.Fatalf("locators = %v", cfg.RepositoryResolution.Locators)
	}
}
