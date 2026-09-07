package present

import (
	"fmt"

	"github.com/gustaborges/work/internal/present/theme"
)

// Mark selects the glyph a receipt or notice leads with. The glyph carries the
// meaning even with colour off; ASCII fallbacks are used when the capability
// probe set AsciiMarks (research R16).
type Mark int

const (
	// MarkSuccess is ✔ (Success token) or the ASCII "ok".
	MarkSuccess Mark = iota
	// MarkFailure is ✘ (Danger token) or the ASCII "x".
	MarkFailure
)

// markGlyph returns the styled glyph string for m under th.
func markGlyph(th theme.Theme, m Mark) string {
	switch m {
	case MarkFailure:
		if th.Cap.AsciiMarks {
			return th.Danger.Render("x")
		}
		return th.Danger.Render("✘")
	default:
		if th.Cap.AsciiMarks {
			return th.Success.Render("ok")
		}
		return th.Success.Render("✔")
	}
}

// Receipt is the compact durable record of an accepted step: the title on its
// own line, then "  <mark> <displayValue>", then a blank separator line
// (contracts/interaction.md §2). A selector's receipt replaces its whole
// expanded list; nothing else survives into terminal history. displayValue is
// caller-formatted safe text — a redaction for a sensitive value.
func Receipt(th theme.Theme, title string, mark Mark, displayValue string) string {
	return fmt.Sprintf("%s\n  %s %s\n\n", title, markGlyph(th, mark), displayValue)
}

// ConfirmReceipt is what an accepted pre-mutation confirmation collapses to,
// written before the command's stable stdout result (FR-030, research R7).
func ConfirmReceipt(th theme.Theme, title string) string {
	return fmt.Sprintf("%s %s confirmed\n", markGlyph(th, MarkSuccess), title)
}
