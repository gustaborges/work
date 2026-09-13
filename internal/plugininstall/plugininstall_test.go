package plugininstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func invalidManifestFixture(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures", "plugins", "invalid-manifest")
}

// localGitRemote initializes a git repository at dir containing src's tree
// plus one commit, and returns a "file://" URL to it — a source Install
// classifies as remote (it contains "://") and can clone with no network.
func localGitRemote(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, rerr := os.ReadFile(filepath.Join(src, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gc"),
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return "file://" + filepath.ToSlash(dir)
}

func wantToken(t *testing.T, err error, c diag.Category) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with token %q, got nil", c.Token)
	}
	if got := diag.Token(err); got != c.Token {
		t.Fatalf("token = %q, want %q (%v)", got, c.Token, err)
	}
}

func TestInstallLocalPinnedCopies(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginLocalPinned {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginLocalPinned)
	}
	if res.Package.Alias != "specific-starter" {
		t.Errorf("Alias = %q, want default manifest name", res.Package.Alias)
	}

	sourceLink := filepath.Join(plugins, "specific-starter", "source")
	info, err := os.Lstat(sourceLink)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("pinned install should copy, not symlink")
	}
	if _, err := os.Stat(filepath.Join(sourceLink, "plugin.json")); err != nil {
		t.Errorf("copied tree missing plugin.json: %v", err)
	}

	if len(reg.Components) != 1 || reg.Components[0].Alias != "specific-starter" {
		t.Errorf("components = %+v", reg.Components)
	}
	if len(reg.Conventions) != 1 || reg.Conventions[0].Name != "gitflow" {
		t.Errorf("conventions = %+v", reg.Conventions)
	}
}

func TestInstallLocalLinkedSymlinksContentNeverDuplicated(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{Link: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginLocalLinked {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginLocalLinked)
	}

	sourceLink := filepath.Join(plugins, "specific-starter", "source")
	info, err := os.Lstat(sourceLink)
	if err != nil {
		t.Fatalf("lstat source: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("--link install should create a symlink, got mode %v", info.Mode())
	}
	target, err := os.Readlink(sourceLink)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	absSrc, _ := filepath.Abs(src)
	if target != absSrc {
		t.Errorf("symlink target = %q, want %q", target, absSrc)
	}
}

func TestInstallLinkWithRemoteSourceRejectedBeforeIO(t *testing.T) {
	plugins := t.TempDir()
	reg := &registry.Registry{}
	_, err := Install(plugins, reg, "https://example.com/plugin.git", Options{Link: true})
	wantToken(t, err, diag.Usage)
	if len(reg.Packages) != 0 {
		t.Errorf("nothing should be registered: %+v", reg.Packages)
	}
	entries, _ := os.ReadDir(plugins)
	if len(entries) != 0 {
		t.Errorf("no I/O should have run: %v", entries)
	}
}

func TestInstallRemotePinnedRecordsClonedHEAD(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	remote := localGitRemote(t, src)
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, remote, Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Origin != registry.OriginRemotePinned {
		t.Errorf("Origin = %q, want %q", res.Package.Origin, registry.OriginRemotePinned)
	}
	if !strings.HasPrefix(res.Package.Reference, remote+"@") {
		t.Errorf("Reference = %q, want prefix %q", res.Package.Reference, remote+"@")
	}
	if strings.Contains(res.Package.Reference, "@@") || strings.HasSuffix(res.Package.Reference, "@") {
		t.Errorf("Reference = %q, malformed pin", res.Package.Reference)
	}

	sourceDirPath := filepath.Join(plugins, "specific-starter", "source")
	if _, err := os.Stat(filepath.Join(sourceDirPath, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git should not be staged: err=%v", err)
	}
}

func TestInstallDefaultAliasVsAs(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	res, err := Install(plugins, reg, src, Options{Alias: "custom-alias"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Package.Alias != "custom-alias" {
		t.Errorf("Alias = %q, want custom-alias", res.Package.Alias)
	}
	if _, ok := reg.PackageByAlias("custom-alias"); !ok {
		t.Errorf("custom-alias not registered")
	}
}

func TestInstallAliasConflictDifferentOriginRegistersNothing(t *testing.T) {
	srcA := fixtures.Prepare(t, "specific-starter")
	srcB := fixtures.Prepare(t, "colliding-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, srcA, Options{Alias: "x"}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	before := registrySnapshot(reg)

	_, err := Install(plugins, reg, srcB, Options{Alias: "x"})
	wantToken(t, err, diag.PluginAliasConflict)

	after := registrySnapshot(reg)
	if before != after {
		t.Errorf("registry mutated by a failed install:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestInstallSameOriginSameAliasIsIdempotent(t *testing.T) {
	src := fixtures.Prepare(t, "specific-starter")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if len(reg.Packages) != 1 {
		t.Errorf("packages = %+v, want exactly one entry", reg.Packages)
	}
}

func TestInstallFallbackConflictAtInstallTime(t *testing.T) {
	srcA := fixtures.Prepare(t, "fallback-starter-a")
	srcB := fixtures.Prepare(t, "fallback-starter-b")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, srcA, Options{}); err != nil {
		t.Fatalf("install fallback-starter-a: %v", err)
	}
	before := registrySnapshot(reg)

	_, err := Install(plugins, reg, srcB, Options{})
	wantToken(t, err, diag.PluginFallbackConflict)

	after := registrySnapshot(reg)
	if before != after {
		t.Errorf("registry mutated by a failed install:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestInstallReinstallingOwnFallbackIsNotAConflict(t *testing.T) {
	src := fixtures.Prepare(t, "fallback-starter-a")
	plugins := t.TempDir()
	reg := &registry.Registry{}

	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := Install(plugins, reg, src, Options{}); err != nil {
		t.Fatalf("reinstalling own fallback: %v", err)
	}
}

func TestInstallInvalidManifestRegistersNothing(t *testing.T) {
	plugins := t.TempDir()
	reg := &registry.Registry{}

	_, err := Install(plugins, reg, invalidManifestFixture(t), Options{})
	wantToken(t, err, diag.PluginInvalid)
	if len(reg.Packages) != 0 || len(reg.Components) != 0 {
		t.Errorf("nothing should be registered: packages=%+v components=%+v", reg.Packages, reg.Components)
	}
	if _, err := os.Stat(filepath.Join(plugins, "invalid-manifest")); !os.IsNotExist(err) {
		t.Errorf("nothing should be staged: err=%v", err)
	}
}

func registrySnapshot(reg *registry.Registry) string {
	var b strings.Builder
	for _, p := range reg.ListPackages() {
		fmt.Fprintf(&b, "%s|%s|%s\n", p.Alias, p.Origin, p.Reference)
	}
	for _, c := range reg.Components {
		fmt.Fprintf(&b, "%s/%s\n", c.Alias, c.Name)
	}
	return b.String()
}
