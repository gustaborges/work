package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T, dir string, remotes map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for name, url := range remotes {
		if out, err := exec.Command("git", "-C", dir, "remote", "add", name, url).CombinedOutput(); err != nil {
			t.Fatalf("git remote add: %v\n%s", err, out)
		}
	}
}

func decode(t *testing.T, s string) []string {
	t.Helper()
	var resp struct {
		Matches []struct {
			RepoPath string `json:"repo_path"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		t.Fatalf("output not JSON: %v (%q)", err, s)
	}
	var paths []string
	for _, m := range resp.Matches {
		paths = append(paths, m.RepoPath)
	}
	return paths
}

func TestLocatorNameMatch(t *testing.T) {
	root := t.TempDir()
	initRepo(t, filepath.Join(root, "project"), nil)
	initRepo(t, filepath.Join(root, "other"), nil)

	var out strings.Builder
	in := `{"repository":{"name":"project"},"repository_roots":["` + root + `"]}`
	if err := run(strings.NewReader(in), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := decode(t, out.String())
	if len(got) != 1 || filepath.Base(got[0]) != "project" {
		t.Errorf("matches = %v, want one 'project'", got)
	}
}

func TestLocatorNameMatchTwoRepos(t *testing.T) {
	root := t.TempDir()
	initRepo(t, filepath.Join(root, "a", "project"), nil)
	initRepo(t, filepath.Join(root, "b", "project"), nil)

	var out strings.Builder
	in := `{"repository":{"name":"project"},"repository_roots":["` + root + `"]}`
	if err := run(strings.NewReader(in), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := decode(t, out.String())
	if len(got) != 2 {
		t.Errorf("matches = %v, want 2", got)
	}
	if strings.Contains(out.String(), "score") || strings.Contains(out.String(), "confidence") || strings.Contains(out.String(), "priority") {
		t.Errorf("output carries ranking keys: %s", out.String())
	}
}

func TestLocatorNoMatchAndEmptyRoots(t *testing.T) {
	root := t.TempDir()
	initRepo(t, filepath.Join(root, "project"), nil)

	var out strings.Builder
	if err := run(strings.NewReader(`{"repository":{"name":"nope"},"repository_roots":["`+root+`"]}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := decode(t, out.String()); len(got) != 0 {
		t.Errorf("matches = %v, want none", got)
	}
	if !strings.Contains(out.String(), `"matches":[]`) {
		t.Errorf("empty result should be an explicit empty array: %s", out.String())
	}

	out.Reset()
	if err := run(strings.NewReader(`{"repository":{"name":"project"},"repository_roots":[]}`), &out); err != nil {
		t.Fatalf("run (empty roots): %v", err)
	}
	if got := decode(t, out.String()); len(got) != 0 {
		t.Errorf("empty roots matches = %v", got)
	}
}

func TestLocatorFetchURLMatchNonOrigin(t *testing.T) {
	root := t.TempDir()
	const url = "https://example.com/team/thing.git"
	initRepo(t, filepath.Join(root, "checkout"), map[string]string{
		"origin":   "https://example.com/fork/thing.git",
		"upstream": url,
	})

	var out strings.Builder
	in := `{"repository":{"git_fetch_urls":["` + url + `"]},"repository_roots":["` + root + `"]}`
	if err := run(strings.NewReader(in), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := decode(t, out.String()); len(got) != 1 {
		t.Errorf("matches = %v, want 1 (upstream url)", got)
	}
}

func TestLocatorGarbageStdin(t *testing.T) {
	var out strings.Builder
	if err := run(strings.NewReader("xxx"), &out); err == nil {
		t.Error("run(garbage): want error")
	}
	if out.Len() != 0 {
		t.Errorf("wrote stdout on failure: %q", out.String())
	}
}

func TestLocatorUnreadableRoot(t *testing.T) {
	var out strings.Builder
	in := `{"repository":{"name":"x"},"repository_roots":["/no/such/path/here"]}`
	if err := run(strings.NewReader(in), &out); err == nil {
		t.Error("run(missing root): want error")
	}
}
