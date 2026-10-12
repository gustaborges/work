package contract

import (
	"bytes"
	"encoding/json"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualImportProtocolAndResults(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "work"+exeSuffix())
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	home := workhome.At(filepath.Join(t.TempDir(), "home"))
	t.Setenv("WORK_HOME", home.Root())
	if err := home.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{}
	if _, err := plugininstall.Install(home.PluginsDir(), reg, fixtures.Prepare(t, "manual-suite"), plugininstall.Options{Alias: "manual"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(home.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"ok", "omitted", "empty", "exit1", "garbage", "overflow", "null", "mixed", "worktree", "snapshot", "apply", "launch"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "in-progress", "demo")
			os.MkdirAll(dir, 0755)
			st := &work.State{Work: work.WorkSection{ID: "opaque-id", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "branch", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}, Links: map[string]string{"github.pull_request": "private reference"}, Meta: map[string]any{"plugin.manual-suite.nullable": nil, "unused": "hidden"}}
			path := filepath.Join(dir, "work-state.json")
			if mode == "omitted" {
				delete(st.Meta, "plugin.manual-suite.nullable")
			}
			work.Write(path, st)
			before, _ := os.ReadFile(path)
			log := filepath.Join(t.TempDir(), "launches")
			t.Setenv("WORK_FIXTURE_LOG", log)
			fixtureMode := mode
			if mode == "omitted" {
				fixtureMode = "ok"
			}
			t.Setenv("WORK_FIXTURE_MODE", "importer-"+fixtureMode)
			want := 0
			count := "4"
			switch mode {
			case "empty":
				count = "0"
			case "exit1":
				want = 45
			case "garbage", "overflow", "null":
				want = 46
			case "mixed", "worktree", "snapshot":
				want = 47
			case "apply":
				want = 48
				t.Setenv("WORK_FIXTURE_MODE", "")
				t.Setenv("WORK_FAIL_AT", "incorporate:1")
			case "launch":
				want = 44
				t.Setenv("WORK_FIXTURE_MODE", "")
				c, _ := findImport(reg)
				c.Runtime = "missing-runtime"
				saved := reg.Components
				reg.Components = []registry.Component{c}
				registry.Save(home.RegistryFile(), reg)
				t.Cleanup(func() { reg.Components = saved; registry.Save(home.RegistryFile(), reg) })
			}
			cmd := exec.Command(bin, "import", "manual/importer")
			cmd.Dir = dir
			var out, ui bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &ui
			err := cmd.Run()
			code := 0
			if e, ok := err.(*exec.ExitError); ok {
				code = e.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != want {
				t.Fatal(code, want, ui.String())
			}
			if want == 0 {
				if out.String() != "work: imported opaque-id manual/importer "+count+"\n" {
					t.Fatal(out.String())
				}
			} else if out.Len() != 0 || strings.Count(ui.String(), "error:") != 1 {
				t.Fatal(out.String(), ui.String())
			}
			if strings.Contains(ui.String(), "private reference") || strings.Contains(ui.String(), "work-import-") {
				t.Fatal(ui.String())
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("snapshot changed")
			}
			if mode == "launch" {
				return
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
			if len(lines) != 1 {
				t.Fatal(string(data))
			}
			var launch struct {
				Role  string
				Stdin struct {
					Inputs    map[string]any `json:"inputs"`
					OutputDir string         `json:"output_dir"`
				}
			}
			if err := json.Unmarshal(lines[0], &launch); err != nil {
				t.Fatal(err)
			}
			inputs := launch.Stdin.Inputs
			inputCount := 3
			if mode == "omitted" {
				inputCount = 2
			}
			if launch.Role != "importer" || len(inputs) != inputCount || inputs["github.pull_request"] != "private reference" || inputs["start_mode"] != "contribution" {
				t.Fatal(string(data))
			}
			if v, ok := inputs["plugin.manual-suite.nullable"]; (mode != "omitted" && !ok) || (mode == "omitted" && ok) || v != nil {
				t.Fatal(inputs)
			}
			if _, err := os.Stat(launch.Stdin.OutputDir); !os.IsNotExist(err) {
				t.Fatal("stage remains", err)
			}
		})
	}
}
func findImport(reg *registry.Registry) (registry.Component, bool) {
	for _, c := range reg.Components {
		if c.Name == "importer" {
			return c, true
		}
	}
	return registry.Component{}, false
}
