package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
)

// CatalogFile is the only catalog location: the source root's marketplace.json.
// A root plugin.json is never a fallback.
const CatalogFile = "marketplace.json"

// CatalogSchema is the only published catalog schema.
const CatalogSchema = 1

// ErrInvalid marks every catalog, path, name or manifest rejection (exit 57).
var ErrInvalid = errors.New("invalid marketplace")

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Listing is one published plugins[] element.
type Listing struct {
	Name        string
	Path        string
	Description string
}

// Catalog is a parsed, structurally valid marketplace.json. It has not yet
// been checked against package content.
type Catalog struct {
	Name        string
	DisplayName string // trimmed; empty when absent
	Description string
	Plugins     []Listing
}

// ParseCatalog strictly decodes marketplace.json: it rejects duplicate keys at
// any depth, unknown fields, nulls, wrong types, future schemas, duplicate or
// portably colliding entries, and unsafe or overlapping package paths. An empty
// plugins array is valid.
func ParseCatalog(data []byte) (*Catalog, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return nil, invalidf("%v", err)
	}
	var doc any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, invalidf("marketplace.json is not valid JSON: %v", err)
	}
	if dec.More() {
		return nil, invalidf("marketplace.json has trailing content")
	}
	top, ok := doc.(map[string]any)
	if !ok {
		return nil, invalidf("marketplace.json must be a JSON object")
	}
	if err := allowKeys(top, "schema", "name", "display_name", "description", "plugins"); err != nil {
		return nil, err
	}
	if n, ok := top["schema"].(json.Number); !ok || n.String() != "1" {
		return nil, invalidf("schema must be the integer 1")
	}
	c := &Catalog{}
	var err error
	if c.Name, err = requiredString(top, "name"); err != nil {
		return nil, err
	}
	if err = pluginid.ValidateName(c.Name); err != nil {
		return nil, invalidf("marketplace name: %v", err)
	}
	if _, present := top["display_name"]; present {
		raw, err := requiredString(top, "display_name")
		if err != nil {
			return nil, err
		}
		if c.DisplayName = strings.TrimSpace(raw); c.DisplayName == "" {
			return nil, invalidf("display_name must not be blank")
		}
	}
	if _, present := top["description"]; present {
		if c.Description, err = requiredString(top, "description"); err != nil {
			return nil, err
		}
	}
	list, ok := top["plugins"].([]any)
	if !ok {
		return nil, invalidf("plugins must be an array")
	}
	for i, raw := range list {
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, invalidf("plugins[%d] must be an object", i)
		}
		if err := allowKeys(obj, "name", "path", "description"); err != nil {
			return nil, invalidf("plugins[%d]: %v", i, err)
		}
		var l Listing
		if l.Name, err = requiredString(obj, "name"); err != nil {
			return nil, invalidf("plugins[%d]: %v", i, err)
		}
		if l.Path, err = requiredString(obj, "path"); err != nil {
			return nil, invalidf("plugins[%d]: %v", i, err)
		}
		if _, present := obj["description"]; present {
			if l.Description, err = requiredString(obj, "description"); err != nil {
				return nil, invalidf("plugins[%d]: %v", i, err)
			}
		}
		if err := pluginid.ValidateName(l.Name); err != nil {
			return nil, invalidf("plugins[%d]: %v", i, err)
		}
		if err := ValidateEntryPath(l.Path); err != nil {
			return nil, invalidf("plugins[%d]: %v", i, err)
		}
		c.Plugins = append(c.Plugins, l)
	}
	if err := checkCollisions(c.Plugins); err != nil {
		return nil, err
	}
	return c, nil
}

func allowKeys(obj map[string]any, allowed ...string) error {
	for k := range obj {
		if !slices.Contains(allowed, k) {
			return invalidf("unknown field %q", k)
		}
	}
	return nil
}

func requiredString(obj map[string]any, key string) (string, error) {
	v, ok := obj[key]
	if !ok {
		return "", invalidf("missing required field %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", invalidf("%q must be a string", key)
	}
	return s, nil
}

// ValidateEntryPath checks a package location: a relative POSIX
// subdirectory whose segments are portable names. The source root, absolute
// paths, URLs, drive or UNC forms, backslashes, controls, empty, dot and
// dot-dot segments and reserved metadata directories are all rejected.
func ValidateEntryPath(p string) error {
	switch {
	case p == "" || p == ".":
		return errors.New("path must name a subdirectory, not the source root")
	case strings.ContainsAny(p, "\\:\x00") || strings.ContainsFunc(p, func(r rune) bool { return r < 0x20 || r == 0x7f }):
		return fmt.Errorf("path %q contains a forbidden character", p)
	case strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/"):
		return fmt.Errorf("path %q must be relative without a trailing separator", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("path %q has an empty, dot or dot-dot segment", p)
		}
		if err := pluginid.ValidateName(seg); err != nil {
			return fmt.Errorf("path %q: %w", p, err)
		}
	}
	return nil
}

// checkCollisions rejects duplicate names and package roots that are equal or
// nested once ASCII case is folded, since either would alias the same
// directory on a case-insensitive filesystem or share files across entries.
func checkCollisions(list []Listing) error {
	for i, a := range list {
		for _, b := range list[:i] {
			if pluginid.Fold(a.Name) == pluginid.Fold(b.Name) {
				return invalidf("plugin names %q and %q collide", b.Name, a.Name)
			}
			pa, pb := pluginid.Fold(a.Path)+"/", pluginid.Fold(b.Path)+"/"
			if strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa) {
				return invalidf("package paths %q and %q overlap", b.Path, a.Path)
			}
		}
	}
	return nil
}

