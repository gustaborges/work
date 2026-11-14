package workhome

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/pluginid"
)

func TestComposedStorageResolution(t *testing.T) {
	h := At(filepath.Join(t.TempDir(), "home"))
	p := pluginid.Provider{Marketplace: "team", Plugin: "github"}
	root, err := h.PackageRoot(p)
	if err != nil || root != filepath.Join(h.PluginsDir(), "team", "github") {
		t.Fatalf("PackageRoot = %q, %v", root, err)
	}
	src, _ := h.PackageSource(p)
	if src != filepath.Join(root, "source") {
		t.Fatalf("PackageSource = %q", src)
	}
	for _, bad := range []pluginid.Provider{
		{}, {Marketplace: "team"}, {Plugin: "x"}, {Marketplace: "..", Plugin: "x"},
		{Marketplace: "team", Plugin: "a/b"}, {Marketplace: "t/m", Plugin: "x"}, {Marketplace: ".admin-stage-x", Plugin: "x"},
		{Marketplace: "team", Plugin: "x.old"}, {Marketplace: "con", Plugin: "x"},
	} {
		if _, err := h.PackageRoot(bad); err == nil {
			t.Errorf("PackageRoot(%+v) accepted", bad)
		}
		if _, err := h.PackageSource(bad); err == nil {
			t.Errorf("PackageSource(%+v) accepted", bad)
		}
	}
	if _, err := h.MarketplaceDir("a/b"); err == nil {
		t.Error("MarketplaceDir accepted a separator")
	}
}

func TestSnapshotDirIsHexOnly(t *testing.T) {
	h := At(t.TempDir())
	good := strings.Repeat("ab", 32)
	dir, err := h.SnapshotDir(good)
	if err != nil || dir != filepath.Join(h.SnapshotsDir(), good) {
		t.Fatalf("SnapshotDir = %q, %v", dir, err)
	}
	for _, bad := range []string{"", "sha256:" + good, good[:62], "../" + good[3:], strings.ToUpper(good), good + "/x"} {
		if _, err := h.SnapshotDir(bad); err == nil {
			t.Errorf("SnapshotDir(%q) accepted", bad)
		}
	}
}

func TestBackupPathEncodesTypedAddress(t *testing.T) {
	h := At(t.TempDir())
	a, err := h.BackupPath("tx1", pluginid.Provider{Marketplace: "m", Plugin: "p"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := h.BackupPath("tx1", pluginid.Provider{Marketplace: "p", Plugin: "m"})
	if a == b || filepath.Dir(a) != h.PluginsDir() || !strings.Contains(filepath.Base(a), "p@m") {
		t.Fatalf("backup addresses collide or escape: %q %q", a, b)
	}
	if _, err := h.BackupPath("../x", pluginid.Provider{Marketplace: "m", Plugin: "p"}); err == nil {
		t.Error("unsafe transaction id accepted")
	}
	if !IsStageName(".admin-stage-abc") || IsStageName(".admin-stage-") || IsStageName("team") || IsStageName("a/.admin-stage-x") {
		t.Error("IsStageName misclassifies")
	}
}
