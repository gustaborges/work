// Package repoconv reads and writes branch_conventions.json, the generated,
// never-hand-edited per-repository-identity memory of which branch
// convention a repository uses (ADR-0011). It is neither the component
// catalog (internal/registry already models that) nor derivable from
// work.db/snapshots — a repository can have a memoized choice before any
// Work has ever been created against it. A remembered choice names its
// provider and the source registration instance that published it, so a
// replacement source that merely reuses the public names can never satisfy
// it. Writes are atomic and Set is idempotent per identity.
package repoconv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/jsonx"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

// Schema is the only supported memory schema.
const Schema = 1

// Reference is a remembered convention choice: the exact contribution and the
// registration instance of the source it came from.
type Reference struct {
	RegistrationID string `json:"registration_id"`
	Marketplace    string `json:"marketplace"`
	Plugin         string `json:"plugin"`
	Name           string `json:"name"`
}

// Ref converts to the typed contribution reference.
func (r Reference) Ref() pluginid.Ref {
	return pluginid.Ref{Provider: pluginid.Provider{Marketplace: r.Marketplace, Plugin: r.Plugin}, Name: r.Name}
}

// String renders plugin@marketplace/name.
func (r Reference) String() string { return r.Ref().String() }

// NewReference binds a contribution to the registration instance of its owner.
func NewReference(registrationID string, ref pluginid.Ref) Reference {
	return Reference{RegistrationID: registrationID, Marketplace: ref.Provider.Marketplace, Plugin: ref.Provider.Plugin, Name: ref.Name}
}

// Entry is one repository's memoized convention choice.
type Entry struct {
	Identity  string    `json:"identity"`
	Reference Reference `json:"reference"`
}

// Store is the whole branch_conventions.json document.
type Store struct {
	Schema  int     `json:"schema"`
	Entries []Entry `json:"entries"`
}

// New returns an empty current-schema Store.
func New() *Store { return &Store{Schema: Schema, Entries: []Entry{}} }

// Validate applies the strict current-schema rules.
func (s *Store) Validate() error {
	if s.Schema != Schema {
		return fmt.Errorf("unsupported convention memory schema %d", s.Schema)
	}
	seen := map[string]bool{}
	for _, e := range s.Entries {
		if e.Identity == "" || seen[e.Identity] {
			return errors.New("empty or duplicate repository identity")
		}
		seen[e.Identity] = true
		if err := pluginid.ValidateRegistrationID(e.Reference.RegistrationID); err != nil {
			return err
		}
		if _, err := pluginid.NewRef(e.Reference.Ref().Provider, e.Reference.Name); err != nil {
			return err
		}
	}
	return nil
}

// Load reads path. A missing file yields an empty Store.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("repoconv: reading %s: %w", path, err)
	}
	s, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("repoconv: %s is invalid: %w", path, err)
	}
	return s, nil
}

// Decode parses and validates branch_conventions.json bytes.
func Decode(data []byte) (*Store, error) {
	var s Store
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if jsonx.TrailingData(dec) {
		return nil, errors.New("trailing data")
	}
	if s.Entries == nil {
		s.Entries = []Entry{}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes s to path atomically.
func Save(path string, s *Store) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("repoconv: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("repoconv: marshaling: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("repoconv: creating state dir: %w", err)
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return fmt.Errorf("repoconv: writing %s: %w", path, err)
	}
	return nil
}

// Get returns the memoized convention for identity, and whether one exists.
// An unknown identity returns (Reference{}, false), never an error.
func (s *Store) Get(identity string) (Reference, bool) {
	for _, e := range s.Entries {
		if e.Identity == identity {
			return e.Reference, true
		}
	}
	return Reference{}, false
}

// Set upserts identity's memoized convention, keyed by Identity.
func (s *Store) Set(identity string, ref Reference) {
	for i := range s.Entries {
		if s.Entries[i].Identity == identity {
			s.Entries[i].Reference = ref
			return
		}
	}
	s.Entries = append(s.Entries, Entry{Identity: identity, Reference: ref})
}

// View reports remembered state without selecting a replacement or writing.
type View struct {
	Identity          string  `json:"identity"`
	Convention        *string `json:"convention"`
	Available         bool    `json:"available"`
	UnavailableReason *string `json:"unavailable_reason"`
}

// Available reports whether ref is satisfied by an enabled provider whose
// installed registration instance is the one the choice was made against.
func Available(ref Reference, reg *registry.Registry) bool {
	eff := reg.Effective()
	pkg, ok := eff.PackageByProvider(ref.Ref().Provider)
	if !ok || pkg.Provenance.RegistrationID != ref.RegistrationID {
		return false
	}
	_, ok = eff.ConventionByRef(ref.Ref())
	return ok
}

// View projects the remembered choice for identity against reg.
func (s *Store) View(identity string, reg *registry.Registry) View {
	v := View{Identity: identity}
	reason := "not-set"
	if ref, ok := s.Get(identity); ok {
		display := ref.String()
		v.Convention = &display
		reason = "provider-unavailable"
		if Available(ref, reg) {
			v.Available = true
			return v
		}
	}
	v.UnavailableReason = &reason
	return v
}
