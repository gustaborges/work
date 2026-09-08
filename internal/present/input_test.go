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
	if got := m.status().receipt; got != "Slug\n  ✔ my-work\n\n" {
		t.Errorf("completed receipt = %q", got)
	}
	if m.status().answer != "my-work" {
		t.Errorf("answer = %v", m.status().answer)
	}
	if stepBody(m) != "" {
		t.Errorf("completed body should be empty so the frame clears, got %q", stepBody(m))
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
	m.value, m.cursorAt = []rune("/bad2"), len("/bad2")
	m = submitInput(t, m)
	if m.curErr != "no such path: /bad2" {
		t.Errorf("second error not shown / stale kept: %q", m.curErr)
	}
	if strings.Count(stepBody(m), "✘") != 1 {
		t.Errorf("more than one error line rendered:\n%s", stepBody(m))
	}

	// A good value finally accepts.
	m.value, m.cursorAt = []rune("/good"), len("/good")
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

	var sm stepModel = next
	for _, msg := range drain(cmd) {
		sm, _ = sm.Update(msg)
	}

	st := sm.status()
	if st.fatal != boom {
		t.Errorf("status.fatal = %v, want %v", st.fatal, boom)
	}
	if st.cancelled || st.done {
		t.Errorf("status = %+v, want fatal only", st)
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
	if !strings.Contains(stepBody(afterTick), "⋯ checking…") {
		t.Errorf("checking… line not shown:\n%s", stepBody(afterTick))
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
	if strings.Contains(stepBody(m), "hunter2") {
		t.Errorf("secret value visible while editing:\n%s", stepBody(m))
	}
	m = submitInput(t, m)
	got := m.status().receipt
	if strings.Contains(got, "hunter2") {
		t.Errorf("secret value leaked into the receipt:\n%s", got)
	}
	if !strings.Contains(got, "••••") {
		t.Errorf("secret receipt missing the redaction mark:\n%s", got)
	}
}

func TestInputCancelledLeavesNoReceipt(t *testing.T) {
	// A cancelled input collapses to nothing; the single "✘ Operation
	// cancelled" line is the CLI diagnostic border's to print.
	for _, key := range []string{"esc", "ctrl+c"} {
		m := typeText(newTestInput(InputSpec{Title: "Slug"}), "wip").(inputModel)
		next, _ := m.Update(press(key))
		nm := next.(inputModel)
		if nm.state != inputCancelled {
			t.Errorf("%s did not cancel: state=%d", key, nm.state)
		}
		if got := nm.status().receipt; got != "" {
			t.Errorf("%s cancelled receipt = %q, want empty", key, got)
		}
		if stepBody(nm) != "" {
			t.Errorf("%s cancelled body = %q, want empty", key, stepBody(nm))
		}
		if !nm.status().cancelled {
			t.Errorf("%s status not cancelled", key)
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

func TestInputBodyFitsAClampedFrame(t *testing.T) {
	spec := InputSpec{Title: "T", Description: "a helper line", Validate: func(context.Context, string) error {
		return errors.New("an error that would add a line")
	}}
	m := newTestInput(spec)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	m3 := typeText(m2, "value").(inputModel)
	m3 = submitInput(t, m3)
	frame := clampedFrame(m3, 40, 3)
	if n := strings.Count(frame, "\n") + 1; n > 3 {
		t.Errorf("clamped input frame is %d lines, exceeds height 3:\n%s", n, frame)
	}
	for _, ln := range strings.Split(frame, "\n") {
		if DisplayWidth(ln) > 40 {
			t.Errorf("clamped input line wider than 40: %q", ln)
		}
	}
}

func TestInputCursorTracksTheEditPosition(t *testing.T) {
	m := typeText(newTestInput(InputSpec{Title: "Slug", Description: "id"}), "abcdef").(inputModel)
	pos, ok := m.cursorPos()
	if !ok {
		t.Fatal("input reports no cursor while editing")
	}
	// title line + description line, then the input line: y == 2.
	if pos.Y != 2 {
		t.Errorf("cursor Y = %d, want 2", pos.Y)
	}
	if want := DisplayWidth("❯ ") + DisplayWidth("abcdef"); pos.X != want {
		t.Errorf("cursor X = %d, want %d", pos.X, want)
	}
	// Move left twice; X retreats by two cells.
	m2, _ := m.Update(press("left"))
	m2, _ = m2.Update(press("left"))
	pos2, _ := m2.(inputModel).cursorPos()
	if pos2.X != DisplayWidth("❯ ")+DisplayWidth("abcd") {
		t.Errorf("cursor X after two lefts = %d", pos2.X)
	}
	// A secret field never shows a cursor.
	sm := typeText(newTestInput(InputSpec{Title: "Token", Secret: true}), "x").(inputModel)
	if _, ok := sm.cursorPos(); ok {
		t.Errorf("secret input exposed a cursor position")
	}
}
