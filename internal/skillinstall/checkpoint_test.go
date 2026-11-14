package skillinstall

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Discover the actual publication checkpoint sequence, then interrupt each
// boundary in a fresh operation for both initially absent and existing trees.
func TestEveryPublicationCheckpoint(t *testing.T) {
	for _, existing := range []bool{false, true} {
		setup := func(t *testing.T) []Destination {
			t.Helper()
			ds, err := resolveAt(Options{Scope: "local", Vendor: "codex"}, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				if err = os.MkdirAll(ds[0].Path, 0755); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(ds[0].Path, "old"), []byte("old bytes"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			return ds
		}
		checkpoints := []string{}
		e := Engine{Checkpoint: func(p string) error {
			if !strings.HasPrefix(p, "removed:") && p != "record-removed" {
				checkpoints = append(checkpoints, p)
			}
			return nil
		}}
		e.Install(context.Background(), setup(t), true)
		for _, point := range checkpoints {
			t.Run(point, func(t *testing.T) {
				ds := setup(t)
				d := ds[0]
				oldExists, old, err := treeEvidence(d.Path)
				if err != nil {
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
					t.Fatal("fault did not fail", point)
				}
				if err = recoverTarget(d); err != nil {
					t.Fatal(err)
				}
				exists, fp, err := treeEvidence(d.Path)
				if err != nil {
					t.Fatal(err)
				}
				if point != "committed" && (exists != oldExists || fp != old) {
					t.Fatalf("lost precommit state at %s", point)
				}
				if result := Install(context.Background(), ds, true); result[0].Error != "" {
					t.Fatal(result)
				}
			})
		}
	}
}
