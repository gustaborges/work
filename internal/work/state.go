// Package work models a Work and its canonical on-disk snapshot,
// work-state.json. The snapshot is the single source of truth for a Work
// (ADR-0013); the SQLite projection is derived from it. The core is the only
// writer, and every write goes through internal/atomicfile.
//
// Schema 2 (F2) is an additive superset of schema 1: work.status gains
// "archived", work.archived_at is new (present iff the Work is archived), and
// work.last_accessed_at is now mutable (bumped on resume, set to the archival
// time on archive).
//
// Schema 3 (F4) is an additive superset of schema 2: work.start_mode widens
// from {"new"} to {"new","contribution","fork"}, and work.slug and
// work.branch_convention both become required unless start_mode ==
// "contribution" — in which case both MUST be absent, since contribution mode
// never runs the slug or convention step (FR-020). This mirrors the
// archived_at-iff-archived conditional schema 2 already established.
//
// Read accepts schema 1, 2, or 3; Write always emits schema 3, so an older
// file is upgraded in place the first time it is rewritten.
package work

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gustaborges/work/internal/atomicfile"
)

// Schema is the schema version Write always emits.
const Schema = 3

// schemaMin is the oldest schema version Read accepts.
const schemaMin = 1

// Status values.
const (
	StatusInProgress = "in-progress"
	StatusArchived   = "archived"
)

// Start mode values (F4). StartModeNew is unchanged from F1/F2 ("start_modes"
// absent from the Starter response). StartModeFork and StartModeContribution
// arrive with F4 — both require start_modes to have been present on the
// Starter response.
const (
	StartModeNew          = "new"
	StartModeFork         = "fork"
	StartModeContribution = "contribution"
)

// State is a whole work-state.json document.
type State struct {
	Schema int               `json:"schema"`
	Work   WorkSection       `json:"work"`
	Meta   map[string]any    `json:"meta"`
	Links  map[string]string `json:"links"`
}

// WorkSection is the core-governed "work" object. Plugins never write here.
type WorkSection struct {
	ID         string `json:"id"`
	Slug       string `json:"slug,omitempty"`
	Status     string `json:"status"`
	ArchivedAt string `json:"archived_at,omitempty"`
	StartMode  string `json:"start_mode"`
	Starter    string `json:"starter"`
	Branch     string `json:"branch"`
	BaseBranch string `json:"base_branch"`
	// BranchConvention is required in "new"/"fork" mode; MUST be absent in
	// "contribution" mode (the convention step never runs, FR-020).
	BranchConvention string `json:"branch_convention,omitempty"`
	CreatedAt        string `json:"created_at"`
	LastAccessedAt   string `json:"last_accessed_at"`
}

// Read parses a work-state.json file. Unknown fields are rejected (the schema is
// closed).
func Read(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("work-state: reading %s: %w", path, err)
	}
	return Decode(data)
}

// Decode parses work-state.json bytes without touching the filesystem. It
// accepts schema 1, 2, or 3 and rejects a structurally impossible document (an
// unsupported schema version, or a schema-1 document that is archived or
// carries archived_at).
func Decode(data []byte) (*State, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s State
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("work-state: %w", err)
	}
	if s.Meta == nil {
		s.Meta = map[string]any{}
	}
	if s.Links == nil {
		s.Links = map[string]string{}
	}
	if s.Schema < schemaMin || s.Schema > Schema {
		return nil, fmt.Errorf("work-state: unsupported schema %d (accept %d..%d)", s.Schema, schemaMin, Schema)
	}
	if s.Schema == 1 {
		if s.Work.Status != StatusInProgress || s.Work.ArchivedAt != "" {
			return nil, fmt.Errorf("work-state: schema 1 document must be in-progress with no archived_at")
		}
	}
	return &s, nil
}

