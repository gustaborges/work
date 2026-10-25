package present

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gustaborges/work/internal/diag"
)

type sessionKey struct{}
type session struct {
	ctx     context.Context
	program *tea.Program
	done    chan struct{}
}
type sessionRequest struct {
	ctx   context.Context
	spec  WizardSpec
	reply chan sessionReply
}
type sessionReply struct {
	answers Answers
	err     error
}
type sessionNotice string
type sessionStop struct{}
type sessionWaiting uint64

type sessionModel struct {
	baseFrame
	ctx        context.Context
	io         IO
	active     *wizard
	reply      chan sessionReply
	history    []string
	durable    strings.Builder
	last       tea.View
	generation uint64
	waiting    bool
	cancel     context.CancelFunc
}

func (m *sessionModel) Init() tea.Cmd { return tea.RequestBackgroundColor }
func (m *sessionModel) finish() tea.Cmd {
	w := m.active
	for _, r := range w.receipts {
		m.durable.WriteString(r)
	}
	m.history = append(m.history, w.receipts...)
	_, err := classifyWizard(*w, nil)
	m.reply <- sessionReply{answers: w.answers, err: err}
	m.active = nil
	m.reply = nil
	m.generation++
	m.waiting = false
	generation := m.generation
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return sessionWaiting(generation) })
}
func (m *sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case sessionStop:
		return m, tea.Quit
	case sessionRequest:
		ctx := v.ctx
		if ctx == nil {
			ctx = m.ctx
		}
		w := newWizard(ctx, m.io, v.spec)
		w.baseFrame = m.baseFrame
		m.active = &w
		m.reply = v.reply
		m.waiting = false
		if w.cur == nil {
			return m, m.finish()
		}
		primed, cmd := w.inheritFrame().prime()
		m.active = &primed
		return m, cmd
	case sessionNotice:
		text := strings.TrimSpace(string(v))
		if text != "" {
			receipt := text + "\n\n"
			m.history = append(m.history, receipt)
		}
		return m, nil
	case sessionWaiting:
		if m.active == nil && uint64(v) == m.generation {
			m.waiting = true
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.absorb(v)
	case tea.BackgroundColorMsg:
		m.absorb(v)
	}
	if m.active != nil {
		updated, cmd := m.active.Update(msg)
		w := updated.(wizard)
		m.active = &w
		if w.done || w.cancelled || w.fatal != nil {
			return m, m.finish()
		}
		return m, cmd
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+c" {
		m.cancel()
	}
	return m, nil
}
func (m *sessionModel) View() tea.View {
	if m.active != nil {
		w := *m.active
		w.receipts = append(append([]string{}, m.history...), w.receipts...)
		m.last = w.View()
		return m.last
	}
	if !m.waiting && m.last.Content != "" {
		return m.last
	}
	v := m.clamp(m.rule() + "\n\nWorking…\n\nctrl+c cancel")
	v.AltScreen = true
	return v
}

// WithSession keeps one terminal lifecycle across a hub's successive wizards.
// The callback owns service work; prompts run on the UI loop, and no terminal
// teardown or background-colour probe occurs between screens.
func WithSession(ctx context.Context, pio IO, run func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(sessionKey{}).(*session); ok {
		return run(ctx)
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &session{done: make(chan struct{})}
	scoped := context.WithValue(child, sessionKey{}, s)
	s.ctx = scoped
	model := &sessionModel{baseFrame: newBaseFrame(pio), ctx: scoped, io: pio, cancel: cancel}
	s.program = tea.NewProgram(model, tea.WithContext(child), tea.WithInput(pio.in()), tea.WithOutput(pio.ui()))
	uiResult := make(chan error, 1)
	go func() { _, err := s.program.Run(); uiResult <- err; close(s.done); cancel() }()
	actionErr := run(scoped)
	s.program.Send(sessionStop{})
	uiErr := <-uiResult
	if receipts := model.durable.String(); receipts != "" {
		_, _ = io.WriteString(pio.ui(), receipts)
	}
	if actionErr != nil {
		return actionErr
	}
	if errors.Is(uiErr, context.Canceled) {
		return diag.New(diag.Cancelled, "cancelled")
	}
	if uiErr != nil {
		return diag.Wrap(diag.Usage, uiErr, "the interactive hub could not be shown")
	}
	return nil
}
func (s *session) wizard(ctx context.Context, spec WizardSpec) (Answers, error) {
	reply := make(chan sessionReply, 1)
	s.program.Send(sessionRequest{ctx: ctx, spec: spec, reply: reply})
	select {
	case result := <-reply:
		return result.answers, result.err
	case <-s.ctx.Done():
		return nil, diag.New(diag.Cancelled, "cancelled")
	case <-s.done:
		return nil, diag.New(diag.Usage, "the interactive hub ended before the prompt completed")
	}
}

// SessionNotice shows safe operation output within the live hub. Callers retain
// responsibility for publishing stable results after the terminal is restored.
func SessionNotice(ctx context.Context, text string) {
	if ctx != nil {
		if s, ok := ctx.Value(sessionKey{}).(*session); ok {
			s.program.Send(sessionNotice(text))
		}
	}
}
