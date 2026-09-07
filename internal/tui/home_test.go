package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "ctrl+c":
		return tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg(tea.Key{Code: r, Text: s})
	}
}

func step(m tea.Model, keys ...string) (homeModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(press(k))
	}
	return m.(homeModel), cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestHomeListsShippedJourneys(t *testing.T) {
	m := newHomeModel()
	if len(m.items) != 3 || m.items[0].choice != HomeStartWork ||
		m.items[1].choice != HomeResumeWork || m.items[2].choice != HomeArchiveWork {
		t.Fatalf("home items = %+v, want [Start a Work, Resume a Work, Archive Works]", m.items)
	}
	view := m.View().Content
	for _, want := range []string{"Start a Work", "Resume a Work", "Archive Works"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	for _, reserved := range []string{"status", "import", "link", "plugin", "repository", "convention"} {
		if strings.Contains(strings.ToLower(view), reserved) {
			t.Errorf("view exposes a later-slice action %q:\n%s", reserved, view)
		}
	}
}

func TestHomeEnterSelectsStartWork(t *testing.T) {
	m, cmd := step(newHomeModel(), "enter")
	if m.choice != HomeStartWork {
		t.Errorf("choice = %d, want HomeStartWork", m.choice)
	}
	if !isQuit(cmd) {
		t.Error("Enter did not quit the program")
	}
}

func TestHomeEnterSelectsResumeWork(t *testing.T) {
	m, cmd := step(newHomeModel(), "down", "enter")
	if m.choice != HomeResumeWork {
		t.Errorf("choice = %d, want HomeResumeWork", m.choice)
	}
	if !isQuit(cmd) {
		t.Error("Enter did not quit the program")
	}
}

func TestHomeEnterSelectsArchiveWork(t *testing.T) {
	m, cmd := step(newHomeModel(), "down", "down", "enter")
	if m.choice != HomeArchiveWork {
		t.Errorf("choice = %d, want HomeArchiveWork", m.choice)
	}
	if !isQuit(cmd) {
		t.Error("Enter did not quit the program")
	}
}

func TestHomeQuitKeysLeaveNoChoice(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		m, cmd := step(newHomeModel(), key)
		if m.choice != HomeQuit {
			t.Errorf("%s: choice = %d, want HomeQuit", key, m.choice)
		}
		if !isQuit(cmd) {
			t.Errorf("%s: did not quit", key)
		}
	}
}

func TestHomeCursorStaysInBounds(t *testing.T) {
	last := len(newHomeModel().items) - 1
	// Moving down past the end stops on the last row, never beyond it.
	m, _ := step(newHomeModel(), "down", "down", "j")
	if m.cursor != last {
		t.Errorf("cursor = %d after moving down past the end, want %d", m.cursor, last)
	}
	m, _ = step(newHomeModel(), "up", "k")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after moving up past the start, want 0", m.cursor)
	}
}

func TestHomeIgnoresNonKeyMessages(t *testing.T) {
	m := newHomeModel()
	got, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("non-key message produced a command")
	}
	if got.(homeModel).choice != HomeQuit {
		t.Error("non-key message changed the choice")
	}
}
