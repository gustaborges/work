package theme

import (
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// Capability is the process-wide terminal probe that decides whether colour is
// emitted, which colour profile to target, whether interactive controls may be
// opened, and whether the ✔/✘ marks fall back to ASCII. It is evaluated once
// per process from the UI writer and the input reader (contracts/theme.md,
// research R13).
type Capability struct {
	// ColorEnabled is true only when the UI writer is a TTY, NO_COLOR is empty
	// or unset, and TERM is not "dumb". A non-empty NO_COLOR of any value
	// disables colour; an empty NO_COLOR does not (spec edge case).
	ColorEnabled bool
	// Profile is the colour depth the terminal supports. The 24-bit brand
	// gradient and true-colour tokens require ANSI256 or TrueColor.
	Profile colorprofile.Profile
	// Interactive is true when both the UI writer and the input reader are
	// TTYs. Interactive controls are opened only then.
	Interactive bool
	// AsciiMarks forces the fixed-width ASCII fallback (ok / x) for the ✔ / ✘
	// marks. It defaults to false and is set process-globally only where PTY
	// testing shows unstable glyph width on a supported platform.
	AsciiMarks bool
}

// Detect probes ui and in for terminal capabilities. ui is normally
// cmd.ErrOrStderr(); in is normally cmd.InOrStdin(). Colour disablement follows
// the NO_COLOR convention (a non-empty value disables) and TERM=dumb.
func Detect(ui io.Writer, in io.Reader) Capability {
	uiTTY := isTerminal(ui)
	c := compute(uiTTY, isTerminal(in), os.LookupEnv)
	c.Profile = colorprofile.Detect(ui, os.Environ())
	return c
}

// compute derives the env-driven capability flags from the two TTY signals and
// an environment lookup. It is the unit-testable core of Detect; the caller
// supplies the colour profile separately.
func compute(uiTTY, inTTY bool, lookupEnv func(string) (string, bool)) Capability {
	noColor, ok := lookupEnv("NO_COLOR")
	colorSuppressed := ok && noColor != ""
	term, _ := lookupEnv("TERM")
	return Capability{
		ColorEnabled: uiTTY && !colorSuppressed && term != "dumb",
		Interactive:  uiTTY && inTTY,
	}
}

// isTerminal reports whether v is an *os.File (or anything exposing Fd())
// attached to a terminal.
func isTerminal(v any) bool {
	f, ok := v.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
