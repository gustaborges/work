package present

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

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

func TestClassifyWizard(t *testing.T) {
	t.Run("program context cancel -> diag.Cancelled", func(t *testing.T) {
		_, err := classifyWizard(nil, context.Canceled)
		if diag.Token(err) != "cancelled" || diag.ExitCode(err) != 20 {
			t.Errorf("err = %v (token %s, code %d)", err, diag.Token(err), diag.ExitCode(err))
		}
	})
	t.Run("other program error -> usage wrap", func(t *testing.T) {
		_, err := classifyWizard(nil, errors.New("tty exploded"))
		if diag.ExitCode(err) != 2 {
			t.Errorf("code = %d, want 2", diag.ExitCode(err))
		}
	})
	t.Run("wizard cancelled -> diag.Cancelled", func(t *testing.T) {
		_, err := classifyWizard(wizard{cancelled: true}, nil)
		if diag.Token(err) != "cancelled" {
			t.Errorf("token = %s, want cancelled", diag.Token(err))
		}
	})
	t.Run("wizard fatal -> unwrapped error returned", func(t *testing.T) {
		boom := diag.New(diag.BootstrapFailed, "git is unusable")
		w, err := classifyWizard(wizard{fatal: boom}, nil)
		if err != boom {
			t.Errorf("err = %v, want the fatal error itself", err)
		}
		if w.fatal != boom {
			t.Errorf("wizard should still be returned alongside a fatal error")
		}
	})
	t.Run("clean completion -> nil error", func(t *testing.T) {
		if _, err := classifyWizard(wizard{done: true}, nil); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
}

// TestRunWizardReprintsReceiptsToUI drives a real two-step wizard to completion
// and checks that the accepted receipts land on the injected UI writer once the
// program has exited (the primary buffer, ahead of any stdout the CLI adds).
func TestRunWizardReprintsReceiptsToUI(t *testing.T) {
	var ui bytes.Buffer
	in := strings.NewReader("alpha\rmain\r")
	pio := IO{In: in, UI: &ui}

	ans, err := Wizard(context.Background(), pio, WizardSpec{
		Title: "Start a Work",
		Steps: []Step{
			InputStep("slug", func(Answers) (InputSpec, error) {
				return InputSpec{Title: "Slug"}, nil
			}),
			SelectStep("base", func(Answers) (SelectSpec[string], error) {
				return SelectSpec[string]{Title: "Base branch", Options: stringOpts("main", "dev")}, nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("wizard: %v", err)
	}
	if ans.String("slug") != "alpha" || ans.Value("base") != "main" {
		t.Fatalf("answers = %+v", ans)
	}
	out := ui.String()
	if !strings.Contains(out, "Slug\n  ✔ alpha") || !strings.Contains(out, "Base branch\n  ✔ main") {
		t.Errorf("receipt trail not reprinted to the UI writer:\n%s", out)
	}
}

func stringOpts(vals ...string) []Option[string] {
	opts := make([]Option[string], len(vals))
	for i, v := range vals {
		opts[i] = Option[string]{Value: v, Primary: v}
	}
	return opts
}
