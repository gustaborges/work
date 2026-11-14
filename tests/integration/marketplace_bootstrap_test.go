package integration

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

const referenceLocator = "work-reference@builtin/filesystem-repository-locator"

func lifecycleOf(t *testing.T, h *f6Harness) adminstate.Lifecycle {
	t.Helper()
	s, err := adminstate.Read(context.Background(), h.home())
	if err != nil {
		t.Fatal(err)
	}
	return *s.Lifecycle
}

func policyList(t *testing.T, h *f6Harness) string {
	t.Helper()
	out, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	return out
}

func firstUse(t *testing.T, h *f6Harness) {
	t.Helper()
	if err := bootstrap.EnsureSeed(h.home()); err != nil {
		t.Fatal(err)
	}
}

// The lifecycle matrix of the default reference package, driven through the
// real binary for every user-visible choice and through the same setup entry
// the start command calls for first use.
func TestRegistrationBeforeFirstUseDoesNotHandlePolicyAndSetupSeedsOnce(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"))
	if l := lifecycleOf(t, h); l.Initialized || l.PolicyHandled || l.Removed {
		t.Fatalf("registration changed the lifecycle: %+v", l)
	}
	if strings.Contains(policyList(t, h), "work-reference") {
		t.Fatal("registration inserted policy")
	}

	firstUse(t, h)
	if l := lifecycleOf(t, h); !l.Initialized || !l.PolicyHandled || l.Removed {
		t.Fatalf("setup lifecycle = %+v", l)
	}
	if got := installedIDs(readInventory(t, h, 0)); len(got) != 1 || got[0] != "work-reference@builtin" {
		t.Fatalf("installed = %v", got)
	}
	if !strings.Contains(policyList(t, h), referenceLocator) {
		t.Fatalf("setup did not seed the default policy: %s", policyList(t, h))
	}

	before := adminTree(t, adminHome(h))
	firstUse(t, h)
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade")
	if after := adminTree(t, adminHome(h)); !equalTrees(before, after) {
		t.Fatal("repeating setup or refreshing sources changed state")
	}
}

func equalTrees(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestExplicitPolicyChoiceSurvivesFirstSetup(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", writeLocatorCatalog(t))
	h.run(t, h.root, 0, "plugin", "add", "loc@team")
	h.run(t, h.root, 0, "repository", "policy", "replace", "loc@team/find")
	if l := lifecycleOf(t, h); !l.PolicyHandled || l.Initialized {
		t.Fatalf("an explicit choice must mark policy handled only: %+v", l)
	}

	firstUse(t, h)
	policy := policyList(t, h)
	if !strings.Contains(policy, "loc@team/find") || strings.Contains(policy, "work-reference") {
		t.Fatalf("setup overwrote an explicit policy: %s", policy)
	}
	if got := installedIDs(readInventory(t, h, 0)); len(got) != 2 {
		t.Fatalf("the default must still be installed: %v", got)
	}
}

func TestExplicitAddOfTheReferenceHandlesPolicyWithoutInsertingLocators(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "add", "work-reference@builtin")
	if l := lifecycleOf(t, h); !l.Initialized || !l.PolicyHandled {
		t.Fatalf("lifecycle = %+v", l)
	}
	if strings.Contains(policyList(t, h), "work-reference") {
		t.Fatal("explicit add inserted a Locator")
	}
	before := adminTree(t, adminHome(h))
	firstUse(t, h)
	if !equalTrees(before, adminTree(t, adminHome(h))) {
		t.Fatal("setup after an explicit add changed state")
	}
}

func TestDisabledAndRemovedDefaultsAreNeverRestoredAutomatically(t *testing.T) {
	h := newAdminHarness(t)
	firstUse(t, h)
	h.run(t, h.root, 0, "plugin", "disable", "work-reference@builtin")
	firstUse(t, h)
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade")
	inv := readInventory(t, h, 0)
	if len(inv.Installed) != 1 || inv.Installed[0].Enabled {
		t.Fatalf("a disabled default was restored: %+v", inv.Installed)
	}

	// Removal needs separate authorization for the seeded policy reference.
	h.run(t, h.root, 52, "plugin", "remove", "work-reference@builtin", "--yes")
	h.run(t, h.root, 0, "plugin", "remove", "work-reference@builtin", "--yes", "--remove-policy-references")
	if l := lifecycleOf(t, h); !l.Removed {
		t.Fatalf("lifecycle = %+v", l)
	}
	firstUse(t, h)
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade")
	if got := installedIDs(readInventory(t, h, 0)); len(got) != 0 {
		t.Fatalf("a removed default was resurrected: %v", got)
	}
	if strings.Contains(policyList(t, h), "work-reference") {
		t.Fatal("a removed default's policy came back")
	}
}

// Damaged installed content is surfaced, never silently repaired or reseeded;
// an explicit reinstall is the recovery and still does not reseed policy.
func TestDamagedDefaultIsReportedAndOnlyExplicitReinstallRepairsIt(t *testing.T) {
	h := newAdminHarness(t)
	firstUse(t, h)
	h.run(t, h.root, 0, "repository", "policy", "remove", referenceLocator)
	root, _ := h.home().PackageRoot(pluginid.Provider{Marketplace: pluginid.Builtin, Plugin: adminstate.ReferencePlugin})
	entries, _ := os.ReadDir(root + "/source")
	if len(entries) == 0 {
		t.Fatal("no installed content")
	}
	if err := os.RemoveAll(root + "/source/" + entries[0].Name()); err != nil {
		t.Fatal(err)
	}

	firstUse(t, h)
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade")
	inv := readInventory(t, h, 0)
	if len(inv.Installed) != 1 || inv.Installed[0].InspectionStatus == "ready" {
		t.Fatalf("damage was hidden: %+v", inv.Installed)
	}

	h.run(t, h.root, 2, "plugin", "add", "work-reference@builtin") // replacing needs --yes
	h.run(t, h.root, 0, "plugin", "add", "work-reference@builtin", "--yes")
	inv = readInventory(t, h, 0)
	if inv.Installed[0].InspectionStatus != "ready" {
		t.Fatalf("reinstall did not repair: %+v", inv.Installed)
	}
	if strings.Contains(policyList(t, h), "work-reference") {
		t.Fatal("repair reseeded policy")
	}
}

func TestMalformedLifecycleIsRejectedUntouchedNotConverted(t *testing.T) {
	h := newAdminHarness(t)
	firstUse(t, h)
	path := h.home().LifecycleFile()
	bad := []byte(`{"initialized":true,"seed_origin":"embedded"}`)
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	h.run(t, h.root, 54, "plugin", "list", "--json")
	h.run(t, h.root, 54, "plugin", "disable", "work-reference@builtin")
	if got, _ := os.ReadFile(path); string(got) != string(bad) {
		t.Fatalf("invalid state was rewritten: %s", got)
	}
}
