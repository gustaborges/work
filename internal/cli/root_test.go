package cli

import (
	"bytes"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestRootHasF1Subcommands(t *testing.T) {
	root := newRootCmd()
	want := map[string]bool{"start": false, "shell-init": false}
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

func TestRootRejectsUnknownSubcommand(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"frobnicate"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("unknown subcommand: want error")
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
