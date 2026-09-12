package present

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const ellipsis = "…"

// Fixed leading-slot widths, in terminal cells. They never change with focus or
// checked state, so the marker, checkbox, primary, and secondary columns stay
// put across every interaction (FR-008, SC-003).
const (
	// MarkerSlot holds "❯ " on the focused row and "  " otherwise.
	MarkerSlot = 2
	// CheckboxSlot holds "[x] " / "[ ] " in multi-select rows.
	CheckboxSlot = 4
)

// Budget is the terminal viewport a bounded control renders within. Reserved is
// the rows the control spends on chrome: title, optional group bar, scroll
// line, filter line, error line, help line, and any confirmation block
// (contracts/interaction.md §4, research R5).
type Budget struct {
	Width    int
	Height   int
	Reserved int
}

// VisibleRows is how many list rows fit after the reserved chrome. It is never
// less than 1: on a viewport too short to honour the budget the control still
// shows its title, one row, and help and lets the terminal scroll (a documented
// degenerate case).
func (b Budget) VisibleRows() int {
	return max(b.Height-b.Reserved, 1)
}

// DisplayWidth is the terminal cell width of s: ANSI styling is ignored, wide
// runes (CJK, emoji) count as two cells, and combining marks count as zero.
// Never use len() or len([]rune()) for layout.
func DisplayWidth(s string) int { return ansi.StringWidth(s) }

// TruncTail shortens s to at most max display cells, appending "…" when it had
// to cut. A max of zero or less yields "". The result is never wider than max.
func TruncTail(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, ellipsis)
}

// collapseToLine joins a bracketed-paste payload into one line, for a text
// field (an InputStep value or a list's filter) that cannot hold a newline.
func collapseToLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
}

// Wrap reflows s to at most width display cells per line, breaking at word
// boundaries (falling back to a hard break inside an over-long word). A width
// of zero or less returns s unchanged. Unlike TruncTail/TruncMiddle, no content
// is discarded — every rune of s appears in the result.
func Wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Wordwrap(s, width, "")
}

// TruncMiddle shortens s to at most max display cells by eliding the middle and
// joining the head and tail with "…" — useful for long paths where both ends
// carry meaning. The result is never wider than max.
func TruncMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= max {
		return s
	}
	if max == 1 {
		return ellipsis
	}
	head := (max - 1) / 2
	tail := max - 1 - head
	return ansi.Truncate(s, head, "") + ellipsis + ansi.TruncateLeft(s, w-tail, "")
}

// RowSpec is one option row to lay out with stable columns.
type RowSpec struct {
	// Width is the total cell width available for the row.
	Width int
	// Focused draws "❯ " in the marker slot and (the caller) bolds the text.
	Focused bool
	// Checkable adds the fixed 4-cell checkbox slot; Checked fills it.
	Checkable bool
	Checked   bool
	// Primary is the identity line; it is truncated only as a last resort.
	Primary string
	// Secondary is optional metadata shown on an indented second line; it is
	// truncated (and, by the caller, dropped) before Primary (research R5).
	Secondary string
}

// Row lays a RowSpec out into one or two strings: line one is
// marker + checkbox + primary; when Secondary is set, line two is the secondary
// text indented to the primary column. Both lines are bounded to Width using
// display width. Focus styling and checkbox glyphs never shift a column because
// their slots are fixed width.
func Row(spec RowSpec) []string {
	marker := "  "
	if spec.Focused {
		marker = "❯ "
	}
	box := ""
	if spec.Checkable {
		box = "[ ] "
		if spec.Checked {
			box = "[x] "
		}
	}
	indent := DisplayWidth(marker) + DisplayWidth(box)
	textWidth := max(spec.Width-indent, 1)
	lines := []string{marker + box + TruncTail(spec.Primary, textWidth)}
	if spec.Secondary != "" {
		lines = append(lines, strings.Repeat(" ", indent)+TruncTail(spec.Secondary, textWidth))
	}
	return lines
}
