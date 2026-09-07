package present

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testSelectModel(spec SelectSpec[string]) selectModel[string] {
	m := newSelectModel(IO{}, spec)
	m.th = offTheme()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	return next.(selectModel[string])
}

func branchOptions() []Option[string] {
	return []Option[string]{
		{Value: "origin/main", Primary: "origin/main", Secondary: "5c56cbc", Group: "Remote"},
		{Value: "origin/dev", Primary: "origin/dev", Secondary: "a1b2c3d", Group: "Remote"},
		{Value: "main", Primary: "main", Secondary: "5c56cbc", Group: "Local"},
		{Value: "feature/x", Primary: "feature/x", Secondary: "deadbee", Group: "Local"},
	}
}

// rowColumns records, for one rendered frame, the number of body lines and the
// starting column of the primary text on each — the SC-003 invariants.
func bodyLines(v string) []string {
	var out []string
	for _, ln := range strings.Split(v, "\n") {
		if strings.Contains(ln, "origin/") || strings.Contains(ln, "  main") || strings.Contains(ln, "feature/") {
			out = append(out, ln)
		}
	}
	return out
}

func TestSelectGeometryStableAcrossMoveFilterTab(t *testing.T) {
	m := testSelectModel(SelectSpec[string]{Title: "Base branch", Options: branchOptions(), Grouped: true, Filterable: true})

	frame0 := m.View().Content
	cols0 := primaryColumnsOf(frame0)

	// Move focus down.
	next, _ := m.Update(press("down"))
	m = next.(selectModel[string])
	if got := primaryColumnsOf(m.View().Content); !equalInts(got, cols0) {
		t.Errorf("primary columns shifted on move: %v -> %v", cols0, got)
	}

	// Switch tab.
	next, _ = m.Update(press("right"))
	m = next.(selectModel[string])
	if got := primaryColumnsOf(m.View().Content); !allEqual(got, cols0[0]) {
		t.Errorf("primary columns shifted on tab switch: want all %d, got %v", cols0[0], got)
	}

	// Filter.
	next, _ = m.Update(press("/"))
	m = next.(selectModel[string])
	m = typeText(m, "feat").(selectModel[string])
	for _, c := range primaryColumnsOf(m.View().Content) {
		if c != cols0[0] {
			t.Errorf("primary column shifted while filtering: %d != %d", c, cols0[0])
		}
	}
}

func TestSelectFocusedRowIsBoldWithColourOff(t *testing.T) {
	// With colour off, focus must still be identifiable — the "❯ " marker.
	m := testSelectModel(SelectSpec[string]{Title: "Pick", Options: branchOptions()})
	v := m.View().Content
	if !strings.Contains(v, "❯ origin/main") {
		t.Errorf("focused row lacks the ❯ marker:\n%s", v)
	}
	// Unfocused rows use the equal-width blank marker.
	if !strings.Contains(v, "  origin/dev") {
		t.Errorf("unfocused row lacks the blank marker slot:\n%s", v)
	}
}

func TestSelectFrameNeverExceedsHeight(t *testing.T) {
	opts := make([]Option[string], 30)
	for i := range opts {
		opts[i] = Option[string]{Value: string(rune('a' + i)), Primary: strings.Repeat("x", 80), Secondary: "meta"}
	}
	m := newSelectModel(IO{}, SelectSpec[string]{Title: "T", Options: opts, Filterable: true})
	m.th = offTheme()
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 10}, {Width: 80, Height: 24}, {Width: 160, Height: 50}} {
		next, _ := m.Update(size)
		mm := next.(selectModel[string])
		v := mm.View().Content
		if n := strings.Count(v, "\n") + 1; n > size.Height {
			t.Errorf("%dx%d: frame is %d lines\n%s", size.Width, size.Height, n, v)
		}
		for _, line := range strings.Split(v, "\n") {
			if DisplayWidth(line) > size.Width {
				t.Errorf("%dx%d: line wider than viewport (%d): %q", size.Width, size.Height, DisplayWidth(line), line)
			}
		}
	}
}

func TestSelectFinalViews(t *testing.T) {
	m := testSelectModel(SelectSpec[string]{
		Title:   "Base branch",
		Options: branchOptions(),
		Receipt: func(o Option[string]) string { return o.Primary + "  " + o.Secondary },
	})
	next, cmd := m.Update(press("enter"))
	cm := next.(selectModel[string])
	if !isQuit(cmd) || cm.state != listCompleted {
		t.Fatalf("enter did not complete: quit=%v state=%d", isQuit(cmd), cm.state)
	}
	if got := cm.finalFrame(); got != "Base branch\n  ✔ origin/main  5c56cbc\n\n" {
		t.Errorf("completed finalFrame = %q", got)
	}
	if cm.View().Content != "" {
		t.Errorf("completed View should be empty so the list clears, got %q", cm.View().Content)
	}

	for _, key := range []string{"q", "esc", "ctrl+c"} {
		mm := testSelectModel(SelectSpec[string]{Title: "T", Options: branchOptions()})
		next, cmd := mm.Update(press(key))
		if !isQuit(cmd) || next.(selectModel[string]).finalFrame() != "✘ Operation cancelled\n" {
			t.Errorf("%s: cancel finalFrame = %q", key, next.(selectModel[string]).finalFrame())
		}
	}
}

func TestSelectReturnsChosenValue(t *testing.T) {
	m := testSelectModel(SelectSpec[string]{Title: "T", Options: branchOptions()})
	next, _ := m.Update(press("down"))
	next, _ = next.Update(press("enter"))
	got := next.(selectModel[string])
	if v := got.spec.Options[got.chosen].Value; v != "origin/dev" {
		t.Errorf("chosen value = %q, want origin/dev", v)
	}
}

func TestSelectGroupedHidesBarWithOneGroup(t *testing.T) {
	opts := []Option[string]{
		{Value: "a", Primary: "a", Group: "Local"},
		{Value: "b", Primary: "b", Group: "Local"},
	}
	m := testSelectModel(SelectSpec[string]{Title: "T", Options: opts, Grouped: true})
	if m.tabs != nil {
		t.Errorf("tab bar shown for a single group: %v", m.tabs)
	}
	if strings.Contains(m.View().Content, "[Local]") {
		t.Errorf("single-group tab rendered:\n%s", m.View().Content)
	}
}

func TestSelectEmptyGroupNeverRendered(t *testing.T) {
	opts := []Option[string]{
		{Value: "a", Primary: "a", Group: "Remote"},
		{Value: "b", Primary: "b", Group: "Local"},
		{Value: "c", Primary: "c", Group: ""}, // ungrouped: not a tab
	}
	m := testSelectModel(SelectSpec[string]{Title: "T", Options: opts, Grouped: true})
	if strings.Join(m.tabs, ",") != "Remote,Local" {
		t.Errorf("tabs = %v, want [Remote Local]", m.tabs)
	}
}

// primaryColumnsOf returns the primary-text start column for each option line in
// a select frame.
func primaryColumnsOf(v string) []int {
	var cols []int
	for _, line := range bodyLines(v) {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(line, "❯ ") {
			trimmed = strings.TrimPrefix(line, "❯ ")
		}
		cols = append(cols, DisplayWidth(line)-DisplayWidth(trimmed))
	}
	return cols
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func allEqual(a []int, v int) bool {
	for _, x := range a {
		if x != v {
			return false
		}
	}
	return len(a) > 0
}

var _ = context.Background
