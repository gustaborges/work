package present

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// SelectSpec describes a single-choice step over a list of options. When Grouped
// and the options carry two or more distinct Group values, a tab bar is shown
// and ←/→/Tab switch tabs. When Filterable, "/" starts a case-insensitive
// substring filter.
type SelectSpec[T any] struct {
	Title       string
	Description string
	Options     []Option[T]
	Grouped     bool
	Filterable  bool
	Receipt     func(Option[T]) string
}

type selectModel[T any] struct {
	baseFrame
	spec SelectSpec[T]

	tabs   []string // non-empty; nil ⇒ no tab bar
	active int

	filtering bool
	filter    string

	visible []int // indices into spec.Options, for the active tab + filter
	cursor  int   // index into visible
	offset  int   // first visible row

	twoLine bool
	state   listState
	chosen  int // index into spec.Options; -1 until chosen
	receipt string
}

func newSelectModel[T any](io IO, spec SelectSpec[T]) selectModel[T] {
	m := selectModel[T]{
		baseFrame: newBaseFrame(io),
		spec:      spec,
		chosen:    -1,
		twoLine:   anySecondary(spec.Options),
	}
	if spec.Grouped {
		if g := distinctGroups(spec.Options); len(g) >= 2 {
			m.tabs = g
		}
	}
	m.refilter()
	return m
}

// activeGroup is the group name filtering the list, or "" when there is no tab
// bar (every option is visible).
func (m selectModel[T]) activeGroup() string {
	if len(m.tabs) == 0 {
		return ""
	}
	return m.tabs[m.active]
}

func (m *selectModel[T]) refilter() {
	group := m.activeGroup()
	m.visible = m.visible[:0]
	for i, o := range m.spec.Options {
		if group != "" && o.Group != group {
			continue
		}
		if matchesFilter(o, m.filter) {
			m.visible = append(m.visible, i)
		}
	}
	m.cursor, m.offset = clampScroll(m.cursor, m.offset, len(m.visible), m.rows())
}

// rows is how many option rows fit the viewport after chrome.
func (m selectModel[T]) rows() int {
	reserved := 3 // title + scroll line + help/filter line
	if m.spec.Description != "" {
		reserved += lineCount(Wrap(m.spec.Description, m.w))
	}
	if len(m.tabs) > 0 {
		reserved += 2
	}
	per := 1
	if m.twoLine {
		per = 2
	}
	return max((Budget{Height: m.h, Reserved: reserved}).VisibleRows()/per, 1)
}

func (m selectModel[T]) Init() tea.Cmd { return nil }

func (m selectModel[T]) withFrame(f baseFrame) stepModel {
	m.baseFrame = f
	return m
}

func (m selectModel[T]) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	if m.absorb(msg) {
		m.refilter()
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		if m.filtering {
			m.filter += collapseToLine(msg.Content)
			m.cursor, m.offset = 0, 0
			m.refilter()
		}
		return m, nil
	}
	return m, nil
}

func (m selectModel[T]) handleKey(key tea.KeyPressMsg) (stepModel, tea.Cmd) {
	s := key.String()
	if s == "ctrl+c" {
		m.state = listCancelled
		return m, nil
	}

	if m.filtering {
		switch s {
		case "esc":
			m.filter, m.filtering = "", false
			m.refilter()
		case "enter":
			return m.choose()
		case "backspace":
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
				m.cursor, m.offset = 0, 0
				m.refilter()
			}
		case "up":
			m.move(-1)
		case "down":
			m.move(1)
		case "left", "right", "tab":
			m.switchTab(s)
		default:
			if key.Mod&^tea.ModShift == 0 && key.Text != "" {
				m.filter += key.Text
				m.cursor, m.offset = 0, 0
				m.refilter()
			}
		}
		return m, nil
	}

	switch s {
	case "q", "esc":
		m.state = listCancelled
		return m, nil
	case "/":
		if m.spec.Filterable {
			m.filtering = true
		}
	case "enter":
		return m.choose()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h", "right", "l", "tab":
		m.switchTab(s)
	}
	return m, nil
}

