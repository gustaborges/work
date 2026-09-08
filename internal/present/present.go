package present

import (
	"context"
	"errors"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/diag"
)

// IO is the explicit input/output pair every primitive runs against. In is
// normally cmd.InOrStdin(); UI is normally cmd.ErrOrStderr(). Interactive UI and
// human diagnostics go to the UI channel; stable command results stay on stdout
// and never pass through present (FR-026, research R14).
type IO struct {
	In io.Reader
	UI io.Writer
}

func (p IO) in() io.Reader {
	if p.In == nil {
		return os.Stdin
	}
	return p.In
}

func (p IO) ui() io.Writer {
	if p.UI == nil {
		return os.Stderr
	}
	return p.UI
}

// fatalError marks a Validate error as non-recoverable.
type fatalError struct{ err error }

func (f fatalError) Error() string { return f.err.Error() }
func (f fatalError) Unwrap() error { return f.err }

// Fatal wraps err so a Validate closure can abort the step instead of showing
// the message in-frame. present hands the unwrapped err to the CLI diagnostic
// border. A nil err stays nil (research R8).
func Fatal(err error) error {
	if err == nil {
		return nil
	}
	return fatalError{err: err}
}

// IsFatal reports whether err was produced by Fatal (directly or wrapped) and
// returns the underlying error; when it was not, it returns err unchanged.
func IsFatal(err error) (error, bool) {
	if f, ok := errors.AsType[fatalError](err); ok {
		return f.err, true
	}
	return err, false
}

// runWizard executes one wizard as a full-screen alternate-buffer Bubble Tea
// program bound to io, then — once the alternate buffer is torn down and the
// primary buffer restored — reprints the compact accepted-step receipts to the
// UI channel, ahead of any stable stdout the CLI writes (FR-007, ADR-0021).
// It maps the terminal outcome to the shared diagnostic vocabulary: context
// cancellation and any step's own cancelled state both become diag.Cancelled
// (exit 20 preserved); a Fatal validation error or a build error is returned
// unwrapped for the CLI border. This is present's only dependency on
// internal/diag.
func runWizard(ctx context.Context, pio IO, m wizard) (wizard, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(pio.in()),
		tea.WithOutput(pio.ui()),
	).Run()
	w, cerr := classifyWizard(final, err)
	if trail := w.finalReceipts(); trail != "" {
		_, _ = io.WriteString(pio.ui(), trail)
	}
	return w, cerr
}

// classifyWizard turns a finished Bubble Tea program into the (wizard, error)
// pair Wizard returns. It is separated from runWizard so the mapping is
// unit-testable without a terminal.
func classifyWizard(final tea.Model, err error) (wizard, error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			w, _ := final.(wizard)
			return w, diag.New(diag.Cancelled, "cancelled")
		}
		return wizard{}, diag.Wrap(diag.Usage, err, "the interactive prompt could not be shown")
	}
	w, ok := final.(wizard)
	if !ok {
		return wizard{}, diag.New(diag.Usage, "the interactive prompt ended in an unexpected state")
	}
	switch {
	case w.fatal != nil:
		return w, w.fatal
	case w.cancelled:
		return w, diag.New(diag.Cancelled, "cancelled")
	default:
		return w, nil
	}
}
