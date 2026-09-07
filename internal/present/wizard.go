package present

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Answers holds the values accepted so far in a wizard, keyed by step key. It is
// opaque to present: values are stored and returned as any, and a later step's
// build closure or the CLI reads them back with a type assertion.
type Answers map[string]any

// String returns the answer for key as a string, or "" when absent or not a
// string.
func (a Answers) String(key string) string {
	s, _ := a[key].(string)
	return s
}

// Bool returns the answer for key as a bool, or false when absent or not a bool.
func (a Answers) Bool(key string) bool {
	b, _ := a[key].(bool)
	return b
}

// Value returns the raw answer for key (nil when absent).
func (a Answers) Value(key string) any { return a[key] }

// Step is one ordered unit of a wizard. Build it with InputStep, SelectStep,
// MultiSelectStep, or ConfirmStep; the build closure runs when the step is
// reached and may read every earlier answer.
type Step struct {
	key  string
	make func(a Answers, io IO, ctx context.Context) (stepModel, error)
}

// resolvedStep is the sentinel a build closure returns via StepResolved.
type resolvedStep struct{ answer any }

func (resolvedStep) Error() string { return "step already resolved" }

// StepResolved lets a step's build closure report that the value is already
// known — supplied by a flag or argument — so the wizard records the answer
// under the step key and moves on without rendering the step.
func StepResolved(answer any) error { return resolvedStep{answer: answer} }

// InputStep is a text-entry step. build receives the answers accepted so far
// and returns the spec; a build error aborts the wizard (handed to the
// diagnostic border, like present.Fatal).
func InputStep(key string, build func(Answers) (InputSpec, error)) Step {
	return Step{key: key, make: func(a Answers, io IO, ctx context.Context) (stepModel, error) {
		spec, err := build(a)
		if err != nil {
			return nil, err
		}
		return newInputModel(ctx, io, spec), nil
	}}
}

// SelectStep is a single-choice step. Option values of any type T are carried
// through untouched and returned as the step's answer (boxed to any).
func SelectStep[T any](key string, build func(Answers) (SelectSpec[T], error)) Step {
	return Step{key: key, make: func(a Answers, io IO, _ context.Context) (stepModel, error) {
		spec, err := build(a)
		if err != nil {
			return nil, err
		}
		return newSelectModel(io, spec), nil
	}}
}

// MultiSelectStep is a multi-choice step; the answer is a []T of the checked
// values in list order.
func MultiSelectStep[T comparable](key string, build func(Answers) (MultiSelectSpec[T], error)) Step {
	return Step{key: key, make: func(a Answers, io IO, _ context.Context) (stepModel, error) {
		spec, err := build(a)
		if err != nil {
			return nil, err
		}
		return newMultiSelectModel(io, spec), nil
	}}
}

// ConfirmStep is a yes/no gate; the answer is a bool.
func ConfirmStep(key string, build func(Answers) (ConfirmSpec, error)) Step {
	return Step{key: key, make: func(a Answers, io IO, _ context.Context) (stepModel, error) {
		spec, err := build(a)
		if err != nil {
			return nil, err
		}
		return newConfirmModel(io, spec), nil
	}}
}

// WizardSpec describes a full-screen multi-step interview.
type WizardSpec struct {
	// Title is shown under the top rule in the Primary token (e.g. "Start a
	// Work"). Optional.
	Title string
	// Initial seeds Answers with values later build closures may read (values
	// the CLI resolved from flags or arguments before the wizard ran).
	Initial Answers
	// Steps run in order; each build closure sees the answers accepted so far.
	Steps []Step
}

// wizard is the single Bubble Tea program that drives one WizardSpec as a
// full-screen alternate-buffer app: a Primary rule across the top, the wizard
// title, the trail of accepted-step receipts, and the current step's body. A
// full clear+repaint every frame makes the inline renderer's resize/back-nav
// ghosting impossible (ADR-0021).
type wizard struct {
	baseFrame
	ctx      context.Context
	io       IO
	spec     WizardSpec
	idx      int
	cur      stepModel
	answers  Answers
	receipts []string

	done      bool
	cancelled bool
	fatal     error
}

func newWizard(ctx context.Context, io IO, spec WizardSpec) wizard {
	if ctx == nil {
		ctx = context.Background()
	}
	w := wizard{
		baseFrame: newBaseFrame(io),
		ctx:       ctx,
		io:        io,
		spec:      spec,
		answers:   Answers{},
		idx:       -1,
	}
	for k, v := range spec.Initial {
		w.answers[k] = v
	}
	return w.advanceTo(0)
}

