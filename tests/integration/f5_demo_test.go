//go:build unix

// F5 demonstration: the roadmap's five-step journey as
// one automated scenario over an external plugin package that needed no change
// to the core.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/projection"
)

// startResult is what one non-interactive `work start` produced.
type startResult struct {
	stdout, stderr string
	workID         string
	dir            string
}

// runContextStart runs `work start demo-ctx-1` with the given fixture modes.
func runContextStart(t *testing.T, bin string, env []string, ws, slug, modes string) startResult {
	t.Helper()
	args := []string{"start", "demo-ctx-1", "--base", "main", "--slug", slug, "--prefix", "{slug}", "--yes"}
	// The first start persists the workspace root; passing it again is an error.
	if _, err := os.Stat(ws); os.IsNotExist(err) {
		args = append(args, "--workspace", ws)
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(append([]string(nil), env...), "WORK_FIXTURE_MODE="+modes)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("work start (%s) failed: %v\nstderr: %s", modes, err, stderr.String())
	}
	m := regexp.MustCompile(`work: created ([0-9A-HJKMNP-TV-Z]{26})`).FindStringSubmatch(stdout.String())
	if m == nil {
		t.Fatalf("no created line in stdout:\n%s", stdout.String())
	}
	return startResult{stdout.String(), stderr.String(), m[1], filepath.Join(ws, "in-progress", "demo-ctx-1_"+slug)}
}

func provenanceOf(t *testing.T, workHome, workID string) map[string]projection.Provenance {
	t.Helper()
	db, err := projection.Open(filepath.Join(workHome, "state", "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Provenance(workID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]projection.Provenance{}
	for _, r := range rows {
		out[r.Section+"/"+r.Key] = r
	}
	return out
}

func TestF5RoadmapJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	// Steps 1-4 in one start: the Starter publishes metadata and a link, the
	// Linker discovers a value the core upserts, the first Importer consumes it
	// through staging, the second collides and contributes nothing.
	a := runContextStart(t, bin, env, ws, "journey",
		"starter-context,linker-value,linker2-none,importer-ok,importer2-collide")

	snap := readSnapshot(t, filepath.Join(a.dir, "work-state.json"))
	if snap.Meta["github.pull_request.number"] != float64(212) {
		t.Errorf("step 1: Starter metadata missing from the first snapshot: %v", snap.Meta)
	}
	if got := snap.Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker" {
		t.Errorf("step 2: the Linker's value was not upserted over the Starter's: %q", got)
	}
	prov := provenanceOf(t, workHome, a.workID)
	if p := prov["meta/github.pull_request.number"]; p.SourceComponent != "context-suite/starter" || p.SourceOperation != "start" {
		t.Errorf("step 1: metadata provenance = %+v", p)
	}
	if p := prov["links/github.pull_request"]; p.SourceComponent != "context-suite/linker" || p.SourceOperation != "discover" {
		t.Errorf("step 2: link provenance = %+v", p)
	}

	got, err := os.ReadFile(filepath.Join(a.dir, "notes", "context.md"))
	if err != nil || string(got) != "imported context\n" {
		t.Errorf("step 3: the Importer's artifact = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(a.dir, "worktree", "notes")); !os.IsNotExist(err) {
		t.Errorf("step 3: output leaked into the worktree: %v", err)
	}
	if !strings.Contains(a.stderr, `warning: extension-output-refused: context-suite/importer2 (import)`) ||
		!strings.Contains(a.stderr, `"notes/context.md"`) {
		t.Errorf("step 4: the colliding Importer was not refused:\n%s", a.stderr)
	}
	if strings.Contains(string(got), "importer2") {
		t.Error("step 4: the colliding Importer's content reached the Work")
	}

	// Step 5: an automatic extension fails; the start still ends with a
	// warning, a usable Work and exit 0 (runContextStart fails on non-zero).
	b := runContextStart(t, bin, env, ws, "failing", "linker-exit1,linker2-none,importer-empty,importer2-empty")
	if !strings.Contains(b.stderr, "warning: extension-failed: context-suite/linker (discover)") {
		t.Errorf("step 5: no warning for the failed extension:\n%s", b.stderr)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "worktree")); err != nil {
		t.Errorf("step 5: the Work is not usable: %v", err)
	}
	if lines := splitNonEmpty(b.stdout); len(lines) != 3 {
		t.Errorf("step 5: stdout = %q, want the three stable lines", b.stdout)
	}
}

// TestExternalPackageJourney: a package installed from a directory,
// with no core change, contributes context at every stage of one start.
func TestExternalPackageJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	r := runContextStart(t, bin, env, ws, "external", "starter-context,linker-value,importer-ok,importer2-ok")
	if strings.Contains(r.stderr, "warning:") {
		t.Fatalf("unexpected warning:\n%s", r.stderr)
	}
	snap := readSnapshot(t, filepath.Join(r.dir, "work-state.json"))
	if snap.Meta["github.pull_request.number"] != float64(212) || snap.Links["github.pull_request"] == "" {
		t.Errorf("snapshot lacks the package's context: meta=%v links=%v", snap.Meta, snap.Links)
	}
	for _, rel := range []string{"notes/context.md", "notes/importer2.md"} {
		if _, err := os.Stat(filepath.Join(r.dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("Importer artifact %s missing: %v", rel, err)
		}
	}
}