// Write validates s, forces schema 3, and writes it to path atomically.
func Write(path string, s *State) error {
	if s.Meta == nil {
		s.Meta = map[string]any{}
	}
	if s.Links == nil {
		s.Links = map[string]string{}
	}
	s.Schema = Schema
	if err := s.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("work-state: marshaling: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.WriteFile(path, data); err != nil {
		return fmt.Errorf("work-state: writing %s: %w", path, err)
	}
	return nil
}

// Touch sets last_accessed_at to now (RFC 3339 UTC). It is the mutation a
// successful `work resume` commits to the snapshot.
func (s *State) Touch(now time.Time) {
	s.Work.LastAccessedAt = now.UTC().Format(time.RFC3339)
}

// Archive flips the Work to archived: status becomes "archived", archived_at and
// last_accessed_at are both set to now (RFC 3339 UTC). It is the canonical
// commit point of `work archive`.
func (s *State) Archive(now time.Time) {
	ts := now.UTC().Format(time.RFC3339)
	s.Work.Status = StatusArchived
	s.Work.ArchivedAt = ts
	s.Work.LastAccessedAt = ts
}

// Validate enforces the schema constraints. It accepts schema 1, 2, or 3; the
// schema-2 rules (archived_at present iff status == "archived") also hold for
// a schema-1 document, which is always in-progress with no archived_at. The
// schema-3 rule (slug and branch_convention present iff start_mode !=
// "contribution") applies uniformly to every accepted schema, since a
// schema-1/2 document's slug/branch_convention are always non-empty in
// practice (those schemas required them unconditionally) and so already
// satisfy the "new"/"fork" branch of the rule.
func (s *State) Validate() error {
	if s.Schema < schemaMin || s.Schema > Schema {
		return fmt.Errorf("work-state: schema = %d, want %d..%d", s.Schema, schemaMin, Schema)
	}
	w := s.Work
	required := map[string]string{
		"id":               w.ID,
		"status":           w.Status,
		"start_mode":       w.StartMode,
		"starter":          w.Starter,
		"branch":           w.Branch,
		"base_branch":      w.BaseBranch,
		"created_at":       w.CreatedAt,
		"last_accessed_at": w.LastAccessedAt,
	}
	for name, val := range required {
		if val == "" {
			return fmt.Errorf("work-state: work.%s is required", name)
		}
	}
	if w.Status != StatusInProgress && w.Status != StatusArchived {
		return fmt.Errorf("work-state: work.status = %q, want %q or %q", w.Status, StatusInProgress, StatusArchived)
	}
	if w.Status == StatusArchived {
		if s.Schema < 2 {
			return fmt.Errorf("work-state: an archived Work requires schema >= 2")
		}
		if w.ArchivedAt == "" {
			return fmt.Errorf("work-state: work.archived_at is required when status is %q", StatusArchived)
		}
	} else if w.ArchivedAt != "" {
		return fmt.Errorf("work-state: work.archived_at must be absent unless status is %q", StatusArchived)
	}
	switch w.StartMode {
	case StartModeNew, StartModeFork:
		if w.Slug == "" || w.BranchConvention == "" {
			return fmt.Errorf("work-state: work.slug and work.branch_convention are required when start_mode is %q", w.StartMode)
		}
	case StartModeContribution:
		if w.Slug != "" || w.BranchConvention != "" {
			return fmt.Errorf("work-state: work.slug and work.branch_convention must be absent when start_mode is %q", StartModeContribution)
		}
	default:
		return fmt.Errorf("work-state: work.start_mode = %q, want %q, %q, or %q", w.StartMode, StartModeNew, StartModeContribution, StartModeFork)
	}
	stamps := map[string]string{"created_at": w.CreatedAt, "last_accessed_at": w.LastAccessedAt}
	if w.ArchivedAt != "" {
		stamps["archived_at"] = w.ArchivedAt
	}
	for name, val := range stamps {
		if _, err := time.Parse(time.RFC3339, val); err != nil {
			return fmt.Errorf("work-state: work.%s = %q is not RFC 3339: %w", name, val, err)
		}
	}
	if s.Meta == nil || s.Links == nil {
		return fmt.Errorf("work-state: meta and links are required objects")
	}
	return nil
}
