package tui

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

var (
	stepTitleStyle = lipgloss.NewStyle().Bold(true)
	stepCheckStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
)

// StepDone renders a completed interview step as a compact, read-only block:
// the step's title, then a green check with the chosen value, followed by one
// blank line.
//
// It is written to the prompt stream (stderr) once a step resolves, so the live
// selector is replaced by its outcome instead of being left on screen. The
// single trailing blank line keeps exactly one empty row between a finished step
// and the next prompt.
func StepDone(w io.Writer, title, value string) {
	fmt.Fprintf(w, "%s\n%s %s\n\n", stepTitleStyle.Render(title), stepCheckStyle.Render("✓"), value)
}
