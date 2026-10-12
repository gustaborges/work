package extension

import (
	"context"
	"errors"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/staging"
	"github.com/gustaborges/work/internal/work"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportArchiveDuringProcess(t *testing.T) {
	p := importParams(t)
	invoke := p.invoke
	p.invoke = func(ctx context.Context, target ipc.Target, in ipc.ImporterInput) (ipc.Result, error) {
		res, err := invoke(ctx, target, in)
		dest := filepath.Join(filepath.Dir(filepath.Dir(p.Expected.WorkDir)), "archived", "moved")
		os.MkdirAll(filepath.Dir(dest), 0755)
		if e := os.Rename(p.Expected.WorkDir, dest); e != nil {
			t.Fatal(e)
		}
		st, e := work.Read(filepath.Join(dest, "work-state.json"))
		if e != nil {
			t.Fatal(e)
		}
		st.Archive(time.Now())
		work.Write(filepath.Join(dest, "work-state.json"), st)
		return res, err
	}
	_, err := ImportOne(context.Background(), p)
	if diag.ExitCode(err) != 22 {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Expected.WorkDir); !os.IsNotExist(err) {
		t.Fatal("recreated active destination")
	}
}

func TestImportCancelledWaitingForCommitLock(t *testing.T) {
	p := importParams(t)
	release, err := lockfile.AcquireContext(context.Background(), p.Home.WorkLockPath(p.Expected.State.Work.ID), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &observedWait{Context: ctx, entered: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := ImportOne(observed, p); done <- err }()
	<-observed.entered
	cancel()
	if err := <-done; diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Expected.WorkDir, "notes")); !os.IsNotExist(err) {
		t.Fatal("cancel added output")
	}
}

func TestImportLockHeldThroughCleanupAndRollback(t *testing.T) {
	p := importParams(t)
	p.remove = func(s *staging.Stage) error {
		release, err := lockfile.AcquireContext(context.Background(), p.Home.WorkLockPath(p.Expected.State.Work.ID), time.Millisecond)
		if err == nil {
			release()
			t.Error("commit lock released during cleanup")
		}
		return s.Remove()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.beforeCommit = cancel
	_, err := ImportOne(ctx, p)
	if diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
	release, err := lockfile.AcquireContext(context.Background(), p.Home.WorkLockPath(p.Expected.State.Work.ID), time.Millisecond)
	if err != nil {
		t.Fatal("lock retained after rollback", err)
	}
	release()
}

func TestImportRecoveryOverridesCancellationAndJoinsCauses(t *testing.T) {
	p := importParams(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := errors.New("cleanup failure")
	p.remove = func(s *staging.Stage) error { cancel(); s.Remove(); return cleanup }
	p.apply = func(ctx context.Context, plan staging.Plan) (*staging.Applied, error) {
		a, err := plan.ApplyContext(ctx)
		os.WriteFile(filepath.Join(p.Expected.WorkDir, "notes", "foreign"), []byte("keep"), 0600)
		return a, err
	}
	_, err := ImportOne(ctx, p)
	if diag.ExitCode(err) != 49 || !errors.Is(err, cleanup) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Expected.WorkDir, "notes", "foreign")); err != nil {
		t.Fatal("removed foreign output")
	}
}

func TestImportLateCancellationIsCommittedSuccess(t *testing.T) {
	p := importParams(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.afterCommit = cancel
	result, err := ImportOne(ctx, p)
	if err != nil || result.Count != 2 || ctx.Err() == nil {
		t.Fatal(result, err)
	}
	if _, err := os.Stat(filepath.Join(p.Expected.WorkDir, "notes", "file")); err != nil {
		t.Fatal(err)
	}
}

func TestImportRevalidatesInputsImmediatelyBeforeInvocation(t *testing.T) {
	p := importParams(t)
	p.Component.Inputs = []string{"link:github.pull_request"}
	c := p.Component
	p.Reload = func() (*registry.Registry, error) {
		return &registry.Registry{Components: []registry.Component{c}}, nil
	}
	st, err := work.Read(p.Expected.SnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	st.Links["github.pull_request"] = "newer reference"
	work.Write(p.Expected.SnapshotPath, st)
	invoke := p.invoke
	p.invoke = func(ctx context.Context, target ipc.Target, in ipc.ImporterInput) (ipc.Result, error) {
		if in.Inputs["github.pull_request"] != "newer reference" {
			t.Fatal(in.Inputs)
		}
		return invoke(ctx, target, in)
	}
	if _, err := ImportOne(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}
