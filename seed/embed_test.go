package seed

import (
	"runtime"
	"testing"
)

func TestHostAssetsPresent(t *testing.T) {
	starter, locator, err := HostAssets()
	if err != nil {
		t.Fatalf("HostAssets: %v (run `make seed`)", err)
	}
	if len(starter) == 0 || len(locator) == 0 {
		t.Errorf("empty asset: starter=%d locator=%d", len(starter), len(locator))
	}
}

func TestAssetsForEveryReleasePlatform(t *testing.T) {
	platforms := []struct{ os, arch string }{
		{"linux", "amd64"}, {"linux", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
		{"windows", "amd64"},
	}
	for _, p := range platforms {
		if _, _, err := AssetsFor(p.os, p.arch); err != nil {
			t.Errorf("AssetsFor(%s/%s): %v", p.os, p.arch, err)
		}
	}
	if _, _, err := AssetsFor("plan9", "mips"); err == nil {
		t.Errorf("AssetsFor(unknown): want error")
	}
}

func TestContentDigestStable(t *testing.T) {
	d1, err := ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := ContentDigest()
	if d1 != d2 || len(d1) != 64 {
		t.Errorf("digest unstable or wrong length: %q / %q", d1, d2)
	}
}

func TestManifestJSONParses(t *testing.T) {
	if len(ManifestJSON()) == 0 {
		t.Fatal("empty manifest")
	}
	// A returned copy must not alias the embedded bytes.
	b := ManifestJSON()
	b[0] = 'X'
	if ManifestJSON()[0] == 'X' {
		t.Errorf("ManifestJSON returns a mutable alias")
	}
	_ = runtime.GOOS
}
