package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

func TestStatusOutputContract(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "work"+exeSuffix())
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	dir := filepath.Join(t.TempDir(), "in-progress", "work")
	os.MkdirAll(dir, 0o755)
	s := &work.State{Work: work.WorkSection{ID: "opaque-id", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "feature/topic", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}, Links: map[string]string{"z.last": "https://example.test/pr/212", "a.first": " \n\t\x1bexact "}}
	path := filepath.Join(dir, "work-state.json")
	if err := work.Write(path, s); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_HOME", filepath.Join(t.TempDir(), "missing"))
	for _, archived := range []bool{false, true} {
		if archived {
			s.Archive(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
			work.Write(path, s)
		}
		for _, machine := range []bool{false, true} {
			args := []string{"status"}
			if machine {
				args = append(args, "--json")
			}
			cmd := exec.Command(bin, args...)
			cmd.Dir = dir
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil || stderr.Len() != 0 {
				t.Fatal(err, stderr.String())
			}
			if machine {
				var v map[string]json.RawMessage
				if err := json.Unmarshal(out.Bytes(), &v); err != nil {
					t.Fatal(err)
				}
				if len(v) != 5 {
					t.Fatalf("%s", out.Bytes())
				}
				for _, key := range []string{"id", "status", "branch", "location", "links"} {
					if v[key] == nil {
						t.Fatal(key)
					}
				}
				var links map[string]string
				json.Unmarshal(v["links"], &links)
				if links["a.first"] != s.Links["a.first"] {
					t.Fatal("link changed")
				}
				var location string
				json.Unmarshal(v["location"], &location)
				want := dir
				if !archived {
					want = filepath.Join(dir, "worktree")
				}
				real, _ := filepath.EvalSymlinks(dir)
				want = strings.Replace(want, dir, real, 1)
				if location != want {
					t.Fatalf("%q != %q", location, want)
				}
			} else if strings.Index(out.String(), "a.first") > strings.Index(out.String(), "z.last") || strings.ContainsAny(out.String(), "\t\x1b") {
				t.Fatal(out.String())
			}
		}
	}
	s.Links = nil
	work.Write(path, s)
	cmd := exec.Command(bin, "status", "--json")
	cmd.Dir = dir
	if b, err := cmd.Output(); err != nil || !bytes.Contains(b, []byte(`"links":{}`)) {
		t.Fatal(string(b), err)
	}
	os.WriteFile(path, []byte("broken"), 0o600)
	cmd = exec.Command(bin, "status", "--json")
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil || out.Len() != 0 || strings.Count(stderr.String(), "error:") != 1 {
		t.Fatal(err, out.String(), stderr.String())
	}
}

// A Linker declaration is sufficient: association never needs its runtime or
// entrypoint, nor its deliberately missing discovery inputs.
func TestLinkOutputAndNoExecutableContract(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "work"+exeSuffix())
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	home := workhome.At(filepath.Join(t.TempDir(), "home"))
	t.Setenv("WORK_HOME", home.Root())
	reg := &registry.Registry{Components: []registry.Component{{Alias: "manual", Name: "linker", Role: plugin.RoleLinker, Key: "github.pull_request", Entrypoint: "absent", Runtime: "missing-runtime", Inputs: []string{"meta:missing.fact"}, Manual: &plugin.Manual{DisplayName: "Pull request"}}}}
	if err := registry.Save(home.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "in-progress", "work")
	os.MkdirAll(dir, 0755)
	state := &work.State{Work: work.WorkSection{ID: "opaque-id", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "branch", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}, Links: map[string]string{"github.pull_request": "starter"}, Meta: map[string]any{"test.number": float64(7)}}
	path := filepath.Join(dir, "work-state.json")
	if err := work.Write(path, state); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{" exact $shell; text ", "--leading-dash", "\n\tcontrol\x1b"} {
		cmd := exec.Command(bin, "link", "manual/linker", "--", value)
		cmd.Dir = dir
		var out, ui bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &ui
		if err := cmd.Run(); err != nil || ui.Len() != 0 || out.String() != "work: linked opaque-id github.pull_request manual/linker\n" {
			t.Fatal(err, out.String(), ui.String())
		}
		actual, err := work.Read(path)
		if err != nil || actual.Links["github.pull_request"] != value || actual.Work != state.Work || actual.Meta["test.number"] != float64(7) {
			t.Fatal(actual, err)
		}
	}
	before, _ := os.ReadFile(path)
	for _, args := range [][]string{{"link"}, {"link", "manual/linker"}, {"link", "manual/linker", ""}, {"link", "manual/linker", "value", "--json=false"}, {"link", "manual/linker", "value", "--yes"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		var out, ui bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &ui
		err := cmd.Run()
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 2 || out.Len() != 0 || strings.Count(ui.String(), "error:") != 1 {
			t.Fatal(args, err, out.String(), ui.String())
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid arguments changed canonical state")
		}
	}
}
