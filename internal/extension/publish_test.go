package extension

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/gustaborges/work/internal/workhome"
)

func publication(t *testing.T) PublishParams {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "in-progress", "repo_branch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	st := &work.State{Work: work.WorkSection{ID: "opaque-id", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "branch", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-02T00:00:00Z"}, Meta: map[string]any{"a.b": float64(1)}, Links: map[string]string{"other.link": "old", "github.pull_request": "starter"}}
	if err := work.Write(filepath.Join(dir, "work-state.json"), st); err != nil {
		t.Fatal(err)
	}
	expected, err := workcontext.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := manualLink("manual")
	return PublishParams{Home: workhome.At(filepath.Join(t.TempDir(), "home")), Expected: expected, Component: c, Operation: OpLink, Value: " exact $;\n\ttext ", Reload: func() (*registry.Registry, error) {
		return &registry.Registry{Components: []registry.Component{c}}, nil
	}}
}
func TestPublishExactAndCurrentSource(t *testing.T) {
	p := publication(t)
	before := p.Expected.State.Work
	var entries []projection.Provenance
	p.Record = func(_ workcontext.Context, entry projection.Provenance) error {
		entries = append(entries, entry)
		return nil
	}
	for i := range 2 {
		p.Now = func() time.Time { return time.Date(2026, 1, 3, i, 0, 0, 0, time.FixedZone("local", 3600)) }
		result, err := PublishLink(context.Background(), p)
		if err != nil || result.State.Links[p.Component.Key] != p.Value || !reflect.DeepEqual(result.State.Work, before) || result.State.Meta["a.b"] != float64(1) || result.State.Links["other.link"] != "old" {
			t.Fatal(result, err)
		}
	}
	if len(entries) != 2 || entries[1].SourceOperation != "link" || entries[1].SourceComponent != "manual/linker" || entries[0].RecordedAt == entries[1].RecordedAt {
		t.Fatal(entries)
	}
	p.Record = func(workcontext.Context, projection.Provenance) error { return errors.New("index unavailable") }
	r, err := PublishLink(context.Background(), p)
	if err != nil || len(r.Diagnostics) != 1 {
		t.Fatal(r, err)
	}
}
func TestPublishRefusalsLeaveBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   int
		change func(*testing.T, *PublishParams)
	}{
		{"identity", 25, func(t *testing.T, p *PublishParams) {
			s, _ := work.Read(p.Expected.SnapshotPath)
			s.Work.ID = "another"
			work.Write(p.Expected.SnapshotPath, s)
		}},
		{"invalid latest", 25, func(t *testing.T, p *PublishParams) {
			os.WriteFile(p.Expected.SnapshotPath, []byte(`{"schema":3,"work":{"id":"opaque-id"}}`), 0600)
		}},
		{"archived", 22, func(t *testing.T, p *PublishParams) {
			s, _ := work.Read(p.Expected.SnapshotPath)
			s.Archive(time.Now())
			work.Write(p.Expected.SnapshotPath, s)
		}},
		{"provider gone", 39, func(t *testing.T, p *PublishParams) {
			p.Reload = func() (*registry.Registry, error) { return &registry.Registry{}, nil }
		}},
		{"key changed", 39, func(t *testing.T, p *PublishParams) {
			c := p.Component
			c.Key = "another.key"
			p.Reload = func() (*registry.Registry, error) {
				return &registry.Registry{Components: []registry.Component{c}}, nil
			}
		}},
		{"lock failure", 43, func(t *testing.T, p *PublishParams) { os.WriteFile(p.Home.Root(), []byte("file"), 0600) }},
		{"write failure", 43, func(t *testing.T, p *PublishParams) {
			p.write = func(string, *work.State) error { return errors.New("write refused") }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := publication(t)
			tc.change(t, &p)
			before, _ := os.ReadFile(p.Expected.SnapshotPath)
			_, err := PublishLink(context.Background(), p)
			after, _ := os.ReadFile(p.Expected.SnapshotPath)
			if diag.ExitCode(err) != tc.code || string(before) != string(after) {
				t.Fatal(err, string(after))
			}
		})
	}
}
func TestPublishCommitCancellationBoundary(t *testing.T) {
	for _, after := range []bool{false, true} {
		p := publication(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if after {
			p.write = func(path string, s *work.State) error { err := work.Write(path, s); cancel(); return err }
		} else {
			reload := p.Reload
			p.Reload = func() (*registry.Registry, error) { cancel(); return reload() }
		}
		_, err := PublishLink(ctx, p)
		if after && err != nil || !after && diag.ExitCode(err) != 20 {
			t.Fatal(after, err)
		}
		s, _ := work.Read(p.Expected.SnapshotPath)
		if (s.Links[p.Component.Key] == p.Value) != after {
			t.Fatal(s.Links)
		}
	}
}
