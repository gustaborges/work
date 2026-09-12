package present

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// MultiSelectSpec describes a multi-choice step. When Confirm is non-nil, Enter
// with at least one row checked moves to a confirmation sub-state showing
// Confirm.Impact before the selection is returned; Esc there returns to the
// list without mutating anything (present cannot mutate).
type MultiSelectSpec[T comparable] struct {
	Title       string
	Description string
	Options     []Option[T]
	Filterable  bool
	Confirm     *ConfirmSpec
	// ConfirmImpact, when set, replaces Confirm.Impact with a line computed from
	// the checked options at the moment the confirmation opens — for a
	// consequence that depends on how many rows are checked. Presentation
	// strings only; present never inspects the underlying Values.
	ConfirmImpact func(picked []Option[T]) string
	Receipt       func(picked []Option[T]) string
}

type multiSelectModel[T comparable] struct {
	baseFrame
	spec MultiSelectSpec[T]

	filtering bool
	filter    string

	visible []int        // indices into spec.Options
	cursor  int          // index into visible
	offset  int          // first visible row
	checked map[int]bool // indices into spec.Options

	twoLine bool
	state   listState
	receipt string
}

func newMultiSelectModel[T comparable](io IO, spec MultiSelectSpec[T]) multiSelectModel[T] {
	m := multiSelectModel[T]{
		baseFrame: newBaseFrame(io),
		spec:      spec,
		checked:   make(map[int]bool, len(spec.Options)),
		twoLine:   anySecondary(spec.Options),
	}
	m.refilter()
	return m
}

func (m *multiSelectModel[T]) refilter() {
	m.visible = m.visible[:0]
	for i, o := range m.spec.Options {
		if matchesFilter(o, m.filter) {
			m.visible = append(m.visible, i)
		}
	}
	m.cursor, m.offset = clampScroll(m.cursor, m.offset, len(m.visible), m.rows())
}

func (m multiSelectModel[T]) rows() int {
	reserved := 3
	if m.spec.Description != "" {
		reserved += lineCount(Wrap(m.spec.Description, m.w))
	}
	per := 1
	if m.twoLine {
		per = 2
	}
	return max((Budget{Height: m.h, Reserved: reserved}).VisibleRows()/per, 1)
}

func (m multiSelectModel[T]) checkedIndices() []int {
	var out []int
	for i := range m.spec.Options {
		if m.checked[i] {
			out = append(out, i)
		}
	}
	return out
}

func (m multiSelectModel[T]) Init() tea.Cmd { return nil }

func (m multiSelectModel[T]) withFrame(f baseFrame) stepModel {
	m.baseFrame = f
	return m
}

