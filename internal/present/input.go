package present

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// InputSpec describes a single text-entry step. Validate runs the CLI's domain
// rules against the current value: nil accepts, a plain error (or a
// field-recoverable *diag.Error) is shown in-frame and editing continues,
// Fatal(err) aborts the step and hands err to the diagnostic border. Validate
// MUST be deterministic and side-effect-free with respect to Work state and
// safe to call repeatedly (data-model §1, research R8).
type InputSpec struct {
	Title       string
	Description string
	Initial     string
	Validate    func(context.Context, string) error
	// Receipt formats the accepted value for the compact receipt. nil ⇒ the
	// value itself (or "••••" when Secret).
	Receipt func(accepted string) string
	// Secret redacts the receipt and guarantees no substring of the typed
	// value appears in any completed View (FR-004).
	Secret bool
}

// checkingDelay is how long a Validate call may run before the frame shows a
// neutral "checking…" line instead of the help line (research R9).
const checkingDelay = 120 * time.Millisecond

type inputState int

const (
	inputEditing inputState = iota
	inputCompleted
	inputCancelled
)

type inputModel struct {
	baseFrame
	spec InputSpec
	ctx  context.Context

	value    []rune
	cursorAt int

	state    inputState
	curErr   string // at most one live error; "" = none
	checking bool   // a Validate call is outstanding
	showWait bool   // the checking… line is due
	seq      int    // Validate generation; stale results are ignored
	fatal    error

	accepted string
	receipt  string
}

type validateResultMsg struct {
	seq int
	err error
}

type checkingTickMsg struct{ seq int }

func newInputModel(ctx context.Context, io IO, spec InputSpec) inputModel {
	if ctx == nil {
		ctx = context.Background()
	}
	v := []rune(spec.Initial)
	return inputModel{
		baseFrame: newBaseFrame(io),
		spec:      spec,
		ctx:       ctx,
		value:     v,
		cursorAt:  len(v),
	}
}

func (m inputModel) Init() tea.Cmd { return nil }

func (m inputModel) withFrame(f baseFrame) stepModel {
	m.baseFrame = f
	return m
}

func (m inputModel) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	if m.absorb(msg) {
		return m, nil
	}
	switch msg := msg.(type) {
	case validateResultMsg:
		if msg.seq != m.seq {
			return m, nil // a newer attempt superseded this one
		}
		m.checking, m.showWait = false, false
		switch underlying, fatal := IsFatal(msg.err); {
		case msg.err == nil:
			return m.accept()
		case fatal:
			m.fatal = underlying
			return m, nil
		default:
			m.curErr = underlying.Error()
			return m, nil
		}
	case checkingTickMsg:
		if msg.seq == m.seq && m.checking {
			m.showWait = true
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m inputModel) handleKey(key tea.KeyPressMsg) (stepModel, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "esc":
		// A text field cannot cancel on "q" — it is a valid character. The help
		// line names Esc as the cancel key.
		m.state = inputCancelled
		return m, nil
	case "enter":
		return m.submit()
	case "backspace":
		if m.cursorAt > 0 {
			m.value = append(m.value[:m.cursorAt-1], m.value[m.cursorAt:]...)
			m.cursorAt--
			m.curErr = "" // an edit clears the current error (contract §3)
		}
		return m, nil
	case "left":
		m.cursorAt = max(m.cursorAt-1, 0)
		return m, nil
	case "right":
		m.cursorAt = min(m.cursorAt+1, len(m.value))
		return m, nil
	case "home", "ctrl+a":
		m.cursorAt = 0
		return m, nil
	case "end", "ctrl+e":
		m.cursorAt = len(m.value)
		return m, nil
	case "ctrl+u":
		m.value, m.cursorAt, m.curErr = m.value[:0], 0, ""
		return m, nil
	}
	// Printable input: insert the produced text at the cursor. Shift is allowed
	// (it is how capitals and shifted symbols arrive); Ctrl/Alt chords are not
	// text and must not be inserted.
	if key.Mod&^tea.ModShift == 0 && key.Text != "" {
		r := []rune(key.Text)
		m.value = append(m.value[:m.cursorAt], append(r, m.value[m.cursorAt:]...)...)
		m.cursorAt += len(r)
		m.curErr = ""
	}
	return m, nil
}

