package present

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// ConfirmSequenceStep presents an ordered set of independent yes/no decisions
// inside one wizard session. Its answer is a []bool in the same order as the
// supplied specs. It deliberately emits no receipt for each item: callers can
// report the resulting domain outcomes once the session has completed.
func ConfirmSequenceStep(key string, build func(Answers) ([]ConfirmSpec, error)) Step {
	return Step{key: key, make: func(a Answers, io IO, _ context.Context) (stepModel, error) {
		specs, err := build(a)
		if err != nil {
			return nil, err
		}
		if len(specs) == 0 {
			return nil, StepResolved([]bool{})
		}
		return newConfirmSequenceModel(io, specs), nil
	}}
}

type confirmSequenceModel struct {
	baseFrame
	io      IO
	specs   []ConfirmSpec
	idx     int
	cur     confirmModel
	answers []bool
	done    bool
	cancel  bool
}

func newConfirmSequenceModel(io IO, specs []ConfirmSpec) confirmSequenceModel {
	return confirmSequenceModel{baseFrame: newBaseFrame(io), io: io, specs: specs, cur: newConfirmModel(io, specs[0])}
}

func (m confirmSequenceModel) Init() tea.Cmd { return nil }

func (m confirmSequenceModel) withFrame(f baseFrame) stepModel {
	m.baseFrame = f
	m.cur = m.cur.withFrame(f).(confirmModel)
	return m
}

func (m confirmSequenceModel) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	if m.absorb(msg) {
		m.cur = m.cur.withFrame(m.baseFrame).(confirmModel)
	}
	next, cmd := m.cur.Update(msg)
	m.cur = next.(confirmModel)
	st := m.cur.status()
	if st.cancelled {
		m.cancel = true
		return m, cmd
	}
	if !st.done {
		return m, cmd
	}
	m.answers = append(m.answers, st.answer.(bool))
	m.idx++
	if m.idx == len(m.specs) {
		m.done = true
		return m, cmd
	}
	m.cur = newConfirmModel(m.io, m.specs[m.idx])
	m.cur = m.cur.withFrame(m.baseFrame).(confirmModel)
	return m, cmd
}

func (m confirmSequenceModel) status() stepStatus {
	return stepStatus{done: m.done, cancelled: m.cancel, answer: m.answers}
}

func (m confirmSequenceModel) cursorPos() (tea.Position, bool) { return tea.Position{}, false }

func (m confirmSequenceModel) body(width, height int) string { return m.cur.body(width, height) }
