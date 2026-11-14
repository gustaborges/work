// Package semconv enforces the Work Semantic Conventions' key grammar,
// private-key ownership and value shape. It is a leaf apart from the shared
// name grammar: it imports nothing else internal so the manifest validator,
// the Starter response check and the extension pipeline can all share one
// definition of a valid key.
//
// The package deliberately holds no list of published keys. Which public keys
// exist is documentation for plugin authors, not something the core checks.
//
// A private key names its owning plugin, so two plugins can never collide on a
// key by accident. Three spellings exist, each valid in exactly one place:
//
//	plugin.<enc(plugin)>.<local>                       author declaration or publication
//	plugin.<enc(plugin)>@<enc(marketplace)>.<local>    explicit input naming a provider
//	plugin.<enc(plugin)>@<enc(marketplace)>~<id>.<local>  core storage only
//
// enc replaces '.' in a name with "%2E" and changes nothing else, which keeps a
// dotted plugin name unambiguous inside a dot-separated key.
package semconv

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gustaborges/work/internal/pluginid"
)

// Facts are the only `work:` inputs an extension may declare: everything else
// about a Work stays private to the core.
var Facts = []string{"worktree_path", "start_mode", "branch", "base_branch", "slug"}

const (
	segment       = `[a-z][a-z0-9_]*`
	privatePrefix = "plugin."
)

var (
	publicKey = regexp.MustCompile(`^` + segment + `(\.` + segment + `)+$`)
	localKey  = regexp.MustCompile(`^` + segment + `(\.` + segment + `)*$`)
)

// PrivateKey is a parsed private key. Marketplace is empty for the relative
// spelling and RegistrationID is set only for the core storage spelling.
type PrivateKey struct {
	Plugin         string
	Marketplace    string
	RegistrationID string
	Local          string
}

// Relative reports whether the key names no marketplace.
func (k PrivateKey) Relative() bool { return k.Marketplace == "" }

// Stored reports whether the key is the core storage spelling.
func (k PrivateKey) Stored() bool { return k.RegistrationID != "" }

// Provider is the owning provider of a qualified or stored key.
func (k PrivateKey) Provider() pluginid.Provider {
	return pluginid.Provider{Marketplace: k.Marketplace, Plugin: k.Plugin}
}

// String renders the key in its own spelling.
func (k PrivateKey) String() string {
	out := privatePrefix + pluginid.EncodeOwner(k.Plugin)
	if k.Marketplace != "" {
		out += "@" + pluginid.EncodeOwner(k.Marketplace)
	}
	if k.RegistrationID != "" {
		out += "~" + k.RegistrationID
	}
	return out + "." + k.Local
}

// ParsePrivate parses any of the three private spellings strictly. The first
// '.' after "plugin." ends the encoded owner token, and exactly one '@'
// separates a qualified pair.
func ParsePrivate(key string) (PrivateKey, error) {
	bad := func(why string) (PrivateKey, error) {
		return PrivateKey{}, fmt.Errorf("key %q is not a valid private key: %s", key, why)
	}
	if !strings.HasPrefix(key, privatePrefix) {
		return bad("it does not start with \"plugin.\"")
	}
	rest := key[len(privatePrefix):]
	end := strings.IndexAny(rest, ".@")
	if end <= 0 {
		return bad("use plugin.<plugin-name>.<local>")
	}
	var k PrivateKey
	var err error
	if k.Plugin, err = pluginid.DecodeOwner(rest[:end]); err != nil {
		return bad(err.Error())
	}
	rest = rest[end:]
	if rest[0] == '@' {
		rest = rest[1:]
		end = strings.IndexAny(rest, ".~@")
		if end <= 0 || rest[end] == '@' {
			return bad("use plugin.<plugin-name>@<marketplace>.<local>")
		}
		if k.Marketplace, err = pluginid.DecodeOwner(rest[:end]); err != nil {
			return bad(err.Error())
		}
		rest = rest[end:]
		if rest[0] == '~' {
			rest = rest[1:]
			dot := strings.IndexByte(rest, '.')
			if dot <= 0 {
				return bad("a stored key needs a source instance and a local name")
			}
			if err = pluginid.ValidateRegistrationID(rest[:dot]); err != nil {
				return bad(err.Error())
			}
			k.RegistrationID = rest[:dot]
			rest = rest[dot:]
		}
	}
	if rest == "" || rest[0] != '.' || !localKey.MatchString(rest[1:]) {
		return bad("the local name must use lowercase dot-separated segments")
	}
	k.Local = rest[1:]
	if k.String() != key {
		return bad("it is not in canonical form")
	}
	return k, nil
}

