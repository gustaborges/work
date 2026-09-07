package present

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// submitInput runs one validation round-trip: press Enter, resolve the batched
// commands, feed the results back, and return the settled model.
func submitInput(t *testing.T, m inputModel) inputModel {
	t.Helper()
	next, cmd := m.Update(press("enter"))
	next, _ = feed(next, drain(cmd)...)
	return next.(inputModel)
}

func newTestInput(spec InputSpec) inputModel {
	m := newInputModel(context.Background(), IO{}, spec)
	m.th = offTheme() // plain output for exact string assertions
	return m
}

func TestInputAcceptsAndRendersExactReceipt(t *testing.T) {
	m := typeText(newTestInput(InputSpec{Title: "Slug"}), "my-work").(inputModel)
	m = submitInput(t, m)

	if m.state != inputCompleted {
		t.Fatalf("state = %d, want completed", m.state)
	}
	if m.accepted != "my-work" {
		t.Errorf("accepted = %q", m.accepted)
	}
	if got := m.finalFrame(); got != "Slug\n  ✔ my-work\n\n" {
		t.Errorf("completed finalFrame = %q", got)
	}
	if m.View().Content != "" {
		t.Errorf("completed View should be empty so the frame clears, got %q", m.View().Content)
	}
}

func TestInputOneErrorReplacedOnRetry(t *testing.T) {
	calls := 0
	spec := InputSpec{
		Title: "Local repository path",
		Validate: func(_ context.Context, s string) error {
			calls++
			if s != "/good" {
				return errors.New("no such path: " + s)
			}
			return nil
		},
	}
	m := typeText(newTestInput(spec), "/bad1").(inputModel)
	m = submitInput(t, m)
	if m.state != inputEditing || m.curErr != "no such path: /bad1" {
		t.Fatalf("after first reject: state=%d curErr=%q", m.state, m.curErr)
	}

	// A keystroke clears the current error immediately (before the next submit).
	m2, _ := m.Update(press("x"))
	if m2.(inputModel).curErr != "" {
		t.Errorf("an edit did not clear the live error: %q", m2.(inputModel).curErr)
	}

	// Retry with a different bad value: still exactly one error, the new one.
	m = m2.(inputModel)
	// reset the field to a second bad value
	m.value, m.cursor = []rune("/bad2"), len("/bad2")
	m = submitInput(t, m)
	if m.curErr != "no such path: /bad2" {
		t.Errorf("second error not shown / stale kept: %q", m.curErr)
	}
	if strings.Count(m.View().Content, "✘") != 1 {
		t.Errorf("more than one error line rendered:\n%s", m.View().Content)
	}

	// A good value finally accepts.
	m.value, m.cursor = []rune("/good"), len("/good")
	m = submitInput(t, m)
	if m.state != inputCompleted {
		t.Errorf("good value not accepted: state=%d curErr=%q", m.state, m.curErr)
	}
}

func TestInputFatalAborts(t *testing.T) {
	boom := errors.New("git is broken")
	spec := InputSpec{
		Title:    "Path",
		Validate: func(context.Context, string) error { return Fatal(boom) },
	}
	m := typeText(newTestInput(spec), "/x").(inputModel)
	next, cmd := m.Update(press("enter"))

	var sm inputModel = next.(inputModel)
	quit := false
	for _, msg := range drain(cmd) {
		var c tea.Cmd
		var tm tea.Model
		tm, c = sm.Update(msg)
		sm = tm.(inputModel)
		quit = quit || isQuit(c)
	}

	if !quit {
		t.Errorf("fatal validation did not quit")
	}
	if sm.fatal != boom {
		t.Errorf("fatal = %v, want %v", sm.fatal, boom)
	}
	if oc := sm.outcome(); oc.fatal != boom || oc.cancelled {
		t.Errorf("outcome = %+v", oc)
	}
}

func TestInputCheckingLineAppearsWhileValidating(t *testing.T) {
	m := typeText(newTestInput(InputSpec{
		Title:    "Path",
		Validate: func(context.Context, string) error { return nil },
	}), "/x").(inputModel)

	next, _ := m.Update(press("enter"))
	nm := next.(inputModel)
	if !nm.checking {
		t.Fatal("model is not in the checking state after submit")
	}
	// The 120ms tick fires before the result arrives.
	afterTick, _ := nm.Update(checkingTickMsg{seq: nm.seq})
	if !strings.Contains(afterTick.(inputModel).View().Content, "⋯ checking…") {
		t.Errorf("checking… line not shown:\n%s", afterTick.(inputModel).View().Content)
	}
	// A stale tick from a superseded attempt is ignored.
	stale, _ := nm.Update(checkingTickMsg{seq: nm.seq - 1})
	if stale.(inputModel).showWait {
		t.Errorf("a stale checking tick was honoured")
	}
}

func TestInputSecretRedaction(t *testing.T) {
	m := typeText(newTestInput(InputSpec{Title: "Token", Secret: true}), "hunter2").(inputModel)

	// While editing, the value is masked.
	if strings.Contains(m.View().Content, "hunter2") {
		t.Errorf("secret value visible while editing:\n%s", m.View().Content)
	}
	m = submitInput(t, m)
	got := m.finalFrame()
	if strings.Contains(got, "hunter2") {
		t.Errorf("secret value leaked into the receipt:\n%s", got)
	}
	if !strings.Contains(got, "••••") {
		t.Errorf("secret receipt missing the redaction mark:\n%s", got)
	}
}

func TestInputCancelledLeavesNoFrame(t *testing.T) {
	// A cancelled input collapses to nothing; the single "✘ Operation
	// cancelled" line is the CLI diagnostic border's to print.
	for _, key := range []string{"esc", "ctrl+c"} {
		m := typeText(newTestInput(InputSpec{Title: "Slug"}), "wip").(inputModel)
		next, cmd := m.Update(press(key))
		nm := next.(inputModel)
		if !isQuit(cmd) || nm.state != inputCancelled {
			t.Errorf("%s did not cancel: quit=%v state=%d", key, isQuit(cmd), nm.state)
		}
		if got := nm.finalFrame(); got != "" {
			t.Errorf("%s cancelled finalFrame = %q, want empty", key, got)
		}
		if nm.View().Content != "" {
			t.Errorf("%s cancelled View = %q, want empty", key, nm.View().Content)
		}
		if !nm.outcome().cancelled {
			t.Errorf("%s outcome not cancelled", key)
		}
	}
}

func TestInputQIsATypableCharacter(t *testing.T) {
	m := typeText(newTestInput(InputSpec{Title: "Slug"}), "queue").(inputModel)
	if string(m.value) != "queue" {
		t.Errorf("q was treated as a control key: value=%q", string(m.value))
	}
	if m.state == inputCancelled {
		t.Errorf("typing q cancelled the input")
	}
}

func TestInputFrameNeverExceedsHeight(t *testing.T) {
	spec := InputSpec{Title: "T", Description: "a helper line", Validate: func(context.Context, string) error {
		return errors.New("an error that would add a line")
	}}
	m := newTestInput(spec)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	m3 := typeText(m2, "value").(inputModel)
	m3 = submitInput(t, m3)
	if n := strings.Count(m3.View().Content, "\n") + 1; n > 3 {
		t.Errorf("editing frame is %d lines, exceeds height 3:\n%s", n, m3.View().Content)
	}
}
