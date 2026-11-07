// Package config reads and writes work.json, the single human-editable
// configuration file. Writes are atomic; a malformed or unsupported file is
// reported as a bootstrap failure naming the file, never as a partial
// overwrite and never converted from another shape.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
)

// Schema is the only supported work.json schema.
const Schema = 1

// Source kinds a user can register.
const (
	KindGit   = pluginid.KindGit
	KindLocal = pluginid.KindLocal
)

// Config is the shape of work.json.
type Config struct {
	Schema               int                  `json:"schema"`
	Workspace            string               `json:"workspace"`
	RepositoryRoots      []string             `json:"repository_roots"`
	RepositoryResolution RepositoryResolution `json:"repository_resolution"`
	Marketplaces         []Marketplace        `json:"marketplaces"`
}

// RepositoryResolution holds the ordered Repository Locator policy. Entries are
// fully qualified contribution references (plugin@marketplace/name).
type RepositoryResolution struct {
	Locators []string `json:"locators"`
}

// Marketplace is one user-managed source registration. RegistrationID is the
// source-instance token: it stays stable across idempotent add and refresh and
// is regenerated when a source is removed and registered again, so retained
// choices never silently rebind to a replacement that merely reuses a name.
type Marketplace struct {
	RegistrationID string `json:"registration_id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Source         string `json:"source"`
	Branch         string `json:"branch,omitempty"`
}

// Default is a fresh config with the stable empty shape.
func Default() *Config {
	return &Config{
		Schema:               Schema,
		RepositoryRoots:      []string{},
		RepositoryResolution: RepositoryResolution{Locators: []string{}},
		Marketplaces:         []Marketplace{},
	}
}

// Marketplace returns the registration with the given canonical name.
func (c *Config) Marketplace(name string) (Marketplace, bool) {
	for _, m := range c.Marketplaces {
		if m.Name == name {
			return m, true
		}
	}
	return Marketplace{}, false
}

// Validate applies the strict current-schema rules.
func (c *Config) Validate() error {
	if c.Schema != Schema {
		return fmt.Errorf("unsupported config schema %d", c.Schema)
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, m := range c.Marketplaces {
		if err := pluginid.ValidateRegisteredMarketplace(m.Name); err != nil {
			return fmt.Errorf("marketplaces: %w", err)
		}
		if id := m.RegistrationID; id == pluginid.BuiltinRegistrationID || pluginid.ValidateRegistrationID(id) != nil {
			return fmt.Errorf("marketplace %q has an invalid registration id", m.Name)
		}
		if ids[m.RegistrationID] || names[pluginid.Fold(m.Name)] {
			return fmt.Errorf("marketplace %q is registered twice", m.Name)
		}
		ids[m.RegistrationID], names[pluginid.Fold(m.Name)] = true, true
		switch m.Kind {
		case KindGit:
			if strings.TrimSpace(m.Branch) == "" {
				return fmt.Errorf("git marketplace %q must record its branch", m.Name)
			}
		case KindLocal:
			if m.Branch != "" {
				return fmt.Errorf("local marketplace %q cannot record a branch", m.Name)
			}
		default:
			return fmt.Errorf("marketplace %q has unsupported kind %q", m.Name, m.Kind)
		}
		if strings.TrimSpace(m.Source) == "" {
			return fmt.Errorf("marketplace %q has no source", m.Name)
		}
	}
	return nil
}

// Load reads work.json from path. A missing file yields Default(); a malformed,
// unknown-field or unsupported-schema file yields a diag bootstrap-failed
// error naming the file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, diag.Wrapf(diag.BootstrapFailed, err, "cannot read config file %s", path)
	}
	c, err := Decode(data)
	if err != nil {
		return nil, diag.Wrapf(diag.BootstrapFailed, err, "config file %s is invalid", path)
	}
	return c, nil
}

// Decode parses and validates work.json bytes under the current schema.
func Decode(data []byte) (*Config, error) {
	var c Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("trailing data")
	}
	if c.RepositoryRoots == nil {
		c.RepositoryRoots = []string{}
	}
	if c.RepositoryResolution.Locators == nil {
		c.RepositoryResolution.Locators = []string{}
	}
	if c.Marketplaces == nil {
		c.Marketplaces = []Marketplace{}
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes c to path atomically, creating the parent directory if needed.
func Save(path string, c *Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshaling: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: creating config dir: %w", err)
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return fmt.Errorf("config: writing %s: %w", path, err)
	}
	return nil
}
