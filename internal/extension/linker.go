package extension

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workcontext"
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

	if err := r.persistLink(ctx, c, resp.Value); err != nil {
		if r.interrupted(ctx, name, OpDiscover) {
			return false
		}
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
func (r *runner) persistLink(ctx context.Context, component registry.Component, value string) error {
	dir := r.c.WorkDir
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	snapshot := r.c.SnapshotPath
	if real, err := filepath.EvalSymlinks(filepath.Dir(snapshot)); err == nil {
		snapshot = filepath.Join(real, filepath.Base(snapshot))
	}
	p := PublishParams{Home: r.c.Home, Expected: workcontext.Context{State: r.state, WorkDir: dir, SnapshotPath: snapshot}, Component: component, Value: value, Operation: OpDiscover, Now: r.now, Reload: func() (*registry.Registry, error) { return r.c.Registry, nil }}
	if r.c.Index != nil {
		p.Record = func(_ workcontext.Context, entry projection.Provenance) error {
			return r.c.Index.RecordProvenance(entry)
		}
	}
	result, err := PublishLink(ctx, p)
	if err != nil {
		return err
	}
	r.state = result.State
	r.report.Diagnostics = append(r.report.Diagnostics, result.Diagnostics...)
	return nil
}

// classify maps how a component's run failed to its stable warning token.
func classify(err error, res ipc.Result) string {
	switch {
	case errors.Is(err, ipc.ErrInvalidResponse):
		return diag.WarnExtensionResponseInvalid
	case res.Exited:
		return diag.WarnExtensionFailed
	default:
		return diag.WarnExtensionStartFailed
	}
}
