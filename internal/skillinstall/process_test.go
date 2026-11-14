package skillinstall

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRecoveryHelperProcess(t *testing.T) {
	if os.Getenv("WORK_SKILL_TEST_HELPER") != "1" {
		return
	}
	ds, err := Resolve(Options{Dir: os.Getenv("WORK_SKILL_TEST_DIR"), Vendors: []string{"codex"}})
	if err != nil {
		os.Exit(92)
	}
	point := os.Getenv("WORK_SKILL_TEST_POINT")
	e := Engine{Checkpoint: func(p string) error {
		if p == point {
			os.Exit(91)
		}
		return nil
	}}
	e.Install(context.Background(), ds, true)
	os.Exit(93)
}
func TestProcessRecovery(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, point := range []string{"prepared", "old-moved", "new-moved", "committed"} {
			t.Run(point, func(t *testing.T) {
				root := t.TempDir()
				ds, err := Resolve(Options{Dir: root, Vendors: []string{"codex"}})
				if err != nil {
					t.Fatal(err)
				}
				d := ds[0]
				if existing {
					if err = os.MkdirAll(filepath.Join(d.Path, "empty"), 0755); err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(filepath.Join(d.Path, MarkerName), []byte("foreign marker bytes"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				oldExists, old, err := treeEvidence(d.Path)
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestRecoveryHelperProcess$")
				cmd.Env = append(os.Environ(), "WORK_SKILL_TEST_HELPER=1", "WORK_SKILL_TEST_DIR="+root, "WORK_SKILL_TEST_POINT="+point)
				err = cmd.Run()
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 91 {
					t.Fatal("helper did not terminate at boundary", err)
				}
				if err = recoverTarget(d); err != nil {
					t.Fatal(err)
				}
				exists, fp, err := treeEvidence(d.Path)
				if err != nil {
					t.Fatal(err)
				}
				if point != "committed" && (exists != oldExists || fp != old) {
					t.Fatal("process crash lost old tree")
				}
				if out := Install(context.Background(), ds, true); out[0].Error != "" {
					t.Fatal(out)
				}
			})
		}
	}
}
func TestInterruptedCompensation(t *testing.T) {
	for _, point := range []string{"old-restored", "removed:", "record-removed"} {
		t.Run(point, func(t *testing.T) {
			ds, _ := Resolve(Options{Dir: t.TempDir(), Vendors: []string{"codex"}})
			d := ds[0]
			if err := os.MkdirAll(d.Path, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(d.Path, "old"), []byte("exact"), 0644); err != nil {
				t.Fatal(err)
			}
			_, old, _ := treeEvidence(d.Path)
			e := Engine{Checkpoint: func(p string) error {
				if p == "new-moved" {
					return errInterrupted
				}
				return nil
			}}
			e.Install(context.Background(), ds, true)
			engine := Engine{Checkpoint: func(p string) error {
				if p == point || (point == "removed:" && len(p) > 8 && p[:8] == "removed:") {
					return errInterrupted
				}
				return nil
			}}
			_ = engine.recoverTarget(d)
			if err := recoverTarget(d); err != nil {
				t.Fatal(err)
			}
			_, fp, err := treeEvidence(d.Path)
			if err != nil || fp != old {
				t.Fatal("compensation lost old state", err)
			}
		})
	}
}
