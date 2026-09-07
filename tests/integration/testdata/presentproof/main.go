// Command presentproof is a throwaway de-risking probe for F2.5 Phase 1: a tiny
// inline Bubble Tea input model that shows an in-frame `✘` error which is
// replaced on the next keystroke and, on Enter with an accepted value, sets its
// final View to a compact receipt (`Title` / `  ✔ <value>`) before quitting.
//
// The PTY proof test (tests/integration/present_proof_test.go) drives this to
// confirm Bubble Tea's graceful final render commits only the compact receipt
// frame on a real terminal. Both this program and that test are deleted in
// Phase 2 once the real internal/present.Input exists.
package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const title = "Local repository path"

type model struct {
	value     string
	lastErr   string
	completed bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "enter":
		if m.value == "good" {
			m.completed = true
			return m, tea.Quit
		}
		// Reject: the error is in-frame only and the field resets, so the next
		// keystrokes type a fresh value. The rejected text is never echoed.
		m.lastErr = "that path does not exist"
		m.value = ""
		return m, nil
	case "backspace":
		if m.value != "" {
			m.value = m.value[:len(m.value)-1]
		}
		return m, nil
	default:
		if s := key.String(); len(s) == 1 {
			m.value += s
			m.lastErr = "" // a new keystroke replaces the current error
		}
		return m, nil
	}
}

func (m model) View() tea.View {
	if m.completed {
		// The final frame: only the compact receipt, no error line, no prompt.
		return tea.NewView(title + "\n  ✔ " + m.value + "\n")
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString("> " + m.value + "\n")
	if m.lastErr != "" {
		b.WriteString("\n✘ " + m.lastErr + "\n")
	}
	return tea.NewView(b.String())
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
