package semconv

import (
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
)

const regA = "0123456789abcdef0123456789abcdef"
const regB = "fedcba9876543210fedcba9876543210"

func TestPrivateKeySpellings(t *testing.T) {
	good := map[string]PrivateKey{
		"plugin.github.build_id":                       {Plugin: "github", Local: "build_id"},
		"plugin.acme%2Etools.build_id":                 {Plugin: "acme.tools", Local: "build_id"},
		"plugin.github@team.build_id":                  {Plugin: "github", Marketplace: "team", Local: "build_id"},
		"plugin.acme%2Etools@team%2Edev.build_id":      {Plugin: "acme.tools", Marketplace: "team.dev", Local: "build_id"},
		"plugin.github@team~" + regA + ".build_id":     {Plugin: "github", Marketplace: "team", RegistrationID: regA, Local: "build_id"},
		"plugin.github@builtin~builtin.a.b":            {Plugin: "github", Marketplace: "builtin", RegistrationID: "builtin", Local: "a.b"},
		"plugin.acme.tools.build_id":                   {Plugin: "acme", Local: "tools.build_id"}, // the first dot ends the owner token
		"plugin.a-b_c.x":                               {Plugin: "a-b_c", Local: "x"},
		"plugin.github@team%2Edev~" + regB + ".x.y.z9": {Plugin: "github", Marketplace: "team.dev", RegistrationID: regB, Local: "x.y.z9"},
	}
	for key, want := range good {
		got, err := ParsePrivate(key)
		if err != nil {
			t.Errorf("ParsePrivate(%q): %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("ParsePrivate(%q) = %+v, want %+v", key, got, want)
		}
		if got.String() != key {
			t.Errorf("round trip of %q gave %q", key, got.String())
		}
	}
	bad := map[string]string{
		"plugin.acme%2etools.x":                              "lowercase escape",
		"plugin.acme%252Etools.x":                            "double encoding",
		"plugin.acme%2Ftools.x":                              "unknown escape",
		"plugin.%2Eacme.x":                                   "decoded name starts with a dot",
		"plugin.acme%2E.x":                                   "decoded name ends with a dot",
		"plugin.@team.x":                                     "empty plugin",
		"plugin.github@.x":                                   "empty marketplace",
		"plugin.github@team@other.x":                         "two @",
		"plugin.github@team~.x":                              "empty registration",
		"plugin.github@team~XYZ.x":                           "malformed registration",
		"plugin.github~" + regA + ".x":                       "registration without a marketplace",
		"plugin.github@team~" + regA:                         "no local name",
		"plugin.github@team.":                                "empty local",
		"plugin.github@team.Bad":                             "uppercase local",
		"plugin.github.":                                     "empty local relative",
		"plugin.github":                                      "no local",
		"plugin.":                                            "nothing",
		"plugin.git hub.x":                                   "space",
		"plugin.github@team/evil.x":                          "slash",
		"plugin.github@team%2Fevil.x":                        "escaped slash",
		"notplugin.github.x":                                 "wrong prefix",
		"plugin.github@team~" + regA + ".x.":                 "trailing dot",
		"plugin.github@team~" + regA + "..x":                 "empty local segment",
		"plugin.github@team~" + strings.ToUpper(regA) + ".x": "uppercase registration",
	}
	for key, why := range bad {
		if _, err := ParsePrivate(key); err == nil {
			t.Errorf("ParsePrivate(%q) accepted (%s)", key, why)
		}
	}
}

func TestStorageSpellingIsCoreOnly(t *testing.T) {
	stored := "plugin.github@team~" + regA + ".build_id"
	if err := ValidKey(stored); err == nil {
		t.Error("an author may not spell the storage form")
	}
	if err := ValidatePublished("github", stored); err == nil {
		t.Error("a plugin may not publish the storage form")
	}
	if err := ValidateOwnInput("github", stored); err == nil {
		t.Error("an input may not name the storage form")
	}
}

func TestPublicationOwnership(t *testing.T) {
	tests := []struct {
		name, owner, key string
		ok               bool
	}{
		{"own relative key", "github", "plugin.github.build_id", true},
		{"dotted owner publishes its encoded key", "acme.tools", "plugin.acme%2Etools.build_id", true},
		{"dotted owner cannot use the ambiguous spelling", "acme.tools", "plugin.acme.tools.build_id", false},
		{"owner acme cannot publish into acme.tools", "acme", "plugin.acme%2Etools.build_id", false},
		{"owner acme.tools cannot publish into acme", "acme.tools", "plugin.acme.build_id", false},
		{"qualified publication is rejected even for itself", "github", "plugin.github@team.build_id", false},
		{"qualified publication into another namespace", "github", "plugin.github@other.build_id", false},
		{"foreign relative key", "github", "plugin.other.build_id", false},
		{"public key", "github", "github.pull_request", true},
	}
	for _, tc := range tests {
		if err := ValidatePublished(tc.owner, tc.key); (err == nil) != tc.ok {
			t.Errorf("%s: ValidatePublished(%q, %q) = %v, ok want %v", tc.name, tc.owner, tc.key, err, tc.ok)
		}
	}
}

func TestInputOwnership(t *testing.T) {
	tests := []struct {
		name, declarer, key string
		ok                  bool
	}{
		{"own relative", "github", "plugin.github.build_id", true},
		{"foreign relative needs qualification", "github", "plugin.other.build_id", false},
		{"explicit qualified foreign", "github", "plugin.other@personal.build_id", true},
		{"explicit qualified self", "github", "plugin.github@personal.build_id", true},
		{"dotted owner own relative", "acme.tools", "plugin.acme%2Etools.x", true},
		{"dotted qualified", "github", "plugin.acme%2Etools@team%2Edev.x", true},
		{"public", "github", "github.pull_request", true},
	}
	for _, tc := range tests {
		if err := ValidateOwnInput(tc.declarer, tc.key); (err == nil) != tc.ok {
			t.Errorf("%s: ValidateOwnInput(%q, %q) = %v, ok want %v", tc.name, tc.declarer, tc.key, err, tc.ok)
		}
	}
}

func TestBindPublishedIsPerSourceInstance(t *testing.T) {
	alpha := pluginid.Provider{Marketplace: "alpha", Plugin: "sample"}
	beta := pluginid.Provider{Marketplace: "beta", Plugin: "sample"}
	a := BindPublished(alpha, regA, "plugin.sample.build_id")
	b := BindPublished(beta, regB, "plugin.sample.build_id")
	if a == b {
		t.Fatalf("homonymous providers share one stored key %q", a)
	}
	if again := BindPublished(alpha, regB, "plugin.sample.build_id"); again == a {
		t.Fatal("a re-registered source of the same names must not reuse the earlier instance's key")
	}
	if got := BindPublished(alpha, regA, "github.pull_request"); got != "github.pull_request" {
		t.Errorf("public keys are shared and unchanged, got %q", got)
	}
	k, err := ParsePrivate(a)
	if err != nil || !k.Stored() || k.Provider() != alpha || k.Local != "build_id" || k.RegistrationID != regA {
		t.Errorf("stored key %q parses to %+v, %v", a, k, err)
	}
	dotted := pluginid.Provider{Marketplace: "team.dev", Plugin: "acme.tools"}
	if got := BindPublished(dotted, regA, "plugin.acme%2Etools.x"); got != "plugin.acme%2Etools@team%2Edev~"+regA+".x" {
		t.Errorf("dotted provider stored key = %q", got)
	}
}
