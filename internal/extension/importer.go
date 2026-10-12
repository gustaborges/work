package extension

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/staging"
	"github.com/gustaborges/work/internal/workcontext"
)

const noneAdded = "none of its files were added"

// importerPhase runs the Importers made eligible by the links that exist now,
// after every Linker result was persisted. Each runs alone, so a later
// Importer's plan sees what an earlier one incorporated.
func (r *runner) importerPhase(ctx context.Context) {
	for _, d := range Eligible(r.c.Registry, plugin.RoleImporter, StartFinalized, r.subject()) {
		if !r.runImporter(ctx, d) {
			return
		}
	}
}

// runImporter starts one Importer against a private stage, then plans and
// incorporates whatever it wrote. It reports false when the run was
// interrupted and the remaining components must be skipped.
func (r *runner) runImporter(ctx context.Context, d Decision) bool {
	name := d.Component.QualifiedName()
	if r.interrupted(ctx, name, OpImport) {
		return false
	}
	r.emit(Event{Kind: Running, Component: name, Operation: OpImport})

	// Capture the real destination before invocation, just as automatic link
	// publication does. Temp directories can contain symlinks or short names.
	dir := r.c.WorkDir
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	snapshot := r.c.SnapshotPath
	if real, err := filepath.EvalSymlinks(filepath.Dir(snapshot)); err == nil {
		snapshot = filepath.Join(real, filepath.Base(snapshot))
	}
	result, err := ImportOne(ctx, ImportParams{Home: r.c.Home, Expected: workcontext.Context{State: r.state, WorkDir: dir, SnapshotPath: snapshot, WorktreePath: r.c.WorktreePath}, Component: d.Component, Automatic: true, Starter: r.c.Starter, Reload: func() (*registry.Registry, error) { return r.c.Registry, nil }})
	if result.Ran {
		r.report.Ran++
	}
	if err != nil {
		var category diag.Category
		if d, ok := errors.AsType[*diag.Error](err); ok {
			category = d.Category
		}
		switch category {
		case diag.Cancelled:
			r.interrupted(ctx, name, OpImport)
			return false
		case diag.ExtensionStartFailed, diag.ExtensionFailed, diag.ExtensionResponseInvalid:
			var failure *Failure
			if errors.As(err, &failure) {
				return r.warnRun(ctx, name, OpImport, noneAdded, failure.Err, ipc.Result{ExitCode: failure.ExitCode, Stderr: failure.Stderr, Exited: category != diag.ExtensionStartFailed})
			}
		case diag.ExtensionOutputRefused:
			r.warn(r.stagingWarning(name, err))
			return true
		}
		summary := "Work could not add its files; " + noneAdded
		if category == diag.ExtensionCleanupFailed {
			summary = "cleanup needs recovery; inspect the retained cause with WORK_DEBUG=1"
		}
		r.warn(r.warning(diag.WarnExtensionPersistFailed, name, OpImport, summary).WithCause(err))
		return true
	}
	n := result.Count
	if n > 0 {
		r.emit(Event{Kind: Imported, Component: name, Operation: OpImport, Count: n})
	}
	return true
}

// stagingWarning maps a failure to plan the output to its warning: a refusal
// names the relative path involved (which is about the refusal, not artifact
// content); anything else means Work could not inspect the stage.
func (r *runner) stagingWarning(component string, err error) diag.Warning {
	var ref *staging.Refusal
	if !errors.As(err, &ref) {
		return r.warning(diag.WarnExtensionPersistFailed, component, OpImport,
			"Work could not read the files it produced; "+noneAdded).WithCause(err)
	}
	var why string
	switch ref.Reason {
	case staging.Exists:
		why = fmt.Sprintf("it would overwrite %q", ref.Rel)
	case staging.ReservedPath:
		why = fmt.Sprintf("it would write %q, which Work reserves", ref.Rel)
	case staging.NotRegular:
		why = fmt.Sprintf("%q is not a regular file or a directory", ref.Rel)
	default:
		why = fmt.Sprintf("%q would land outside the Work", ref.Rel)
	}
	return r.warning(diag.WarnExtensionOutputRefused, component, OpImport, why+"; "+noneAdded).
		WithHint("Nothing in the Work was changed.").WithCause(err)
}
