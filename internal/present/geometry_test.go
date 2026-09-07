package present

import (
	"strings"
	"testing"
)

func TestBudgetVisibleRows(t *testing.T) {
	tests := []struct {
		h, reserved, want int
	}{
		{24, 8, 16},
		{10, 8, 2},
		{10, 9, 1},
		{10, 20, 1}, // degenerate: never below 1
		{0, 0, 1},
	}
	for _, tt := range tests {
		if got := (Budget{Height: tt.h, Reserved: tt.reserved}).VisibleRows(); got != tt.want {
			t.Errorf("VisibleRows(h=%d,reserved=%d) = %d, want %d", tt.h, tt.reserved, got, tt.want)
		}
	}
}

func TestFixedSlotsHaveEqualWidth(t *testing.T) {
	if DisplayWidth("❯ ") != DisplayWidth("  ") {
		t.Errorf("focus marker slots differ: %d vs %d", DisplayWidth("❯ "), DisplayWidth("  "))
	}
	if DisplayWidth("❯ ") != MarkerSlot {
		t.Errorf("marker slot = %d, want %d", DisplayWidth("❯ "), MarkerSlot)
	}
	if DisplayWidth("[ ] ") != DisplayWidth("[x] ") {
		t.Errorf("checkbox slots differ: %d vs %d", DisplayWidth("[ ] "), DisplayWidth("[x] "))
	}
	if DisplayWidth("[x] ") != CheckboxSlot {
		t.Errorf("checkbox slot = %d, want %d", DisplayWidth("[x] "), CheckboxSlot)
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"abc", 3},
		{"日本語", 6},                         // wide runes count as two cells
		{"é", 1},                          // combining acute accent counts as zero
		{"\x1b[1;38;2;1;2;3mred\x1b[m", 3}, // ANSI styling ignored
		{"❯ ", 2},
	}
	for _, tt := range tests {
		if got := DisplayWidth(tt.s); got != tt.want {
			t.Errorf("DisplayWidth(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestTruncTail(t *testing.T) {
	tests := []struct {
		s      string
		max    int
		want   string
		maxOut int
	}{
		{"hello world", 20, "hello world", 20},
		{"hello world", 5, "hell…", 5},
		{"日本語テスト", 5, "日本…", 5}, // never overshoots on a wide-rune boundary
		{"anything", 0, "", 0},
	}
	for _, tt := range tests {
		got := TruncTail(tt.s, tt.max)
		if got != tt.want {
			t.Errorf("TruncTail(%q,%d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
		if DisplayWidth(got) > tt.maxOut {
			t.Errorf("TruncTail(%q,%d) width %d exceeds %d", tt.s, tt.max, DisplayWidth(got), tt.maxOut)
		}
	}
}

func TestTruncMiddle(t *testing.T) {
	got := TruncMiddle("/home/gustavo/workspace/projects/realizzo", 20)
	if DisplayWidth(got) > 20 {
		t.Errorf("TruncMiddle width %d exceeds 20: %q", DisplayWidth(got), got)
	}
	if !strings.Contains(got, ellipsis) {
		t.Errorf("TruncMiddle did not elide: %q", got)
	}
	if !strings.HasPrefix(got, "/home") {
		t.Errorf("TruncMiddle dropped the head: %q", got)
	}
	if !strings.HasSuffix(got, "realizzo") {
		t.Errorf("TruncMiddle dropped the tail: %q", got)
	}
	if s := "short"; TruncMiddle(s, 20) != s {
		t.Errorf("TruncMiddle shortened a string that already fit")
	}
}

func TestRowStableColumns(t *testing.T) {
	base := RowSpec{Width: 40, Checkable: true, Primary: "demo  my-work", Secondary: "3 hours ago • feature/x"}

	unfocused := Row(base)
	focused := Row(withFocus(base, true))
	checked := Row(withChecked(base, true))

	// Two lines each, and every line the same rendered width regardless of
	// focus or checked state.
	for _, got := range [][]string{unfocused, focused, checked} {
		if len(got) != 2 {
			t.Fatalf("Row produced %d lines, want 2: %q", len(got), got)
		}
	}
	want := MarkerSlot + CheckboxSlot
	for _, got := range [][]string{unfocused, focused, checked} {
		if col := primaryColumn(got[0]); col != want {
			t.Errorf("primary column = %d, want %d (line %q)", col, want, got[0])
		}
	}
	// The second line is indented to exactly the primary column.
	if got := leadingSpaces(unfocused[1]); got != want {
		t.Errorf("secondary indent = %d, want %d", got, want)
	}
}

func TestRowTruncatesToWidth(t *testing.T) {
	spec := RowSpec{Width: 24, Checkable: true, Focused: true,
		Primary:   "a-really-long-primary-identity-string",
		Secondary: "and-an-even-longer-secondary-metadata-line",
	}
	for _, line := range Row(spec) {
		if DisplayWidth(line) > 24 {
			t.Errorf("row line width %d exceeds 24: %q", DisplayWidth(line), line)
		}
	}
}

func TestRowHandlesWideUnicodePrimary(t *testing.T) {
	spec := RowSpec{Width: 20, Primary: "日本語-ブランチ-very-long", Secondary: "メタ"}
	for _, line := range Row(spec) {
		if DisplayWidth(line) > 20 {
			t.Errorf("row line width %d exceeds 20: %q", DisplayWidth(line), line)
		}
	}
}

func withFocus(s RowSpec, v bool) RowSpec   { s.Focused = v; return s }
func withChecked(s RowSpec, v bool) RowSpec { s.Checked = v; return s }

// primaryColumn is the cell offset at which the primary text begins, after the
// fixed marker and checkbox slots are stripped.
func primaryColumn(line string) int {
	rest := line
	for _, m := range []string{"❯ ", "  "} {
		if strings.HasPrefix(rest, m) {
			rest = rest[len(m):]
			break
		}
	}
	for _, b := range []string{"[ ] ", "[x] "} {
		if strings.HasPrefix(rest, b) {
			rest = rest[len(b):]
			break
		}
	}
	return DisplayWidth(line) - DisplayWidth(rest)
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
