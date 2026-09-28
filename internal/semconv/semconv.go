// Package semconv enforces the Work Semantic Conventions' key grammar,
// private-key ownership and value shape. It is a leaf: it imports nothing
// internal so the manifest validator, the Starter response check and the
// extension pipeline can all share one definition of a valid key.
//
// The package deliberately holds no list of published keys. Which public keys
// exist is documentation for plugin authors, not something the core checks.
package semconv

import (
	"fmt"
	"regexp"
	"strings"
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
	// privateKey captures the plugin name so ownership can be compared without
	// splitting on dots (a plugin name is a single segment by rule).
	privateKey = regexp.MustCompile(`^plugin\.([A-Za-z0-9][A-Za-z0-9_-]*)\.(` + segment + `(?:\.` + segment + `)*)$`)
)

// ValidKey reports why key is not a valid Semantic Conventions key. A public
// key is two or more lowercase dot-separated segments outside the reserved
// `plugin` and `work` namespaces; a private key is `plugin.<name>.<local>`.
func ValidKey(key string) error {
	if strings.HasPrefix(key, privatePrefix) {
		if !privateKey.MatchString(key) {
			return fmt.Errorf("key %q is not a valid private key: use plugin.<plugin-name>.<local> with lowercase segments", key)
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
// the grammar of ValidKey plus ownership of private keys. A private key may
// be published only by the plugin whose name it carries, and a plugin whose
// name contains a dot owns none, because plugin.foo.bar.x would then be
// ambiguous with plugin foo's key bar.x. Public keys are open to every plugin.
func ValidatePublished(owner, key string) error {
	if err := ValidKey(key); err != nil {
		return err
	}
	if !strings.HasPrefix(key, privatePrefix) {
		return nil
	}
	if strings.Contains(owner, ".") {
		return fmt.Errorf("key %q is private, but plugin %q has a dot in its name and cannot own private keys", key, owner)
	}
	name := privateKey.FindStringSubmatch(key)[1]
	if name != owner {
		return fmt.Errorf("key %q is private to plugin %q, not %q", key, name, owner)
	}
	return nil
}

// ValidLinkValue reports why v cannot be a link value. Values are stored as
// received, so the only requirement is that one exists.
func ValidLinkValue(v string) error {
	if v == "" {
		return fmt.Errorf("a link value must be a non-empty string")
	}
	return nil
}
