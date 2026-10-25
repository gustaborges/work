//go:build unix

package plugininstall

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPrepareLinkedSourceToleratesSpecialFiles(t *testing.T) {
	dir := prepareSource(t)
	if err := syscall.Mkfifo(filepath.Join(dir, "dev.fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), dir, Options{Link: true})
	if err != nil {
		t.Fatalf("linked developer tree with a FIFO must stay installable: %v", err)
	}
	defer p.Close()
	if p.Digest == "" {
		t.Fatal("linked baseline digest is empty")
	}
	if q, err := Prepare(context.Background(), dir, Options{}); err == nil {
		q.Close()
		t.Fatal("a pinned copy must still reject special files")
	}
}
