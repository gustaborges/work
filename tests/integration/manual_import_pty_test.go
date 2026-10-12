//go:build unix

package integration

import (
	"encoding/json"
	"github.com/creack/pty"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestManualImportJourney(t *testing.T) {
	needSeed(t)
	bin := buildWorkBin(t)
	for _, mode := range []string{"omitted", "explicit", "cancel-selection", "interrupt"} {
		t.Run(mode, func(t *testing.T) {
			env, home, _ := ptyEnv(t)
			repo := filepath.Join(home, "demo")
			makeRepo(t, repo)
			ws := filepath.Join(home, "ws")
			seedWorks(t, bin, env, repo, ws, "topic")
			installPluginFixture(t, bin, env, "manual-suite")
			dir := filepath.Join(ws, "in-progress", "demo_topic")
			cmd := exec.Command(bin, "link", "manual-suite/linker", "saved reference")
			cmd.Env = env
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatal(err, string(out))
			}
			before, _ := os.ReadFile(filepath.Join(dir, "work-state.json"))
			args := []string{"import"}
			if mode == "explicit" || mode == "interrupt" {
				args = append(args, "manual-suite/importer")
			}
			log := filepath.Join(home, "launches.jsonl")
			env = append(env, "WORK_FIXTURE_LOG="+log)
			var ready chan error
			if mode == "interrupt" {
				fifo := filepath.Join(home, "ready")
				if err := syscall.Mkfifo(fifo, 0600); err != nil {
					t.Fatal(err)
				}
				env = append(env, "WORK_FIXTURE_MODE=importer-hang", "WORK_FIXTURE_READY="+fifo)
				ready = make(chan error, 1)
				go func() {
					f, err := os.Open(fifo)
					if err == nil {
						_, err = io.ReadAll(f)
						f.Close()
					}
					ready <- err
				}()
			}
			cmd = exec.Command(bin, args...)
			cmd.Env = env
			cmd.Dir = dir
			c := startConsole(t, pty.Winsize{Rows: 24, Cols: 80}, cmd)
			if mode == "omitted" || mode == "cancel-selection" {
				c.expect("Choose an Importer")
				c.expect("manual-suite/importer")
				if strings.Contains(c.screen(), "manual-suite/needs-input") || strings.Contains(c.screen(), "manual-suite/automatic-only") {
					t.Fatal(c.screen())
				}
				if mode == "cancel-selection" {
					c.send("\x1b")
				} else {
					c.send("\r")
				}
			}
			if mode == "interrupt" {
				if err := <-ready; err != nil {
					t.Fatal(err)
				}
				c.send("\x03")
			}
			code := c.wait()
			want := 0
			if mode == "cancel-selection" || mode == "interrupt" {
				want = 20
			}
			if code != want {
				t.Fatal(code, c.screen())
			}
			after, _ := os.ReadFile(filepath.Join(dir, "work-state.json"))
			if string(before) != string(after) {
				t.Fatal("snapshot changed")
			}
			if want == 0 {
				if !strings.Contains(c.screen(), "work: imported ") || !strings.Contains(c.screen(), "manual-suite/importer 4") {
					t.Fatal(c.screen())
				}
			} else {
				assertOneCancelNotice(t, c.screen())
				if _, err := os.Stat(filepath.Join(dir, "notes")); !os.IsNotExist(err) {
					t.Fatal("partial output remains")
				}
			}
			if mode != "cancel-selection" {
				data, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				var launch struct {
					Stdin struct {
						OutputDir string `json:"output_dir"`
					}
				}
				if err := json.Unmarshal(data, &launch); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(launch.Stdin.OutputDir); !os.IsNotExist(err) {
					t.Fatal("stage remains", err)
				}
			}
		})
	}
}
