package extension

import (
	"context"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/work"
)

// runner carries the state of one Run: the Work's snapshot as later phases
// must see it, and the report being built.
type runner struct {
	c      Context
	state  *work.State
	report Report
}

func newRunner(c Context) *runner {
	st := *c.State
	st.Links = cloneLinks(c.State.Links)
	return &runner{c: c, state: &st}
}

func cloneLinks(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (r *runner) now() time.Time {
	if r.c.Now != nil {
		return r.c.Now()
	}
	return time.Now()
}

func (r *runner) subject() Subject {
	return Subject{State: r.state, WorktreePath: r.c.WorktreePath, Starter: r.c.Starter}
}

func (r *runner) emit(e Event) {
	if r.c.Observer != nil {
		r.c.Observer.OnEvent(e)
	}
}

func (r *runner) warning(token, component, operation, summary string) diag.Warning {
	return diag.NewWarning(token, r.state.Work.ID, component, operation, summary)
}

func (r *runner) warn(w diag.Warning) {
	r.report.Warnings = append(r.report.Warnings, w)
	r.emit(Event{Kind: Warned, Component: w.Component, Operation: w.Operation, Warning: &w})
}

// interrupted reports whether the run was cancelled and, if so, records the
// single interrupted warning. The caller must then skip every remaining
// component.
func (r *runner) interrupted(ctx context.Context, component, operation string) bool {
	if ctx.Err() == nil {
		return false
	}
	r.warn(r.warning(diag.WarnExtensionInterrupted, component, operation,
		"it was interrupted; the remaining extensions were skipped").
		WithHint("The Work was created and is ready to use."))
	return true
}

// warnRun turns a failed run into its warning. It reports false when the
// failure was an interrupt, which ends the whole run; any other failure leaves
// the run going with the next component.
func (r *runner) warnRun(ctx context.Context, component, operation, effect string, err error, res ipc.Result) bool {
	if r.interrupted(ctx, component, operation) {
		return false
	}
	token := classify(err, res)
	var summary, hint string
	switch token {
	case diag.WarnExtensionFailed:
		summary, hint = "it exited unsuccessfully; "+effect, "Run it again with WORK_DEBUG=1 to see why."
	case diag.WarnExtensionResponseInvalid:
		summary, hint = "it returned a response Work could not use; "+effect, "This is a bug in the plugin, not something fixable from the command line."
	default:
		summary, hint = "it could not be started; "+effect, "Check that the plugin is still installed with: work plugin list"
	}
	r.warn(r.warning(token, component, operation, summary).WithHint(hint).
		WithCause(&Failure{ExitCode: res.ExitCode, Stderr: res.Stderr, Err: err}))
	return true
}

// linkerPhase runs every eligible Linker in order. Eligibility is decided once,
// before any Linker runs, so a Linker never depends on another's result. It
// reports false when the run was interrupted.
func (r *runner) linkerPhase(ctx context.Context) bool {
	for _, d := range Eligible(r.c.Registry, plugin.RoleLinker, StartFinalized, r.subject()) {
		if !r.runLinker(ctx, d) {
			return false
		}
	}
	return true
}

// importerPhase is filled in with the Importer runner.
func (r *runner) importerPhase(ctx context.Context) {}
