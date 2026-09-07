package present

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ConfirmSpec describes a yes/no gate in front of a mutation. Impact is the
// multi-line preview shown until the user decides; it is the consent surface,
// not the audit record (research R7).
type ConfirmSpec struct {
	Title  string
	Impact string
	Accept string // affirmative label, e.g. "Create"
	Reject string // negative label, e.g. "Cancel"
	// Receipt optionally overrides the collapsed line for the accepted case.
	Receipt func(accepted bool) string
}

func (s ConfirmSpec) accept() string {
	if s.Accept == "" {
		return "Yes"
	}
	return s.Accept
}

func (s ConfirmSpec) reject() string {
	if s.Reject == "" {
		return "No"
	}
	return s.Reject
}

type confirmModel struct {
	baseFrame
	spec  ConfirmSpec
	focus int // 0 = accept, 1 = reject

	state    listState
	accepted bool
	final    string
}

func newConfirmModel(io IO, spec ConfirmSpec) confirmModel {
	return confirmModel{baseFrame: newBaseFrame(io), spec: spec, focus: 1} // default to Reject
}

func (m confirmModel) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.absorb(msg) {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc":
		m.state = listCancelled
		return m, leave()
	case "left", "h", "right", "l", "tab":
		m.focus ^= 1
	case "y", "Y":
		return m.decide(true)
	case "n", "N":
		return m.decide(false)
	case "enter":
		return m.decide(m.focus == 0)
	}
	return m, nil
}

func (m confirmModel) decide(accepted bool) (tea.Model, tea.Cmd) {
	m.accepted = accepted
	m.state = listCompleted
	switch {
	case m.spec.Receipt != nil:
		m.final = m.spec.Receipt(accepted)
		if !strings.HasSuffix(m.final, "\n") {
			m.final += "\n"
		}
	case accepted:
		m.final = ConfirmReceipt(m.th, m.spec.Title)
	default:
		m.final = fmt.Sprintf("%s %s\n", markGlyph(m.th, MarkFailure), m.spec.reject())
	}
	return m, leave()
}

func (m confirmModel) outcome() outcome {
	return outcome{cancelled: m.state == listCancelled}
}

func (m confirmModel) finalFrame() string {
	if m.state == listCompleted {
		return m.final
	}
	// A cancelled confirmation leaves nothing in history; the "✘ Operation
	// cancelled" line is the CLI diagnostic border's (contracts/diagnostics.md).
	return ""
}

func (m confirmModel) View() tea.View {
	switch m.state {
	case listCancelled, listCompleted:
		return tea.NewView("")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.th.Primary.Render(m.spec.Title))
	if m.spec.Impact != "" {
		fmt.Fprintf(&b, "%s\n\n", m.spec.Impact)
	}
	b.WriteString(m.choiceBar() + "\n\n")
	b.WriteString(m.th.Muted.Render("←/→ move · enter choose · y/n · esc cancel"))
	return m.clamp(b.String())
}

func (m confirmModel) choiceBar() string {
	accept, reject := "  "+m.spec.accept()+"  ", "  "+m.spec.reject()+"  "
	if m.focus == 0 {
		accept = m.th.Primary.Render("[ " + m.spec.accept() + " ]")
	} else {
		reject = m.th.Primary.Render("[ " + m.spec.reject() + " ]")
	}
	return accept + "  " + reject
}

// Confirm runs a yes/no gate. It returns (true,nil) on accept, (false,nil) on
// reject, and diag.Cancelled on Esc / Ctrl-C. Callers gate on an interactive
// terminal first.
func Confirm(ctx context.Context, io IO, spec ConfirmSpec) (bool, error) {
	final, err := run(ctx, io, newConfirmModel(io, spec))
	if err != nil {
		return false, err
	}
	return final.(confirmModel).accepted, nil
}
