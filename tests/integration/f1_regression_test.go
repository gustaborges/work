//go:build unix

// F1 regression guard (SC-007): F2 must not change the `work start` journey.
// The full F1 quickstart S1–S12 runs as the create_*/invalid_*/branch_collision/
// rollback/shell_integration/non_interactive_missing scenarios in this package,
// executed unchanged by `go test ./...` on every CI OS. This file adds an
// explicit assertion that `work start`'s stdout contract is byte-stable and that
// the lazy schema-2 upgrade is the only F1-visible change F2 introduces.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// f1ScenarioFiles are the F1 quickstart scenarios that must keep running in CI.
// Deleting or renaming one silently drops SC-007 coverage, so the guard fails
// loudly instead.
var f1ScenarioFiles = []string{
	"create_happy.txtar",            // S1
	"create_base_branch.txtar",      // S2
	"create_offline.txtar",          // S3
	"invalid_path.txtar",            // S4
	"invalid_slug.txtar",            // S5
	"branch_collision.txtar",        // S6
	"rollback.txtar",                // S7
	"non_interactive_missing.txtar", // S9
	"shell_integration.txtar",       // S10
}

func TestF1QuickstartScenariosStillPresent(t *testing.T) {
	for _, f := range f1ScenarioFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("F1 regression scenario %s is missing (SC-007): %v", f, err)
		}
	}
}

// TestF1StartOutputContractUnchanged pins the exact three-line stdout shape F1
// release 0.1 emits from `work start`, plus the FR-023 stderr notice and the
// schema-2 snapshot (the single intended additive change).
func TestF1StartOutputContractUnchanged(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	env, homeDir, _ := ptyEnv(t)

	repo := filepath.Join(homeDir, "src")
	makeRepo(t, repo)
	ws := filepath.Join(homeDir, "ws")

	cmd := exec.Command(bin, "start", repo,
		"--workspace", ws, "--base", "main", "--slug", "regress", "--prefix", "{slug}", "--yes")
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("work start failed: %v\nstderr: %s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout is not the 3-line F1 contract:\n%s", stdout.String())
	}
	wt := filepath.Join(ws, "in-progress", "src_regress", "worktree")
	want := []*regexp.Regexp{
		regexp.MustCompile(`^work: created [0-9A-HJKMNP-TV-Z]{26}$`),
		regexp.MustCompile(`^work: branch regress  \(from main @ [0-9a-f]{7,}\)$`),
		regexp.MustCompile(`^work: path ` + regexp.QuoteMeta(wt) + `$`),
	}
	for i, re := range want {
		if !re.MatchString(lines[i]) {
			t.Errorf("stdout line %d = %q, want match %s", i+1, lines[i], re)
		}
	}
	if !strings.Contains(stderr.String(), "this shell session was not moved") {
		t.Errorf("FR-023 stderr notice missing:\n%s", stderr.String())
	}

	snap, err := os.ReadFile(filepath.Join(ws, "in-progress", "src_regress", "work-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(snap), `"schema": 2`) {
		t.Errorf("snapshot is not schema 2 (the one intended F2 change):\n%s", snap)
	}
}