func (m *selectModel[T]) move(delta int) {
	m.cursor, m.offset = clampScroll(m.cursor+delta, m.offset, len(m.visible), m.rows())
}

func (m *selectModel[T]) switchTab(key string) {
	if len(m.tabs) < 2 {
		return
	}
	switch key {
	case "left", "h", "shift+tab":
		m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
	default:
		m.active = (m.active + 1) % len(m.tabs)
	}
	m.filter, m.filtering = "", false
	m.cursor, m.offset = 0, 0
	m.refilter()
}

func (m selectModel[T]) choose() (stepModel, tea.Cmd) {
	if m.cursor >= len(m.visible) {
		return m, nil // empty filter result
	}
	m.chosen = m.visible[m.cursor]
	opt := m.spec.Options[m.chosen]
	value := opt.Primary
	if m.spec.Receipt != nil {
		value = m.spec.Receipt(opt)
	}
	m.receipt = Receipt(m.th, m.spec.Title, MarkSuccess, value)
	m.state = listCompleted
	return m, nil
}

func (m selectModel[T]) status() stepStatus {
	st := stepStatus{
		done:      m.state == listCompleted,
		cancelled: m.state == listCancelled,
		receipt:   m.receipt,
	}
	if m.chosen >= 0 {
		st.answer = m.spec.Options[m.chosen].Value
	}
	return st
}

func (m selectModel[T]) cursorPos() (tea.Position, bool) { return tea.Position{}, false }

func (m selectModel[T]) body(width, height int) string {
	m.w, m.h = width, height
	switch m.state {
	case listCompleted, listCancelled:
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.th.Primary.Render(m.spec.Title))
	if m.spec.Description != "" {
		fmt.Fprintf(&b, "%s\n", m.th.Muted.Render(Wrap(m.spec.Description, m.w)))
	}
	if len(m.tabs) > 0 {
		b.WriteString(m.tabBar() + "\n\n")
	}

	rows := m.rows()
	end := min(m.offset+rows, len(m.visible))
	if len(m.visible) == 0 {
		b.WriteString(m.th.Muted.Render("  no match") + "\n")
	}
	for i := m.offset; i < end; i++ {
		opt := m.spec.Options[m.visible[i]]
		spec := RowSpec{Width: m.w, Focused: i == m.cursor, Primary: opt.Primary, Secondary: opt.Secondary}
		writeRow(&b, m.th, Row(spec), i == m.cursor)
	}

	if len(m.visible) > rows {
		fmt.Fprintf(&b, "%s\n", m.th.Muted.Render(fmt.Sprintf("  %d–%d of %d", m.offset+1, end, len(m.visible))))
	} else {
		b.WriteString("\n")
	}

	if m.filtering {
		fmt.Fprintf(&b, "%s%s", m.th.Secondary.Render("/"), m.filter)
	} else {
		b.WriteString(m.th.Muted.Render(m.helpLine()))
	}
	return b.String()
}

func (m selectModel[T]) tabBar() string {
	var parts []string
	for i, name := range m.tabs {
		if i == m.active {
			parts = append(parts, m.th.Primary.Render("["+name+"]"))
		} else {
			parts = append(parts, m.th.Muted.Render(" "+name+" "))
		}
	}
	return strings.Join(parts, "  ")
}

func (m selectModel[T]) helpLine() string {
	h := "↑/↓ move · enter select · q quit"
	if m.spec.Filterable {
		h = "↑/↓ move · / filter · enter select · q quit"
	}
	if len(m.tabs) > 1 {
		h = "←/→ switch · " + h
	}
	return h
}

// Select runs a single-choice step and returns the chosen option's Value.
// Cancellation (q / Esc / Ctrl-C) returns diag.Cancelled (exit 20). Callers gate
// on an interactive terminal first.
func Select[T any](ctx context.Context, io IO, spec SelectSpec[T]) (T, error) {
	var zero T
	ans, err := Wizard(ctx, io, WizardSpec{Steps: []Step{
		SelectStep("value", func(Answers) (SelectSpec[T], error) { return spec, nil }),
	}})
	if err != nil {
		return zero, err
	}
	v, _ := ans.Value("value").(T)
	return v, nil
}
