// Package staging is the boundary between an Importer's output and a Work. An
// Importer writes into a stage that belongs to it alone; Work then plans what
// that output would add, refuses the whole execution if any part of it is
// unsafe, and only then incorporates it, all or nothing.
//
// Staging is a data boundary, not a sandbox: the Importer is still a process
// with the user's permissions.
package staging

import (
	"fmt"
	"os"
	"path/filepath"
)

// Stage is a new, empty directory an Importer may write into.
type Stage struct {
	// Dir is the absolute path handed to the Importer as its output directory.
	Dir string
}

// NewStage creates a fresh stage under <os temp dir>/work/. It is mode 0700,
// unique to this call, and never inside a Work or the workspace, so nothing an
// Importer writes there can land in a Work before Work has checked it.
func NewStage() (*Stage, error) {
	parent := filepath.Join(os.TempDir(), "work")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("staging: creating %s: %w", parent, err)
	}
	dir, err := os.MkdirTemp(parent, "import-*")
	if err != nil {
		return nil, fmt.Errorf("staging: creating a stage: %w", err)
	}
	return &Stage{Dir: dir}, nil
}

// Remove deletes the stage and everything in it. It is safe to call more than
// once, and it must run on every outcome — success, refusal, failure and
// interruption alike.
func (s *Stage) Remove() error {
	return os.RemoveAll(s.Dir)
}
