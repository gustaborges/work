package skillinstall

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryPreservesMismatchedBackup(t *testing.T) {
	ds, _ := Resolve(Options{Dir: t.TempDir(), Vendors: []string{"codex"}})
	d := ds[0]
	if err := os.MkdirAll(d.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.Path, "old"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	e := Engine{Checkpoint: func(p string) error {
		if p == "new-moved" {
			return errInterrupted
		}
		return nil
	}}
	e.Install(context.Background(), ds, true)
	r, err := loadRecord(d)
	if err != nil {
		t.Fatal(err)
	}
	_, backup := recordPaths(d, r)
	if err = os.WriteFile(filepath.Join(backup, "old"), []byte("external edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if recoverTarget(d) == nil {
		t.Fatal("recovered mismatched backup")
	}
	b, err := os.ReadFile(filepath.Join(backup, "old"))
	if err != nil || string(b) != "external edit" {
		t.Fatal("deleted evidence")
	}
}
func TestRecoveryRejectsInjectedPaths(t *testing.T) {
	ds, _ := Resolve(Options{Dir: t.TempDir(), Vendors: []string{"codex"}})
	d := ds[0]
	if err := os.MkdirAll(filepath.Dir(d.Path), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"schema":1,"operation":"0123456789abcdef0123456789abcdef","skill":"work-plugin-builder","stage":"../../outside","backup":"../../outside","phase":"prepared"}`)
	p := sibling(d, ".transaction.json")
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	if recoverTarget(d) == nil {
		t.Fatal("accepted injected paths")
	}
	got, _ := os.ReadFile(p)
	if string(got) != string(data) {
		t.Fatal("changed invalid record")
	}
}
func TestChangedObservationRefusedWithForce(t *testing.T) {
	ds, _ := Resolve(Options{Dir: t.TempDir(), Vendors: []string{"codex"}})
	d := ds[0]
	Install(context.Background(), ds, false)
	if err := os.WriteFile(filepath.Join(d.Path, "edit"), []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	e := Engine{Checkpoint: func(p string) error {
		if p == "stage" {
			return os.WriteFile(filepath.Join(d.Path, "edit"), []byte("second"), 0644)
		}
		return nil
	}}
	out := e.Install(context.Background(), ds, true)
	if out[0].Error != "skill-target-conflict" {
		t.Fatal(out)
	}
	b, _ := os.ReadFile(filepath.Join(d.Path, "edit"))
	if string(b) != "second" {
		t.Fatal("overwrote edit")
	}
}
func TestLaterFailureRetainsEarlierCommit(t *testing.T) {
	ds, _ := Resolve(Options{Dir: t.TempDir()})
	calls := 0
	e := Engine{Checkpoint: func(p string) error {
		if p == "prepared" {
			calls++
			if calls == 2 {
				return errInterrupted
			}
		}
		return nil
	}}
	out := e.Install(context.Background(), ds, false)
	if out[0].Commit != "committed" || out[1].Error != "skill-recovery-required" {
		t.Fatal(out)
	}
}