// IsPrivate reports whether key is in the plugin-private namespace.
func IsPrivate(key string) bool { return strings.HasPrefix(key, privatePrefix) }

// ValidKey reports why key is not a valid author-facing Semantic Conventions
// key. A public key is two or more lowercase dot-separated segments outside the
// reserved `plugin` and `work` namespaces; a private key is the relative or
// qualified spelling. The storage spelling belongs to the core and is rejected
// here.
func ValidKey(key string) error {
	if IsPrivate(key) {
		k, err := ParsePrivate(key)
		if err != nil {
			return err
		}
		if k.Stored() {
			return fmt.Errorf("key %q uses the core storage spelling, which only the core may write", key)
		}
		return nil
	}
	if key == "plugin" || key == "work" || strings.HasPrefix(key, "work.") {
		return fmt.Errorf("key %q uses a reserved namespace", key)
	}
	if !publicKey.MatchString(key) {
		return fmt.Errorf("key %q is invalid: use two or more dot-separated segments of lowercase letters, digits and underscores, each starting with a letter", key)
	}
	return nil
}

// ValidatePublished reports why the plugin named owner may not publish key:
// the grammar of ValidKey plus ownership of private keys. A private key may be
// published only in its relative spelling and only by the plugin whose name it
// carries; a qualified key would let a plugin write into another provider's
// namespace. Public keys are open to every plugin.
func ValidatePublished(owner, key string) error {
	if err := ValidKey(key); err != nil {
		return err
	}
	if !IsPrivate(key) {
		return nil
	}
	k, _ := ParsePrivate(key)
	if !k.Relative() {
		return fmt.Errorf("key %q is qualified; a plugin publishes only its own relative private keys", key)
	}
	if k.Plugin != owner {
		return fmt.Errorf("key %q is private to plugin %q, not %q", key, k.Plugin, owner)
	}
	return nil
}

// ValidateOwnInput reports why the plugin named declarer may not declare key as
// an input. A public key and an explicitly qualified private key are always
// readable; a relative private key must name the declaring plugin, so reading
// another plugin's data always takes the explicit qualified spelling.
func ValidateOwnInput(declarer, key string) error {
	if err := ValidKey(key); err != nil {
		return err
	}
	if !IsPrivate(key) {
		return nil
	}
	if k, _ := ParsePrivate(key); k.Relative() && k.Plugin != declarer {
		return fmt.Errorf("key %q belongs to plugin %q; name its provider explicitly as plugin.%s@<marketplace>.%s", key, k.Plugin, pluginid.EncodeOwner(k.Plugin), k.Local)
	}
	return nil
}

// StorageKey is the core-only key under which provider's source instance
// stores local. Equal public names with another registration cannot read it.
func StorageKey(p pluginid.Provider, registrationID, local string) string {
	return PrivateKey{Plugin: p.Plugin, Marketplace: p.Marketplace, RegistrationID: registrationID, Local: local}.String()
}

// BindPublished translates a key a provider's source instance publishes into
// its stored form. Public keys are shared semantic keys and stay unchanged.
// The key must already satisfy ValidatePublished for the provider.
func BindPublished(p pluginid.Provider, registrationID, key string) string {
	if !IsPrivate(key) {
		return key
	}
	k, err := ParsePrivate(key)
	if err != nil {
		return key
	}
	return StorageKey(p, registrationID, k.Local)
}

// ValidLinkValue reports why v cannot be a link value. Values are stored as
// received, so the only requirement is that one exists.
func ValidLinkValue(v string) error {
	if v == "" {
		return fmt.Errorf("a link value must be a non-empty string")
	}
	return nil
}
