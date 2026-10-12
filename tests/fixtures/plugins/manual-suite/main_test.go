package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualModes(t *testing.T) {
	for _, mode := range []string{"", "empty", "exit1", "garbage", "overflow", "mixed", "worktree", "snapshot", "symlink", "case-alias", "null"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(t.TempDir(), "launch.jsonl")
			t.Setenv("WORK_FIXTURE_MODE", "importer-"+mode)
			t.Setenv("WORK_FIXTURE_LOG", log)
			input, _ := json.Marshal(map[string]any{"output_dir": dir, "inputs": map[string]any{"meta:plugin.manual-suite.nullable": nil}})
			var out bytes.Buffer
			err := run("importer", bytes.NewReader(input), &out)
			if mode == "exit1" {
				if err == nil {
					t.Fatal("expected failure")
				}
				return
			}
			if mode == "symlink" && err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(log)
			if err != nil || !strings.Contains(string(b), "nullable") {
				t.Fatalf("launch log: %s %v", b, err)
			}
			if mode == "" {
				for _, p := range []string{"notes/context.md", "refs/source.txt"} {
					if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "null" && out.String() != "null\n" {
				t.Fatal(out.String())
			}
		})
	}
}
func TestLinkerAlwaysFailsAndLogs(t *testing.T) {
	log := filepath.Join(t.TempDir(), "log")
	t.Setenv("WORK_FIXTURE_LOG", log)
	if err := run("linker", strings.NewReader("{}"), &bytes.Buffer{}); err == nil {
		t.Fatal("Linker ran")
	}
	if b, err := os.ReadFile(log); err != nil || !strings.Contains(string(b), "linker") {
		t.Fatalf("%s %v", b, err)
	}
}
