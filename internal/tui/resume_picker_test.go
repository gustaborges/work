package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/worklist"
)

func rpRows() []worklist.WorkRow {
	return []worklist.WorkRow{
		{ID: "id-charlie", DisplayName: "demo  charlie", Branch: "charlie", RelativeTime: "just now"},
		{ID: "id-bravo", DisplayName: "demo  bravo", Branch: "bravo", RelativeTime: "a minute ago"},
		{ID: "id-alpha", DisplayName: "demo  alpha", Branch: "alpha", RelativeTime: "2 minutes ago"},
	}
}

func rpStep(m resumeModel, keys ...string) (resumeModel, tea.Cmd) {
	var cmd tea.Cmd
	var tm tea.Model = m
	for _, k := range keys {
		tm, cmd = tm.Update(press(k))
	}
	return tm.(resumeModel), cmd
}

func TestResumePickerSelectsByCursor(t *testing.T) {
	m, cmd := rpStep(newResumeModel(rpRows()), "down", "down", "enter")
	if !isQuit(cmd) {
		t.Fatal("Enter did not quit")
	}
	if m.selected < 0 || m.rows[m.selected].ID != "id-alpha" {
		t.Fatalf("selected = %d, want the row for id-alpha", m.selected)
	}
}

func TestResumePickerCursorClamps(t *testing.T) {
	m, _ := rpStep(newResumeModel(rpRows()), "down", "down", "down", "down", "j")
	if m.cursor != len(m.visible)-1 {
		t.Errorf("cursor = %d, want %d (clamped to last row)", m.cursor, len(m.visible)-1)
	}
	m, _ = rpStep(newResumeModel(rpRows()), "up", "k")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped to first row)", m.cursor)
	}
}

func TestResumePickerFilters(t *testing.T) {
	// "/" then "alp" narrows to the alpha row; Enter selects it.
	m, cmd := rpStep(newResumeModel(rpRows()), "/", "a", "l", "p", "enter")
	if !isQuit(cmd) {
		t.Fatal("Enter did not quit after filtering")
	}
	if m.selected < 0 || m.rows[m.selected].ID != "id-alpha" {
		t.Fatalf("filtered selection = %d, want id-alpha", m.selected)
	}
}

func TestResumePickerFilterNoMatchIsInertOnEnter(t *testing.T) {
	m, cmd := rpStep(newResumeModel(rpRows()), "/", "z", "z", "z", "enter")
	if isQuit(cmd) {
		t.Error("Enter with an empty filter result should be a no-op, not a quit")
	}
	if m.selected != -1 {
		t.Errorf("selected = %d, want -1", m.selected)
	}
}

func TestResumePickerCancelKeysLeaveNoSelection(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		m, cmd := rpStep(newResumeModel(rpRows()), key)
		if !isQuit(cmd) {
			t.Errorf("%s: did not quit", key)
		}
		if m.selected != -1 {
			t.Errorf("%s: selected = %d, want -1", key, m.selected)
		}
	}
}

func TestResumePickerViewShowsTwoLineRows(t *testing.T) {
	view := newResumeModel(rpRows()).View().Content
	if !strings.Contains(view, "demo  charlie") {
		t.Errorf("view missing the name line:\n%s", view)
	}
	if !strings.Contains(view, "just now • charlie") {
		t.Errorf("view missing the relative-time/branch line:\n%s", view)
	}
}
