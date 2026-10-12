package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The complete offline F6 product journey uses no snapshot editing: create a
// Work, inspect, associate, import, inspect outside, then archive its artifacts.
func TestF6RoadmapJourney(t *testing.T) {
	h := newF6Harness(t)
	value := " https://example.test/pr/212 exact "
	out, ui := h.run(t, h.dir, 0, "status", "--json")
	if ui != "" {
		t.Fatal(ui)
	}
	var view struct {
		ID    string            `json:"id"`
		Links map[string]string `json:"links"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil || view.ID != h.id || len(view.Links) != 0 {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(h.log); !os.IsNotExist(err) {
		t.Fatal("status launched a plugin", err)
	}
	handoff := filepath.Join(h.root, "handoff")
	if err := os.WriteFile(handoff, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	h.env = append(h.env, "WORK_CD_FILE="+handoff)
	h.run(t, h.dir, 0, "link", "manual/linker", value)
	if _, err := os.Stat(h.log); !os.IsNotExist(err) {
		t.Fatal("link launched a plugin", err)
	}
	before, err := os.ReadFile(filepath.Join(h.dir, "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, ui = h.run(t, h.dir, 0, "import", "manual/importer")
	if out != "work: imported "+h.id+" manual/importer 4\n" {
		t.Fatal(out, ui)
	}
	for _, rel := range []string{"notes/context.md", "refs/source.txt"} {
		if b, err := os.ReadFile(filepath.Join(h.dir, rel)); err != nil || len(b) == 0 {
			t.Fatal(rel, err)
		}
	}
	launches, err := os.ReadFile(h.log)
	if err != nil {
		t.Fatal(err)
	}
	if lines := bytes.Count(launches, []byte("\n")); lines != 1 {
		t.Fatal("want exactly one process", string(launches))
	}
	var launch struct {
		Role  string `json:"role"`
		Stdin struct {
			Inputs    map[string]any `json:"inputs"`
			OutputDir string         `json:"output_dir"`
		} `json:"stdin"`
	}
	if err := json.Unmarshal(launches, &launch); err != nil || launch.Role != "importer" || launch.Stdin.Inputs["github.pull_request"] != value {
		t.Fatal(string(launches), err)
	}
	if _, err := os.Stat(launch.Stdin.OutputDir); !os.IsNotExist(err) {
		t.Fatal("stage retained", err)
	}
	out, ui = h.run(t, h.root, 0, "status", h.id, "--json")
	if err := json.Unmarshal([]byte(out), &view); err != nil || view.Links["github.pull_request"] != value || ui != "" {
		t.Fatal(out, ui, err)
	}
	for _, args := range [][]string{{"status"}, {"link", "manual/linker", "unused"}, {"import", "manual/importer"}} {
		out, ui = h.run(t, h.root, 21, args...)
		if out != "" || strings.Count(ui, "error: target-not-found:") != 1 {
			t.Fatal(out, ui)
		}
	}
	after, err := os.ReadFile(filepath.Join(h.dir, "work-state.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("import/status changed canonical state", err)
	}
	if b, err := os.ReadFile(handoff); err != nil || string(b) != "untouched" {
		t.Fatal("F6 changed shell handoff", err)
	}
	if b, err := os.ReadFile(h.log); err != nil || !bytes.Equal(b, launches) {
		t.Fatal("unexpected extra invocation", err)
	}
	artifact, err := os.ReadFile(filepath.Join(h.dir, "notes", "context.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Archive owns its separate shell handoff behavior.
	h.run(t, h.root, 0, "archive", h.id, "--yes")
	matches, err := filepath.Glob(filepath.Join(h.root, "ws", "archived", "*", "notes", "context.md"))
	if err != nil || len(matches) != 1 {
		t.Fatal(matches, err)
	}
	if b, err := os.ReadFile(matches[0]); err != nil || !bytes.Equal(artifact, b) {
		t.Fatal("archive lost artifact", err)
	}
	h.run(t, h.root, 0, "status", h.id)
}
