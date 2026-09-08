package present

import (
	"strings"

	"github.com/gustaborges/work/internal/present/theme"
)

// writeRow renders one option row (one or two lines from Row) into b: the
// primary line takes the Primary token when focused (bold, plus accent when
// colour is on) and is otherwise plain; the secondary line is always Muted, so
// metadata never competes with identity (contracts/interaction.md §5).
func writeRow(b *strings.Builder, th theme.Theme, lines []string, focused bool) {
	for i, line := range lines {
		switch {
		case i == 0 && focused:
			line = th.Primary.Render(line)
		case i > 0:
			line = th.Muted.Render(line)
		}
		b.WriteString(line + "\n")
	}
}

// Option is one selectable value. present sees only the presentation strings and
// an opaque Value; the CLI maps Value back to its domain object (data-model §2).
type Option[T any] struct {
	Value     T
	Primary   string // identity line
	Secondary string // optional metadata line; truncated before Primary
	Group     string // optional; when the spec is Grouped this becomes a tab
}

type listState int

const (
	listChoosing listState = iota
	listConfirming
	listCompleted
	listCancelled
)

// distinctGroups returns the non-empty Group values in first-appearance order.
// Empty groups are never surfaced (contracts/interaction.md §5).
func distinctGroups[T any](opts []Option[T]) []string {
	seen := map[string]bool{}
	var out []string
	for _, o := range opts {
		if o.Group == "" || seen[o.Group] {
			continue
		}
		seen[o.Group] = true
		out = append(out, o.Group)
	}
	return out
}

// matchesFilter reports whether an option's visible text contains needle
// (case-insensitive). An empty needle matches everything.
func matchesFilter[T any](o Option[T], needle string) bool {
	if needle == "" {
		return true
	}
	hay := strings.ToLower(o.Primary + " " + o.Secondary)
	return strings.Contains(hay, strings.ToLower(needle))
}

// anySecondary reports whether any option carries a secondary line, so the list
// can reserve two rows per option and keep a stable frame height.
func anySecondary[T any](opts []Option[T]) bool {
	for _, o := range opts {
		if o.Secondary != "" {
			return true
		}
	}
	return false
}

// clampScroll keeps cursor in [0,n) and offset within a window of height rows
// that keeps the cursor visible.
func clampScroll(cursor, offset, n, height int) (int, int) {
	if height < 1 {
		height = 1
	}
	cursor = max(min(cursor, n-1), 0)
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	offset = max(min(offset, max(n-height, 0)), 0)
	return cursor, offset
}
