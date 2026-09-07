package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/worklist"
)

func apRows() []worklist.WorkRow {
	return []worklist.WorkRow{
		{ID: "id-charlie", DisplayName: "demo  charlie", Branch: "charlie", RelativeTime: "just now"},
		{ID: "id-bravo", DisplayName: "demo  bravo", Branch: "bravo", RelativeTime: "a minute ago"},
		{ID: "id-alpha", DisplayName: "demo  alpha", Branch: "alpha", RelativeTime: "2 minutes ago"},
	}
}

func apStep(m archiveModel, keys ...string) (archiveModel, tea.Cmd) {
	var cmd tea.Cmd
	var tm tea.Model = m
	for _, k := range keys {
		tm, cmd = tm.Update(press(k))
	}
	return tm.(archiveModel), cmd
}

func TestArchivePickerNothingPreselected(t *testing.T) {
	m := newArchiveModel(apRows(), "/ws")
	if len(m.checkedIndices()) != 0 {
		t.Fatalf("archive picker preselected %d rows, want 0", len(m.checkedIndices()))
	}
}

func TestArchivePickerSpaceTogglesCheckbox(t *testing.T) {
	m, _ := apStep(newArchiveModel(apRows(), "/ws"), " ")
	if !m.checked[0] {
		t.Fatal("space did not check the cursor row")
	}
	m, _ = apStep(m, " ")
	if m.checked[0] {
		t.Fatal("second space did not uncheck the row")
	}
}

func TestArchivePickerEmptyEnterIsNoOp(t *testing.T) {
	m, cmd := apStep(newArchiveModel(apRows(), "/ws"), "enter")
	if isQuit(cmd) {
		t.Error("Enter with nothing checked should not quit")
	}
	if m.stage != apList {
		t.Errorf("stage = %d, want apList after empty Enter", m.stage)
	}
}

func TestArchivePickerConfirmFlow(t *testing.T) {
	// Check charlie and alpha, continue to the confirmation view, confirm.
	m, _ := apStep(newArchiveModel(apRows(), "/ws"), " ", "down", "down", " ", "enter")
	if m.stage != apConfirm {
		t.Fatalf("stage = %d, want apConfirm", m.stage)
	}
	view := m.View().Content
	if !strings.Contains(view, "2 worktree(s) will be destroyed") {
		t.Errorf("confirm view missing the consequence line:\n%s", view)
	}
	if !strings.Contains(view, "Branches are kept") {
		t.Errorf("confirm view missing the branch-kept assurance:\n%s", view)
	}

	m, cmd := apStep(m, "enter")
	if !isQuit(cmd) {
		t.Fatal("Enter on the confirmation view did not quit")
	}
	if !m.confirmed {
		t.Fatal("confirmed flag not set")
	}
	ids := checkedIDs(m)
	if strings.Join(ids, ",") != "id-charlie,id-alpha" {
		t.Fatalf("confirmed ids = %v, want [id-charlie id-alpha]", ids)
	}
}

func TestArchivePickerEscGoesBackFromConfirm(t *testing.T) {
	m, _ := apStep(newArchiveModel(apRows(), "/ws"), " ", "enter")
	if m.stage != apConfirm {
		t.Fatalf("stage = %d, want apConfirm", m.stage)
	}
	m, _ = apStep(m, "esc")
	if m.stage != apList {
		t.Errorf("Esc did not return to the list (stage = %d)", m.stage)
	}
	if !m.checked[0] {
		t.Error("Esc back to the list dropped the selection")
	}
}

func TestArchivePickerCancelKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		m, cmd := apStep(newArchiveModel(apRows(), "/ws"), key)
		if !isQuit(cmd) {
			t.Errorf("%s: did not quit", key)
		}
		if !m.cancelled {
			t.Errorf("%s: cancelled flag not set", key)
		}
	}
	// Ctrl-C also cancels from the confirmation view.
	m, cmd := apStep(newArchiveModel(apRows(), "/ws"), " ", "enter", "ctrl+c")
	if !isQuit(cmd) || !m.cancelled {
		t.Error("Ctrl-C on the confirmation view did not cancel")
	}
}

func checkedIDs(m archiveModel) []string {
	var ids []string
	for _, idx := range m.checkedIndices() {
		ids = append(ids, m.rows[idx].ID)
	}
	return ids
}
