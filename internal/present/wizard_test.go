package present

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/diag"
)

func testWizard(spec WizardSpec) wizard {
	w := newWizard(context.Background(), IO{}, spec)
	w.th = offTheme()
	return w
}

// send applies msgs to the wizard in order, resolving each batched command so a
// step transition's follow-up messages (prime, Init) are delivered too.
func send(w wizard, msgs ...tea.Msg) wizard {
	var m tea.Model = w
	for _, msg := range msgs {
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		for _, out := range drain(cmd) {
			if _, quit := out.(tea.QuitMsg); quit {
				continue
			}
			m, _ = m.Update(out)
		}
	}
	return m.(wizard)
}

func TestWizardAdvancesAndRecordsAnswers(t *testing.T) {
	w := testWizard(WizardSpec{
		Title: "Start a Work",
		Steps: []Step{
			InputStep("slug", func(Answers) (InputSpec, error) {
				return InputSpec{Title: "Slug"}, nil
			}),
			ConfirmStep("confirm", func(a Answers) (ConfirmSpec, error) {
				if a.String("slug") != "alpha" {
					t.Errorf("confirm build saw slug=%q, want alpha", a.String("slug"))
				}
				return ConfirmSpec{Title: "Create Work"}, nil
			}),
		},
	})

	w = send(w, typeKeys("alpha")...)
	w = send(w, press("enter")) // accept slug
	if _, ok := w.cur.(confirmModel); !ok {
		t.Fatalf("did not advance to the confirm step: %T", w.cur)
	}
	w = send(w, press("y")) // accept confirm

	if !w.done {
		t.Fatalf("wizard not done")
	}
	if w.answers.String("slug") != "alpha" || !w.answers.Bool("confirm") {
		t.Errorf("answers = %+v", w.answers)
	}
	trail := w.finalReceipts()
	if !strings.Contains(trail, "Slug\n  ✔ alpha\n\n") || !strings.Contains(trail, "✔ Create Work confirmed") {
		t.Errorf("receipt trail = %q", trail)
	}
}

func TestWizardStepResolvedSkipsWithoutRendering(t *testing.T) {
	rendered := false
	w := testWizard(WizardSpec{Steps: []Step{
		InputStep("slug", func(Answers) (InputSpec, error) {
			return InputSpec{}, StepResolved("from-flag")
		}),
		InputStep("path", func(Answers) (InputSpec, error) {
			rendered = true
			return InputSpec{Title: "Path"}, nil
		}),
	}})

	if w.answers.String("slug") != "from-flag" {
		t.Errorf("resolved answer not recorded during construction: %+v", w.answers)
	}
	if !rendered {
		t.Errorf("wizard did not build the next real step after the skip")
	}
	if m, ok := w.cur.(inputModel); !ok || m.spec.Title != "Path" {
		t.Errorf("current step is not the Path input: %T", w.cur)
	}
}

func TestWizardBuildFatalAborts(t *testing.T) {
	boom := diag.New(diag.BootstrapFailed, "no base branch")
	w := testWizard(WizardSpec{Steps: []Step{
		SelectStep("base", func(Answers) (SelectSpec[string], error) {
			return SelectSpec[string]{}, Fatal(boom)
		}),
	}})
	if w.cur != nil {
		t.Fatalf("build fatal should leave no current step")
	}
	if w.fatal != boom {
		t.Errorf("fatal = %v, want %v", w.fatal, boom)
	}
	if _, err := classifyWizard(w, nil); err != boom {
		t.Errorf("classifyWizard = %v, want the fatal error", err)
	}
}

func TestWizardCancelPropagates(t *testing.T) {
	w := testWizard(WizardSpec{Steps: []Step{
		InputStep("slug", func(Answers) (InputSpec, error) { return InputSpec{Title: "Slug"}, nil }),
	}})
	w = send(w, press("esc"))
	if !w.cancelled {
		t.Fatalf("wizard did not record cancellation")
	}
	if _, err := classifyWizard(w, nil); diag.Token(err) != "cancelled" || diag.ExitCode(err) != 20 {
		t.Errorf("classify = %v (token %s code %d)", err, diag.Token(err), diag.ExitCode(err))
	}
}

func TestWizardViewIsAltScreenBoundedWithRuleAndTitle(t *testing.T) {
	w := testWizard(WizardSpec{
		Title: "Start a Work",
		Steps: []Step{InputStep("slug", func(Answers) (InputSpec, error) {
			return InputSpec{Title: "Slug"}, nil
		})},
	})
	w = send(w, tea.WindowSizeMsg{Width: 40, Height: 12})

	view := w.View()
	if !view.AltScreen {
		t.Errorf("wizard view is not alt-screen")
	}
	if !strings.HasPrefix(view.Content, strings.Repeat("─", 40)) {
		t.Errorf("view does not start with the full-width rule:\n%q", view.Content)
	}
	if !strings.Contains(view.Content, "Start a Work") || !strings.Contains(view.Content, "Slug") {
		t.Errorf("view missing wizard title or step title:\n%s", view.Content)
	}
	if n := strings.Count(view.Content, "\n") + 1; n > 12 {
		t.Errorf("view is %d lines, exceeds height 12:\n%s", n, view.Content)
	}
	for _, line := range strings.Split(view.Content, "\n") {
		if DisplayWidth(line) > 40 {
			t.Errorf("view line wider than 40: %q", line)
		}
	}
}

func TestWizardKeepsActiveConfirmVisibleWhenReceiptsFillViewport(t *testing.T) {
	w := testWizard(WizardSpec{Title: "Start a Work", Steps: []Step{
		ConfirmStep("confirm", func(Answers) (ConfirmSpec, error) {
			return ConfirmSpec{
				Title:  "Create Work",
				Impact: "  repository: /src/demo\n  base: main\n  branch: feat/demo\n  workspace: /work\n  directory: /work/in-progress/demo_feat-demo",
				Accept: "Create", Reject: "Cancel",
			}, nil
		}),
	}})
	w.receipts = []string{
		Receipt(w.th, "Local repository path", MarkSuccess, "/src/demo"),
		Receipt(w.th, "Branch prefix", MarkSuccess, "feat"),
		Receipt(w.th, "Slug", MarkSuccess, "demo"),
		Receipt(w.th, "Base branch", MarkSuccess, "main"),
		Receipt(w.th, "Workspace root", MarkSuccess, "/work"),
	}
	w = send(w, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := w.View()
	for _, want := range []string{"[ Create ]", "Cancel", "y/n · esc cancel"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("view lost active confirm content %q:\n%s", want, view.Content)
		}
	}
	if n := strings.Count(view.Content, "\n") + 1; n > 24 {
		t.Errorf("view is %d lines, exceeds height 24", n)
	}
}

func typeKeys(s string) []tea.Msg {
	out := make([]tea.Msg, 0, len(s))
	for _, r := range s {
		out = append(out, tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	return out
}
