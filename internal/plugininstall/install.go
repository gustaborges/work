package plugininstall

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
)

// ReferenceAlias is the alias the embedded reference package installs under.
// It is reserved: the seed is installed by bootstrap, which may not have run
// yet when `work plugin install` does, so the registry cannot be what
// protects it.
const ReferenceAlias = "work-reference"

// Options configures Install.
type Options struct {
	// Link installs a local SOURCE by reference (a directory symlink) instead
	// of copying. Invalid together with a remote SOURCE (FR-002).
	Link bool
	// Alias overrides the default alias (the manifest's name).
	Alias string
}

// Result is what a successful Install produced, for the caller to render and
// persist.
type Result struct {
	Package    registry.Package
	Components []plugin.Component
}

// Install runs the full pipeline documented in
// specs/005-plugin-origins/contracts/cli-work-plugin.md: classify source,
// obtain its content, parse and fully validate plugin.json, resolve the
// alias and check for a conflict, check fallback-Starter uniqueness,
// stage-then-atomically-swap the content into pluginsDir/<alias> (the same
// primitives internal/bootstrap now uses for the embedded seed, research
// R1), and register one registry.Package plus one registry.Component per
// manifest component and one registry.Convention per manifest convention
// into reg, replacing whatever the alias registered before. On any failure reg is left completely untouched and nothing is
// written under pluginsDir (SC-005).
func Install(pluginsDir string, reg *registry.Registry, source string, opts Options) (Result, error) {
	local := isLocalSource(source)
	if opts.Link && !local {
		return Result{}, diag.New(diag.Usage, "--link requires a local SOURCE (a remote SOURCE is always pinned by content)")
	}

	if a := strings.TrimSpace(opts.Alias); a != "" {
		if err := plugin.ValidateAlias(a); err != nil {
			return Result{}, diag.Wrap(diag.PluginInvalid, err, "--as is not a valid alias")
		}
	}

	contentDir, origin, identity, reference, cleanup, err := obtainContent(source, local, opts.Link)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return Result{}, err
	}

	manifestBytes, err := os.ReadFile(filepath.Join(contentDir, "plugin.json"))
	if err != nil {
		return Result{}, diag.Wrapf(diag.PluginInvalid, err, "cannot read plugin.json at %s", source)
	}
	manifest, err := plugin.Parse(manifestBytes)
	if err != nil {
		return Result{}, diag.Wrap(diag.PluginInvalid, err, "plugin.json is invalid")
	}

	alias := strings.TrimSpace(opts.Alias)
	if alias == "" {
		alias = manifest.Name
	}

	explicitAlias := strings.TrimSpace(opts.Alias) != ""
	existing, installed := reg.PackageByAlias(alias)
	switch {
	case alias == ReferenceAlias:
		return Result{}, aliasConflict(manifest.Name, alias, explicitAlias, "the reference package", source)
	case installed && existingIdentity(existing) != identity:
		return Result{}, aliasConflict(manifest.Name, alias, explicitAlias, existing.Reference, source)
	}

	for _, c := range manifest.Components {
		if c.IsFallbackStarter() {
			if fb, ok := reg.StarterFallback(); ok && fb.Alias != alias {
				return Result{}, diag.Newf(diag.PluginFallbackConflict,
					"a fallback Starter is already registered by %q", fb.Alias).
					WithSummary("Only one fallback Starter may be registered at a time.").
					WithHint("Uninstall the existing fallback Starter first, or give this component a pattern.")
			}
		}
	}

	err = StageThenSwap(pluginsDir, alias, func(staging string) error {
		if opts.Link {
			return linkSource(staging, contentDir)
		}
		return copyTree(contentDir, sourceDir(staging))
	})
	if err != nil {
		return Result{}, diag.Wrap(diag.PluginInstallFailed, err, "cannot install the plugin package")
	}

	var conventionNames []string
	for _, cv := range manifest.Conventions {
		conventionNames = append(conventionNames, cv.Name)
	}
	pkg := registry.Package{Alias: alias, Origin: origin, Reference: reference, Conventions: conventionNames}
	reg.RemoveAlias(alias)
	reg.UpsertPackage(pkg)
	for _, c := range manifest.Components {
		entry := registry.Component{
			Alias:       alias,
			Name:        c.Name,
			Role:        c.Role,
			Entrypoint:  c.Entrypoint,
			Runtime:     c.Runtime,
			Pattern:     c.Pattern,
			Accepts:     c.Accepts,
			DisplayName: c.DisplayName,
			Description: c.Description,
		}
		if c.IsFallbackStarter() {
			entry.StarterLayer = registry.LayerFallback
		}
		reg.UpsertComponent(entry)
	}
	for _, cv := range manifest.Conventions {
		reg.UpsertConvention(registry.Convention{Name: cv.Name, Prefixes: cv.Prefixes})
	}

	return Result{Package: pkg, Components: manifest.Components}, nil
}

