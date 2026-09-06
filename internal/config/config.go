// Package config reads and writes work.json, the single human-editable
// configuration file. Writes are atomic; a malformed file is reported as a
// bootstrap failure naming the file, never as a partial overwrite.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/diag"
)

// Config is the shape of work.json for F1.
type Config struct {
	Workspace            string               `json:"workspace"`
	RepositoryRoots      []string             `json:"repository_roots"`
	RepositoryResolution RepositoryResolution `json:"repository_resolution"`
}

// RepositoryResolution holds the ordered Repository Locator policy.
type RepositoryResolution struct {
	Locators []string `json:"locators"`
}

// Default is a fresh config with the stable empty shape.
func Default() *Config {
	return &Config{
		RepositoryRoots:      []string{},
		RepositoryResolution: RepositoryResolution{Locators: []string{}},
	}
}

// Load reads work.json from path. A missing file yields Default(); a malformed
// file yields a diag bootstrap-failed error naming the file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, diag.Wrapf(diag.BootstrapFailed, err, "cannot read config file %s", path)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, diag.Wrapf(diag.BootstrapFailed, err, "config file %s is not valid JSON", path)
	}
	if c.RepositoryRoots == nil {
		c.RepositoryRoots = []string{}
	}
	if c.RepositoryResolution.Locators == nil {
		c.RepositoryResolution.Locators = []string{}
	}
	return &c, nil
}

// Save writes c to path atomically, creating the parent directory if needed.
func Save(path string, c *Config) error {
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
