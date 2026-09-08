package present

import (
	"os"

	"golang.org/x/term"
)

// IsInteractive reports whether both stdin and stdout are terminals. Every
// interactive control is gated on this; when it is false the process is a
// pipeline and a missing value must fail with guidance rather than open a
// prompt.
func IsInteractive() bool {
	return isTTY(os.Stdin) && isTTY(os.Stdout)
}

func isTTY(f *os.File) bool {
	return f != nil && term.IsTerminal(int(f.Fd()))
}
