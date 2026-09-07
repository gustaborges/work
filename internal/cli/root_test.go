package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestRootHasSubcommands(t *testing.T) {
	root := newRootCmd()
	want := map[string]bool{"start": false, "resume": false, "archive": false, "shell-init": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

func TestRootRegistersHelpGroups(t *testing.T) {
	root := newRootCmd()
	got := map[string]bool{}
	for _, g := range root.Groups() {
		got[g.ID] = true
	}
	for _, want := range []string{groupDaily, groupInWork, groupAdmin, groupSetup} {
		if !got[want] {
			t.Errorf("root is missing command group %q", want)
		}
	}
}

// Interactive bare `work` prints the brand to stdout and exits 0; that path is
// gated on a real TTY, so its end-to-end coverage is the PTY test
// tests/integration/brand_test.go. Here we only assert the non-interactive
// contract is unchanged (TestBareWorkNonInteractiveIsUsage) and the renderer is
// wired (brand.Render is unit-tested in internal/present/brand).

func TestRootRejectsUnknownSubcommand(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"frobnicate"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("unknown subcommand: want error")
	}
}

func TestBareWorkNonInteractiveIsUsage(t *testing.T) {
	out, errb, code := runWork(t)
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
	if !strings.Contains(errb, "work start") {
		t.Errorf("stderr does not point at `work start`: %s", errb)
	}
	if !strings.Contains(errb, "work resume") {
		t.Errorf("stderr does not point at `work resume`: %s", errb)
	}
	if !strings.Contains(errb, "work archive") {
		t.Errorf("stderr does not point at `work archive`: %s", errb)
	}
}

func TestArchiveRejectsJSON(t *testing.T) {
	_, _, code := runWork(t, "archive", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestArchiveNonInteractiveNoTargetIsUsage(t *testing.T) {
	out, errb, code := runWork(t, "archive")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
}

func TestArchiveNonInteractiveMissingYesIsUsage(t *testing.T) {
	_, errb, code := runWork(t, "archive", "01000000000000000000000000")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if !strings.Contains(errb, "--yes") {
		t.Errorf("stderr does not name --yes: %s", errb)
	}
}

func TestArchiveFlagSurface(t *testing.T) {
	a := newArchiveCmd()
	for _, f := range []string{"yes", "force-dirty"} {
		if a.Flags().Lookup(f) == nil {
			t.Errorf("`work archive` missing --%s", f)
		}
	}
}

func TestResumeRejectsJSON(t *testing.T) {
	_, _, code := runWork(t, "resume", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestResumeNonInteractiveNoTargetIsUsage(t *testing.T) {
	// No workspace configured, no target, not a TTY: exit 2, no TUI.
	out, errb, code := runWork(t, "resume")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstderr: %s", code, errb)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
}

func TestStartFlagSurface(t *testing.T) {
	start := newStartCmd()
	for _, f := range []string{"workspace", "base", "slug", "prefix", "yes"} {
		if start.Flags().Lookup(f) == nil {
			t.Errorf("`work start` missing --%s", f)
		}
	}
}

func TestStartNonInteractiveMissingSourceIsUsage(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"start"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("empty non-interactive `work start`: want a usage error")
	}
	if got := diag.ExitCode(err); got != 2 {
		t.Errorf("exit code = %d, want 2 (%v)", got, err)
	}
}

func TestJSONFlagIsPersistent(t *testing.T) {
	root := newRootCmd()
	if root.PersistentFlags().Lookup("json") == nil {
		t.Error("--json persistent flag not defined")
	}
}
