package pluginid

import "testing"

func TestValidateName(t *testing.T) {
	good := []string{"a", "A1", "team", "acme.tools", "my_plugin-2", "x.y.z", "builtin", strings64()}
	for _, n := range good {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want ok", n, err)
		}
	}
	bad := []string{
		"", ".hidden", "-x", "_x", "a/b", `a\b`, "a@b", "a:b", "a%b", "a b", "a\x00b", "a\nb",
		"x.", "x.old", "..", ".", "con", "CON", "Con.txt", "nul", "aux.x", "com1", "LPT9.log", "é", strings64() + "x",
	}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) accepted", n)
		}
	}
	for _, n := range []string{"com10", "console", "lpt0", "auxiliary"} {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want ok", n, err)
		}
	}
}

func strings64() string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func TestReservedMarketplace(t *testing.T) {
	for _, n := range []string{"builtin", "BUILTIN", "BuiltIn"} {
		if err := ValidateRegisteredMarketplace(n); err == nil {
			t.Errorf("%q accepted as registered marketplace", n)
		}
		if err := ValidateName(n); err != nil {
			t.Errorf("%q must remain a valid name for the builtin provider: %v", n, err)
		}
	}
	if err := ValidateRegisteredMarketplace("team"); err != nil {
		t.Error(err)
	}
	if Fold("Team") != Fold("tEAM") {
		t.Error("Fold must detect ASCII case-equivalent names")
	}
}

func TestProviderAddress(t *testing.T) {
	p, err := ParseProvider("github@team")
	if err != nil || p.Plugin != "github" || p.Marketplace != "team" || p.String() != "github@team" {
		t.Fatalf("ParseProvider = %+v, %v", p, err)
	}
	for _, s := range []string{"", "github", "@team", "github@", "a@b@c", "a b@team", "github@Team/x", "../x@team"} {
		if _, err := ParseProvider(s); err == nil {
			t.Errorf("ParseProvider(%q) accepted", s)
		}
	}
	if _, err := NewProvider("x.old", "team"); err == nil {
		t.Error("invalid plugin name accepted")
	}
}

func TestRefAddress(t *testing.T) {
	r, err := ParseRef("github@team/build")
	if err != nil || r.Name != "build" || r.Provider.String() != "github@team" || r.String() != "github@team/build" {
		t.Fatalf("ParseRef = %+v, %v", r, err)
	}
	for _, s := range []string{"github@team", "github@team/", "github@team/a/b", "github/build", "github@team/.x", "x@y/con"} {
		if _, err := ParseRef(s); err == nil {
			t.Errorf("ParseRef(%q) accepted", s)
		}
	}
}

func TestOwnerEncoding(t *testing.T) {
	if got := EncodeOwner("acme.tools"); got != "acme%2Etools" {
		t.Fatalf("EncodeOwner = %q", got)
	}
	if got, err := DecodeOwner("acme%2Etools"); err != nil || got != "acme.tools" {
		t.Fatalf("DecodeOwner = %q, %v", got, err)
	}
	for _, tok := range []string{"acme%2etools", "acme%252Etools", "acme%41", "acme.tools", "%2Efoo", "a%", "a%2"} {
		if _, err := DecodeOwner(tok); err == nil {
			t.Errorf("DecodeOwner(%q) accepted", tok)
		}
	}
}

func TestStorageTokens(t *testing.T) {
	if err := ValidateRegistrationID("builtin"); err != nil {
		t.Error(err)
	}
	if err := ValidateRegistrationID("0123456789abcdef0123456789abcdef"); err != nil {
		t.Error(err)
	}
	for _, id := range []string{"", "0123456789ABCDEF0123456789ABCDEF", "short", "0123456789abcdef0123456789abcdeg", "builtin2"} {
		if ValidateRegistrationID(id) == nil {
			t.Errorf("registration id %q accepted", id)
		}
	}
	good := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := ValidateHexDigest(good); err != nil {
		t.Error(err)
	}
	for _, d := range []string{"", "sha256:" + good, good[:63], good + "0", "G" + good[1:], "../" + good[3:]} {
		if ValidateHexDigest(d) == nil {
			t.Errorf("digest payload %q accepted", d)
		}
	}
}
