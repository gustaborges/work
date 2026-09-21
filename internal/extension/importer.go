package extension

import (
	"context"
	"errors"
	"fmt"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/staging"
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

	stage, err := staging.NewStage()
	if err != nil {
		r.warn(r.warning(diag.WarnExtensionPersistFailed, name, OpImport,
			"Work could not prepare a place for its files; "+noneAdded).
			WithHint("Check that the temporary directory is writable, then start again.").
			WithCause(err))
		return true
	}
	// Removal must not depend on ctx: an interrupt is exactly when it matters.
	defer stage.Remove()

	res, err := ipc.InvokeImporter(ctx, d.Component.Target(r.c.Home.PluginsDir()),
		ipc.ImporterInput{Inputs: d.Inputs, OutputDir: stage.Dir})
	r.report.Ran++
	if err != nil {
		return r.warnRun(ctx, name, OpImport, noneAdded, err, res)
	}
	// Whatever arrives after an interrupt is discarded, not incorporated.
	if r.interrupted(ctx, name, OpImport) {
		return false
	}

	plan, err := staging.Build(stage.Dir, r.c.WorkDir)
	if err != nil {
		r.warn(r.stagingWarning(name, err))
		return true
	}
	n, err := plan.Incorporate()
	if err != nil {
		r.warn(r.warning(diag.WarnExtensionPersistFailed, name, OpImport,
			"Work could not add its files and undid the attempt; "+noneAdded).
			WithHint("Check that the Work's directory is writable, then start again.").
			WithCause(err))
		return true
	}
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
