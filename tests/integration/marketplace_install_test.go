package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

type addResult struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Marketplace   string `json:"marketplace"`
	Revision      string `json:"revision"`
	ContentDigest string `json:"content_digest"`
	Result        string `json:"result"`
	Committed     bool   `json:"committed"`
}

type inventory struct {
	Installed []struct {
		ID               string `json:"id"`
		Enabled          bool   `json:"enabled"`
		InspectionStatus string `json:"inspection_status"`
		SourceKind       string `json:"source_kind"`
		Components       []struct {
			ID string `json:"id"`
		} `json:"components"`
		Conventions []string `json:"conventions"`
	} `json:"installed"`
	Available []struct {
		ID        string `json:"id"`
		Installed bool   `json:"installed"`
		Enabled   *bool  `json:"enabled"`
	} `json:"available"`
	CatalogErrors []struct {
		Marketplace string `json:"marketplace"`
		Error       string `json:"error"`
	} `json:"catalog_errors"`
}

func readInventory(t *testing.T, h *f6Harness, want int, args ...string) inventory {
	t.Helper()
	out, _ := h.run(t, h.root, want, append([]string{"plugin", "list", "--json"}, args...)...)
	var inv inventory
	if err := json.Unmarshal([]byte(out), &inv); err != nil {
		t.Fatal(err, out)
	}
	return inv
}

func installedIDs(inv inventory) []string {
	ids := []string{}
	for _, p := range inv.Installed {
		ids = append(ids, p.ID)
	}
	return ids
}

func addJSON(t *testing.T, h *f6Harness, args ...string) addResult {
	t.Helper()
	out, _ := h.run(t, h.root, 0, append([]string{"plugin", "add"}, append(args, "--json")...)...)
	var r addResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err, out)
	}
	return r
}

// One selected entry per add from local, Git and builtin offers; siblings stay
// offered but uninstalled, and the copies keep working after both authors'
// sources disappear.
func TestAddCopiesOnlyTheSelectedEntryFromEverySourceKind(t *testing.T) {
	h := newAdminHarness(t)
	local := marketplaces.Copy(t, "multiple")
	gitDir, gitURL := gitMarketplace(t, h, "single", "main")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", local)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", gitURL)

	a := addJSON(t, h, "alpha@demo")
	if a.Result != "installed" || a.ID != "alpha@demo" || !a.Committed || !strings.HasPrefix(a.ContentDigest, "sha256:") {
		t.Fatalf("local add = %+v", a)
	}
	g := addJSON(t, h, "sample@solo")
	if g.Result != "installed" || g.Revision != gittest.Git(t, gitDir, "rev-parse", "HEAD") {
		t.Fatalf("git add = %+v", g)
	}
	b := addJSON(t, h, "work-reference@builtin")
	if b.Result != "installed" {
		t.Fatalf("builtin add = %+v", b)
	}

	inv := readInventory(t, h, 0)
	if got := installedIDs(inv); !reflect.DeepEqual(got, []string{"work-reference@builtin", "alpha@demo", "sample@solo"}) {
		t.Fatalf("installed = %v", got)
	}
	for _, p := range inv.Installed {
		if !p.Enabled || p.InspectionStatus != "ready" {
			t.Errorf("%s: %+v", p.ID, p)
		}
	}

	if err := os.RemoveAll(local); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gitDir); err != nil {
		t.Fatal(err)
	}
	for _, p := range readInventory(t, h, 0).Installed {
		if p.InspectionStatus != "ready" {
			t.Fatalf("%s depends on its author source: %+v", p.ID, p)
		}
	}
	for _, sibling := range []string{"beta", "gamma"} {
		if _, err := os.Stat(filepath.Join(adminHome(h), "plugins", "demo", sibling)); !os.IsNotExist(err) {
			t.Fatalf("sibling %s was copied: %v", sibling, err)
		}
	}
	if again := addJSON(t, h, "alpha@demo"); again.Result != "unchanged" {
		t.Fatalf("repeat add = %+v", again)
	}
	pol, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if !strings.Contains(pol, `"policy": []`) {
		t.Fatalf("add changed policy: %s", pol)
	}
}

