package main

import (
	"encoding/json"
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
