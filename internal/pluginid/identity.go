// Package pluginid is the leaf package that defines plugin provider identity:
// the typed (marketplace, plugin) pair, its contribution references and the
// portable name grammar every marketplace, plugin and contribution name obeys.
// It imports nothing from the rest of the module so storage, registry, policy
// and presentation packages can all share one definition.
package pluginid

import (
	"fmt"
	"regexp"
	"strings"
)

// Builtin is the marketplace namespace owned by the binary. It is reserved:
// no user-registered source may claim it, in any letter case.
const Builtin = "builtin"

// BuiltinRegistrationID is the stable source-instance token of the builtin
// provider, which has no user-managed registration.
const BuiltinRegistrationID = "builtin"

// Source kinds recorded in configuration and installed provenance.
const (
	KindGit     = "git"
	KindLocal   = "local"
	KindBuiltin = "builtin"
)

const maxNameBytes = 64

var nameGrammar = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var windowsDevice = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateName reports why name cannot be a marketplace, plugin or
// contribution name. Names are single, portable filesystem segments: they are
// never corrected, only accepted or rejected, so author spelling is preserved.
func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("name is empty")
	case len(name) > maxNameBytes:
		return fmt.Errorf("name %q exceeds %d bytes", name, maxNameBytes)
	case !nameGrammar.MatchString(name):
		return fmt.Errorf("name %q must start with a letter or digit and contain only letters, digits, '.', '_' and '-'", name)
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".old"):
		return fmt.Errorf("name %q must not end in \".\" or \".old\"", name)
	}
	base, _, _ := strings.Cut(name, ".")
	if windowsDevice[strings.ToLower(base)] {
		return fmt.Errorf("name %q is a reserved device name on some platforms", name)
	}
	return nil
}

// Fold returns the comparison key under which two names collide on a
// case-insensitive filesystem.
func Fold(name string) string { return strings.ToLower(name) }

// IsReserved reports whether name is the builtin namespace in any case.
func IsReserved(name string) bool { return Fold(name) == Builtin }

// ValidateRegisteredMarketplace additionally rejects the reserved namespace.
func ValidateRegisteredMarketplace(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if IsReserved(name) {
		return fmt.Errorf("marketplace name %q is reserved for the included provider", name)
	}
	return nil
}

var registrationGrammar = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ValidateRegistrationID accepts a user-managed source instance token (32
// lowercase hex characters) or the stable builtin token.
func ValidateRegistrationID(id string) error {
	if id == BuiltinRegistrationID || registrationGrammar.MatchString(id) {
		return nil
	}
	return fmt.Errorf("invalid source registration id")
}

var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidateHexDigest reports whether s is a bare 64-character lowercase hex
// SHA-256 payload, the only form allowed as a capture directory name.
func ValidateHexDigest(s string) error {
	if !hexDigest.MatchString(s) {
		return fmt.Errorf("invalid digest payload")
	}
	return nil
}

// Provider is the identity of one installed plugin: the marketplace that
// offered it and the plugin's name within that marketplace. It is the only
// plugin identity; there is no global alias.
type Provider struct {
	Marketplace string `json:"marketplace"`
	Plugin      string `json:"plugin"`
}

// NewProvider validates both segments.
func NewProvider(plugin, marketplace string) (Provider, error) {
	p := Provider{Marketplace: marketplace, Plugin: plugin}
	return p, p.Validate()
}

// Validate checks both segments against the portable name grammar.
func (p Provider) Validate() error {
	if err := ValidateName(p.Marketplace); err != nil {
		return fmt.Errorf("marketplace: %w", err)
	}
	if err := ValidateName(p.Plugin); err != nil {
		return fmt.Errorf("plugin: %w", err)
	}
	return nil
}

// String renders plugin@marketplace. The result is for display and selectors,
// never for building filesystem paths.
func (p Provider) String() string { return p.Plugin + "@" + p.Marketplace }

// IsZero reports whether the provider is unset.
func (p Provider) IsZero() bool { return p == Provider{} }

// ParseProvider parses plugin@marketplace with exactly one '@'.
func ParseProvider(s string) (Provider, error) {
	plugin, marketplace, ok := strings.Cut(s, "@")
	if !ok || strings.Contains(marketplace, "@") {
		return Provider{}, fmt.Errorf("%q is not plugin@marketplace", s)
	}
	return NewProvider(plugin, marketplace)
}

// Ref names one contribution (component or convention) of a provider.
type Ref struct {
	Provider Provider `json:"-"`
	Name     string   `json:"-"`
}

// NewRef validates the provider and the contribution name.
func NewRef(p Provider, name string) (Ref, error) {
	r := Ref{Provider: p, Name: name}
	if err := p.Validate(); err != nil {
		return r, err
	}
	if err := ValidateName(name); err != nil {
		return r, fmt.Errorf("contribution: %w", err)
	}
	return r, nil
}

// String renders plugin@marketplace/name.
func (r Ref) String() string { return r.Provider.String() + "/" + r.Name }

// ParseRef parses plugin@marketplace/name.
func ParseRef(s string) (Ref, error) {
	head, name, ok := strings.Cut(s, "/")
	if !ok || strings.Contains(name, "/") {
		return Ref{}, fmt.Errorf("%q is not plugin@marketplace/name", s)
	}
	p, err := ParseProvider(head)
	if err != nil {
		return Ref{}, err
	}
	return NewRef(p, name)
}

// EncodeOwner encodes a validated name for use inside a private-key token:
// only '.' changes, to the uppercase escape "%2E".
func EncodeOwner(name string) string { return strings.ReplaceAll(name, ".", "%2E") }

// DecodeOwner is the strict inverse of EncodeOwner. It rejects lowercase and
// unnecessary escapes, double encoding and decoded names that are invalid.
func DecodeOwner(token string) (string, error) {
	if strings.Contains(token, "%") {
		for rest := token; ; {
			i := strings.Index(rest, "%")
			if i < 0 {
				break
			}
			if !strings.HasPrefix(rest[i:], "%2E") {
				return "", fmt.Errorf("non-canonical escape in %q", token)
			}
			rest = rest[i+3:]
		}
	}
	name := strings.ReplaceAll(token, "%2E", ".")
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if EncodeOwner(name) != token {
		return "", fmt.Errorf("non-canonical encoding of %q", token)
	}
	return name, nil
}
