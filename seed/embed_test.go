package seed

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
)

type releasePlatform struct{ os, arch string }

var releasePlatforms = []releasePlatform{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"windows", "amd64"},
}

type releaseMatrixStatus struct {
	hostErr  error
	hostOnly bool
	missing  []string
	empty    []string
}

func inspectReleaseMatrix(hostOS, hostArch string, assets func(string, string) ([]byte, []byte, error)) releaseMatrixStatus {
	status := releaseMatrixStatus{hostOnly: true}
	for _, p := range releasePlatforms {
		starter, locator, err := assets(p.os, p.arch)
		if p.os == hostOS && p.arch == hostArch && err != nil {
			status.hostErr = err
		}
		if err != nil {
			status.missing = append(status.missing, fmt.Sprintf("%s/%s", p.os, p.arch))
			continue
		}
		if p.os != hostOS || p.arch != hostArch {
			status.hostOnly = false
		}
		if len(starter) == 0 || len(locator) == 0 {
			status.empty = append(status.empty, fmt.Sprintf("%s/%s", p.os, p.arch))
		}
	}
	return status
}

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
	status := inspectReleaseMatrix(runtime.GOOS, runtime.GOARCH, AssetsFor)
	if status.hostErr != nil {
		t.Fatalf("AssetsFor(host %s/%s): %v (run `make seed`)", runtime.GOOS, runtime.GOARCH, status.hostErr)
	}
	if status.hostOnly {
		t.Skip("full-matrix check needs `make seed-all`")
	}
	for _, platform := range status.missing {
		t.Errorf("full-matrix seed is missing %s (run `make seed-all`)", platform)
	}
	for _, platform := range status.empty {
		t.Errorf("AssetsFor(%s): empty asset", platform)
	}
}

func TestInspectReleaseMatrix(t *testing.T) {
	key := func(goos, goarch string) string { return goos + "/" + goarch }
	assets := func(staged map[string][2][]byte) func(string, string) ([]byte, []byte, error) {
		return func(goos, goarch string) ([]byte, []byte, error) {
			pair, ok := staged[key(goos, goarch)]
			if !ok {
				return nil, nil, errors.New("not staged")
			}
			return pair[0], pair[1], nil
		}
	}
	host := key("linux", "amd64")
	nonHost := key("darwin", "arm64")

	t.Run("host only", func(t *testing.T) {
		status := inspectReleaseMatrix("linux", "amd64", assets(map[string][2][]byte{
			host: {[]byte("starter"), []byte("locator")},
		}))
		if !status.hostOnly || status.hostErr != nil {
			t.Fatalf("status = %+v, want host-only with host present", status)
		}
	})

	t.Run("partial matrix fails", func(t *testing.T) {
		status := inspectReleaseMatrix("linux", "amd64", assets(map[string][2][]byte{
			host:    {[]byte("starter"), []byte("locator")},
			nonHost: {[]byte("starter"), []byte("locator")},
		}))
		if status.hostOnly || len(status.missing) != len(releasePlatforms)-2 {
			t.Fatalf("status = %+v, want partial matrix failures", status)
		}
	})

	t.Run("empty asset fails", func(t *testing.T) {
		status := inspectReleaseMatrix("linux", "amd64", assets(map[string][2][]byte{
			host:    {[]byte("starter"), []byte("locator")},
			nonHost: {nil, []byte("locator")},
		}))
		if status.hostOnly || len(status.empty) != 1 || status.empty[0] != nonHost {
			t.Fatalf("status = %+v, want empty %s", status, nonHost)
		}
	})
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
