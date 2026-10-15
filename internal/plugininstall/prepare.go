package plugininstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
)

// Prepared retains exactly one parsed manifest and frozen pinned source.
// Linked sources deliberately remain developer-managed; their metadata is
// the baseline captured here, never silently refreshed by inventory reads.
type Prepared struct {
	ContentDir                                 string
	Manifest                                   *plugin.Manifest
	Alias, Origin, Identity, Reference, Digest string
	link                                       bool
	explicitAlias                              bool
	cleanup                                    func()
	once                                       sync.Once
}

func (p *Prepared) Close() {
	p.once.Do(func() {
		if p.cleanup != nil {
			p.cleanup()
		}
	})
}

// ValidateRequest checks source/option grammar without retrieving content or
// consulting administrative state, so invalid invocations cannot recover it.
func ValidateRequest(source string, opts Options) error {
	if strings.TrimSpace(source) == "" {
		return diag.New(diag.Usage, "supply a plugin SOURCE")
	}
	if opts.Link && !isLocalSource(source) {
		return diag.New(diag.Usage, "--link requires a local SOURCE (a remote SOURCE is always pinned by content)")
	}
	if alias := strings.TrimSpace(opts.Alias); alias != "" && plugin.ValidateAlias(alias) != nil {
		return diag.New(diag.PluginInvalid, "--as is not a valid alias")
	}
	return nil
}

func Prepare(ctx context.Context, source string, opts Options) (_ *Prepared, err error) {
	if ctx.Err() != nil {
		return nil, diag.Wrap(diag.Cancelled, ctx.Err(), "plugin preparation cancelled")
	}
	if err = ValidateRequest(source, opts); err != nil {
		return nil, err
	}
	local := isLocalSource(source)
	alias := strings.TrimSpace(opts.Alias)
	dir, origin, identity, reference, release, err := obtainContent(source, local, opts.Link)
	if release != nil {
		defer release()
	}
	if err != nil {
		return nil, err
	}
	p := &Prepared{Origin: origin, Identity: identity, Reference: reference, link: opts.Link, explicitAlias: alias != ""}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	if opts.Link {
		p.ContentDir = dir
	} else {
		if err = validatePinned(dir); err != nil {
			return nil, diag.Wrap(diag.PluginInvalid, err, "plugin content contains unsafe paths or file kinds")
		}
		tmp, e := os.MkdirTemp("", "work-plugin-prepared-*")
		if e != nil {
			return nil, e
		}
		p.cleanup = func() { _ = os.RemoveAll(tmp) }
		p.ContentDir = filepath.Join(tmp, "content")
		if err = copyTree(dir, p.ContentDir); err != nil {
			return nil, diag.Wrap(diag.PluginInstallFailed, err, "cannot freeze plugin content")
		}
		if err = validatePinned(p.ContentDir); err != nil {
			return nil, diag.Wrap(diag.PluginInvalid, err, "frozen plugin content is unsafe")
		}
	}
	b, err := os.ReadFile(filepath.Join(p.ContentDir, "plugin.json"))
	if err != nil {
		return nil, diag.Wrap(diag.PluginInvalid, err, "cannot read plugin.json")
	}
	p.Manifest, err = plugin.Parse(b)
	if err != nil {
		return nil, diag.Wrap(diag.PluginInvalid, err, "plugin.json is invalid")
	}
	if alias == "" {
		alias = p.Manifest.Name
	}
	p.Alias = alias
	if err = preflightRuntimes(p.Manifest); err != nil {
		return nil, err
	}
	for _, c := range p.Manifest.Components {
		if !filepath.IsLocal(c.Entrypoint) || strings.Contains(c.Entrypoint, "\\") {
			return nil, diag.New(diag.PluginInvalid, "component entrypoint must stay inside its package")
		}
		entry := filepath.Join(p.ContentDir, c.Entrypoint)
		if c.Runtime == "" && runtime.GOOS == "windows" {
			entry += ".exe"
		}
		info, e := os.Stat(entry)
		if e != nil || !info.Mode().IsRegular() {
			return nil, diag.New(diag.PluginInvalid, "component entrypoint is unavailable")
		}
	}
	p.Digest, err = ContentDigest(p.ContentDir)
	if err != nil {
		return nil, diag.Wrap(diag.PluginInvalid, err, "cannot inspect plugin content")
	}
	if ctx.Err() != nil {
		return nil, diag.Wrap(diag.Cancelled, ctx.Err(), "plugin preparation cancelled")
	}
	return p, nil
}

