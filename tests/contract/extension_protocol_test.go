package contract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/ipc"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// extensionTarget returns how to start one binary of the context-suite
// fixture, with the fixture behaviour selected through WORK_FIXTURE_MODE.
func extensionTarget(t *testing.T, entrypoint, mode string) ipc.Target {
	t.Helper()
	t.Setenv("WORK_FIXTURE_MODE", mode)
	bin := filepath.Join(fixtures.Prepare(t, "context-suite"), entrypoint)
	return ipc.Target{Path: bin + exeSuffix()}
}

// TestLinkerProtocolContract runs the fixture Linker under the exact rules of
// the extension protocol: one JSON document in, at most one out, and only the
// documented shapes count as an answer.
func TestLinkerProtocolContract(t *testing.T) {
	cases := []struct {
		mode        string
		wantValue   string
		wantInvalid bool
		wantExitErr bool
	}{
		{mode: "linker-value", wantValue: "https://example.test/pr/212-from-linker"},
		{mode: "linker-none"},
		{mode: "linker-empty-value", wantInvalid: true},
		{mode: "linker-garbage", wantInvalid: true},
		{mode: "linker-exit1", wantExitErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			target := extensionTarget(t, "linker", tc.mode)
			resp, res, err := ipc.InvokeLinker(context.Background(), target,
				ipc.LinkerInput{Inputs: map[string]any{"worktree_path": "/w/worktree"}})

			switch {
			case tc.wantInvalid:
				if !errors.Is(err, ipc.ErrInvalidResponse) {
					t.Fatalf("err = %v, want an invalid response", err)
				}
			case tc.wantExitErr:
				if err == nil || res.ExitCode != 1 {
					t.Fatalf("err = %v, exit = %d, want exit 1", err, res.ExitCode)
				}
			default:
				if err != nil {
					t.Fatalf("InvokeLinker: %v", err)
				}
				if resp.Value != tc.wantValue {
					t.Errorf("value = %q, want %q", resp.Value, tc.wantValue)
				}
			}
		})
	}
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// TestLinkerReceivesExactlyOneInputsDocument pins what a Linker is sent: one
// JSON object whose only member is the resolved inputs.
func TestLinkerReceivesExactlyOneInputsDocument(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "launches.log")
	t.Setenv("WORK_FIXTURE_LOG", logPath)
	target := extensionTarget(t, "linker", "linker-none")

	if _, _, err := ipc.InvokeLinker(context.Background(), target,
		ipc.LinkerInput{Inputs: map[string]any{"worktree_path": "/w/worktree"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(data)), `{"role":"linker","stdin":{"inputs":{"worktree_path":"/w/worktree"}}}`; got != want {
		t.Errorf("launch log = %s, want %s", got, want)
	}
}

// TestImporterProtocolContract: an Importer's stdout is empty or one JSON
// object whose content is ignored; anything else is an invalid response, and
// everything it produces travels through the files under output_dir.
func TestImporterProtocolContract(t *testing.T) {
	cases := []struct {
		mode        string
		wantInvalid bool
		wantExitErr bool
		wantFile    string
	}{
		{mode: "importer-ok", wantFile: "notes/context.md"},
		{mode: "importer-empty"},
		{mode: "importer-garbage", wantInvalid: true},
		{mode: "importer-exit1", wantExitErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			target := extensionTarget(t, "importer", tc.mode)
			out := t.TempDir()
			res, err := ipc.InvokeImporter(context.Background(), target,
				ipc.ImporterInput{Inputs: map[string]any{"github.pull_request": "https://x"}, OutputDir: out})

			switch {
			case tc.wantInvalid:
				if !errors.Is(err, ipc.ErrInvalidResponse) {
					t.Fatalf("err = %v, want an invalid response", err)
				}
			case tc.wantExitErr:
				if err == nil || res.ExitCode != 1 {
					t.Fatalf("err = %v, exit = %d, want exit 1", err, res.ExitCode)
				}
			default:
				if err != nil {
					t.Fatalf("InvokeImporter: %v", err)
				}
			}
			if tc.wantFile != "" {
				if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(tc.wantFile))); err != nil {
					t.Errorf("expected output file: %v", err)
				}
			}
		})
	}
}
