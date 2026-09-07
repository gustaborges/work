package present

import (
	"os"

	"golang.org/x/term"

	"github.com/gustaborges/work/internal/diag"
)

// IsInteractive reports whether both stdin and stdout are terminals. Every
// interactive control is gated on this; when it is false the process is a
// pipeline and a missing value must fail with guidance rather than open a
// prompt.
func IsInteractive() bool {
	return isTTY(os.Stdin) && isTTY(os.Stdout)
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

func isTTY(f *os.File) bool {
	return f != nil && term.IsTerminal(int(f.Fd()))
}