// obtainContent resolves source into a readable directory containing
// plugin.json (contentDir), the Package fields to record, and an origin
// identity used for alias-collision comparison (research R5): the absolute
// path for a local install, or the raw remote source string (never the
// pinned SHA) for a remote one. cleanup releases any temporary directory a
// remote clone allocated; it is nil for a local source.
func obtainContent(source string, local, link bool) (contentDir, origin, identity, reference string, cleanup func(), err error) {
	if local {
		abs, aerr := filepath.Abs(source)
		if aerr != nil {
			return "", "", "", "", nil, diag.Wrap(diag.PluginInstallFailed, aerr, "cannot resolve SOURCE")
		}
		if link {
			return abs, registry.OriginLocalLinked, abs, abs, nil, nil
		}
		return abs, registry.OriginLocalPinned, abs, abs, nil, nil
	}

	tmp, terr := os.MkdirTemp("", "work-plugin-clone-*")
	if terr != nil {
		return "", "", "", "", nil, diag.Wrap(diag.PluginInstallFailed, terr, "cannot create a temporary clone directory")
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	sha, cerr := gitx.Clone(source, tmp)
	if cerr != nil {
		return "", "", "", "", cleanup, diag.Wrap(diag.PluginInstallFailed, cerr, "cannot clone the plugin source")
	}
	if rerr := os.RemoveAll(filepath.Join(tmp, ".git")); rerr != nil {
		return "", "", "", "", cleanup, diag.Wrap(diag.PluginInstallFailed, rerr, "cannot prepare the cloned plugin")
	}
	return tmp, registry.OriginRemotePinned, source, source + "@" + sha, cleanup, nil
}

// existingIdentity extracts the origin identity (research R5) from an
// already-registered Package: the source URL with the pinned SHA suffix
// stripped for a remote package, or the reference verbatim (already an
// absolute path) for a local one.
func existingIdentity(p registry.Package) string {
	if p.Origin == registry.OriginRemotePinned {
		if i := strings.LastIndex(p.Reference, "@"); i >= 0 {
			return p.Reference[:i]
		}
	}
	return p.Reference
}

// scpLikeGitRef matches an scp-style git remote (e.g. "git@host:org/repo.git"):
// a bare user@host prefix followed by a colon, with no URL scheme.
var scpLikeGitRef = regexp.MustCompile(`^[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+:`)

// isLocalSource classifies source as a local filesystem path using the same
// rule seed/starter/main.go's looksLikePath already applies to work start's
// argument (a path separator, a "."/".."/"~"/drive-letter prefix, or an
// existing filesystem entry, research R4) — with one necessary refinement: a
// URL-scheme prefix ("https://", "ssh://", ...) or an scp-style git remote
// ("git@host:org/repo.git") is remote even though it contains a path
// separator, since a literal reuse of looksLikePath's separator-only rule
// would make every real git URL misclassify as local and remote install
// unreachable in practice.
func isLocalSource(source string) bool {
	if strings.Contains(source, "://") || scpLikeGitRef.MatchString(source) {
		return false
	}
	if strings.ContainsRune(source, '/') || strings.ContainsRune(source, '\\') {
		return true
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
		return true
	}
	if hasDriveLetterPrefix(source) {
		return true
	}
	if _, err := os.Stat(source); err == nil {
		return true
	}
	return false
}

// hasDriveLetterPrefix reports whether source starts with a Windows drive
// letter, e.g. "C:\repo" or "C:/repo".
func hasDriveLetterPrefix(source string) bool {
	if len(source) < 2 || source[1] != ':' {
		return false
	}
	c := source[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// copyTree recursively copies src into dst (created if necessary),
// preserving each entry's permission bits — critical for entrypoint
// executables, which must stay executable after a pinned local or remote
// install.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, lerr := os.Readlink(path)
			if lerr != nil {
				return lerr
			}
			return os.Symlink(linkTarget, target)
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if merr := os.MkdirAll(filepath.Dir(target), 0o755); merr != nil {
			return merr
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// aliasConflict builds the plugin-alias-conflict error. When the alias came
// from the manifest name the plugin itself is what collides; when the user
// proposed it with --as, the proposed alias is what collides. The existing
// package's location is deliberately kept out of the user-facing text and
// only retained in the debug message.
func aliasConflict(name, alias string, explicit bool, existingRef, source string) *diag.Error {
	err := diag.Newf(diag.PluginAliasConflict, "alias %q is already installed from %s", alias, existingRef)
	if explicit {
		return err.
			WithSummary(fmt.Sprintf("Plugin %q was not installed: the alias %q you proposed with --as conflicts with a plugin already installed under that alias.", name, alias)).
			WithHint(fmt.Sprintf("Choose a different alias: work plugin install %q --as <alias>", source))
	}
	return err.
		WithSummary(fmt.Sprintf("Plugin %q was not installed: its name collides with the name of a plugin already installed.", name)).
		WithHint(fmt.Sprintf("Install it under another name: work plugin install %q --as <alias>, or remove the existing plugin first: work plugin uninstall %s", source, alias))
}
