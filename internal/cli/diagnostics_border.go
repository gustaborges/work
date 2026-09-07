package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gustaborges/work/internal/diag"
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
		if chain := causeChain(err); chain != "" {
			fmt.Fprintf(ui, "\n%s\n", chain)
		}
	}
	return diag.ExitCode(err)
}

// causeChain renders the wrapped error chain, innermost cause last, for the
// WORK_DEBUG affordance only. It is never part of normal output.
func causeChain(err error) string {
	var lines []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		lines = append(lines, "  "+e.Error())
	}
	if len(lines) < 2 {
		return ""
	}
	return strings.Join(lines, "\n")
}
