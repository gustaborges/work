package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

// workCLI builds the binary once per test and runs it against a disposable home.
type workCLI struct {
	t    *testing.T
	bin  string
	home string
}

func newWorkCLI(t *testing.T) *workCLI {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "work"+exeSuffix())
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	return &workCLI{t: t, bin: bin, home: filepath.Join(t.TempDir(), "home")}
}

func (c *workCLI) run(want int, args ...string) (stdout, stderr string) {
	c.t.Helper()
	cmd := exec.Command(c.bin, args...)
	cmd.Env = append(os.Environ(), "WORK_HOME="+c.home, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			c.t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	if code != want {
		c.t.Fatalf("work %v exited %d, want %d\nstdout: %s\nstderr: %s", args, code, want, out.String(), errb.String())
	}
	return out.String(), errb.String()
}

func keys(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, raw)
	}
	return m
}

func TestMarketplaceCommandResultsAndErrorTable(t *testing.T) {
	c := newWorkCLI(t)
	src := marketplaces.Copy(t, "multiple")

	// Source add: name/result/revision (+committed).
	out, ui := c.run(0, "plugin", "marketplace", "add", src, "--json")
	add := keys(t, out)
	for _, k := range []string{"name", "result", "revision", "committed"} {
		if _, ok := add[k]; !ok {
			t.Errorf("add JSON missing %q: %s", k, out)
		}
	}
	if add["name"] != "demo" || add["result"] != "registered" || ui != "" {
		t.Errorf("add = %s (stderr %q)", out, ui)
	}
	out, _ = c.run(0, "plugin", "marketplace", "add", src, "--json")
	if again := keys(t, out); again["result"] != "unchanged" {
		t.Errorf("identical add = %s", out)
	}

	// Marketplace list: an array with exactly the contracted keys, nulls for unknowns.
	out, _ = c.run(0, "plugin", "marketplace", "list", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("list = %s (%v)", out, err)
	}
	want := []string{"name", "display_name", "source_kind", "source", "branch", "revision", "plugin_count", "catalog_status", "installed_count", "enabled_count"}
	for _, r := range rows {
		if len(r) != len(want) {
			t.Errorf("row has %d keys, want %d: %v", len(r), len(want), r)
		}
		for _, k := range want {
			if _, ok := r[k]; !ok {
				t.Errorf("row missing %q: %v", k, r)
			}
		}
	}
	if rows[0]["name"] != "builtin" || rows[1]["name"] != "demo" || rows[1]["branch"] != nil || rows[1]["source_kind"] != "local" {
		t.Errorf("rows = %v", rows)
	}

	// Upgrade: a results array by source with nullable revision/error.
	out, _ = c.run(0, "plugin", "marketplace", "upgrade", "--json")
	var up struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &up); err != nil || len(up.Results) != 2 {
		t.Fatalf("upgrade = %s (%v)", out, err)
	}
	for _, r := range up.Results {
		for _, k := range []string{"name", "status", "revision", "error", "committed"} {
			if _, ok := r[k]; !ok {
				t.Errorf("result missing %q: %v", k, r)
			}
		}
		if r["status"] != "unchanged" || r["error"] != nil {
			t.Errorf("result = %v", r)
		}
	}

	// Human receipts are single lines on stdout; stderr stays empty.
	out, ui = c.run(0, "plugin", "marketplace", "upgrade", "demo")
	if out != "work: unchanged demo ("+up.Results[1]["revision"].(string)+")\n" || ui != "" {
		t.Errorf("receipt = %q / %q", out, ui)
	}

	// The error table: categories 57–61 and usage/2 with their stable tokens.
	for _, tc := range []struct {
		args  []string
		code  int
		token string
	}{
		{[]string{"plugin", "marketplace", "add", marketplaces.Copy(t, "invalid-schema")}, 57, "marketplace-invalid"},
		{[]string{"plugin", "marketplace", "add", marketplaces.Copy(t, "no-catalog")}, 57, "marketplace-invalid"},
		{[]string{"plugin", "marketplace", "add", marketplaces.Copy(t, "invalid-reserved-namespace")}, 58, "marketplace-name-conflict"},
		{[]string{"plugin", "marketplace", "add", marketplaces.Copy(t, "multiple")}, 58, "marketplace-name-conflict"},
		{[]string{"plugin", "marketplace", "add", filepath.Join(t.TempDir(), "gone")}, 59, "marketplace-source-unavailable"},
		{[]string{"plugin", "marketplace", "add", marketplaces.Copy(t, "single"), "--branch", "main"}, 2, "usage"},
		{[]string{"plugin", "marketplace", "upgrade", "no-such-source"}, 2, "usage"},
		{[]string{"plugin", "marketplace", "list", "extra"}, 2, "usage"},
	} {
		out, ui := c.run(tc.code, tc.args...)
		if out != "" || !strings.Contains(ui, "error: "+tc.token+":") || strings.Count(ui, "error:") != 1 {
			t.Errorf("%v: stdout %q stderr %q", tc.args, out, ui)
		}
	}

	// A failing refresh reports per-source results on stdout and one aggregate on stderr (61).
	if err := os.WriteFile(filepath.Join(src, "marketplace.json"), []byte(`{"schema":99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, ui = c.run(61, "plugin", "marketplace", "upgrade", "--json")
	var failed struct {
		Results []struct {
			Name, Status string
			Error        *struct{ Token string }
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &failed); err != nil {
		t.Fatal(err, out)
	}
	got := map[string]string{}
	for _, r := range failed.Results {
		got[r.Name] = r.Status
	}
	if got["demo"] != "failed" || got["builtin"] != "unchanged" || !strings.Contains(ui, "error: marketplace-upgrade-failed:") {
		t.Errorf("results %v / %s", got, ui)
	}
	if strings.Contains(out+ui, src) {
		t.Errorf("output leaks the local source path:\n%s\n%s", out, ui)
	}
}

func TestMarketplaceStableOutputCarriesNoGitTextOrSecrets(t *testing.T) {
	c := newWorkCLI(t)
	out, ui := c.run(59, "plugin", "marketplace", "add", "https://alice:s3cret@127.0.0.1:1/team/plugins.git?token=hidden", "--branch", "main")
	if out != "" {
		t.Errorf("failed add wrote to stdout: %q", out)
	}
	for _, leak := range []string{"s3cret", "alice", "hidden", "127.0.0.1", "fatal:", "git "} {
		if strings.Contains(ui, leak) {
			t.Errorf("stderr leaks %q: %s", leak, ui)
		}
	}
	// The mutation preflight may create lock scaffolding, but no configuration,
	// discovery or capture state is published by a failed registration.
	for _, f := range []string{"config/work.json", "state/marketplace-catalogs.json", "state/registry.json"} {
		if _, err := os.Stat(filepath.Join(c.home, filepath.FromSlash(f))); !os.IsNotExist(err) {
			t.Errorf("a failed registration published %s", f)
		}
	}
	// Reads on a fresh home never create it.
	fresh := newWorkCLI(t)
	fresh.run(0, "plugin", "marketplace", "list")
	fresh.run(0, "plugin", "list", "--json")
	if _, err := os.Stat(fresh.home); !os.IsNotExist(err) {
		t.Error("pure reads created the home")
	}
}

func TestRetiredPublicationSyntaxIsUsageBeforeAnyStateOrRetrieval(t *testing.T) {
	c := newWorkCLI(t)
	for _, args := range [][]string{
		{"plugin", "install", "./plugin"}, {"plugin", "install", "--embedded"}, {"plugin", "install", "x", "--link"}, {"plugin", "install", "x", "--as", "y"},
		{"plugin", "update"}, {"plugin", "update", "--check"}, {"marketplace", "add", "x"}, {"plugin", "marketplace", "remove", "x"},
	} {
		_, ui := c.run(2, args...)
		if !strings.Contains(ui, "error: usage:") {
			t.Errorf("%v: %s", args, ui)
		}
	}
	if _, err := os.Stat(c.home); !os.IsNotExist(err) {
		t.Error("retired syntax created state")
	}
}
