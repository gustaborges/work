// Package skillinstall manages filesystem-local agent skill copies independently
// of Work application state.
package skillinstall

import "os"

const MarkerName = ".work-skill.json"

// Options always selects the complete bundled collection for one vendor.
type Options struct {
	Scope  string
	Vendor string
}
type Destination struct{ Vendor, Skill, Scope, Anchor, Path string }
type Marker struct {
	Schema    int    `json:"schema"`
	Skill     string `json:"skill"`
	Algorithm string `json:"digest_algorithm"`
	Digest    string `json:"digest"`
}
type Result struct {
	Vendor   string  `json:"vendor"`
	Skill    string  `json:"skill"`
	Path     string  `json:"path"`
	State    string  `json:"state"`
	Embedded string  `json:"embedded_digest"`
	Recorded *string `json:"recorded_digest"`
	Actual   *string `json:"actual_digest"`
	Error    string  `json:"error,omitempty"`
}
type identity struct {
	Path string
	Info os.FileInfo
}
type Observation struct {
	Identities  []identity
	Result      Result
	Fingerprint string
}
type Plan struct {
	Destination Destination
	Before      Observation
	Force       bool
}
type Outcome struct {
	Destination           Destination
	Action, Commit, Error string
}
type Record struct {
	Schema         int    `json:"schema"`
	Operation      string `json:"operation"`
	Skill          string `json:"skill"`
	Stage          string `json:"stage"`
	Backup         string `json:"backup"`
	Phase          string `json:"phase"`
	OldExists      bool   `json:"old_exists"`
	OldFingerprint string `json:"old_fingerprint"`
	NewFingerprint string `json:"new_fingerprint"`
	NewDigest      string `json:"new_digest"`
	NewMarker      string `json:"new_marker"`
}
