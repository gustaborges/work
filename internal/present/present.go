package present

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

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
// them uniformly. On reaching a terminal state the model renders an empty View
// (so Bubble Tea's inline renderer clears the whole active frame, however tall)
// and exposes the compact receipt or notice through finalFrame; run writes that
// into the freshly cleared area once the program has exited. Committing the
// final frame through Bubble Tea's shrink path instead is unreliable for a
// frame that was many rows tall, e.g. a long selector (research R4).
type sessionModel interface {
	tea.Model
	outcome() outcome
	// finalFrame is the compact string to print after the program exits, or ""
	// when the step left nothing to show (a Fatal abort, or a context-driven
	// interrupt caught outside the model).
	finalFrame() string
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
	sm, cerr := classify(final, err)
	// The program has cleared its active frame; drop the compact receipt or
	// notice into the now-empty area, ahead of any stable stdout the CLI adds.
	if sm != nil {
		if frame := sm.finalFrame(); frame != "" {
			if !strings.HasSuffix(frame, "\n") {
				frame += "\n"
			}
			_, _ = io.WriteString(pio.ui(), frame)
		}
	}
	return sm, cerr
}

// classify turns a finished Bubble Tea program into the (model, error) pair the
// primitives return. It is separated from run so the mapping is unit-testable
// without a terminal.
func classify(final tea.Model, err error) (sessionModel, error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fm, _ := final.(sessionModel)
			return fm, diag.New(diag.Cancelled, "cancelled")
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
