package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

func statusSnapshot(t *testing.T, dir, id string, archived bool) *work.State {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	s := &work.State{Schema: 3, Work: work.WorkSection{ID: id, Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "feature/status", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}, Meta: map[string]any{}, Links: map[string]string{}}
	if archived {
		s.Archive(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	}
	if err := work.Write(filepath.Join(dir, "work-state.json"), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func statusRun(t *testing.T, ctx context.Context, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(append([]string{"status"}, args...))
	err := root.ExecuteContext(ctx)
	return out.String(), errOut.String(), err
}
func TestStatusView(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(fmt.Sprint(archived), func(t *testing.T) {
			area := "in-progress"
			if archived {
				area = "archived"
			}
			dir := filepath.Join(t.TempDir(), area, "space ação")
			s := statusSnapshot(t, dir, "opaque-ID", archived)
			value := "line\n\t\x1b[31m" + strings.Repeat("x", 500)
			s.Links = map[string]string{"z.last": "ordinary", "a.first": value}
			work.Write(filepath.Join(dir, "work-state.json"), s)
			t.Chdir(dir)
			t.Setenv("WORK_HOME", filepath.Join(t.TempDir(), "nonexistent"))
			out, stderr, err := statusRun(t, context.Background())
			if err != nil || stderr != "" {
				t.Fatalf("%v %s", err, stderr)
			}
			if strings.ContainsAny(out, "\t\x1b") || len(strings.Split(strings.TrimSuffix(out, "\n"), "\n")) != 6 {
				t.Fatalf("unsafe output %q", out)
			}
			if strings.Index(out, "a.first") > strings.Index(out, "z.last") || !strings.Contains(out, strconv.Quote(value)) {
				t.Fatal(out)
			}
			out, stderr, err = statusRun(t, context.Background(), "--json")
			if err != nil || stderr != "" {
				t.Fatal(err, stderr)
			}
			var v map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &v); err != nil {
				t.Fatal(err)
			}
			if len(v) != 5 {
				t.Fatalf("fields: %s", out)
			}
			var links map[string]string
			json.Unmarshal(v["links"], &links)
			if links["a.first"] != value {
				t.Fatal("value changed")
			}
			s.Links = map[string]string{}
			work.Write(filepath.Join(dir, "work-state.json"), s)
			out, _, err = statusRun(t, context.Background(), "--json")
			if err != nil || !strings.Contains(out, `"links":{}`) {
				t.Fatal(out, err)
			}
		})
	}
}
func TestStatusExplicitPrecedenceAndFailures(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "in-progress", "a")
	b := filepath.Join(root, "archived", "b")
	statusSnapshot(t, a, "A", false)
	statusSnapshot(t, b, "B", true)
	home := workhome.At(t.TempDir())
	t.Setenv("WORK_HOME", home.Root())
	cfg := config.Default()
	cfg.Workspace = root
	config.Save(home.ConfigFile(), cfg)
	t.Chdir(a)
	out, _, err := statusRun(t, context.Background(), "B")
	if err != nil || !strings.Contains(out, "work: id B\n") {
		t.Fatal(out, err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"unknown"}, 21}, {[]string{""}, 21}, {[]string{"A", "B"}, 2}} {
		out, _, err := statusRun(t, context.Background(), tc.args...)
		if out != "" || diag.ExitCode(err) != tc.code {
			t.Fatalf("%q %v", out, err)
		}
	}
	os.WriteFile(filepath.Join(a, "work-state.json"), []byte("broken"), 0o644)
	out, _, err = statusRun(t, context.Background())
	if out != "" || diag.ExitCode(err) != 25 {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, _, err = statusRun(t, ctx)
	if out != "" || diag.ExitCode(err) != 20 {
		t.Fatal(out, err)
	}
}

func TestStatusExplicitInvalidAndDuplicateTargets(t *testing.T) {
	ws := t.TempDir()
	a := filepath.Join(ws, "in-progress", "a")
	b := filepath.Join(ws, "in-progress", "b")
	statusSnapshot(t, a, "A", false)
	statusSnapshot(t, b, "B", false)
	home := workhome.At(t.TempDir())
	t.Setenv("WORK_HOME", home.Root())
	cfg := config.Default()
	cfg.Workspace = ws
	config.Save(home.ConfigFile(), cfg)
	t.Chdir(a)
	os.WriteFile(filepath.Join(b, "work-state.json"), []byte(`{"schema":3,"work":{"id":"B"}}`), 0o644)
	out, _, err := statusRun(t, context.Background(), "B")
	if out != "" || diag.ExitCode(err) != 25 {
		t.Fatal(out, err)
	}
	statusSnapshot(t, b, "A", false)
	out, _, err = statusRun(t, context.Background(), "A")
	if out != "" || diag.ExitCode(err) != 25 {
		t.Fatal(out, err)
	}
	out, _, err = statusRun(t, context.Background(), "--unsupported")
	if out != "" || diag.ExitCode(err) != 2 {
		t.Fatal(out, err)
	}
}
