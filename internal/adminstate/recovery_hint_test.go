package adminstate

import "testing"

func TestRecoveryCommandNamesOnlyCommandsTheUserRan(t *testing.T) {
	const generic = "the interrupted administrative command"
	for operation, want := range map[string]string{
		"work plugin marketplace add ./x":      "work plugin marketplace add",
		"work plugin marketplace upgrade":      "work plugin marketplace upgrade",
		"work plugin marketplace evil":         generic,
		"work plugin marketplace":              generic,
		"work plugin install a":                generic,
		"work plugin update a --yes":           "work plugin update",
		"work plugin marketplace remove x":     "work plugin marketplace remove",
		"work plugin enable a b":               "work plugin enable",
		"work plugin disable a":                "work plugin disable",
		"work plugin remove a@m":               "work plugin remove",
		"work plugin add a@m":                  "work plugin add",
		"work repository policy add":           "work repository policy",
		"work repository policy move":          "work repository policy",
		"work repository policy remove":        "work repository policy",
		"work repository policy replace":       "work repository policy",
		"work repository root add":             "work repository root",
		"work repository root remove":          "work repository root",
		"work repository root replace":         "work repository root",
		"work convention set":                  "work convention set",
		"work start":                           generic,
		"work (automatic setup)":               generic,
		"":                                     generic,
		"other plugin enable":                  generic,
		"work plugin enable-all --token=s3cr3": generic,
	} {
		if got := recoveryCommand(operation); got != want {
			t.Errorf("%q: got %q, want %q", operation, got, want)
		}
	}
}
