package skillinstall

import (
	"context"
	skills "github.com/gustaborges/work/ai-skills"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyOwnership(t *testing.T) {
	ds, err := testTargets(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := Install(context.Background(), ds, false)
	if len(out) != 2 || out[0].Action != "installed" || out[1].Action != "installed" {
		t.Fatal(out)
	}
	digest, _ := skills.Digest(ds[0].Skill)
	if r := Inspect(ds[0], digest); r.State != "current" {
		t.Fatal(r)
	}
	info, _ := os.Stat(filepath.Join(ds[0].Path, "SKILL.md"))
	out = Install(context.Background(), ds, false)
	after, _ := os.Stat(filepath.Join(ds[0].Path, "SKILL.md"))
	if out[0].Action != "unchanged" || !after.ModTime().Equal(info.ModTime()) {
		t.Fatal(out)
	}
	if err = os.WriteFile(filepath.Join(ds[1].Path, "local"), []byte("edit"), 0644); err != nil {
		t.Fatal(err)
	}
	out = Install(context.Background(), ds, false)
	if out[0].Action != "not-attempted" || out[1].Action != "failed" {
		t.Fatal(out)
	}
	out = Install(context.Background(), ds, true)
	if out[1].Action != "updated" {
		t.Fatal(out)
	}
}
func TestApplyRecovery(t *testing.T) {
	for _, point := range []string{"prepared", "old-moved", "new-moved", "committed"} {
		t.Run(point, func(t *testing.T) {
			ds, _ := resolveAt(Options{Scope: "local", Vendor: "codex"}, t.TempDir())
			d := ds[0]
			if err := os.MkdirAll(d.Path, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(d.Path, "old"), []byte("exact old"), 0644); err != nil {
				t.Fatal(err)
			}
			engine := Engine{Checkpoint: func(p string) error {
				if p == point {
					return errInterrupted
				}
				return nil
			}}
			out := engine.Install(context.Background(), ds, true)
			if out[0].Error == "" {
				t.Fatal(out)
			}
			// Explicit recovery restores precommit content, retains committed content.
			if err := recoverTarget(d); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(filepath.Join(d.Path, "old"))
			if point == "committed" {
				if !os.IsNotExist(err) {
					t.Fatal("lost committed content")
				}
			} else if err != nil {
				t.Fatal("lost old content", err)
			}
		})
	}
}
