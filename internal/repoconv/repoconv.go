// Package repoconv reads and writes branch_conventions.json, the generated,
// never-hand-edited per-repository-identity memory of which branch
// convention a repository uses (ADR-0011). It is neither the component
// catalog (internal/registry already models that) nor derivable from
// work.db/snapshots — a repository can have a memoized choice before any
// Work has ever been created against it. Writes are atomic and Set is
// idempotent per identity.
package repoconv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/atomicfile"
)

// Entry is one repository's memoized convention choice.
type Entry struct {
	Identity   string `json:"identity"`
	Convention string `json:"convention"`
}

// Store is the whole branch_conventions.json document.
type Store struct {
	Entries []Entry `json:"entries"`
}

// Load reads path. A missing file yields an empty Store.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("repoconv: reading %s: %w", path, err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("repoconv: %s is not valid JSON: %w", path, err)
	}
	return &s, nil
}

// Save writes s to path atomically.
func Save(path string, s *Store) error {
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
// An unknown identity returns ("", false), never an error.
func (s *Store) Get(identity string) (string, bool) {
	for _, e := range s.Entries {
		if e.Identity == identity {
			return e.Convention, true
		}
	}
	return "", false
}

// Set upserts identity's memoized convention, keyed by Identity.
func (s *Store) Set(identity, convention string) {
	for i := range s.Entries {
		if s.Entries[i].Identity == identity {
			s.Entries[i].Convention = convention
			return
		}
	}
	s.Entries = append(s.Entries, Entry{Identity: identity, Convention: convention})
}