func (m multiSelectModel[T]) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	if m.absorb(msg) {
		m.refilter()
		return m, nil
	}
	if paste, ok := msg.(tea.PasteMsg); ok {
		if m.filtering {
			m.filter += collapseToLine(paste.Content)
			m.cursor, m.offset = 0, 0
			m.refilter()
		}
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	s := key.String()
	if s == "ctrl+c" {
		m.state = listCancelled
		return m, nil
	}

	if m.state == listConfirming {
		switch s {
		case "esc":
			m.state = listChoosing // back to the list; nothing was mutated
		case "enter", "y", "Y":
			return m.complete()
		case "n", "N":
			m.state = listChoosing
		}
		return m, nil
	}

	if m.filtering {
		switch s {
		case "esc":
			m.filter, m.filtering = "", false
			m.refilter()
		case "enter":
			return m.advance()
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
		case "space":
			m.toggle()
		default:
			if key.Mod&^tea.ModShift == 0 && key.Text != "" && key.Text != " " {
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
	case "space":
		m.toggle()
	case "enter":
		return m.advance()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	}
	return m, nil
}

func (m *multiSelectModel[T]) move(delta int) {
	m.cursor, m.offset = clampScroll(m.cursor+delta, m.offset, len(m.visible), m.rows())
}

func (m *multiSelectModel[T]) toggle() {
	if m.cursor >= len(m.visible) {
		return
	}
	idx := m.visible[m.cursor]
	if m.checked[idx] {
		delete(m.checked, idx)
	} else {
		m.checked[idx] = true
	}
}

// advance handles Enter on the list: a no-op with nothing checked, otherwise
// either the confirmation sub-state or straight to completion.
func (m multiSelectModel[T]) advance() (stepModel, tea.Cmd) {
	if len(m.checkedIndices()) == 0 {
		return m, nil
	}
	m.filtering = false
	if m.spec.Confirm != nil {
		m.state = listConfirming
		return m, nil
	}
	return m.complete()
}

func (m multiSelectModel[T]) complete() (stepModel, tea.Cmd) {
	picked := make([]Option[T], 0, len(m.checkedIndices()))
	for _, idx := range m.checkedIndices() {
		picked = append(picked, m.spec.Options[idx])
	}
	switch {
	case m.spec.Confirm != nil:
		m.receipt = ConfirmReceipt(m.th, m.spec.Confirm.Title)
	case m.spec.Receipt != nil:
		m.receipt = Receipt(m.th, m.spec.Title, MarkSuccess, m.spec.Receipt(picked))
	default:
		labels := make([]string, len(picked))
		for i, p := range picked {
			labels[i] = p.Primary
		}
		m.receipt = Receipt(m.th, m.spec.Title, MarkSuccess, strings.Join(labels, ", "))
	}
	m.state = listCompleted
	return m, nil
}

func (m multiSelectModel[T]) status() stepStatus {
	return stepStatus{
		done:      m.state == listCompleted,
		cancelled: m.state == listCancelled,
		receipt:   m.receipt,
		answer:    m.picked(),
	}
}

func (m multiSelectModel[T]) cursorPos() (tea.Position, bool) { return tea.Position{}, false }

func (m multiSelectModel[T]) picked() []T {
	var out []T
	for _, idx := range m.checkedIndices() {
		out = append(out, m.spec.Options[idx].Value)
	}
	return out
}

func (m multiSelectModel[T]) body(width, height int) string {
	m.w, m.h = width, height
	switch m.state {
	case listCompleted, listCancelled:
		return ""
	case listConfirming:
		return m.confirmView()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.th.Primary.Render(m.spec.Title))
	if m.spec.Description != "" {
		fmt.Fprintf(&b, "%s\n", m.th.Muted.Render(Wrap(m.spec.Description, m.w)))
	}

	rows := m.rows()
	end := min(m.offset+rows, len(m.visible))
	for i := m.offset; i < end; i++ {
		idx := m.visible[i]
		opt := m.spec.Options[idx]
		spec := RowSpec{
			Width: m.w, Focused: i == m.cursor,
			Checkable: true, Checked: m.checked[idx],
			Primary: opt.Primary, Secondary: opt.Secondary,
		}
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
		h := "space toggle · ↑/↓ move · enter continue · q quit"
		if m.spec.Filterable {
			h = "space toggle · ↑/↓ move · / filter · enter continue · q quit"
		}
		b.WriteString(m.th.Muted.Render(h))
	}
	return b.String()
}

func (m multiSelectModel[T]) confirmView() string {
	c := m.spec.Confirm
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.th.Primary.Render(c.Title))
	for _, idx := range m.checkedIndices() {
		fmt.Fprintf(&b, "  %s\n", m.spec.Options[idx].Primary)
	}
	impact := c.Impact
	if m.spec.ConfirmImpact != nil {
		picked := make([]Option[T], 0, len(m.checkedIndices()))
		for _, idx := range m.checkedIndices() {
			picked = append(picked, m.spec.Options[idx])
		}
		impact = m.spec.ConfirmImpact(picked)
	}
	if impact != "" {
		fmt.Fprintf(&b, "\n%s\n", impact)
	}
	fmt.Fprintf(&b, "\n%s", m.th.Muted.Render("enter confirm · esc back · ctrl+c cancel"))
	return b.String()
}

// MultiSelect runs a multi-choice step and returns the checked options' Values
// in list order. Enter with nothing checked is a no-op. Cancellation
// (q / Esc on the list, Ctrl-C anywhere) returns diag.Cancelled (exit 20).
// Callers gate on an interactive terminal first.
func MultiSelect[T comparable](ctx context.Context, io IO, spec MultiSelectSpec[T]) ([]T, error) {
	ans, err := Wizard(ctx, io, WizardSpec{Steps: []Step{
		MultiSelectStep("value", func(Answers) (MultiSelectSpec[T], error) { return spec, nil }),
	}})
	if err != nil {
		return nil, err
	}
	v, _ := ans.Value("value").([]T)
	return v, nil
}
