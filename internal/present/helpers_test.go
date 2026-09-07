package present

import (
	"reflect"

	tea "charm.land/bubbletea/v2"
)

// press builds a KeyPressMsg from a short spec: named special keys, "ctrl+X"
// combos, or a literal string that becomes typed text.
func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "backspace":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
	case "space":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Text: " "})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "left":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft})
	case "right":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyRight})
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	}
	if len(s) > 5 && s[:5] == "ctrl+" {
		return tea.KeyPressMsg(tea.Key{Code: rune(s[5]), Mod: tea.ModCtrl})
	}
	return tea.KeyPressMsg(tea.Key{Code: []rune(s)[0], Text: s})
}

// typeText feeds each rune of s as its own key press.
func typeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	return m
}

// drain resolves a command (recursing into tea.BatchMsg and the unexported
// []Cmd message tea.Sequence produces) and returns every concrete message.
func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			out = append(out, drain(c)...)
		}
	case nil:
	default:
		if rv := reflect.ValueOf(msg); rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
					out = append(out, drain(c)...)
				}
			}
			break
		}
		out = append(out, msg)
	}
	return out
}

// feed applies every message in msgs to m in order.
func feed(m tea.Model, msgs ...tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		m, cmd = m.Update(msg)
	}
	return m, cmd
}

func isQuit(cmd tea.Cmd) bool {
	for _, msg := range drain(cmd) {
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}
