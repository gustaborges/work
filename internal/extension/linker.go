package extension

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
)

// runLinker starts one Linker and persists the value it returns. It reports
// false when the run was interrupted and the remaining components must be
// skipped.
func (r *runner) runLinker(ctx context.Context, d Decision) bool {
	c := d.Component
	name := c.QualifiedName()
	if r.interrupted(ctx, name, OpDiscover) {
		return false
	}
	r.emit(Event{Kind: Running, Component: name, Operation: OpDiscover})

	resp, res, err := ipc.InvokeLinker(ctx, c.Target(r.c.Home.PluginsDir()), ipc.LinkerInput{Inputs: d.Inputs})
	r.report.Ran++
	if err != nil {
		return r.warnRun(ctx, name, OpDiscover, "no link was recorded", err, res)
	}
	// A value that arrives after the interrupt is discarded, not persisted.
	if r.interrupted(ctx, name, OpDiscover) {
		return false
	}
	if resp.Value == "" {
		return true
	}

	if err := r.persistLink(c.Key, resp.Value, name); err != nil {
		w := r.warning(diag.WarnExtensionPersistFailed, name, OpDiscover,
			"Work could not save the link it found; no link was recorded").
			WithHint("Check that the Work's directory is writable, then start again.").
			WithCause(err)
		r.warn(w)
		return true
	}
	r.emit(Event{Kind: Linked, Component: name, Operation: OpDiscover, Key: c.Key})
	return true
}

// persistLink stores value as links[key] under the per-Work lock, so a
// concurrent resume or archive cannot interleave with the read-modify-write,
// then records where it came from. The snapshot is authoritative: a
// provenance failure is noted for WORK_DEBUG and does not undo the link.
func (r *runner) persistLink(key, value, component string) error {
	release, err := lockfile.Acquire(r.c.Home.WorkLockPath(r.state.Work.ID))
	if err != nil {
		return fmt.Errorf("locking the Work: %w", err)
	}
	defer release()

	st, err := work.Read(r.c.SnapshotPath)
	if err != nil {
		return fmt.Errorf("reading the snapshot: %w", err)
	}
	st.Links[key] = value
	if err := work.Write(r.c.SnapshotPath, st); err != nil {
		return fmt.Errorf("writing the snapshot: %w", err)
	}
	r.state = st

	if r.c.Index != nil {
		err := r.c.Index.RecordProvenance(projection.Provenance{
			WorkID: st.Work.ID, Section: "links", Key: key,
			SourceComponent: component, SourceOperation: "discover",
			RecordedAt: r.now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			r.report.Diagnostics = append(r.report.Diagnostics,
				fmt.Sprintf("%s: provenance for link %q was not recorded: %v", component, key, err))
		}
	}
	return nil
}

// classify maps how a component's run failed to its stable warning token.
func classify(err error, res ipc.Result) string {
	switch {
	case errors.Is(err, ipc.ErrInvalidResponse):
		return diag.WarnExtensionResponseInvalid
	case res.ExitCode > 0:
		return diag.WarnExtensionFailed
	default:
		return diag.WarnExtensionStartFailed
	}
}
