package extension

import (
	"context"
	"errors"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/staging"
	"os"
	"path/filepath"
	"testing"
)

func importParams(t *testing.T) ImportParams {
	p := publication(t)
	c := p.Component
	c.Role = plugin.RoleImporter
	c.Key = ""
	c.Name = "importer"
	c.Inputs = nil
	return ImportParams{Home: p.Home, Expected: p.Expected, Component: c, Reload: func() (*registry.Registry, error) {
		return &registry.Registry{Components: []registry.Component{c}}, nil
	}, invoke: func(_ context.Context, _ ipc.Target, in ipc.ImporterInput) (ipc.Result, error) {
		if err := os.Mkdir(filepath.Join(in.OutputDir, "notes"), 0755); err != nil {
			t.Fatal(err)
		}
		return ipc.Result{Exited: true}, os.WriteFile(filepath.Join(in.OutputDir, "notes", "file"), []byte("data"), 0600)
	}}
}

func TestImportOneCommitAndFailures(t *testing.T) {
	for _, mode := range []string{"ok", "empty", "launch", "exit", "response", "refusal", "apply", "cleanup", "cancel", "provider"} {
		t.Run(mode, func(t *testing.T) {
			p := importParams(t)
			before, _ := os.ReadFile(p.Expected.SnapshotPath)
			var stage string
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := 0
			switch mode {
			case "launch":
				want = 44
				p.invoke = func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error) {
					return ipc.Result{}, errors.New("launch")
				}
			case "exit":
				want = 45
				p.invoke = func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error) {
					return ipc.Result{Exited: true, ExitCode: 1}, errors.New("exit")
				}
			case "response":
				want = 46
				p.invoke = func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error) {
					return ipc.Result{Exited: true}, ipc.ErrInvalidResponse
				}
			case "empty":
				p.invoke = func(context.Context, ipc.Target, ipc.ImporterInput) (ipc.Result, error) {
					return ipc.Result{Exited: true}, nil
				}
			case "refusal":
				want = 47
				os.Mkdir(filepath.Join(p.Expected.WorkDir, "notes"), 0755)
				os.WriteFile(filepath.Join(p.Expected.WorkDir, "notes", "file"), []byte("old"), 0600)
			case "apply":
				want = 48
				t.Setenv("WORK_FAIL_AT", "incorporate:1")
			case "cleanup":
				want = 49
				p.remove = func(s *staging.Stage) error { s.Remove(); return errors.New("remove") }
			case "cancel":
				want = 20
				p.beforeCommit = cancel
			case "provider":
				want = 39
				calls := 0
				reload := p.Reload
				p.Reload = func() (*registry.Registry, error) {
					calls++
					if calls > 1 {
						return &registry.Registry{}, nil
					}
					return reload()
				}
			}
			invoke := p.invoke
			p.invoke = func(c context.Context, target ipc.Target, in ipc.ImporterInput) (ipc.Result, error) {
				stage = in.OutputDir
				return invoke(c, target, in)
			}
			result, err := ImportOne(ctx, p)
			if diag.ExitCode(err) != want {
				t.Fatal(result, err, want)
			}
			if want == 0 && mode == "ok" && result.Count != 2 {
				t.Fatal(result)
			}
			if _, err := os.Stat(stage); !os.IsNotExist(err) {
				t.Fatal("stage remains", stage, err)
			}
			after, _ := os.ReadFile(p.Expected.SnapshotPath)
			if string(before) != string(after) {
				t.Fatal("snapshot changed")
			}
			if want != 0 && mode != "refusal" {
				if _, err := os.Stat(filepath.Join(p.Expected.WorkDir, "notes")); !os.IsNotExist(err) {
					t.Fatal("partial output remains")
				}
			}
		})
	}
}
