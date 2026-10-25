package adminstate

import "testing"

func TestRecoveryCommandNamesOnlyCommandsTheUserRan(t *testing.T) {
	const generic = "the interrupted administrative command"
	for operation, want := range map[string]string{
		"work plugin install":                  "work plugin install",
		"work plugin install --embedded":       "work plugin install",
		"work plugin enable a b":               "work plugin enable",
		"work plugin disable a":                "work plugin disable",
		"work plugin update a":                 "work plugin update",
		"work plugin uninstall a":              "work plugin uninstall",
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
