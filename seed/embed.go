// Package seed embeds the official reference package (its two component
// binaries, cross-compiled per platform, plus plugin.json) so that bootstrap
// can install it with no network and no interpreter dependency.
package seed

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
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

// ContentDigest is a stable sha256 over the host-platform binary pair and the
// manifest. Bootstrap compares it against the recorded install to decide
// whether a re-seed is needed.
func ContentDigest() (string, error) {
	starter, locator, err := HostAssets()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(starter)
	h.Write(locator)
	h.Write(manifestJSON)
	return hex.EncodeToString(h.Sum(nil)), nil
}
