package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/extension"
	"github.com/gustaborges/work/internal/present/diagrender"
	"github.com/gustaborges/work/internal/present/theme"
)

// renderDiagnostic writes the single human-facing rendering of err and returns
// the process exit code. This is the only place in the tree a terminal failure
// is printed (contracts/diagnostics.md, FR-014): every lower layer — present,
// the orchestrators, the domain/infra packages — returns a structured error and
// never writes to a diagnostic stream.
//
//   - non-interactive UI: the frozen "error: <token>: <message>" line, exit code
//     the category's code — byte-for-byte as F1/F2.
//   - interactive + diag.Cancelled: "✘ Operation cancelled", exit 20.
//   - interactive + other *diag.Error: "✘ <Summary, else Msg>" and, when Hint is
//     set, a second "  → <Hint>" line.
//   - interactive + non-diag error: a generic line plus the WORK_DEBUG affordance.
//
// When WORK_DEBUG is non-empty the wrapped cause chain is appended after the
// human line (an env affordance, not a flag — FR-016).
func renderDiagnostic(ui io.Writer, in io.Reader, interactive bool, err error) int {
	if err == nil {
		return 0
	}

	if !interactive {
		fmt.Fprintln(ui, diag.Format(err))
		return diag.ExitCode(err)
	}

	th := theme.New(theme.Detect(ui, in), true)
	var d *diag.Error
	switch {
	case errors.As(err, &d) && d.Category == diag.Cancelled:
		fmt.Fprint(ui, diagrender.Cancel(th))
	case errors.As(err, &d):
		summary := d.Summary
		if summary == "" {
			summary = d.Msg
		}
		fmt.Fprint(ui, diagrender.Human(th, summary, d.Hint))
	default:
		fmt.Fprint(ui, diagrender.Unexpected(th))
	}

	if os.Getenv("WORK_DEBUG") != "" {
		// A non-diag error's message is hidden from the human line, so a
		// single-entry chain is still new information; a *diag.Error's message
		// is already printed above and would only be duplicated.
		if chain := causeChain(err, d == nil); chain != "" {
			fmt.Fprintf(ui, "\n%s\n", chain)
		}
	}
	return diag.ExitCode(err)
}

// causeChain renders the wrapped error chain, innermost cause last, for the
// WORK_DEBUG affordance only. It is never part of normal output. Chains of a
// single entry are dropped unless keepSingle is set.
func causeChain(err error, keepSingle bool) string {
	var lines []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		lines = append(lines, "  "+e.Error())
	}
	if len(lines) < 2 && !keepSingle {
		return ""
	}
	return strings.Join(lines, "\n")
}

// renderWarning writes one extension warning to the UI channel. It never
// touches stdout and never changes an exit code.
//
//   - non-interactive: the frozen "warning: <token>: <alias>/<name> (<op>) for
//     Work <id>: <summary>" line.
//   - interactive: "⚠ <alias>/<name>: <summary>" and, when there is one,
//     "  → <hint>".
//
// When WORK_DEBUG is non-empty the component's exit status and the tail of its
// stderr follow the human line. Neither the human line nor the debug detail
// ever carries a link or metadata value or artifact content; stderr is the
// extension's own text and is shown only because the user asked for it.
func renderWarning(ui io.Writer, in io.Reader, interactive bool, w diag.Warning) {
	if interactive {
		th := theme.New(theme.Detect(ui, in), true)
		fmt.Fprint(ui, diagrender.Warn(th, w.Component+": "+w.Summary, w.Hint))
	} else {
		fmt.Fprintln(ui, diag.FormatWarning(w))
	}
	if os.Getenv("WORK_DEBUG") == "" || w.Cause == nil {
		return
	}
	var f *extension.Failure
	if errors.As(w.Cause, &f) {
		if f.ExitCode > 0 {
			fmt.Fprintf(ui, "  exit status: %d\n", f.ExitCode)
		}
		fmt.Fprintf(ui, "  cause: %v\n", f.Err)
		if tail := strings.TrimSpace(f.Stderr); tail != "" {
			fmt.Fprintln(ui, "  stderr:")
			for _, line := range strings.Split(tail, "\n") {
				fmt.Fprintf(ui, "    %s\n", line)
			}
		}
		return
	}
	fmt.Fprintf(ui, "  cause: %v\n", w.Cause)
}

// progressObserver renders the automatic extension phases on the UI channel:
// one line when a component starts, one when it has a result to report, and a
// warning when it fails. The text is the same interactive or not; only the
// muted colour differs, and only when the terminal supports it.
type progressObserver struct {
	ui          io.Writer
	in          io.Reader
	interactive bool
	th          theme.Theme
}

func newProgressObserver(ui io.Writer, in io.Reader, interactive bool) *progressObserver {
	p := &progressObserver{ui: ui, in: in, interactive: interactive}
	if interactive {
		p.th = theme.New(theme.Detect(ui, in), true)
	}
	return p
}

// OnEvent implements extension.Observer.
func (p *progressObserver) OnEvent(e extension.Event) {
	switch e.Kind {
	case extension.Running:
		p.line(fmt.Sprintf("work: running %s (%s)", e.Component, e.Operation))
	case extension.Linked:
		p.line(fmt.Sprintf("work: %s: linked %s", e.Component, e.Key))
	case extension.Imported:
		p.line(fmt.Sprintf("work: %s: imported %d item(s)", e.Component, e.Count))
	case extension.Warned:
		renderWarning(p.ui, p.in, p.interactive, *e.Warning)
	}
}

func (p *progressObserver) line(s string) {
	if p.interactive {
		s = p.th.Muted.Render(s)
	}
	fmt.Fprintln(p.ui, s)
}
