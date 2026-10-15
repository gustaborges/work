package adminstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

const ReferenceAlias = "work-reference"

// loadLifecycle projects proven legacy ownership without persisting it.
func loadLifecycle(h workhome.Home, reg *registry.Registry) (*Lifecycle, error) {
	b, err := os.ReadFile(h.ReferencePackageFile())
	if err == nil {
		var l Lifecycle
		if json.Unmarshal(b, &l) != nil || l.Schema != 1 || (!l.Initialized && (l.Removed || l.PolicySeeded)) {
			return nil, recoveryError(errors.New("invalid reference package history"))
		}
		return &l, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, recoveryError(err)
	}
	root := filepath.Join(h.PluginsDir(), ReferenceAlias)
	meta, metaErr := os.ReadFile(filepath.Join(root, ".install-meta.json"))
	var old struct {
		Origin        string `json:"origin"`
		ContentDigest string `json:"content_digest"`
		PolicySeeded  bool   `json:"policy_seeded"`
	}
	proven := false
	if metaErr == nil {
		if json.Unmarshal(meta, &old) != nil || old.Origin != registry.OriginEmbedded {
			return nil, recoveryError(errors.New("ambiguous reference installation history"))
		}
		proven = true
	}
	if p, ok := reg.PackageByAlias(ReferenceAlias); ok {
		if p.Origin != registry.OriginEmbedded {
			return nil, recoveryError(errors.New("reserved package ownership conflict"))
		}
		proven = true
	}
	// Complete historical seed declarations are independent initialization
	// evidence even when the old install-meta file is missing.
	starter, locator := false, false
	for _, c := range reg.Components {
		if c.Alias != ReferenceAlias {
			continue
		}
		starter = starter || (c.Name == "local-path-starter" && c.Role == registry.RoleStarter && c.StarterLayer == registry.LayerFallback)
		locator = locator || (c.Name == "filesystem-repository-locator" && c.Role == registry.RoleRepositoryLocator)
	}
	_, hasConvention := reg.ConventionByName("freeform")
	if starter && locator && hasConvention {
		proven = true
		old.PolicySeeded = true
	}
	if !proven {
		if _, err := os.Lstat(root); err == nil {
			return nil, recoveryError(errors.New("ambiguous reference installation history"))
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, recoveryError(err)
		}
		return &Lifecycle{Schema: 1}, nil
	}
	if _, ok := reg.PackageByAlias(ReferenceAlias); !ok {
		p := registry.Package{Alias: ReferenceAlias, PluginName: ReferenceAlias, Origin: registry.OriginEmbedded, Reference: "embedded", ContentDigest: old.ContentDigest}
		if _, ok := reg.ConventionByName("freeform"); ok {
			p.Conventions = []string{"freeform"}
		}
		b, err := os.ReadFile(filepath.Join(root, "plugin.json"))
		if err == nil {
			m, err := plugin.Parse(b)
			if err != nil || m.Name != ReferenceAlias {
				return nil, recoveryError(errors.New("ambiguous legacy reference manifest"))
			}
			p.Version = m.Version
			p.Conventions = []string{}
			for _, cv := range m.Conventions {
				p.Conventions = append(p.Conventions, cv.Name)
			}
		}
		reg.UpsertPackage(p)
	}

	return &Lifecycle{Schema: 1, Initialized: true, PolicySeeded: old.PolicySeeded}, nil
}

// MarkRemoved retains proven initialization/policy history outside the package
// root. Enablement stays solely in registry records.
func (l *Lifecycle) MarkRemoved() {
	l.Initialized = true
	l.Removed = true
}
