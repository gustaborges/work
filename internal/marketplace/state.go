package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/jsonx"
	"github.com/gustaborges/work/internal/pluginid"
)

// StateSchema is the only supported stored-catalog schema.
const StateSchema = 1

// Stored is the persisted discovery offer of one registered source: the whole
// validated catalog plus the exact revision it was obtained at. Browsing it
// needs no network and no access to the source.
type Stored struct {
	RegistrationID  string  `json:"registration_id"`
	Namespace       string  `json:"namespace"`
	DisplayName     string  `json:"display_name,omitempty"`
	Description     string  `json:"description,omitempty"`
	Source          Source  `json:"source"`
	Revision        string  `json:"revision"`
	CatalogDigest   string  `json:"catalog_digest"`
	DigestAlgorithm string  `json:"digest_algorithm"`
	CaptureDigest   *string `json:"capture_digest"`
	Entries         []Entry `json:"entries"`
}

// State is the whole marketplace-catalogs.json document.
type State struct {
	Schema   int      `json:"schema"`
	Catalogs []Stored `json:"catalogs"`
}

// NewState returns an empty current-schema State.
func NewState() *State { return &State{Schema: StateSchema, Catalogs: []Stored{}} }

// ByRegistration returns the catalog stored for a source instance.
func (s *State) ByRegistration(id string) (*Stored, bool) {
	for i := range s.Catalogs {
		if s.Catalogs[i].RegistrationID == id {
			return &s.Catalogs[i], true
		}
	}
	return nil, false
}

// Upsert inserts or replaces the catalog keyed by registration id, keeping the
// list sorted by namespace so equal states serialize identically.
func (s *State) Upsert(c Stored) {
	replaced := false
	for i := range s.Catalogs {
		if s.Catalogs[i].RegistrationID == c.RegistrationID {
			s.Catalogs[i], replaced = c, true
			break
		}
	}
	if !replaced {
		s.Catalogs = append(s.Catalogs, c)
	}
	slices.SortFunc(s.Catalogs, func(a, b Stored) int { return strings.Compare(a.Namespace, b.Namespace) })
}

// Remove deletes the catalog of a source instance.
func (s *State) Remove(id string) {
	s.Catalogs = slices.DeleteFunc(s.Catalogs, func(c Stored) bool { return c.RegistrationID == id })
}

// StoredFrom converts a validated fixed snapshot into its persisted form.
func StoredFrom(registrationID string, snap *Snapshot) Stored {
	o := snap.Offer
	c := Stored{
		RegistrationID: registrationID, Namespace: o.Name, DisplayName: o.DisplayName, Description: o.Description,
		Source: snap.Source, Revision: snap.Revision, CatalogDigest: o.CatalogDigest, DigestAlgorithm: AlgorithmCatalog,
		Entries: slices.Clone(o.Entries),
	}
	if snap.Source.Kind == pluginid.KindLocal {
		d := o.CaptureDigest()
		c.CaptureDigest = &d
	}
	return c
}

// Validate applies the strict rules, including recomputing the catalog digest
// from the stored entries so an edited or inconsistent offer is rejected.
func (s *State) Validate() error {
	if s.Schema != StateSchema {
		return fmt.Errorf("unsupported catalog state schema %d", s.Schema)
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, c := range s.Catalogs {
		if c.RegistrationID == pluginid.BuiltinRegistrationID {
			return errors.New("the builtin catalog is code-owned and never stored")
		}
		if err := pluginid.ValidateRegistrationID(c.RegistrationID); err != nil {
			return err
		}
		if err := pluginid.ValidateRegisteredMarketplace(c.Namespace); err != nil {
			return err
		}
		if ids[c.RegistrationID] || names[pluginid.Fold(c.Namespace)] {
			return fmt.Errorf("catalog %q is stored twice", c.Namespace)
		}
		ids[c.RegistrationID], names[pluginid.Fold(c.Namespace)] = true, true
		if err := c.validate(); err != nil {
			return fmt.Errorf("catalog %q: %w", c.Namespace, err)
		}
	}
	return nil
}

func (c Stored) validate() error {
	switch c.Source.Kind {
	case pluginid.KindGit:
		if c.Source.Branch == "" || c.CaptureDigest != nil || !gitRevision(c.Revision) {
			return errors.New("git catalogs record a branch and an exact commit, not a capture")
		}
	case pluginid.KindLocal:
		if c.Source.Branch != "" || c.CaptureDigest == nil || c.Revision != *c.CaptureDigest {
			return errors.New("local catalogs record a capture digest as their revision")
		}
		if _, ok := HexPayload(*c.CaptureDigest); !ok {
			return errors.New("invalid capture digest")
		}
	default:
		return fmt.Errorf("unsupported source kind %q", c.Source.Kind)
	}
	if c.Source.Source == "" {
		return errors.New("source is empty")
	}
	if c.DigestAlgorithm != AlgorithmCatalog || !strings.HasPrefix(c.CatalogDigest, digestPrefix) {
		return errors.New("unsupported catalog digest")
	}
	names := map[string]bool{}
	var offer []OfferEntry
	for _, e := range c.Entries {
		if err := pluginid.ValidateName(e.Name); err != nil {
			return err
		}
		if err := ValidateEntryPath(e.Path); err != nil {
			return err
		}
		if e.Version == "" || e.DigestAlgorithm != AlgorithmPackage || !strings.HasPrefix(e.PackageDigest, digestPrefix) {
			return fmt.Errorf("entry %q has incomplete package facts", e.Name)
		}
		if names[pluginid.Fold(e.Name)] {
			return fmt.Errorf("entry %q is stored twice", e.Name)
		}
		names[pluginid.Fold(e.Name)] = true
		offer = append(offer, OfferEntry{Name: e.Name, Path: e.Path, Description: e.Description, PackageDigest: e.PackageDigest})
	}
	if got := CatalogDigest(c.Namespace, c.DisplayName, c.Description, offer); got != c.CatalogDigest {
		return errors.New("catalog digest does not match its entries")
	}
	if c.CaptureDigest != nil && CaptureDigest(c.CatalogDigest, offer) != *c.CaptureDigest {
		return errors.New("capture digest does not match its entries")
	}
	return nil
}

func gitRevision(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// LoadState reads path. A missing file yields an empty State; anything that is
// not the current strict shape is an error and is never repaired.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NewState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace: reading %s: %w", path, err)
	}
	s, err := DecodeState(data)
	if err != nil {
		return nil, fmt.Errorf("marketplace: %s is invalid: %w", path, err)
	}
	return s, nil
}

// DecodeState parses and validates marketplace-catalogs.json bytes.
func DecodeState(data []byte) (*State, error) {
	var s State
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if jsonx.TrailingData(dec) {
		return nil, errors.New("trailing data")
	}
	if s.Catalogs == nil {
		s.Catalogs = []Stored{}
	}
	for i := range s.Catalogs {
		if s.Catalogs[i].Entries == nil {
			s.Catalogs[i].Entries = []Entry{}
		}
		for j := range s.Catalogs[i].Entries {
			e := &s.Catalogs[i].Entries[j]
			if e.Components == nil {
				e.Components = []Component{}
			}
			if e.Conventions == nil {
				e.Conventions = []Convention{}
			}
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveState writes s to path atomically.
func SaveState(path string, s *State) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("marketplace: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, append(data, '\n'))
}
