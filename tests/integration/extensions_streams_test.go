//go:build unix

package integration

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestExtensionOutputIsOnlyOnTheUIChannel: progress and warnings never reach
// stdout, which stays exactly the three stable lines, and a warning never
// changes the exit code.
func TestExtensionOutputIsOnlyOnTheUIChannel(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, workHome := ptyEnv(t)
	ws := contextSuiteWorkspace(t, bin, env, homeDir, workHome)

	env = append(env, "WORK_FIXTURE_MODE=linker-value,importer-exit1,importer2-empty")
	s := startSplit(t, bin, env, "start", "demo-ctx-1",
		"--workspace", ws, "--base", "main", "--slug", "streams", "--prefix", "{slug}", "--yes")
	s.expectOut("work: path ")
	if code := s.wait(); code != 0 {
		t.Fatalf("exited %d with a warning, want 0\nUI:\n%s\nOUT:\n%s", code, s.ui(), s.out())
	}

	out := stripANSI(s.out())
	lines := splitNonEmpty(out)
	if len(lines) != 3 {
		t.Fatalf("stdout is not the 3-line contract, got %d lines:\n%q", len(lines), out)
	}
	wt := filepath.Join(ws, "in-progress", "demo-ctx-1_streams", "worktree")
	for i, re := range []*regexp.Regexp{
		regexp.MustCompile(`^work: created [0-9A-HJKMNP-TV-Z]{26}$`),
		regexp.MustCompile(`^work: branch streams  \(from main @ [0-9a-f]{7,}\)$`),
		regexp.MustCompile(`^work: path ` + regexp.QuoteMeta(wt) + `$`),
	} {
		if !re.MatchString(lines[i]) {
			t.Errorf("stdout line %d = %q, want %s", i+1, lines[i], re)
		}
	}
	for _, leak := range []string{"running", "linked", "imported", "⚠", "warning:", "context-suite"} {
		if strings.Contains(out, leak) {
			t.Errorf("extension output %q leaked onto stdout:\n%q", leak, out)
		}
	}

	ui := s.ui()
	for _, want := range []string{
		"work: running context-suite/linker (discover)",
		"work: context-suite/linker: linked github.pull_request",
		"⚠ context-suite/importer: it exited unsuccessfully",
	} {
		if !strings.Contains(ui, want) {
			t.Errorf("UI channel missing %q:\n%s", want, ui)
		}
	}
}
