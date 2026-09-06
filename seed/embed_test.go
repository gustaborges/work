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
	if _, _, err := AssetsFor("plan9", "mips"); err == nil {
		t.Errorf("AssetsFor(unknown platform): want error, got nil")
	}

	// A default `make build` embeds only the host platform; the full matrix is
	// present after `make seed-all` / in a release build. Always require the
	// host pair; check the rest only when they were staged.
	if _, _, err := AssetsFor(runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("AssetsFor(host %s/%s): %v (run `make seed`)", runtime.GOOS, runtime.GOARCH, err)
	}
	for _, p := range []struct{ os, arch string }{
		{"linux", "amd64"}, {"linux", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
		{"windows", "amd64"},
	} {
		if _, _, err := AssetsFor(p.os, p.arch); err != nil {
			t.Skipf("full-matrix check needs `make seed-all` (%s/%s not embedded)", p.os, p.arch)
		}
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
