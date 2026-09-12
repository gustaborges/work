package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHappy(t *testing.T) {
	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"/tmp/x/repo"}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp struct {
		Repository struct {
			Path string `json:"path"`
		} `json:"repository"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("output not JSON: %v (%q)", err, out.String())
	}
	// filepath.Abs leaves an absolute POSIX path alone but adds a drive prefix
	// on Windows.
	want, _ := filepath.Abs("/tmp/x/repo")
	if resp.Repository.Path != want {
		t.Errorf("path = %q, want %q", resp.Repository.Path, want)
	}
}

func TestRunRelativeBecomesAbsolute(t *testing.T) {
	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"./repo"}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp struct {
		Repository struct {
			Path string `json:"path"`
		} `json:"repository"`
	}
	_ = json.Unmarshal([]byte(out.String()), &resp)
	if !filepath.IsAbs(resp.Repository.Path) {
		t.Errorf("path = %q, want absolute", resp.Repository.Path)
	}
}

func TestRunExtraFieldsIgnored(t *testing.T) {
	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"/r","x":1}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRunBareTokenEmitsName(t *testing.T) {
	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"payments"}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp struct {
		Repository struct {
			Path string `json:"path"`
			Name string `json:"name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("output not JSON: %v (%q)", err, out.String())
	}
	if resp.Repository.Path != "" {
		t.Errorf("path = %q, want empty for a bare token", resp.Repository.Path)
	}
	if resp.Repository.Name != "payments" {
		t.Errorf("name = %q, want payments", resp.Repository.Name)
	}
}

func TestRunClassifiesPathLikeArguments(t *testing.T) {
	cases := map[string]struct {
		arg      string
		wantPath bool
	}{
		"contains slash":      {"a/b", true},
		"dot prefix":          {"./x", true},
		"dotdot prefix":       {"../x", true},
		"tilde prefix":        {"~/x", true},
		"bare name":           {"payments", false},
		"bare name with dash": {"my-project", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out strings.Builder
			if err := run(strings.NewReader(`{"arg":"`+tc.arg+`"}`), &out); err != nil {
				t.Fatalf("run: %v", err)
			}
			var resp struct {
				Repository struct {
					Path string `json:"path"`
					Name string `json:"name"`
				} `json:"repository"`
			}
			json.Unmarshal([]byte(out.String()), &resp)
			gotPath := resp.Repository.Path != ""
			if gotPath != tc.wantPath {
				t.Errorf("arg %q: path=%q name=%q, want path emitted = %v",
					tc.arg, resp.Repository.Path, resp.Repository.Name, tc.wantPath)
			}
		})
	}
}

func TestRunClassifiesExistingEntryAsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "payments"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"payments"}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp struct {
		Repository struct {
			Path string `json:"path"`
			Name string `json:"name"`
		} `json:"repository"`
	}
	json.Unmarshal([]byte(out.String()), &resp)
	if resp.Repository.Path == "" {
		t.Errorf("an existing filesystem entry should classify as a path, got name=%q", resp.Repository.Name)
	}
}

func TestRunExpandsTildeToAbsolutePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	var out strings.Builder
	if err := run(strings.NewReader(`{"arg":"~/somewhere"}`), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp struct {
		Repository struct {
			Path string `json:"path"`
		} `json:"repository"`
	}
	json.Unmarshal([]byte(out.String()), &resp)
	want := filepath.Join(home, "somewhere")
	if resp.Repository.Path != want {
		t.Errorf("path = %q, want %q", resp.Repository.Path, want)
	}
}

func TestRunErrors(t *testing.T) {
	for name, in := range map[string]string{
		"empty arg":   `{"arg":""}`,
		"whitespace":  `{"arg":"   "}`,
		"missing arg": `{}`,
		"garbage":     `not json`,
	} {
		var out strings.Builder
		if err := run(strings.NewReader(in), &out); err == nil {
			t.Errorf("%s: want error", name)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote stdout on failure: %q", name, out.String())
		}
	}
}
