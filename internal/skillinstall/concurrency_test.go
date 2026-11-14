package skillinstall

import (
	"context"
	skills "github.com/gustaborges/work/ai-skills"
	"testing"
)

func TestConcurrentInstallStatusAndCancellation(t *testing.T) {
	ds, _ := Resolve(Options{Dir: t.TempDir(), Vendors: []string{"codex"}})
	prepared := make(chan struct{})
	resume := make(chan struct{})
	finished := make(chan []Outcome, 1)
	e := Engine{Checkpoint: func(p string) error {
		if p == "prepared" {
			close(prepared)
			<-resume
		}
		return nil
	}}
	go func() { finished <- e.Install(context.Background(), ds, false) }()
	<-prepared
	digest, _ := skills.Digest(ds[0].Skill)
	r := Inspect(ds[0], digest)
	if r.State != "recovery_required" {
		close(resume)
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := Install(ctx, ds, false)
	if out[0].Error != "cancelled" {
		close(resume)
		t.Fatal(out)
	}
	close(resume)
	out = <-finished
	if out[0].Action != "installed" {
		t.Fatal(out)
	}
	out = Install(context.Background(), ds, false)
	if out[0].Action != "unchanged" {
		t.Fatal(out)
	}
}
