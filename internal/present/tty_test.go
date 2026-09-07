package present

import (
	"os"
	"testing"
)

func TestIsInteractiveWithPipes(t *testing.T) {
	// A pipe is never a terminal; the exported IsInteractive keys off the real
	// process streams, so exercise the underlying predicate directly.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if isTTY(r) || isTTY(w) {
		t.Errorf("isTTY(pipe) = true, want false")
	}
}