func (m wizard) Init() tea.Cmd {
	if m.cur == nil {
		return tea.Quit
	}
	return tea.Batch(tea.RequestBackgroundColor, m.cur.Init())
}

// prime hands the current step its interior size so its own rows()/Budget math
// is correct before the first render. It is applied synchronously, never as a
// forwarded tea.WindowSizeMsg — that message would re-enter wizard.Update and be
// mistaken for a real terminal resize.
func (m wizard) prime() (wizard, tea.Cmd) {
	if m.cur == nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.cur, cmd = m.cur.Update(m.sizeMsg())
	return m, cmd
}

// advanceTo builds steps starting at idx until one needs to prompt, or records
// the terminal state (done, or a build fatal) when none remain. A step whose
// build returns StepResolved is recorded and skipped without rendering.
func (m wizard) advanceTo(idx int) wizard {
	for idx < len(m.spec.Steps) {
		s := m.spec.Steps[idx]
		cur, err := s.make(m.answers, m.io, m.ctx)
		if err != nil {
			if r, ok := err.(resolvedStep); ok {
				if s.key != "" {
					m.answers[s.key] = r.answer
				}
				idx++
				continue
			}
			if u, ok := IsFatal(err); ok {
				m.fatal = u
			} else {
				m.fatal = err
			}
			m.cur = nil
			return m
		}
		m.idx, m.cur = idx, cur
		return m
	}
	m.idx, m.cur, m.done = idx, nil, true
	return m
}

func (m wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.cur == nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.cur, cmd = m.cur.Update(m.sizeMsg())
		return m, cmd
	case tea.BackgroundColorMsg:
		m.absorb(msg)
		if m.cur != nil {
			var cmd tea.Cmd
			m.cur, cmd = m.cur.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if m.cur == nil {
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.cur, cmd = m.cur.Update(msg)
	st := m.cur.status()
	switch {
	case st.fatal != nil:
		m.fatal = st.fatal
		return m, tea.Quit
	case st.cancelled:
		m.cancelled = true
		return m, tea.Quit
	case st.done:
		if st.receipt != "" {
			m.receipts = append(m.receipts, st.receipt)
		}
		if key := m.spec.Steps[m.idx].key; key != "" {
			m.answers[key] = st.answer
		}
		next := m.advanceTo(m.idx + 1)
		if next.cur == nil {
			return next, tea.Quit
		}
		primed, sizeCmd := next.prime()
		return primed, tea.Batch(primed.cur.Init(), sizeCmd)
	}
	return m, cmd
}

func (m wizard) View() tea.View {
	if m.cur == nil {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	var b strings.Builder
	b.WriteString(m.rule() + "\n")
	if m.spec.Title != "" {
		b.WriteString(m.th.Primary.Render(m.spec.Title) + "\n")
	}
	b.WriteString("\n")
	for _, r := range m.receipts {
		b.WriteString(r)
	}
	chrome := m.chromeLines()
	b.WriteString(m.cur.body(max(m.w, 1), max(m.h-chrome, 1)))

	v := m.clamp(b.String())
	v.AltScreen = true
	if pos, ok := m.cur.cursorPos(); ok {
		v.Cursor = tea.NewCursor(pos.X, chrome+pos.Y)
	}
	return v
}

// chromeLines is how many lines the wizard paints before the current step's
// body: the rule, the optional title, one blank line, and every receipt line
// (each receipt string ends with its own blank separator).
func (m wizard) chromeLines() int {
	n := 2 // rule + blank
	if m.spec.Title != "" {
		n++
	}
	for _, r := range m.receipts {
		n += strings.Count(r, "\n")
	}
	return n
}

// sizeMsg is the window size the current step should lay out against: the
// terminal width and the rows left under the wizard chrome.
func (m wizard) sizeMsg() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: max(m.w, 1), Height: max(m.h-m.chromeLines(), 1)}
}

func (m wizard) finalReceipts() string { return strings.Join(m.receipts, "") }

// Wizard runs spec as one full-screen program bound to io and returns the
// accepted answers. Cancellation (Esc / q / Ctrl-C in any step) returns
// diag.Cancelled (exit 20); a Fatal validation error or a build error is
// returned unwrapped for the CLI diagnostic border. Callers gate on an
// interactive terminal first (contracts/presentation-boundary.md).
func Wizard(ctx context.Context, io IO, spec WizardSpec) (Answers, error) {
	w, err := runWizard(ctx, io, newWizard(ctx, io, spec))
	if err != nil {
		return nil, err
	}
	return w.answers, nil
}
