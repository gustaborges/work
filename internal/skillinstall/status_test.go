package skillinstall

import (
	"encoding/json"
	skills "github.com/gustaborges/work/ai-skills"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusStateMatrix(t *testing.T) {
	for _, state := range []string{"absent", "foreign", "modified", "current", "outdated", "recovery_required", "conflict"} {
		t.Run(state, func(t *testing.T) {
			ds, err := resolveAt(Options{Scope: "local", Vendor: "codex"}, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			d := ds[0]
			embedded := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			if state != "absent" {
				if err = os.MkdirAll(d.Path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if state == "conflict" {
				if err = os.Remove(d.Path); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(d.Path, []byte("file"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if state == "current" || state == "outdated" || state == "modified" {
				actual, err := skills.TreeDigest(os.DirFS(d.Path))
				if err != nil {
					t.Fatal(err)
				}
				m := Marker{1, d.Skill, skills.Algorithm, actual}
				b, _ := json.Marshal(m)
				if err = os.WriteFile(filepath.Join(d.Path, MarkerName), b, 0644); err != nil {
					t.Fatal(err)
				}
				if state == "current" {
					embedded = actual
				}
				if state == "modified" {
					if err = os.WriteFile(filepath.Join(d.Path, "edit"), []byte("local"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if state == "recovery_required" {
				if err = os.WriteFile(sibling(d, ".transaction.json"), []byte("invalid"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			beforeExists, before, _ := treeEvidence(d.Path)
			r := Inspect(d, embedded)
			if r.State != state {
				t.Fatal(r)
			}
			afterExists, after, _ := treeEvidence(d.Path)
			if beforeExists != afterExists || before != after {
				t.Fatal("status changed tree")
			}
			if _, err = os.Stat(sibling(d, ".lock")); !os.IsNotExist(err) {
				t.Fatal("status created lock")
			}
		})
	}
}
