//go:build unix

package staging

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestBuildRefusesAFifo(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(stage, "pipe"), 0o644); err != nil {
		t.Skipf("cannot create a FIFO: %v", err)
	}
	_, err := Build(stage, work)
	wantRefusal(t, err, "pipe", NotRegular)
}
