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

func TestHomeListsOnlyStartAWork(t *testing.T) {
	m := newHomeModel()
	if len(m.items) != 1 || m.items[0].choice != HomeStartWork {
		t.Fatalf("home items = %+v, want exactly [Start a Work]", m.items)
	}
	view := m.View().Content
	if !strings.Contains(view, "Start a Work") {
		t.Errorf("view missing the journey label:\n%s", view)
	}
	for _, reserved := range []string{"resume", "archive", "status", "import", "link", "plugin", "repository", "convention"} {
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
	// Single item: up and down are no-ops and never move off the list.
	m, _ := step(newHomeModel(), "down", "down", "j")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after moving down past the end, want 0", m.cursor)
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
