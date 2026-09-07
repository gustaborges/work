package present

import (
	"strings"
	"testing"
)

// A completed Secret input renders only the redaction mark: the exact receipt
// is "<title>\n  ✔ ••••\n\n" and no substring of the typed value survives into
// any View the caller can print (FR-004).
func TestInputSecretCompletedViewHasNoPlaintext(t *testing.T) {
	const secret = "XY9-hunter2-p4ss"
	m := typeText(newTestInput(InputSpec{Title: "Token", Secret: true}), secret).(inputModel)

	// Editing frame: masked, never the plaintext.
	if strings.Contains(stepBody(m), secret) {
		t.Fatalf("plaintext visible while editing:\n%s", stepBody(m))
	}

	m = submitInput(t, m)
	got := m.status().receipt
	if got != "Token\n  ✔ ••••\n\n" {
		t.Errorf("completed secret View = %q, want the redacted receipt", got)
	}
	// No run of the typed value (length 4+) may appear anywhere in the frame.
	for n := 4; n <= len(secret); n++ {
		for i := 0; i+n <= len(secret); i++ {
			if sub := secret[i : i+n]; strings.Contains(got, sub) {
				t.Errorf("secret substring %q leaked into the receipt:\n%s", sub, got)
			}
		}
	}
}
