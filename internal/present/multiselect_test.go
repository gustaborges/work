package present

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func workRows() []Option[string] {
	return []Option[string]{
		{Value: "id-a", Primary: "demo  alpha", Secondary: "4 hours ago • feature/alpha"},
		{Value: "id-b", Primary: "demo  bravo", Secondary: "3 hours ago • feature/bravo"},
		{Value: "id-c", Primary: "demo  charlie", Secondary: "1 hour ago • feature/charlie"},
	}
}

func testMultiModel(spec MultiSelectSpec[string]) multiSelectModel[string] {
	m := newMultiSelectModel(IO{}, spec)
	m.th = offTheme()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	return next.(multiSelectModel[string])
}

func TestMultiCheckboxAndFocusAreIndependent(t *testing.T) {
	m := testMultiModel(MultiSelectSpec[string]{Title: "Archive Works", Options: workRows()})

	// Check row 0, then move focus to row 1. Row 0 stays checked; focus moved.
	next, _ := m.Update(press("space"))
	next, _ = next.Update(press("down"))
	mm := next.(multiSelectModel[string])
	if !mm.checked[0] {
		t.Errorf("row 0 lost its check when focus moved")
	}
	if mm.cursor != 1 {
		t.Errorf("cursor = %d, want 1", mm.cursor)
	}

	v := mm.View().Content
	// Row 0: unfocused + checked → "  [x] demo  alpha"
	if !strings.Contains(v, "  [x] demo  alpha") {
		t.Errorf("checked-unfocused row wrong:\n%s", v)
	}
	// Row 1: focused + unchecked → "❯ [ ] demo  bravo" (styled, but colour off)
	if !strings.Contains(v, "❯ [ ] demo  bravo") {
		t.Errorf("focused-unchecked row wrong:\n%s", v)
	}
}

// TestMultiUnselectedRowsAreStableAcrossFocusMove is the explicit regression
// guard for the F2 archive_picker double-render defect: every unselected row
// keeps identical rendered height and column starts before and after a focus
// move.
func TestMultiUnselectedRowsAreStableAcrossFocusMove(t *testing.T) {
	m := testMultiModel(MultiSelectSpec[string]{Title: "Archive Works", Options: workRows()})

	before := rowGeometry(m.View().Content)
	next, _ := m.Update(press("down"))
	next, _ = next.Update(press("down"))
	after := rowGeometry(next.(multiSelectModel[string]).View().Content)

	if len(before) != len(after) {
		t.Fatalf("row count changed on focus move: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("row %d geometry changed on focus move: %+v -> %+v", i, before[i], after[i])
		}
	}
}

func TestMultiEmptyEnterIsNoOp(t *testing.T) {
	m := testMultiModel(MultiSelectSpec[string]{Title: "T", Options: workRows()})
	next, cmd := m.Update(press("enter"))
	if isQuit(cmd) || next.(multiSelectModel[string]).state != listChoosing {
		t.Errorf("Enter with nothing checked advanced the step")
	}
}

func TestMultiConfirmFlow(t *testing.T) {
	spec := MultiSelectSpec[string]{
		Title:   "Archive Works",
		Options: workRows(),
		Confirm: &ConfirmSpec{Title: "Archive Works", Impact: "2 worktree(s) will be destroyed"},
	}
	m := testMultiModel(spec)
	// check alpha and charlie, Enter → confirming
	next, _ := m.Update(press("space"))
	next, _ = next.Update(press("down"))
	next, _ = next.Update(press("down"))
	next, _ = next.Update(press("space"))
	next, _ = next.Update(press("enter"))
	cm := next.(multiSelectModel[string])
	if cm.state != listConfirming {
		t.Fatalf("state = %d, want confirming", cm.state)
	}
	if !strings.Contains(cm.View().Content, "2 worktree(s) will be destroyed") {
		t.Errorf("impact not shown in confirm view:\n%s", cm.View().Content)
	}

	// Esc goes back to the list without dropping the checks.
	next, _ = cm.Update(press("esc"))
	back := next.(multiSelectModel[string])
	if back.state != listChoosing {
		t.Errorf("Esc did not return to the list: state=%d", back.state)
	}
	if !back.checked[0] || !back.checked[2] {
		t.Errorf("Esc from confirming dropped a selection: %v", back.checked)
	}

	// Enter again → confirming → Enter → completed.
	next, _ = back.Update(press("enter"))
	next, cmd := next.Update(press("enter"))
	done := next.(multiSelectModel[string])
	if !isQuit(cmd) || done.state != listCompleted {
		t.Fatalf("confirm accept did not complete: quit=%v state=%d", isQuit(cmd), done.state)
	}
	if done.View().Content != "✔ Archive Works confirmed\n" {
		t.Errorf("completed view = %q", done.View().Content)
	}
	if got := done.picked(); strings.Join(got, ",") != "id-a,id-c" {
		t.Errorf("picked = %v, want [id-a id-c]", got)
	}
}

func TestMultiCancelKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		m := testMultiModel(MultiSelectSpec[string]{Title: "T", Options: workRows()})
		next, cmd := m.Update(press(key))
		mm := next.(multiSelectModel[string])
		if !isQuit(cmd) || !mm.outcome().cancelled {
			t.Errorf("%s: quit=%v cancelled=%v", key, isQuit(cmd), mm.outcome().cancelled)
		}
		if mm.View().Content != "✘ Operation cancelled\n" {
			t.Errorf("%s: view = %q", key, mm.View().Content)
		}
	}
	// Ctrl-C also cancels from the confirming sub-state.
	spec := MultiSelectSpec[string]{Title: "T", Options: workRows(), Confirm: &ConfirmSpec{Title: "T"}}
	m := testMultiModel(spec)
	next, _ := m.Update(press("space"))
	next, _ = next.Update(press("enter"))
	next, cmd := next.Update(press("ctrl+c"))
	if !isQuit(cmd) || !next.(multiSelectModel[string]).outcome().cancelled {
		t.Errorf("Ctrl-C from confirming did not cancel")
	}
}

// rowGeometry is the (indent, width) of each option line in a list frame.
type geom struct{ indent, width int }

func rowGeometry(v string) []geom {
	var out []geom
	for _, line := range strings.Split(v, "\n") {
		if !strings.Contains(line, "demo  ") {
			continue
		}
		trimmed := line
		for _, p := range []string{"❯ ", "  "} {
			if strings.HasPrefix(trimmed, p) {
				trimmed = trimmed[len(p):]
				break
			}
		}
		for _, p := range []string{"[ ] ", "[x] "} {
			if strings.HasPrefix(trimmed, p) {
				trimmed = trimmed[len(p):]
				break
			}
		}
		out = append(out, geom{indent: DisplayWidth(line) - DisplayWidth(trimmed), width: DisplayWidth(line)})
	}
	return out
}