// rejectDuplicateKeys scans the token stream because encoding/json silently
// keeps the last duplicate, which would let two parsers disagree on an offer.
func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				key := k.(string)
				if seen[key] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("marketplace.json is empty or truncated")
		}
		return err
	}
	return nil
}

// Component is the static, passive description of one contribution.
type Component struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// Convention is the static description of one offered branch convention.
type Convention struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

// Entry is one validated offered plugin: the listing facts plus everything
// derived from its fixed content.
type Entry struct {
	Name            string       `json:"name"`
	Path            string       `json:"path"`
	Description     string       `json:"description"`
	Version         string       `json:"version"`
	PackageDigest   string       `json:"package_digest"`
	DigestAlgorithm string       `json:"digest_algorithm"`
	Components      []Component  `json:"components"`
	Conventions     []Convention `json:"conventions"`
}

// Offer is a complete validated catalog: identity, entries and digests.
type Offer struct {
	Name          string
	DisplayName   string
	Description   string
	Entries       []Entry
	CatalogDigest string
}

// OfferEntries is the digest view of the entries.
func (o *Offer) OfferEntries() []OfferEntry {
	out := make([]OfferEntry, len(o.Entries))
	for i, e := range o.Entries {
		out[i] = OfferEntry{Name: e.Name, Path: e.Path, Description: e.Description, PackageDigest: e.PackageDigest}
	}
	return out
}

// CaptureDigest is the work-capture-v1 digest of the whole offer.
func (o *Offer) CaptureDigest() string { return CaptureDigest(o.CatalogDigest, o.OfferEntries()) }

// Entry returns the named entry.
func (o *Offer) Entry(name string) (Entry, bool) {
	for _, e := range o.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// View is a read-only package source that BuildOffer can validate: the catalog
// bytes and the tree of each listed package.
type View interface {
	// CatalogBytes returns marketplace.json.
	CatalogBytes() ([]byte, error)
	// PackageTree returns the tree rooted at the entry path. Every ancestor of
	// the path must be a real directory, not a link.
	PackageTree(entryPath string) (*Tree, error)
}

// BuildOffer validates an entire source view. Any malformed listing, unsafe
// package or manifest that disagrees with its listing rejects the whole offer
// so no partial state can ever be published. Nothing is executed and a missing
// runtime or entrypoint file does not prevent discovery.
func BuildOffer(v View) (*Offer, error) {
	data, err := v.CatalogBytes()
	if err != nil {
		return nil, err
	}
	cat, err := ParseCatalog(data)
	if err != nil {
		return nil, err
	}
	o := &Offer{Name: cat.Name, DisplayName: cat.DisplayName, Description: cat.Description, Entries: []Entry{}}
	for _, l := range cat.Plugins {
		tree, err := v.PackageTree(l.Path)
		if err != nil {
			return nil, err
		}
		e, err := ValidatePackage(l, tree)
		if err != nil {
			return nil, err
		}
		o.Entries = append(o.Entries, e)
	}
	o.CatalogDigest = CatalogDigest(o.Name, o.DisplayName, o.Description, o.OfferEntries())
	return o, nil
}

// ValidatePackage statically validates one package tree against its listing.
func ValidatePackage(l Listing, tree *Tree) (Entry, error) {
	if err := tree.ValidateLinks(); err != nil {
		return Entry{}, invalidf("plugin %q: %v", l.Name, err)
	}
	raw, err := tree.ReadFile("plugin.json")
	if err != nil {
		return Entry{}, invalidf("plugin %q: plugin.json: %v", l.Name, err)
	}
	m, err := plugin.Parse(raw)
	if err != nil {
		return Entry{}, invalidf("plugin %q: %v", l.Name, err)
	}
	if m.Name != l.Name {
		return Entry{}, invalidf("plugin %q: manifest name %q does not match its listing", l.Name, m.Name)
	}
	for _, c := range m.Components {
		if !isLocalEntrypoint(c.Entrypoint) {
			return Entry{}, invalidf("plugin %q: component %q entrypoint must stay inside its package", l.Name, c.Name)
		}
		// An entrypoint that exists must resolve inside the package; one that is
		// absent is a selected-installation failure, not a discovery failure.
		if n, err := resolveOptional(tree, c.Entrypoint); err != nil {
			return Entry{}, invalidf("plugin %q: component %q entrypoint: %v", l.Name, c.Name, err)
		} else if n != nil && n.Kind != KindFile {
			return Entry{}, invalidf("plugin %q: component %q entrypoint is not a regular file", l.Name, c.Name)
		}
	}
	digest, err := tree.Digest()
	if err != nil {
		return Entry{}, invalidf("plugin %q: %v", l.Name, err)
	}
	e := Entry{
		Name: l.Name, Path: l.Path, Description: l.Description, Version: m.Version,
		PackageDigest: digest, DigestAlgorithm: AlgorithmPackage,
		Components: []Component{}, Conventions: []Convention{},
	}
	for _, c := range m.Components {
		e.Components = append(e.Components, Component{Name: c.Name, Role: c.Role})
	}
	for _, c := range m.Conventions {
		e.Conventions = append(e.Conventions, Convention{Name: c.Name, Prefixes: slices.Clone(c.Prefixes)})
	}
	return e, nil
}

func isLocalEntrypoint(p string) bool {
	return p != "" && !strings.ContainsAny(p, "\\\x00") && !path.IsAbs(p) && path.Clean(p) == p && p != "." && !strings.HasPrefix(p, "../")
}

func resolveOptional(t *Tree, rel string) (*Node, error) {
	n, err := resolve(t.index(), rel)
	switch {
	case err == nil:
		return &n, nil
	case errors.Is(err, errDangling):
		return nil, nil
	default:
		return nil, err
	}
}
