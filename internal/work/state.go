// Package work models a Work and its canonical on-disk snapshot,
// work-state.json (schema 1). The snapshot is the single source of truth for a
// Work (ADR-0013); the SQLite projection is derived from it. The core is the
// only writer, and every write goes through internal/atomicfile.
package work

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gustaborges/work/internal/atomicfile"
)

// Schema is the only work-state.json schema version F1 emits or accepts.
const Schema = 1

// Status / start-mode values F1 uses.
const (
	StatusInProgress = "in-progress"
	StartModeNew     = "new"
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
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Status           string `json:"status"`
	StartMode        string `json:"start_mode"`
	Starter          string `json:"starter"`
	Branch           string `json:"branch"`
	BaseBranch       string `json:"base_branch"`
	BranchConvention string `json:"branch_convention"`
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

// Decode parses work-state.json bytes without touching the filesystem.
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
	return &s, nil
}

// Write validates s and writes it to path atomically.
func Write(path string, s *State) error {
	if s.Meta == nil {
		s.Meta = map[string]any{}
	}
	if s.Links == nil {
		s.Links = map[string]string{}
	}
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

// Validate enforces the schema-1 constraints.
func (s *State) Validate() error {
	if s.Schema != Schema {
		return fmt.Errorf("work-state: schema = %d, want %d", s.Schema, Schema)
	}
	w := s.Work
	required := map[string]string{
		"id":                w.ID,
		"slug":              w.Slug,
		"status":            w.Status,
		"start_mode":        w.StartMode,
		"starter":           w.Starter,
		"branch":            w.Branch,
		"base_branch":       w.BaseBranch,
		"branch_convention": w.BranchConvention,
		"created_at":        w.CreatedAt,
		"last_accessed_at":  w.LastAccessedAt,
	}
	for name, val := range required {
		if val == "" {
			return fmt.Errorf("work-state: work.%s is required", name)
		}
	}
	if w.Status != StatusInProgress {
		return fmt.Errorf("work-state: work.status = %q, want %q", w.Status, StatusInProgress)
	}
	if w.StartMode != StartModeNew {
		return fmt.Errorf("work-state: work.start_mode = %q, want %q", w.StartMode, StartModeNew)
	}
	for name, val := range map[string]string{"created_at": w.CreatedAt, "last_accessed_at": w.LastAccessedAt} {
		if _, err := time.Parse(time.RFC3339, val); err != nil {
			return fmt.Errorf("work-state: work.%s = %q is not RFC 3339: %w", name, val, err)
		}
	}
	if s.Meta == nil || s.Links == nil {
		return fmt.Errorf("work-state: meta and links are required objects")
	}
	return nil
}
