package diagrender

import (
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/present/theme"
)

// Human renders a terminal failure for an interactive terminal: a single
// "✘ <summary>" line and, when hint is non-empty, a second "  → <hint>" line
// naming the next action. summary and hint are caller-supplied user-vocabulary
// text (a diag.Error's Summary/Msg and Hint); the internal cause chain is never
// shown here (FR-015). The trailing newline is included.
func Human(th theme.Theme, summary, hint string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", danger(th), summary)
	if hint != "" {
		fmt.Fprintf(&b, "  %s %s\n", th.Muted.Render(arrow(th)), hint)
	}
	return b.String()
}

// Cancel is the single line an interactively cancelled operation collapses to
// (contracts/diagnostics.md; exit 20 is unchanged and set by the caller).
func Cancel(th theme.Theme) string {
	return fmt.Sprintf("%s Operation cancelled\n", danger(th))
}

// Unexpected renders a non-diag error: the human sees a generic line and the
// WORK_DEBUG affordance rather than an internal message.
func Unexpected(th theme.Theme) string {
	return Human(th, "something went wrong", "run with WORK_DEBUG=1 for details")
}

// danger returns the ✘ mark under th, honouring the ASCII fallback. The glyph
// carries the meaning with colour off (theme.md).
func danger(th theme.Theme) string {
	if th.Cap.AsciiMarks {
		return th.Danger.Render("x")
	}
	return th.Danger.Render("✘")
}

func arrow(th theme.Theme) string {
	if th.Cap.AsciiMarks {
		return "->"
	}
	return "→"
}
