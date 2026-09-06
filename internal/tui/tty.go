// Package tui holds the Bubble Tea home model and the huh prompt wrappers, plus
// the interactive-terminal gate every TUI path must pass first.
package tui

import (
	"os"

	"golang.org/x/term"

	"github.com/gustaborges/work/internal/diag"
)

// IsInteractive reports whether both stdin and stdout are terminals. Any TUI is
// gated on this; when it is false the process is a pipeline and a missing value
// must fail with guidance rather than open a prompt.
func IsInteractive() bool {
	return interactive(os.Stdin, os.Stdout)
}

func interactive(in, out *os.File) bool {
	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

// MustInteractive returns a diag usage error when the terminal is not
// interactive.
func MustInteractive() error {
	if !IsInteractive() {
		return diag.New(diag.Usage,
			"this step needs an interactive terminal; supply the required values as flags")
	}
	return nil
}
