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

// outcome is how an interaction session ended, read by run once the program has
// returned.
type outcome struct {
	cancelled bool
	fatal     error
}

// sessionModel is the contract every primitive model satisfies so run can drive
// them uniformly. The model MUST set its View to the compact final frame before
// returning tea.Quit, so Bubble Tea's graceful final render commits only that
// frame (research R4).
type sessionModel interface {
	tea.Model
	outcome() outcome
}

// run executes one primitive model as a bounded inline Bubble Tea program bound
// to io, and maps the terminal outcome to the shared diagnostic vocabulary:
// context cancellation and the model's own cancelled state both become
// diag.Cancelled (exit 20 preserved); a Fatal validation error is returned
// unwrapped for the CLI border. This is present's only dependency on
// internal/diag.
func run(ctx context.Context, pio IO, m sessionModel) (sessionModel, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(pio.in()),
		tea.WithOutput(pio.ui()),
	).Run()
	return classify(final, err)
}

// classify turns a finished Bubble Tea program into the (model, error) pair the
// primitives return. It is separated from run so the mapping is unit-testable
// without a terminal.
func classify(final tea.Model, err error) (sessionModel, error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, diag.New(diag.Cancelled, "cancelled")
		}
		return nil, diag.Wrap(diag.Usage, err, "the interactive prompt could not be shown")
	}
	fm, ok := final.(sessionModel)
	if !ok {
		return nil, diag.New(diag.Usage, "the interactive prompt ended in an unexpected state")
	}
	oc := fm.outcome()
	switch {
	case oc.fatal != nil:
		return fm, oc.fatal
	case oc.cancelled:
		return fm, diag.New(diag.Cancelled, "cancelled")
	default:
		return fm, nil
	}
}
