package extension

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/staging"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/gustaborges/work/internal/workhome"
)

// ImportParams captures one provider and canonical destination. Automatic
// callers retain their event policy; manual callers reload retained state.
type ImportParams struct {
	Home      workhome.Home
	Expected  workcontext.Context
	Component registry.Component
	Reload    func() (*registry.Registry, error)
	Automatic bool
	Starter   registry.Component
	// Started, when set, runs once validation has passed and just before the
	// Importer is invoked, so callers announce only work that actually begins.
	Started      func()
	invoke       func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error)
	remove       func(*staging.Stage) error
	apply        func(context.Context, staging.Plan) (*staging.Applied, error)
	beforeCommit func()
	afterCommit  func()
}

// ImportResult reports only committed entries and whether invocation began.
type ImportResult struct {
	Count int
	Ran   bool
}

var importMessages = map[diag.Category]string{
	diag.Cancelled:                  "import cancelled",
	diag.ManualComponentUnavailable: "the selected provider changed; select it again",
	diag.RequiredInputMissing:       "the Importer declares an input Work cannot read",
	diag.ExtensionStartFailed:       "the Importer could not be started",
	diag.ExtensionFailed:            "the Importer exited unsuccessfully",
	diag.ExtensionResponseInvalid:   "the Importer returned an invalid response",
	diag.ExtensionOutputRefused:     "unsafe or colliding output was refused",
	diag.ExtensionPersistFailed:     "the files could not be incorporated",
	diag.ExtensionCleanupFailed:     "cleanup or rollback needs recovery",
}

// ImportOne runs outside the Work lock, then validates, applies, cleans and
// commits under that lock. Cleanup and compensation never use cancelled ctx.
func ImportOne(ctx context.Context, p ImportParams) (result ImportResult, err error) {
	failure := func(cat diag.Category, cause error) error {
		hint := "Check the plugin and Work directory; use WORK_DEBUG=1 for details."
		message := importMessages[cat]
		switch {
		case cat == diag.ExtensionCleanupFailed:
			hint = "Cleanup could not finish. Use WORK_DEBUG=1 to inspect unresolved paths and removal errors before retrying."
		case errors.Is(cause, lockfile.ErrTimeout):
			message, hint = "another work operation is using this Work", "Try again in a moment."
		}
		return diag.Wrapf(cat, cause, "%s (import) for Work %s: %s", p.Component.QualifiedName(), p.Expected.State.Work.ID, message).WithHint(hint)
	}
	validate := func() (workcontext.Context, registry.Component, error) {
		if ctx.Err() != nil {
			return workcontext.Context{}, registry.Component{}, failure(diag.Cancelled, ctx.Err())
		}
		current, err := publicationContext(ctx, p.Expected)
		if err != nil {
			return current, registry.Component{}, err
		}
		reg, err := p.Reload()
		if err != nil {
			return current, registry.Component{}, err
		}
		var c registry.Component
		if p.Automatic {
			for _, d := range Eligible(reg, plugin.RoleImporter, StartFinalized, Subject{State: current.State, WorktreePath: current.WorktreePath, Starter: p.Starter}) {
				if d.Component.QualifiedName() == p.Component.QualifiedName() {
					c = d.Component
					break
				}
			}
			if c.Name == "" {
				return current, c, failure(diag.ManualComponentUnavailable, nil)
			}
		} else {
			c, err = ResolveManual(reg, plugin.RoleImporter, p.Component.QualifiedName())
			if err != nil {
				return current, c, err
			}
		}
		if !reflect.DeepEqual(c, p.Component) {
			return current, c, failure(diag.ManualComponentUnavailable, fmt.Errorf("provider changed"))
		}
		return current, c, nil
	}
	current, c, err := validate()
	if err != nil {
		return result, err
	}
	inputs, missing, err := ResolveInputs(c.Inputs, Subject{State: current.State, WorktreePath: current.WorktreePath, Starter: p.Starter})
	if err != nil {
		return result, failure(diag.RequiredInputMissing, err)
	}
	if len(missing) > 0 {
		return result, diag.Newf(diag.RequiredInputMissing, "%s (import) for Work %s: required inputs missing: %s; associate required links with `work link LINKER VALUE` or supply declared metadata", c.QualifiedName(), current.State.Work.ID, strings.Join(missing, ", ")).WithHint("Only declared present inputs are sent to the Importer; no discovery runs automatically.")
	}
	stage, err := staging.NewStage()
	if err != nil {
		return result, failure(diag.ExtensionPersistFailed, err)
	}
	remove := p.remove
	if remove == nil {
		remove = func(s *staging.Stage) error { return s.Remove() }
	}
	cleaned := false
	var applied *staging.Applied
	var release func()
	defer func() {
		if !cleaned {
			if e := remove(stage); e != nil {
				err = failure(diag.ExtensionCleanupFailed, errors.Join(err, e))
			}
		}
		if err != nil && applied != nil {
			if e := applied.Rollback(); e != nil {
				err = failure(diag.ExtensionCleanupFailed, errors.Join(err, e))
			}
			result.Count = 0
		}
		if release != nil {
			release()
		}
	}()
	invoke := p.invoke
	if invoke == nil {
		invoke = ipc.InvokeImporter
	}
	result.Ran = true
	if p.Started != nil {
		p.Started()
	}
	res, runErr := invoke(ctx, c.Target(p.Home.PluginsDir()), ipc.ImporterInput{Inputs: inputs, OutputDir: stage.Dir})
	if runErr != nil {
		cat := diag.ExtensionStartFailed
		switch {
		case ctx.Err() != nil:
			cat = diag.Cancelled
		case errors.Is(runErr, ipc.ErrInvalidResponse):
			cat = diag.ExtensionResponseInvalid
		case res.Exited:
			cat = diag.ExtensionFailed
		}
		return result, failure(cat, &Failure{ExitCode: res.ExitCode, Stderr: res.Stderr, Err: runErr})
	}
	release, err = lockfile.AcquireContext(ctx, p.Home.WorkLockPath(current.State.Work.ID), lockTimeout)
	if err != nil {
		if ctx.Err() != nil {
			return result, failure(diag.Cancelled, ctx.Err())
		}
		return result, failure(diag.ExtensionPersistFailed, err)
	}
	current, _, err = validate()
	if err != nil {
		return result, err
	}
	plan, err := staging.Build(stage.Dir, current.WorkDir)
	classify := func(err error) error {
		cat := diag.ExtensionPersistFailed
		var refusal *staging.Refusal
		if errors.As(err, &refusal) {
			cat = diag.ExtensionOutputRefused
		} else if ctx.Err() != nil {
			cat = diag.Cancelled
		}
		return failure(cat, err)
	}
	if err != nil {
		return result, classify(err)
	}
	apply := p.apply
	if apply == nil {
		apply = func(ctx context.Context, plan staging.Plan) (*staging.Applied, error) { return plan.ApplyContext(ctx) }
	}
	applied, err = apply(ctx, plan)
	if err != nil {
		return result, classify(err)
	}
	if err := remove(stage); err != nil {
		return result, failure(diag.ExtensionCleanupFailed, err)
	}
	cleaned = true
	if p.beforeCommit != nil {
		p.beforeCommit()
	}
	if ctx.Err() != nil {
		return result, failure(diag.Cancelled, ctx.Err())
	}
	if err := applied.Commit(); err != nil {
		return result, failure(diag.ExtensionPersistFailed, err)
	}
	result.Count = applied.Created
	if p.afterCommit != nil {
		p.afterCommit()
	}
	return result, nil
}
