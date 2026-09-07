package present

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/diag"
)

func TestFatalRoundTrip(t *testing.T) {
	if got := Fatal(nil); got != nil {
		t.Errorf("Fatal(nil) = %v, want nil", got)
	}
	base := errors.New("disk on fire")
	wrapped := Fatal(base)
	got, ok := IsFatal(wrapped)
	if !ok || got != base {
		t.Errorf("IsFatal(Fatal(e)) = (%v,%v), want (%v,true)", got, ok, base)
	}
	// A plain error is not fatal and passes through unchanged.
	plain := errors.New("recoverable")
	if got, ok := IsFatal(plain); ok || got != plain {
		t.Errorf("IsFatal(plain) = (%v,%v), want (plain,false)", got, ok)
	}
	// Fatal survives further wrapping.
	if got, ok := IsFatal(diag.Wrap(diag.Usage, wrapped, "ctx")); !ok || got != base {
		t.Errorf("IsFatal through diag.Wrap = (%v,%v)", got, ok)
	}
}

func TestClassify(t *testing.T) {
	t.Run("program context cancel -> diag.Cancelled", func(t *testing.T) {
		_, err := classify(nil, context.Canceled)
		if diag.Token(err) != "cancelled" || diag.ExitCode(err) != 20 {
			t.Errorf("err = %v (token %s, code %d)", err, diag.Token(err), diag.ExitCode(err))
		}
	})
	t.Run("other program error -> usage wrap", func(t *testing.T) {
		_, err := classify(nil, errors.New("tty exploded"))
		if diag.ExitCode(err) != 2 {
			t.Errorf("code = %d, want 2", diag.ExitCode(err))
		}
	})
	t.Run("model cancelled -> diag.Cancelled", func(t *testing.T) {
		_, err := classify(stubModel{oc: outcome{cancelled: true}}, nil)
		if diag.Token(err) != "cancelled" {
			t.Errorf("token = %s, want cancelled", diag.Token(err))
		}
	})
	t.Run("model fatal -> unwrapped error returned", func(t *testing.T) {
		boom := diag.New(diag.BootstrapFailed, "git is unusable")
		m, err := classify(stubModel{oc: outcome{fatal: boom}}, nil)
		if err != boom {
			t.Errorf("err = %v, want the fatal error itself", err)
		}
		if m == nil {
			t.Errorf("model should still be returned alongside a fatal error")
		}
	})
	t.Run("clean completion -> nil error", func(t *testing.T) {
		if _, err := classify(stubModel{}, nil); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
}

func TestRunReturnsTheFinishedModel(t *testing.T) {
	var ui bytes.Buffer
	m, err := run(context.Background(), IO{In: strings.NewReader(""), UI: &ui}, stubModel{view: "  ✔ done"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.outcome() != (outcome{}) {
		t.Errorf("outcome = %+v, want zero", m.outcome())
	}
	// The program was bound to the injected UI writer, not a process default.
	if ui.Len() == 0 {
		t.Errorf("run did not write to the injected UI writer")
	}
}

func TestRunMapsModelCancellation(t *testing.T) {
	_, err := run(context.Background(), IO{In: strings.NewReader(""), UI: &bytes.Buffer{}},
		stubModel{oc: outcome{cancelled: true}})
	if diag.Token(err) != "cancelled" || diag.ExitCode(err) != 20 {
		t.Errorf("run cancellation mapping: %v (token %s, code %d)", err, diag.Token(err), diag.ExitCode(err))
	}
}

// stubModel quits immediately from Init and reports a fixed outcome.
type stubModel struct {
	oc   outcome
	view string
}

func (m stubModel) Init() tea.Cmd                       { return tea.Quit }
func (m stubModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, tea.Quit }
func (m stubModel) View() tea.View                      { return tea.NewView(m.view) }
func (m stubModel) outcome() outcome                    { return m.oc }
