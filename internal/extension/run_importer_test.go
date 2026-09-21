package extension

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// importerCounts returns the Imported counts reported, keyed by component.
func importerCounts(h *harness) map[string]int {
	out := map[string]int{}
	for _, e := range h.rec.events {
		if e.Kind == Imported {
			out[e.Component] = e.Count
		}
	}
	return out
}

func stageDirsFrom(h *harness) []string {
	var dirs []string
	for _, l := range h.launches() {
		if d, ok := l.Stdin["output_dir"].(string); ok {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

func TestImporterRunsAfterLinkerPersistsItsRequiredLink(t *testing.T) {
	h := newHarness(t, "linker-value,importer-ok,importer2-empty")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	var importerStdin map[string]any
	for _, l := range h.launches() {
		if l.Role == "importer" {
			importerStdin = l.Stdin
		}
	}
	if importerStdin == nil {
		t.Fatal("the importer never started")
	}
	inputs, _ := importerStdin["inputs"].(map[string]any)
	if inputs["github.pull_request"] != "https://example.test/pr/212-from-linker" || inputs["start_mode"] != "new" {
		t.Errorf("importer inputs = %v", inputs)
	}
	if out, _ := importerStdin["output_dir"].(string); out == "" || len(importerStdin) != 2 {
		t.Errorf("importer stdin = %v, want inputs and output_dir only", importerStdin)
	}
}

func TestImporterOutputLandsBesideTheWorktreeNotInIt(t *testing.T) {
	h := newHarness(t, "linker-value,importer-ok,importer2-empty")
	Run(context.Background(), h.ctx)

	got, err := os.ReadFile(filepath.Join(h.ctx.WorkDir, "notes", "context.md"))
	if err != nil || string(got) != "imported context\n" {
		t.Fatalf("notes/context.md = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(h.ctx.WorktreePath, "notes")); !os.IsNotExist(err) {
		t.Errorf("output leaked into the worktree: %v", err)
	}
	if n := importerCounts(h)["context-suite/importer"]; n != 2 {
		t.Errorf("imported = %d, want 2 (directory and file)", n)
	}
}

func TestImporterWithAbsentRequiredInputNeitherRunsNorWarns(t *testing.T) {
	h := newHarness(t, "linker-none,linker2-none,importer-ok")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 {
		t.Errorf("warnings = %+v", rep.Warnings)
	}
	for _, l := range h.launches() {
		if l.Role == "importer" || l.Role == "importer2" {
			t.Errorf("%s started without its required link", l.Role)
		}
	}
	if len(stageDirsFrom(h)) != 0 {
		t.Error("a stage was created")
	}
}

func TestStageIsRemovedWhateverTheOutcome(t *testing.T) {
	for _, mode := range []string{"importer-ok", "importer-empty", "importer-exit1", "importer-garbage", "importer-mixed"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t, "linker-value,importer2-empty,"+mode)
			Run(context.Background(), h.ctx)

			dirs := stageDirsFrom(h)
			if len(dirs) == 0 {
				t.Fatal("no importer ran")
			}
			for _, d := range dirs {
				if _, err := os.Stat(d); !os.IsNotExist(err) {
					t.Errorf("stage %s survived: %v", d, err)
				}
			}
		})
	}
}

func TestEmptyImporterOutputSucceedsSilently(t *testing.T) {
	h := newHarness(t, "linker-value,importer-empty,importer2-empty")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 || len(importerCounts(h)) != 0 {
		t.Errorf("warnings=%+v imported=%v", rep.Warnings, importerCounts(h))
	}
	if entries, _ := os.ReadDir(h.ctx.WorkDir); len(entries) != 2 {
		t.Errorf("Work directory changed: %v", entries)
	}
}

func TestLaterImporterPlanSeesEarlierImportersFiles(t *testing.T) {
	// importer writes notes/context.md; importer2 writes notes/importer2.md
	// into the notes directory the first one just created, so its plan must
	// merge into it rather than treat it as absent.
	h := newHarness(t, "linker-value,importer-ok,importer2-ok")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	counts := importerCounts(h)
	if counts["context-suite/importer"] != 2 || counts["context-suite/importer2"] != 1 {
		t.Errorf("counts = %v, want importer 2 (dir+file) and importer2 1 (file into the existing dir)", counts)
	}
	var running []string
	for _, e := range h.rec.events {
		if e.Kind == Running && e.Operation == OpImport {
			running = append(running, e.Component)
		}
	}
	if !slices.Equal(running, []string{"context-suite/importer", "context-suite/importer2"}) {
		t.Errorf("importer order = %v", running)
	}
}
