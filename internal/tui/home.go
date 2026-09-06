package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/diag"
)

// HomeChoice identifies what the user selected from the `work` home screen.
type HomeChoice int

const (
	// HomeQuit means the user left the home without choosing a journey
	// (q / Esc / Ctrl-C). The caller exits 0 with no state change.
	HomeQuit HomeChoice = iota
	// HomeStartWork is the "Start a Work" journey — equivalent to `work start`
	// with no SOURCE.
	HomeStartWork
	// HomeResumeWork is the "Resume a Work" journey — equivalent to `work
	// resume` with no target (the recency picker).
	HomeResumeWork
	// HomeArchiveWork is the "Archive Works" journey — equivalent to `work
	// archive` with no targets (the multi-select picker).
	HomeArchiveWork
)

// homeItem is one selectable row in the home menu.
type homeItem struct {
	label  string
	choice HomeChoice
}

// homeModel is the Bubble Tea model for `work` with no arguments. It lists the
// daily journeys shipped so far; actions reserved for later slices (status,
// import, link, plugin, repository, convention) are intentionally absent
// (contracts/cli-work-home.md, FR-029, FR-030).
type homeModel struct {
	items  []homeItem
	cursor int
	choice HomeChoice
}

func newHomeModel() homeModel {
	return homeModel{
		items: []homeItem{
			{label: "Start a Work", choice: HomeStartWork},
			{label: "Resume a Work", choice: HomeResumeWork},
			{label: "Archive Works", choice: HomeArchiveWork},
		},
	}
}

func (m homeModel) Init() tea.Cmd { return nil }

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "esc", "ctrl+c":
		m.choice = HomeQuit
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		m.choice = m.items[m.cursor].choice
		return m, tea.Quit
	}
	return m, nil
}

func (m homeModel) View() tea.View {
	var b strings.Builder
	b.WriteString("work — start an isolated unit of work\n\n")
	for i, it := range m.items {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", marker, it.label)
	}
	b.WriteString("\n↑/↓ move · enter select · q quit\n")
	return tea.NewView(b.String())
}

// RunHome opens the interactive `work` home and returns the journey the user
// selected, or HomeQuit when they leave without choosing. It requires an
// interactive terminal; callers gate on tui.IsInteractive first.
func RunHome(ctx context.Context) (HomeChoice, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(newHomeModel(), tea.WithContext(ctx)).Run()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return HomeQuit, diag.New(diag.Cancelled, "cancelled")
		}
		return HomeQuit, diag.Wrap(diag.Usage, err, "the home screen could not be shown")
	}
	m, ok := final.(homeModel)
	if !ok {
		return HomeQuit, diag.New(diag.Usage, "the home screen ended in an unexpected state")
	}
	return m.choice, nil
}