func TestAvailableViewIsStoredOffersWithInstalledState(t *testing.T) {
	h := newAdminHarness(t)
	src := marketplaces.Copy(t, "multiple")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	h.run(t, h.root, 0, "plugin", "add", "beta@demo")
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, adminHome(h))

	inv := readInventory(t, h, 0, "--available", "-m", "demo")
	state := map[string]string{}
	for _, a := range inv.Available {
		switch {
		case !a.Installed && a.Enabled == nil:
			state[a.ID] = "offered"
		case a.Installed && a.Enabled != nil && *a.Enabled:
			state[a.ID] = "enabled"
		default:
			state[a.ID] = "other"
		}
	}
	want := map[string]string{"alpha@demo": "offered", "beta@demo": "enabled", "gamma@demo": "offered"}
	if !reflect.DeepEqual(state, want) {
		t.Fatalf("available = %v", state)
	}
	if inv.Installed == nil || inv.CatalogErrors == nil || len(inv.CatalogErrors) != 0 {
		t.Fatalf("envelope arrays must be present and empty: %+v", inv)
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("browsing changed state")
	}
	h.run(t, h.root, 2, "plugin", "list", "--available")
	h.run(t, h.root, 2, "plugin", "list", "-m", "nowhere", "--json")
	if inv := readInventory(t, h, 0); len(inv.Available) != 0 {
		t.Fatalf("available without --available: %+v", inv.Available)
	}
}

func TestMissingStoredOfferIsReportedAndExitsSixty(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "multiple"))
	h.run(t, h.root, 0, "plugin", "add", "alpha@demo")
	if err := os.Remove(filepath.Join(adminHome(h), "state", "marketplace-catalogs.json")); err != nil {
		t.Fatal(err)
	}
	inv := readInventory(t, h, 60, "--available")
	if len(inv.CatalogErrors) != 1 || inv.CatalogErrors[0].Marketplace != "demo" || inv.CatalogErrors[0].Error != "marketplace-entry-unavailable" {
		t.Fatalf("catalog errors = %+v", inv.CatalogErrors)
	}
	if got := installedIDs(inv); !reflect.DeepEqual(got, []string{"alpha@demo"}) {
		t.Fatalf("valid rows were dropped: %v", got)
	}
	h.run(t, h.root, 60, "plugin", "add", "beta@demo")
}

// Retired syntax and non-selector operands fail as usage before any state is
// created or any source is contacted.
func TestRetiredAndMalformedFormsFailBeforeAcquisition(t *testing.T) {
	h := newAdminHarness(t)
	src := marketplaces.Copy(t, "single")
	for _, args := range [][]string{
		{"plugin", "install", src}, {"plugin", "install", "--embedded"}, {"plugin", "install", "--link", src},
		{"plugin", "uninstall", "sample@solo"}, {"plugin", "update"}, {"marketplace", "add", src},
		{"plugin", "add", src}, {"plugin", "add", filepath.Join(src, "plugins", "sample", "plugin.json")},
		{"plugin", "add", "https://example.invalid/p.git"}, {"plugin", "add", "sample"}, {"plugin", "add", "sample@solo", "-m", "solo"},
		{"plugin", "add", "sample@solo", "--as", "alias"}, {"plugin", "add", "sample@solo", "--link"}, {"plugin", "add", "sample@solo", "--embedded"},
		{"plugin", "add", "a@m", "b@m"}, {"plugin", "remove", "sample"}, {"plugin", "remove", "a@m", "b", "-m", "m"},
		{"plugin", "enable", "sample"}, {"plugin", "disable", "a@m", "-m", "m"},
	} {
		h.run(t, h.root, 2, args...)
		if _, err := os.Stat(adminHome(h)); !os.IsNotExist(err) {
			t.Fatalf("%v created a Work home", args)
		}
	}
}

