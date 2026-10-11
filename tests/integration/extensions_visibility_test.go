//go:build unix

package integration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// inOrder fails unless every needle appears in s, each after the previous one.
func inOrder(t *testing.T, s string, needles ...string) {
	t.Helper()
	pos := 0
	for _, n := range needles {
		i := strings.Index(s[pos:], n)
		if i < 0 {
			t.Fatalf("%q not found (in order) after offset %d in:\n%s", n, pos, s)
		}
		pos += i + len(n)
	}
}

// TestInteractiveStartShowsWhatRuns: after the confirmation
// receipt and the three stable lines, the automatic phases show one progress
// line per event in execution order, and a failing component adds a ⚠ warning
// with a → hint. Everything is on the UI channel and the start still succeeds.
func TestInteractiveStartShowsWhatRuns(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	env = append(env, "WORK_FIXTURE_MODE=linker-value,importer-ok,importer2-exit1")
	c := newConsole(t, bin, env, "start", "demo-ctx-1",
		"--workspace", ws, "--base", "main", "--slug", "visible", "--prefix", "{slug}", "--yes")
	c.expect("importer2")
	if code := c.wait(); code != 0 {
		t.Fatalf("start exited %d, want 0 even with a failing Importer\n%s", code, c.screen())
	}

	out := c.snapshot()
	inOrder(t, out,
		"work: created ",
		"work: path ",
		"work: running context-suite/linker (discover)",
		"work: context-suite/linker: linked github.pull_request",
		"work: running context-suite/linker2 (discover)",
		"work: running context-suite/importer (import)",
		"work: context-suite/importer: imported 2 item(s)",
		"work: running context-suite/importer2 (import)",
		"⚠ context-suite/importer2: it exited unsuccessfully",
		"→ ",
	)
	if n := strings.Count(out, "⚠"); n != 1 {
		t.Errorf("warnings shown = %d, want exactly one:\n%s", n, out)
	}
	if strings.Contains(out, "example.test") {
		t.Errorf("a link value reached the terminal:\n%s", out)
	}
	if regexp.MustCompile(`warning: extension-`).MatchString(out) {
		t.Errorf("the interactive form used the frozen non-interactive line:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(ws, "in-progress", "demo-ctx-1_visible", "notes", "context.md")); err != nil {
		t.Errorf("the healthy Importer's output is missing: %v", err)
	}
}

// TestWizardIsUnchangedByInstalledExtensions: installing Importers and Linkers
// adds nothing to the guided start — no step, prompt or receipt of its own.
func TestWizardIsUnchangedByInstalledExtensions(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	env = append(env, "WORK_FIXTURE_MODE=linker-none,linker2-none,importer-empty,importer2-empty")
	c := newConsole(t, bin, env, "start", "demo-ctx-1",
		"--workspace", ws, "--base", "main", "--slug", "guided", "--prefix", "{slug}")
	c.expect("Create Work")
	before := c.snapshot()
	for _, extra := range []string{"Linker", "Importer", "discover", "import"} {
		if strings.Contains(before, extra) {
			t.Errorf("the wizard mentions %q before the Work exists:\n%s", extra, before)
		}
	}
	c.send("y")
	c.expect("work: created ")
	if code := c.wait(); code != 0 {
		t.Fatalf("exited %d\n%s", code, c.screen())
	}
}
