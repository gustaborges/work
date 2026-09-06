package tui

import (
	"os"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestInteractiveWithPipes(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if interactive(r, w) {
		t.Errorf("interactive(pipe, pipe) = true, want false")
	}
}

func TestMustInteractiveNonTTY(t *testing.T) {
	// `go test` runs with stdout redirected, so this process is non-interactive.
	err := MustInteractive()
	if err == nil {
		t.Skip("stdout is a TTY in this environment")
	}
	if diag.ExitCode(err) != diag.Usage.Code {
		t.Errorf("exit code = %d, want %d", diag.ExitCode(err), diag.Usage.Code)
	}
}