func TestHomonymProvidersAreSelectedAndRemovedIndependently(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "homonym-alpha"))
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "homonym-beta"))
	h.run(t, h.root, 0, "plugin", "add", "sample@alpha-market")
	h.run(t, h.root, 0, "plugin", "add", "sample", "-m", "beta-market")

	inv := readInventory(t, h, 0)
	if got := installedIDs(inv); !reflect.DeepEqual(got, []string{"sample@alpha-market", "sample@beta-market"}) {
		t.Fatalf("installed = %v", got)
	}
	for _, p := range inv.Installed {
		want := strings.TrimPrefix(p.ID, "sample@") + "/"
		if len(p.Components) != 1 || !strings.Contains(p.Components[0].ID, p.ID+"/") || len(p.Conventions) != 1 || !strings.HasPrefix(p.Conventions[0], p.ID+"/") {
			t.Fatalf("%s did not keep its own contributions (%s): %+v", p.ID, want, p)
		}
	}

	h.run(t, h.root, 0, "plugin", "disable", "sample", "-m", "alpha-market")
	for _, p := range readInventory(t, h, 0).Installed {
		if p.Enabled != (p.ID == "sample@beta-market") {
			t.Fatalf("toggling alpha-market affected %s: %+v", p.ID, p)
		}
	}
	h.run(t, h.root, 0, "plugin", "remove", "sample@alpha-market", "--yes")
	if got := installedIDs(readInventory(t, h, 0)); !reflect.DeepEqual(got, []string{"sample@beta-market"}) {
		t.Fatalf("after removing alpha-market's copy: %v", got)
	}
	// Removal never touched the source: the offer is still browsable and reinstallable.
	if rows := listSources(t, h); rows["alpha-market"].CatalogStatus != "ready" {
		t.Fatalf("source = %+v", rows["alpha-market"])
	}
	h.run(t, h.root, 0, "plugin", "add", "sample@alpha-market")
}

func TestConventionSelectionNeverSubstitutesAHomonym(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "homonym-alpha"))
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "homonym-beta"))
	h.run(t, h.root, 0, "plugin", "add", "sample@alpha-market")
	h.run(t, h.root, 0, "plugin", "add", "sample@beta-market")
	repo := gittest.Repo(t)

	_, ui := h.run(t, repo, 2, "convention", "set", "freeform")
	if !strings.Contains(ui, "more than one provider") {
		t.Fatalf("a bare name shared by several providers must be refused: %s", ui)
	}
	out, _ := h.run(t, repo, 0, "convention", "set", "sample@beta-market/freeform")
	if !strings.Contains(out, "sample@beta-market/freeform") {
		t.Fatalf("set = %q", out)
	}
	shown, _ := h.run(t, repo, 0, "convention", "show", "--json")
	if !strings.Contains(shown, `"convention": "sample@beta-market/freeform"`) || !strings.Contains(shown, `"available": true`) {
		t.Fatalf("show = %s", shown)
	}

	// Disabling the chosen provider leaves the choice remembered but unavailable;
	// the other provider's equally named convention never takes over.
	h.run(t, h.root, 0, "plugin", "disable", "sample@beta-market")
	shown, _ = h.run(t, repo, 0, "convention", "show", "--json")
	if !strings.Contains(shown, `"convention": "sample@beta-market/freeform"`) || !strings.Contains(shown, `"available": false`) {
		t.Fatalf("show after disable = %s", shown)
	}
}

func TestRemovingTheIncludedReferenceIsDeliberateAndExplicitAddRestoresWithoutPolicy(t *testing.T) {
	h := newAdminHarness(t)
	h.run(t, h.root, 0, "plugin", "add", "work-reference@builtin")
	h.run(t, h.root, 0, "plugin", "remove", "work-reference@builtin", "--yes")
	if got := installedIDs(readInventory(t, h, 0)); len(got) != 0 {
		t.Fatalf("installed = %v", got)
	}
	// Later commands neither resurrect it nor seed policy.
	h.run(t, h.root, 0, "plugin", "marketplace", "list", "--json")
	if got := installedIDs(readInventory(t, h, 0)); len(got) != 0 {
		t.Fatalf("a later command resurrected the default: %v", got)
	}
	if r := addJSON(t, h, "work-reference@builtin"); r.Result != "installed" {
		t.Fatalf("restore = %+v", r)
	}
	pol, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if !strings.Contains(pol, `"policy": []`) {
		t.Fatalf("restoring inserted policy: %s", pol)
	}
}