// submit starts a validation pass. With no validator the value is accepted
// immediately.
func (m inputModel) submit() (stepModel, tea.Cmd) {
	if m.spec.Validate == nil {
		return m.accept()
	}
	m.seq++
	m.checking, m.showWait, m.curErr = true, false, ""
	seq, ctx, fn, val := m.seq, m.ctx, m.spec.Validate, string(m.value)
	return m, tea.Batch(
		func() tea.Msg { return validateResultMsg{seq: seq, err: fn(ctx, val)} },
		tea.Tick(checkingDelay, func(time.Time) tea.Msg { return checkingTickMsg{seq: seq} }),
	)
}

func (m inputModel) accept() (stepModel, tea.Cmd) {
	m.accepted = string(m.value)
	m.state = inputCompleted
	m.receipt = Receipt(m.th, m.spec.Title, MarkSuccess, m.displayValue())
	return m, nil
}

func (m inputModel) displayValue() string {
	switch {
	case m.spec.Receipt != nil:
		return m.spec.Receipt(m.accepted)
	case m.spec.Secret:
		return "••••"
	default:
		return m.accepted
	}
}

func (m inputModel) status() stepStatus {
	return stepStatus{
		done:      m.state == inputCompleted,
		cancelled: m.state == inputCancelled,
		fatal:     m.fatal,
		receipt:   m.receipt,
		answer:    m.accepted,
	}
}

// cursor sits at the edit position on the input line: past the "❯ " chevron and
// the display width of the value left of the cursor. The input line is body's
// first line after the title and the optional description.
func (m inputModel) cursorPos() (tea.Position, bool) {
	if m.state != inputEditing || m.fatal != nil || m.spec.Secret {
		return tea.Position{}, false
	}
	y := 1 // past the title line
	if m.spec.Description != "" {
		y++
	}
	x := DisplayWidth("❯ ") + DisplayWidth(string(m.value[:m.cursorAt]))
	return tea.Position{X: x, Y: y}, true
}

func (m inputModel) body(width, height int) string {
	m.w, m.h = width, height
	if m.fatal != nil || m.state != inputEditing {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.th.Primary.Render(m.spec.Title))
	if m.spec.Description != "" {
		fmt.Fprintf(&b, "%s\n", m.th.Muted.Render(m.spec.Description))
	}

	shown := string(m.value)
	if m.spec.Secret {
		shown = strings.Repeat("•", len(m.value))
	}
	fmt.Fprintf(&b, "❯ %s\n\n", TruncTail(shown, max(m.w-2, 1)))

	switch {
	case m.curErr != "":
		fmt.Fprintf(&b, "%s%s", m.th.Danger.Render("✘ "), TruncTail(m.curErr, max(m.w-2, 1)))
	case m.showWait:
		b.WriteString(m.th.Muted.Render("⋯ checking…"))
	default:
		b.WriteString(m.th.Muted.Render("enter accept · esc cancel"))
	}
	return b.String()
}

// Input runs an interactive text-entry step and returns the accepted value.
// Cancellation (Esc / Ctrl-C) returns diag.Cancelled (exit 20); a Fatal
// validation error is returned unwrapped for the CLI diagnostic border. Callers
// gate on an interactive terminal first (contracts/presentation-boundary.md).
func Input(ctx context.Context, io IO, spec InputSpec) (string, error) {
	ans, err := Wizard(ctx, io, WizardSpec{Steps: []Step{
		InputStep("value", func(Answers) (InputSpec, error) { return spec, nil }),
	}})
	if err != nil {
		return "", err
	}
	return ans.String("value"), nil
}
