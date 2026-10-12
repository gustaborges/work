package extension

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
)

func TestPublishConcurrentUpdates(t *testing.T) {
	p := publication(t)
	q := p
	q.Component = manualLink("alternate")
	q.Component.Key = "other.link"
	q.Value = "newer"
	q.Reload = func() (*registry.Registry, error) {
		return &registry.Registry{Components: []registry.Component{q.Component}}, nil
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, params := range []PublishParams{p, q} {
		go func() { <-start; _, err := PublishLink(context.Background(), params); results <- err }()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	s, _ := work.Read(p.Expected.SnapshotPath)
	if s.Links[p.Component.Key] != p.Value || s.Links[q.Component.Key] != q.Value {
		t.Fatal(s.Links)
	}
}
func TestPublishMovedAndDisappearedDestination(t *testing.T) {
	for _, archived := range []bool{false, true} {
		p := publication(t)
		dir := p.Expected.WorkDir
		if archived {
			dest := filepath.Join(filepath.Dir(filepath.Dir(dir)), "archived", "moved")
			os.MkdirAll(filepath.Dir(dest), 0755)
			os.Rename(dir, dest)
			path := filepath.Join(dest, "work-state.json")
			s, _ := work.Read(path)
			s.Archive(time.Now())
			work.Write(path, s)
		} else {
			os.RemoveAll(dir)
		}
		_, err := PublishLink(context.Background(), p)
		want := 21
		if archived {
			want = 22
		}
		if diag.ExitCode(err) != want {
			t.Fatal(err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("stale directory recreated", err)
		}
	}
}

// The lock wait calls Done in its select. Observing it gives a deterministic
// barrier without sleeps or a public fault-injection option.
type observedWait struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *observedWait) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

func TestPublishCancelledLockWait(t *testing.T) {
	p := publication(t)
	release, err := lockfile.Acquire(p.Home.WorkLockPath(p.Expected.State.Work.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &observedWait{Context: base, entered: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := PublishLink(ctx, p); done <- err }()
	<-ctx.entered
	cancel()
	if err := <-done; diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
	st, _ := work.Read(p.Expected.SnapshotPath)
	if st.Links[p.Component.Key] == p.Value {
		t.Fatal("cancel published a value")
	}
}

func TestPublishRevalidatesAfterLockWait(t *testing.T) {
	for _, action := range []string{"archive", "disable", "remove", "key-change", "unrelated-update"} {
		t.Run(action, func(t *testing.T) {
			p := publication(t)
			reg := &registry.Registry{Components: []registry.Component{p.Component}}
			ready := make(chan struct{})
			p.Reload = func() (*registry.Registry, error) { <-ready; return reg, nil }
			release, err := lockfile.Acquire(p.Home.WorkLockPath(p.Expected.State.Work.ID))
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &observedWait{Context: base, entered: make(chan struct{})}
			done := make(chan error, 1)
			go func() { _, err := PublishLink(ctx, p); done <- err }()
			<-ctx.entered
			want := 0
			switch action {
			case "archive":
				st, _ := work.Read(p.Expected.SnapshotPath)
				st.Archive(time.Now())
				work.Write(p.Expected.SnapshotPath, st)
				want = 22
			case "disable":
				disabled := false
				reg.Packages = []registry.Package{{Alias: p.Component.Alias, Enabled: &disabled}}
				want = 39
			case "remove":
				reg.Components = nil
				want = 39
			case "key-change":
				reg.Components[0].Key = "changed.key"
				want = 39
			case "unrelated-update":
				st, _ := work.Read(p.Expected.SnapshotPath)
				st.Links["other.link"] = "newer"
				st.Work.LastAccessedAt = "2026-01-03T00:00:00Z"
				work.Write(p.Expected.SnapshotPath, st)
			}
			close(ready)
			release()
			if err := <-done; diag.ExitCode(err) != want {
				t.Fatal(action, err)
			}
			st, _ := work.Read(p.Expected.SnapshotPath)
			if want != 0 && st.Links[p.Component.Key] == p.Value {
				t.Fatal("refused publication changed link")
			}
			if action == "unrelated-update" && (st.Links["other.link"] != "newer" || st.Work.LastAccessedAt != "2026-01-03T00:00:00Z") {
				t.Fatal(st)
			}
		})
	}
}

func TestPublishProviderDisabledUnderLock(t *testing.T) {
	p := publication(t)
	disabled := false
	p.Reload = func() (*registry.Registry, error) {
		return &registry.Registry{Packages: []registry.Package{{Alias: p.Component.Alias, Enabled: &disabled}}, Components: []registry.Component{p.Component}}, nil
	}
	_, err := PublishLink(context.Background(), p)
	if diag.ExitCode(err) != 39 {
		t.Fatal(err)
	}
	p.Reload = func() (*registry.Registry, error) {
		c := p.Component
		c.Role = plugin.RoleImporter
		return &registry.Registry{Components: []registry.Component{c}}, nil
	}
	_, err = PublishLink(context.Background(), p)
	if diag.ExitCode(err) != 39 {
		t.Fatal(err)
	}
}
