// Package seed embeds the official reference package (its two component
// binaries, cross-compiled per platform, plus plugin.json) so that bootstrap
// can install it with no network and no interpreter dependency.
package seed

import (
	"embed"
	"fmt"
	"runtime"
)

// dist holds seed/dist/<goos>_<goarch>/{starter,locator}[.exe], populated by
// `make seed`.
//
//go:embed dist
var dist embed.FS

//go:embed manifest/plugin.json
var manifestJSON []byte

//go:embed marketplace.json
var marketplaceJSON []byte

// ReferencePath is the catalog path of the reference package inside the
// included marketplace.
const ReferencePath = "plugins/work-reference"

// MarketplaceJSON returns the included marketplace's catalog bytes. The
// catalog is code-owned: it is never extracted into editable configuration.
func MarketplaceJSON() []byte {
	out := make([]byte, len(marketplaceJSON))
	copy(out, marketplaceJSON)
	return out
}

// PackageFile is one file of an embedded package.
type PackageFile struct {
	Content []byte
	Exec    bool
}

// Package returns the host-platform files of the embedded package at a catalog
// path: plugin.json plus the native starter and locator, with the executable
// bit recorded for the binaries.
func Package(path string) (map[string]PackageFile, error) {
	if path != ReferencePath {
		return nil, fmt.Errorf("seed: no embedded package at %q", path)
	}
	starter, locator, err := HostAssets()
	if err != nil {
		return nil, err
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return map[string]PackageFile{
		"plugin.json":   {Content: ManifestJSON()},
		"starter" + ext: {Content: starter, Exec: true},
		"locator" + ext: {Content: locator, Exec: true},
	}, nil
}

// ManifestJSON returns the seed plugin.json bytes.
func ManifestJSON() []byte {
	out := make([]byte, len(manifestJSON))
	copy(out, manifestJSON)
	return out
}

// AssetsFor returns the starter and locator binaries for a target platform.
func AssetsFor(goos, goarch string) (starter, locator []byte, err error) {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	base := fmt.Sprintf("dist/%s_%s", goos, goarch)

	starter, err = dist.ReadFile(base + "/starter" + ext)
	if err != nil {
		return nil, nil, fmt.Errorf("seed: no embedded starter for %s/%s: %w", goos, goarch, err)
	}
	locator, err = dist.ReadFile(base + "/locator" + ext)
	if err != nil {
		return nil, nil, fmt.Errorf("seed: no embedded locator for %s/%s: %w", goos, goarch, err)
	}
	return starter, locator, nil
}

// HostAssets returns the binaries for the running platform.
func HostAssets() (starter, locator []byte, err error) {
	return AssetsFor(runtime.GOOS, runtime.GOARCH)
}
