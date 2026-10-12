package extension

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/registry"
)

func shortLockWait(t *testing.T) {
	t.Helper()
	old := lockTimeout
	lockTimeout = 50 * time.Millisecond
	t.Cleanup(func() { lockTimeout = old })
}

func TestPublishLockContentionIsNotReportedAsWriteFailure(t *testing.T) {
	shortLockWait(t)
	p := publication(t)
	release, err := lockfile.Acquire(p.Home.WorkLockPath(p.Expected.State.Work.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = PublishLink(context.Background(), p)
	var d *diag.Error
	if diag.ExitCode(err) != diag.LinkFailed.Code || !errors.As(err, &d) {
		t.Fatal(err)
	}
	if !strings.Contains(d.Msg, "another work operation") || strings.Contains(d.Msg, "cannot save") || strings.Contains(d.Hint, "writable") {
		t.Fatalf("msg=%q hint=%q", d.Msg, d.Hint)
	}
}

func TestImportOneLockContentionIsNotReportedAsWriteFailure(t *testing.T) {
	shortLockWait(t)
	p := importParams(t)
	release, err := lockfile.Acquire(p.Home.WorkLockPath(p.Expected.State.Work.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = ImportOne(context.Background(), p)
	var d *diag.Error
	if diag.ExitCode(err) != diag.ExtensionPersistFailed.Code || !errors.As(err, &d) {
		t.Fatal(err)
	}
	if !strings.Contains(d.Msg, "another work operation") || strings.Contains(d.Hint, "plugin") {
		t.Fatalf("msg=%q hint=%q", d.Msg, d.Hint)
	}
}

func TestImportOneInvalidInputDeclarationHasReason(t *testing.T) {
	p := importParams(t)
	c := p.Component
	c.Inputs = []string{"not-an-input"}
	p.Component = c
	p.Reload = func() (*registry.Registry, error) {
		return &registry.Registry{Components: []registry.Component{c}}, nil
	}

	_, err := ImportOne(context.Background(), p)
	if diag.ExitCode(err) != diag.RequiredInputMissing.Code {
		t.Fatal(err)
	}
	if msg := err.Error(); strings.HasSuffix(strings.TrimSpace(msg), ":") || !strings.Contains(msg, "input") {
		t.Fatalf("message lacks a reason: %q", msg)
	}
}

func TestImportMessagesCoverEveryFailureCategory(t *testing.T) {
	for _, c := range []diag.Category{
		diag.Cancelled, diag.ManualComponentUnavailable, diag.RequiredInputMissing,
		diag.ExtensionStartFailed, diag.ExtensionFailed, diag.ExtensionResponseInvalid,
		diag.ExtensionOutputRefused, diag.ExtensionPersistFailed, diag.ExtensionCleanupFailed,
	} {
		if importMessages[c] == "" {
			t.Errorf("no import message for %s", c.Token)
		}
	}
}

func TestImportOneAnnouncesOnlyWhenTheImporterStarts(t *testing.T) {
	t.Run("started", func(t *testing.T) {
		p := importParams(t)
		started := 0
		p.Started = func() { started++ }
		if _, err := ImportOne(context.Background(), p); err != nil || started != 1 {
			t.Fatal(started, err)
		}
	})
	t.Run("missing required input", func(t *testing.T) {
		p := importParams(t)
		c := p.Component
		c.Inputs = []string{"link:github.issue"}
		p.Component = c
		p.Reload = func() (*registry.Registry, error) {
			return &registry.Registry{Components: []registry.Component{c}}, nil
		}
		p.Started = func() { t.Fatal("announced an Importer that never ran") }
		p.invoke = func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error) {
			t.Fatal("Importer ran")
			return ipc.Result{}, nil
		}
		result, err := ImportOne(context.Background(), p)
		if diag.ExitCode(err) != diag.RequiredInputMissing.Code || result.Ran {
			t.Fatal(result, err)
		}
	})
}