func validatePinned(root string) error {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			text, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(text) || strings.Contains(text, "\\") {
				return errors.New("absolute or nonportable package symlink")
			}
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(resolved, target)
			if err != nil || !filepath.IsLocal(rel) {
				return errors.New("escaping package symlink")
			}
			return nil
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return errors.New("special package file")
		}
		return nil
	})
}

// ContentDigest normalizes relative paths, kinds, executability and link text.
// It never follows a symlink, so linked external trees are never owned by it.
func ContentDigest(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		kind := "file"
		var b []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			kind = "link"
			var text string
			text, err = os.Readlink(path)
			b = []byte(text)
		case d.IsDir():
			kind = "dir"
		case info.Mode().IsRegular():
			b, err = os.ReadFile(path)
		default:
			return errors.New("unsupported file kind")
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(h, "%q %s %t %q\n", filepath.ToSlash(rel), kind, info.Mode().Perm()&0111 != 0, b)
		return err
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ValidateCatalog is pure and repeated against the latest locked catalog.
func (p *Prepared) ValidateCatalog(reg *registry.Registry) error {
	existing, installed := reg.PackageByAlias(p.Alias)
	if p.Alias == ReferenceAlias {
		return aliasConflict(p.Manifest.Name, p.Alias, p.explicitAlias, "the reference package", "<source>")
	}
	if installed && existingIdentity(existing) != p.Identity {
		return aliasConflict(p.Manifest.Name, p.Alias, p.explicitAlias, "another source", "<source>")
	}
	if installed && existing.PluginName != "" && existing.PluginName != p.Manifest.Name {
		return diag.New(diag.PluginInvalid, "plugin name changed; private namespace ownership cannot change on reinstall")
	}
	for _, c := range p.Manifest.Components {
		if c.IsFallbackStarter() {
			if fb, ok := reg.StarterFallback(); ok && fb.Alias != p.Alias {
				return diag.Newf(diag.PluginFallbackConflict, "a fallback Starter is already registered by %q", fb.Alias)
			}
		}
	}
	return nil
}
func (p *Prepared) Register(reg *registry.Registry) Result {
	old, _ := reg.PackageByAlias(p.Alias)
	names := []string{}
	for _, cv := range p.Manifest.Conventions {
		names = append(names, cv.Name)
	}
	pkg := registry.Package{Alias: p.Alias, Origin: p.Origin, Reference: p.Reference, Source: p.Identity, Version: p.Manifest.Version, ContentDigest: p.Digest, PluginName: p.Manifest.Name, Conventions: names, Enabled: old.Enabled}
	reg.RemoveAlias(p.Alias)
	reg.UpsertPackage(pkg)
	for _, c := range p.Manifest.Components {
		reg.UpsertComponent(ComponentEntry(p.Alias, c))
	}
	for _, cv := range p.Manifest.Conventions {
		reg.UpsertConvention(registry.Convention{Name: cv.Name, Prefixes: cv.Prefixes})
	}
	return Result{Package: pkg, Components: p.Manifest.Components}
}

// Stage materializes an owned installation root on the destination filesystem.
func (p *Prepared) Stage(pluginsDir string) (string, error) {
	if err := os.MkdirAll(pluginsDir, 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(pluginsDir, ".admin-stage-")
	if err != nil {
		return "", err
	}
	if p.link {
		err = linkSource(stage, p.ContentDir)
	} else {
		err = copyTree(p.ContentDir, sourceDir(stage))
	}
	if err != nil {
		_ = os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}
