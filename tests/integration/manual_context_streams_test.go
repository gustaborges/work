package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/seed"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// f6Harness uses only product commands to prepare its Work and saved context.
// It is portable: the Unix PTY harness supplements these redirected streams.
type f6Harness struct {
	bin, root, dir, id, log string
	env                     []string
}

func newF6Harness(t *testing.T) *f6Harness {
	t.Helper()
	if _, _, err := seed.HostAssets(); err != nil {
		t.Fatalf("run make seed: %v", err)
	}
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	h := &f6Harness{root: root, bin: filepath.Join(root, "work"+suffix), log: filepath.Join(root, "launches.jsonl")}
	if b, err := exec.Command("go", "build", "-o", h.bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatal(err, string(b))
	}
	h.env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "USERPROFILE=" + root, "WORK_HOME=" + filepath.Join(root, "home"), "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "WORK_FIXTURE_LOG=" + h.log, "TERM=xterm-256color"}
	// Windows needs SystemRoot for subprocess startup.
	if v := os.Getenv("SystemRoot"); v != "" {
		h.env = append(h.env, "SystemRoot="+v)
	}
	repo := filepath.Join(root, "demo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = h.env
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(b))
		}
	}
	h.run(t, root, 0, "start", repo, "--workspace", filepath.Join(root, "ws"), "--base", "main", "--slug", "topic", "--prefix", "{slug}", "--yes")
	h.dir = filepath.Join(root, "ws", "in-progress", "demo_topic")
	b, _ := os.ReadFile(filepath.Join(h.dir, "work-state.json"))
	var state struct {
		Work struct {
			ID string `json:"id"`
		} `json:"work"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	h.id = state.Work.ID
	h.run(t, root, 0, "plugin", "install", fixtures.Prepare(t, "manual-suite"), "--as", "manual")
	return h
}
func (h *f6Harness) run(t *testing.T, dir string, want int, args ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(h.bin, args...)
	cmd.Dir = dir
	cmd.Env = h.env
	var out, ui bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &ui
	code := 0
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	if code != want {
		t.Fatalf("%v exit %d want %d\nstdout %s\nstderr %s", args, code, want, out.String(), ui.String())
	}
	return out.String(), ui.String()
}
func TestManualContextStreams(t *testing.T) {
	h := newF6Harness(t)
	value := "private-reference\n\t\x1b[31m exact "
	for _, mode := range []string{"redirected", "no-color", "dumb"} {
		t.Run(mode, func(t *testing.T) {
			saved := h.env
			defer func() { h.env = saved }()
			if mode == "no-color" {
				h.env = append(h.env, "NO_COLOR=1")
			}
			if mode == "dumb" {
				h.env = append(h.env, "TERM=dumb")
			}
			out, ui := h.run(t, h.dir, 0, "link", "manual/linker", value)
			if out != "work: linked "+h.id+" github.pull_request manual/linker\n" || ui != "" {
				t.Fatal(out, ui)
			}
			out, ui = h.run(t, h.dir, 0, "status")
			if ui != "" || strings.ContainsAny(out, "\x1b\t") || strings.Count(out, "work: link ") != 1 {
				t.Fatal(out, ui)
			}
			out, ui = h.run(t, h.dir, 0, "status", "--json")
			var view struct {
				Links map[string]string `json:"links"`
			}
			if err := json.Unmarshal([]byte(out), &view); err != nil || view.Links["github.pull_request"] != value || ui != "" {
				t.Fatal(out, ui, err)
			}
			h.env = append(h.env, "WORK_FIXTURE_MODE=importer-empty")
			out, ui = h.run(t, h.dir, 0, "import", "manual/importer")
			if out != "work: imported "+h.id+" manual/importer 0\n" || ui != "work: running manual/importer (import)\n" {
				t.Fatal(out, ui)
			}
			h.env = append(h.env, "WORK_FIXTURE_MODE=importer-exit1")
			out, ui = h.run(t, h.dir, 45, "import", "manual/importer")
			if out != "" || strings.Count(ui, "error: extension-failed:") != 1 || !strings.Contains(ui, h.id) || strings.Contains(ui, "simulated importer failure") || strings.Contains(ui, "private-reference") || strings.ContainsAny(ui, "\x1b\t") {
				t.Fatal(out, ui)
			}
			h.env = append(h.env, "WORK_DEBUG=1")
			out, ui = h.run(t, h.dir, 45, "import", "manual/importer")
			if out != "" || !strings.Contains(ui, "simulated importer failure") || strings.Count(ui, "error: extension-failed:") != 1 {
				t.Fatal(out, ui)
			}
		})
	}
}
