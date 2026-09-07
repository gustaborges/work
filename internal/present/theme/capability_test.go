package theme

import (
	"os"
	"testing"
)

// envFunc builds a lookupEnv closure over a fixed map for compute's table.
func envFunc(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestComputeColorAndInteractive(t *testing.T) {
	tests := []struct {
		name            string
		uiTTY, inTTY    bool
		env             map[string]string
		wantColor       bool
		wantInteractive bool
	}{
		{"tty, no env", true, true, nil, true, true},
		{"non-tty ui", false, true, nil, false, false},
		{"tty ui, non-tty in", true, false, nil, true, false},
		{"NO_COLOR=1 disables", true, true, map[string]string{"NO_COLOR": "1"}, false, true},
		{"NO_COLOR=0 disables (non-empty)", true, true, map[string]string{"NO_COLOR": "0"}, false, true},
		{"NO_COLOR set empty does not disable", true, true, map[string]string{"NO_COLOR": ""}, true, true},
		{"TERM=dumb disables", true, true, map[string]string{"TERM": "dumb"}, false, true},
		{"TERM=xterm-256color keeps colour", true, true, map[string]string{"TERM": "xterm-256color"}, true, true},
		{"non-tty ui ignores empty NO_COLOR", false, true, map[string]string{"NO_COLOR": ""}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := compute(tt.uiTTY, tt.inTTY, envFunc(tt.env))
			if c.ColorEnabled != tt.wantColor {
				t.Errorf("ColorEnabled = %v, want %v", c.ColorEnabled, tt.wantColor)
			}
			if c.Interactive != tt.wantInteractive {
				t.Errorf("Interactive = %v, want %v", c.Interactive, tt.wantInteractive)
			}
			if c.AsciiMarks {
				t.Errorf("AsciiMarks = true, want false by default")
			}
		})
	}
}

func TestDetectWithPipesIsNonInteractive(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	c := Detect(w, r)
	if c.ColorEnabled {
		t.Errorf("ColorEnabled = true for a pipe, want false")
	}
	if c.Interactive {
		t.Errorf("Interactive = true for pipes, want false")
	}
}

func TestDetectNonFileStreamsAreNonInteractive(t *testing.T) {
	c := Detect(newDiscardWriter(), newEmptyReader())
	if c.ColorEnabled || c.Interactive {
		t.Errorf("non-file streams should not be a terminal: %+v", c)
	}
}

type discardWriter struct{}

func newDiscardWriter() *discardWriter             { return &discardWriter{} }
func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }

type emptyReader struct{}

func newEmptyReader() *emptyReader              { return &emptyReader{} }
func (*emptyReader) Read(p []byte) (int, error) { return 0, os.ErrClosed }
