package semconv

import (
	"slices"
	"testing"
)

func TestValidKey(t *testing.T) {
	valid := []string{
		"github.pull_request",
		"github.pull_request.number",
		"a1.b2",
		"plugin.acme-tools.build_id",
		"plugin.acme.x.y",
	}
	for _, k := range valid {
		if err := ValidKey(k); err != nil {
			t.Errorf("ValidKey(%q) = %v, want nil", k, err)
		}
	}

	invalid := map[string]string{
		"":                      "empty",
		"github":                "single segment",
		"GitHub.pr":             "uppercase",
		"github.Pull":           "uppercase segment",
		"github.pull-request":   "hyphen in public key",
		"github..pr":            "empty segment",
		".github.pr":            "leading dot",
		"github.pr.":            "trailing dot",
		"github.1pr":            "segment starting with a digit",
		"plugin.x":              "plugin. prefix without a local part",
		"plugin.":               "plugin. prefix with nothing",
		"plugin.acme.":          "empty local",
		"plugin.acme..x":        "empty local segment",
		"work.slug":             "reserved work namespace",
		"work.anything.else":    "reserved work namespace, deep",
		"plugin.acme.Bad":       "uppercase local",
		"github.pull request":   "space",
		"github.pull_request:x": "colon",
	}
	for k, why := range invalid {
		if err := ValidKey(k); err == nil {
			t.Errorf("ValidKey(%q) = nil, want error (%s)", k, why)
		}
	}
}

func TestValidatePublished(t *testing.T) {
	tests := []struct {
		name  string
		owner string
		key   string
		ok    bool
	}{
		{"public key, any owner", "acme", "github.pull_request", true},
		{"own private key", "acme-tools", "plugin.acme-tools.build_id", true},
		{"own private key with deep local", "acme", "plugin.acme.a.b", true},
		{"foreign private key", "acme", "plugin.other.x", false},
		{"private key of a prefix of the owner", "acme", "plugin.acme-tools.x", false},
		{"dotted owner cannot own private keys", "foo.bar", "plugin.foo.bar.x", false},
		{"dotted owner may publish public keys", "foo.bar", "github.pull_request", true},
		{"invalid grammar", "acme", "GitHub.pr", false},
		{"reserved namespace", "acme", "work.slug", false},
		{"empty owner cannot own private keys", "", "plugin.acme.x", false},
	}
	for _, tc := range tests {
		err := ValidatePublished(tc.owner, tc.key)
		if (err == nil) != tc.ok {
			t.Errorf("%s: ValidatePublished(%q, %q) = %v, ok want %v", tc.name, tc.owner, tc.key, err, tc.ok)
		}
	}
}

func TestValidLinkValue(t *testing.T) {
	if err := ValidLinkValue("https://example.test/pr/1"); err != nil {
		t.Errorf("non-empty value: %v", err)
	}
	if err := ValidLinkValue(" "); err != nil {
		t.Errorf("values are stored as received, whitespace-only is non-empty: %v", err)
	}
	if err := ValidLinkValue(""); err == nil {
		t.Error("empty value accepted")
	}
}

func TestFacts(t *testing.T) {
	want := []string{"worktree_path", "start_mode", "branch", "base_branch", "slug"}
	if !slices.Equal(Facts, want) {
		t.Errorf("Facts = %v, want %v", Facts, want)
	}
}
