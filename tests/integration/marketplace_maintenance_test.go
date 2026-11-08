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

type updateRows struct {
	Updates []struct {
		ID                string  `json:"id"`
		Status            string  `json:"status"`
		CurrentVersion    *string `json:"current_version"`
		CandidateVersion  *string `json:"candidate_version"`
		CandidateRevision *string `json:"candidate_revision"`
		Error             *struct {
			Token string `json:"token"`
		} `json:"error"`
	} `json:"updates"`
}

func checkJSON(t *testing.T, h *f6Harness, want int, args ...string) updateRows {
	t.Helper()
	out, _ := h.run(t, h.root, want, append([]string{"plugin", "update", "--check", "--json"}, args...)...)
	var rows updateRows
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err, out)
	}
	return rows
}

func statusOf(rows updateRows) map[string]string {
	out := map[string]string{}
	for _, r := range rows.Updates {
		out[r.ID] = r.Status
	}
	return out
}

// publishVersion changes a fixture plugin's opaque version, and optionally its bytes.
func publishVersion(t *testing.T, src, plugin, from, to, tool string) {
	t.Helper()
	dir := filepath.Join(src, "plugins", plugin)
	path := filepath.Join(dir, "plugin.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next := strings.Replace(string(raw), `"version": "`+from+`"`, `"version": "`+to+`"`, 1)
	if next == string(raw) {
		t.Fatalf("version %s not found in %s", from, raw)
	}
	if err = os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	if tool != "" {
		if err = os.WriteFile(filepath.Join(dir, "bin", "tool"), []byte(tool), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func installedVersion(t *testing.T, h *f6Harness, id string) string {
	t.Helper()
	out, _ := h.run(t, h.root, 0, "plugin", "list", "--json")
	var inv struct {
		Installed []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"installed"`
	}
	if err := json.Unmarshal([]byte(out), &inv); err != nil {
		t.Fatal(err, out)
	}
	for _, p := range inv.Installed {
		if p.ID == id {
			return p.Version
		}
	}
	t.Fatalf("%s is not installed: %s", id, out)
	return ""
}

func TestRefreshAndCheckArePureAndUpdateAppliesTheReviewedCandidate(t *testing.T) {
	h := newAdminHarness(t)
	src := marketplaces.Copy(t, "multiple")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	h.run(t, h.root, 0, "plugin", "add", "alpha@demo")
	h.run(t, h.root, 0, "plugin", "add", "beta@demo")
	publishVersion(t, src, "alpha", "1.0.0", "2.0.0", "#!/bin/sh\nexit 2\n")

	// A check judges the recorded source and persists nothing, even though no
	// refresh has happened.
	before := adminTree(t, adminHome(h))
	rows := checkJSON(t, h, 0)
	if want := map[string]string{"alpha@demo": "update-available", "beta@demo": "unchanged"}; !reflect.DeepEqual(statusOf(rows), want) {
		t.Fatalf("check = %v", statusOf(rows))
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("a check changed the Work home")
	}
	if rows := checkJSON(t, h, 0, "-m", "demo"); len(rows.Updates) != 2 {
		t.Fatalf("an untargeted marketplace check covers every installed plugin of it: %+v", rows)
	}

	// Refresh changes discovery only.
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade", "demo")
	if v := installedVersion(t, h, "alpha@demo"); v != "1.0.0" {
		t.Fatalf("refresh changed an installation: %s", v)
	}

	out, _ := h.run(t, h.root, 0, "plugin", "update", "alpha@demo", "--yes")
	if !strings.Contains(out, "work: updated alpha@demo") {
		t.Fatalf("update output = %q", out)
	}
	if installedVersion(t, h, "alpha@demo") != "2.0.0" || installedVersion(t, h, "beta@demo") != "1.0.0" {
		t.Fatal("update touched the wrong plugin")
	}
	if rows := checkJSON(t, h, 0); !reflect.DeepEqual(statusOf(rows), map[string]string{"alpha@demo": "unchanged", "beta@demo": "unchanged"}) {
		t.Fatalf("after update: %v", statusOf(rows))
	}
}

func TestOneSourceRevisionServesEveryTargetOfABatch(t *testing.T) {
	h := newAdminHarness(t)
	dir, url := gitMarketplace(t, h, "multiple", "main")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", url)
	h.run(t, h.root, 0, "plugin", "add", "alpha@demo")
	h.run(t, h.root, 0, "plugin", "add", "beta@demo")
	publishVersion(t, dir, "alpha", "1.0.0", "2.0.0", "")
	publishVersion(t, dir, "beta", "1.0.0", "2.0.0", "")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "both")
	rows := checkJSON(t, h, 0)
	if len(rows.Updates) != 2 || rows.Updates[0].CandidateRevision == nil || *rows.Updates[0].CandidateRevision != *rows.Updates[1].CandidateRevision {
		t.Fatalf("rows = %+v", rows)
	}
	h.run(t, h.root, 0, "plugin", "update", "--all", "--yes")
	if installedVersion(t, h, "alpha@demo") != "2.0.0" || installedVersion(t, h, "beta@demo") != "2.0.0" {
		t.Fatal("update --all did not update both")
	}
}

func TestSameVersionBytesNeedAnExplicitConfirmedReinstall(t *testing.T) {
	h := newAdminHarness(t)
	src := marketplaces.Copy(t, "single")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	h.run(t, h.root, 0, "plugin", "add", "sample@solo")
	h.run(t, h.root, 0, "plugin", "disable", "sample@solo")
	if err := os.WriteFile(filepath.Join(src, "plugins", "sample", "bin", "tool"), []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if rows := checkJSON(t, h, 0); statusOf(rows)["sample@solo"] != "unchanged" {
		t.Fatalf("check = %v", statusOf(rows))
	}
	h.run(t, h.root, 0, "plugin", "update", "sample@solo", "--yes")
	h.run(t, h.root, 0, "plugin", "marketplace", "upgrade", "solo")
	// Replacing bytes is explicit and never silent.
	h.run(t, h.root, 2, "plugin", "add", "sample@solo")
	out, _ := h.run(t, h.root, 0, "plugin", "add", "sample@solo", "--yes")
	if !strings.Contains(out, "work: reinstalled sample@solo") {
		t.Fatalf("reinstall output = %q", out)
	}
	inv := readInventory(t, h, 0)
	if len(inv.Installed) != 1 || inv.Installed[0].Enabled {
		t.Fatalf("reinstall did not keep the disabled state: %+v", inv.Installed)
	}
	tool, _ := os.ReadFile(filepath.Join(adminHome(h), "plugins", "solo", "sample", "source", "bin", "tool"))
	if !strings.Contains(string(tool), "exit 7") {
		t.Fatalf("replacement bytes = %q", tool)
	}
}

func TestWithdrawalInvalidCandidatesAndBatchPrecedence(t *testing.T) {
	h := newAdminHarness(t)
	src := marketplaces.Copy(t, "multiple")
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	for _, p := range []string{"alpha@demo", "beta@demo", "gamma@demo"} {
		h.run(t, h.root, 0, "plugin", "add", p)
	}
	// gamma is withdrawn, beta turns invalid, alpha has a clean update.
	raw := `{"schema":1,"name":"demo","plugins":[{"name":"alpha","path":"plugins/alpha"},{"name":"beta","path":"plugins/beta"}]}`
	if err := os.WriteFile(filepath.Join(src, "marketplace.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	publishVersion(t, src, "alpha", "1.0.0", "2.0.0", "#!/bin/sh\nexit 2\n")
	if err := os.Remove(filepath.Join(src, "plugins", "beta", "bin", "tool")); err != nil {
		t.Fatal(err)
	}

	rows := checkJSON(t, h, 50)
	want := map[string]string{"alpha@demo": "update-available", "beta@demo": "failed", "gamma@demo": "withdrawn"}
	if !reflect.DeepEqual(statusOf(rows), want) {
		t.Fatalf("check = %v", statusOf(rows))
	}
	h.run(t, h.root, 50, "plugin", "update", "--all", "--yes")
	if installedVersion(t, h, "alpha@demo") != "2.0.0" {
		t.Fatal("an independent success did not survive a sibling failure")
	}
	if installedVersion(t, h, "gamma@demo") != "1.0.0" {
		t.Fatal("withdrawal changed an installed copy")
	}
}

func TestSourceRemovalIsOfflineAtomicAndDependencyAuthorized(t *testing.T) {
	h := newAdminHarness(t)
	src := writeLocatorCatalog(t)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", src)
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"))
	h.run(t, h.root, 0, "plugin", "add", "loc@team")
	h.run(t, h.root, 0, "plugin", "add", "sample@solo")
	h.run(t, h.root, 0, "plugin", "disable", "loc@team")
	h.run(t, h.root, 0, "repository", "policy", "replace", "loc@team/find")
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}

	// --yes confirms the set; it never authorizes the policy dependency.
	before := adminTree(t, adminHome(h))
	h.run(t, h.root, 52, "plugin", "marketplace", "remove", "team", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("a refused removal changed state")
	}
	h.run(t, h.root, 2, "plugin", "marketplace", "remove", "team") // not interactive: needs --yes
	h.run(t, h.root, 2, "plugin", "marketplace", "remove", "builtin", "--yes")
	h.run(t, h.root, 2, "plugin", "marketplace", "remove", "nowhere", "--yes")

	out, _ := h.run(t, h.root, 0, "plugin", "marketplace", "remove", "team", "--yes", "--remove-policy-references", "--json")
	var res struct {
		Name      string   `json:"name"`
		Result    string   `json:"result"`
		Removed   []string `json:"removed"`
		Committed bool     `json:"committed"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err, out)
	}
	if res.Name != "team" || res.Result != "removed" || !res.Committed || !reflect.DeepEqual(res.Removed, []string{"loc@team"}) {
		t.Fatalf("result = %+v", res)
	}
	if got := installedIDs(readInventory(t, h, 0)); !reflect.DeepEqual(got, []string{"sample@solo"}) {
		t.Fatalf("installed = %v", got)
	}
	if _, ok := listSources(t, h)["team"]; ok {
		t.Fatal("the source is still registered")
	}
	pol, _ := h.run(t, h.root, 0, "repository", "policy", "list", "--json")
	if strings.Contains(pol, "loc@team") {
		t.Fatalf("policy = %s", pol)
	}
	// Nothing re-registers or reinstalls it; the included provider is unaffected.
	if rows := listSources(t, h); rows["builtin"].CatalogStatus != "ready" {
		t.Fatalf("builtin = %+v", rows["builtin"])
	}
}

func TestUpdateGrammarIsValidatedBeforeAnyStateOrSource(t *testing.T) {
	h := newAdminHarness(t)
	for _, args := range [][]string{
		{"plugin", "update", "--check", "--all"}, {"plugin", "update", "--check", "--yes"},
		{"plugin", "update", "--check", "--remove-policy-references"},
		{"plugin", "update", "--all", "a@m"}, {"plugin", "update", "--all", "-m", "m"},
		{"plugin", "update", "--json"}, {"plugin", "update", "--all", "--json"},
		{"plugin", "update", "a"}, {"plugin", "update", "a@m", "-m", "m"}, {"plugin", "update", "a@m", "b", "-m", "m"},
		{"plugin", "update", "--yes"}, {"plugin", "update", "a@m"}, {"plugin", "update", "--all"},
		{"plugin", "update", "../x@m", "--yes"},
	} {
		h.run(t, h.root, 2, args...)
		if _, err := os.Stat(adminHome(h)); !os.IsNotExist(err) {
			t.Fatalf("%v created a Work home", args)
		}
	}
	// An unknown installed target is usage, not a retrieval attempt.
	h.run(t, h.root, 0, "plugin", "marketplace", "add", marketplaces.Copy(t, "single"))
	h.run(t, h.root, 2, "plugin", "update", "--check", "sample@solo")
	h.run(t, h.root, 2, "plugin", "update", "ghost@solo", "--yes")
}

func writeLocatorCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "plugins", "loc")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"loc","version":"1","conventions":[],"components":[{"name":"find","role":"repository-locator","entrypoint":"bin/tool","accepts":["name"]}]}`
	for path, body := range map[string]string{
		filepath.Join(root, "plugin.json"):     manifest,
		filepath.Join(dir, "marketplace.json"): `{"schema":1,"name":"team","plugins":[{"name":"loc","path":"plugins/loc"}]}`,
		filepath.Join(root, "bin", "tool"):     "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
