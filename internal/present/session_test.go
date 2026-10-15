package present

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gustaborges/work/internal/diag"
)

func TestSessionKeepsFrameAndReceiptsAcrossPrompts(t *testing.T) {
	m := &sessionModel{baseFrame: newBaseFrame(IO{}), ctx: context.Background()}
	m.th = offTheme()
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	request := func(title string) chan sessionReply {
		reply := make(chan sessionReply, 1)
		m.Update(sessionRequest{reply: reply, spec: WizardSpec{Steps: []Step{
			InputStep("value", func(Answers) (InputSpec, error) { return InputSpec{Title: title}, nil }),
		}}})
		if m.active.w != 60 || m.active.h != 20 || !m.View().AltScreen {
			t.Fatal("new prompt lost the shared terminal frame")
		}
		return reply
	}
	first := request("First field")
	for _, key := range typeKeys("alpha") {
		m.Update(key)
	}
	previous := m.View().Content
	_, cmd := m.Update(press("enter"))
	if got := <-first; got.err != nil || got.answers.String("value") != "alpha" {
		t.Fatal(got)
	}
	if m.View().Content != previous {
		t.Fatal("prompt completion flashed an empty or loading frame")
	}
	if _, quit := cmd().(tea.QuitMsg); quit {
		t.Fatal("prompt completion stopped the terminal session")
	}
	m.Update(sessionNotice("work: operation completed"))
	second := request("Second field")
	if view := m.View().Content; !strings.Contains(view, "First field") || !strings.Contains(view, "work: operation completed") {
		t.Fatal(view)
	}
	// A late loading timer from the first prompt must not replace this prompt.
	m.Update(sessionWaiting(m.generation))
	if !strings.Contains(m.View().Content, "Second field") {
		t.Fatal(m.View().Content)
	}
	m.Update(press("esc"))
	if got := <-second; diag.ExitCode(got.err) != 20 {
		t.Fatal(got.err)
	}
	if receipts := m.durable.String(); strings.Count(receipts, "✔ alpha") != 1 || strings.Contains(receipts, "Second field") || strings.Contains(receipts, "operation completed") {
		t.Fatal("accepted receipts must be published once; notices belong to their caller:", receipts)
	}
}

func TestSessionCancelsServiceWorkBetweenPrompts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &sessionModel{baseFrame: newBaseFrame(IO{}), ctx: ctx, cancel: cancel}
	m.Update(press("ctrl+c"))
	if ctx.Err() != context.Canceled {
		t.Fatal("Ctrl-C did not cancel service work")
	}
}
