package marketplace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gittest"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func stageDir(t *testing.T) string { return filepath.Join(t.TempDir(), "stage") }

func TestCaptureLocalFixesCatalogAndPackages(t *testing.T) {
	src := marketplaces.Copy(t, "multiple")
	stage := stageDir(t)
	snap, err := CaptureLocal(context.Background(), src, stage, CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	if snap.Source.Kind != pluginid.KindLocal || snap.Source.Source != src || snap.Source.Branch != "" {
		t.Errorf("source = %+v", snap.Source)
	}
	if snap.Revision != snap.Offer.CaptureDigest() {
		t.Error("a local revision is the capture digest")
	}
	if len(snap.Offer.Entries) != 3 {
		t.Fatalf("entries = %+v", snap.Offer.Entries)
	}
	// The capture is self-contained: it survives deleting the author's source.
	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}
	again, err := BuildOffer(dirView{root: stage})
	if err != nil || again.CatalogDigest != snap.Offer.CatalogDigest {
		t.Fatalf("capture is not self-contained: %v", err)
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := os.Stat(filepath.Join(stage, "plugins", name, "plugin.json")); err != nil {
			t.Errorf("package %s missing from capture: %v", name, err)
		}
	}
}

func TestCaptureLocalOmitsUnlistedSiblingsAndMetadata(t *testing.T) {
	src := marketplaces.Copy(t, "single")
	writeTree(t, src, map[string]string{
		"plugins/unlisted/plugin.json":       minimalManifest,
		"plugins/sample/.install-meta.json":  "{}",
		"plugins/sample/.admin-stage-x/file": "x",
		"README.md":                          "docs",
	})
	snap, err := CaptureLocal(context.Background(), src, stageDir(t), CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	for _, rel := range []string{"plugins/unlisted", "plugins/sample/.install-meta.json", "plugins/sample/.admin-stage-x", "README.md"} {
		if _, err := os.Lstat(filepath.Join(snap.Dir, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s leaked into the capture", rel)
		}
	}
}

func TestCaptureLocalDetectsMutationAtTheBoundary(t *testing.T) {
	mutations := map[string]func(src string){
		"edited file": func(src string) {
			writeTree(t, src, map[string]string{"plugins/alpha/bin/tool": "#!/bin/sh\necho changed\n"})
		},
		"added file":   func(src string) { writeTree(t, src, map[string]string{"plugins/beta/new.txt": "x"}) },
		"removed file": func(src string) { _ = os.Remove(filepath.Join(src, "plugins", "gamma", "bin", "tool")) },
		"edited catalog": func(src string) {
			writeTree(t, src, map[string]string{CatalogFile: `{"schema":1,"name":"demo","plugins":[]}`})
		},
		"chmod": func(src string) {
			_ = os.Chmod(filepath.Join(src, "plugins", "alpha", "bin", "tool"), 0o644)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			src := marketplaces.Copy(t, "multiple")
			stage := stageDir(t)
			snap, err := CaptureLocal(context.Background(), src, stage, CaptureOptions{afterCopy: func() { mutate(src) }})
			if err == nil {
				snap.Close()
				t.Fatal("a mixed capture was published")
			}
			if !errors.Is(err, ErrChanged) || !errors.Is(err, ErrSourceUnavailable) {
				t.Errorf("err = %v, want ErrChanged", err)
			}
			if _, statErr := os.Stat(stage); statErr == nil {
				t.Error("stage not cleaned after a failed capture")
			}
		})
	}
}

func TestCaptureLocalFailures(t *testing.T) {
	ctx := context.Background()
	if _, err := CaptureLocal(ctx, filepath.Join(t.TempDir(), "absent"), stageDir(t), CaptureOptions{}); !errors.Is(err, ErrSourceUnavailable) {
		t.Errorf("missing source = %v", err)
	}
	file := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(file, nil, 0o644)
	if _, err := CaptureLocal(ctx, file, stageDir(t), CaptureOptions{}); !errors.Is(err, ErrSourceUnavailable) {
		t.Errorf("file source = %v", err)
	}
	for _, name := range []string{"no-catalog", "invalid-schema", "invalid-name-mismatch"} {
		stage := stageDir(t)
		if _, err := CaptureLocal(ctx, marketplaces.Copy(t, name), stage, CaptureOptions{}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s = %v, want ErrInvalid", name, err)
		}
		if _, err := os.Stat(stage); err == nil {
			t.Errorf("%s left a stage behind", name)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	stage := stageDir(t)
	if _, err := CaptureLocal(cancelled, marketplaces.Copy(t, "multiple"), stage, CaptureOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled = %v", err)
	}
	if _, err := os.Stat(stage); err == nil {
		t.Error("cancelled capture left a stage behind")
	}
}

func TestCaptureLocalIsDeterministic(t *testing.T) {
	a, err := CaptureLocal(context.Background(), marketplaces.Copy(t, "multiple"), stageDir(t), CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := CaptureLocal(context.Background(), marketplaces.Copy(t, "multiple"), stageDir(t), CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.Revision != b.Revision {
		t.Error("equal sources captured in different locations must have equal digests")
	}
}

// gitSource commits a fixture catalog to a file:// remote.
func gitSource(t *testing.T, fixture, branch string) (dir, url string) {
	t.Helper()
	dir = marketplaces.Copy(t, fixture)
	gittest.Git(t, dir, "init", "-q", "-b", branch)
	gittest.Git(t, dir, "config", "commit.gpgsign", "false")
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "publish")
	return dir, "file://" + filepath.ToSlash(dir)
}

func head(t *testing.T, dir string) string { return gittest.Git(t, dir, "rev-parse", "HEAD") }

func TestOpenGitFixedRevision(t *testing.T) {
	dir, url := gitSource(t, "multiple", "main")
	first := head(t, dir)
	ctx := context.Background()

	branch, err := DiscoverBranch(ctx, url)
	if err != nil || branch != "main" {
		t.Fatalf("DiscoverBranch = %q, %v", branch, err)
	}
	snap, err := OpenGit(ctx, url, "main", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Revision != first || snap.Source != (Source{Kind: pluginid.KindGit, Source: url, Branch: "main"}) {
		t.Errorf("snapshot = %+v", snap)
	}
	if len(snap.Offer.Entries) != 3 || snap.Offer.Name != "demo" {
		t.Errorf("offer = %+v", snap.Offer)
	}
	snap.Close()

	// The tip moves; an exact request still gets the earlier commit.
	writeTree(t, dir, map[string]string{"plugins/alpha/extra.txt": "new"})
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "later")
	tip, err := OpenGit(ctx, url, "main", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer tip.Close()
	exact, err := OpenGit(ctx, url, "main", first, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer exact.Close()
	if exact.Revision != first || tip.Revision == first || exact.Offer.CatalogDigest == tip.Offer.CatalogDigest {
		t.Errorf("exact=%s tip=%s", exact.Revision, tip.Revision)
	}
}

func TestOpenGitFailuresAreClassifiedAndSafe(t *testing.T) {
	dir, url := gitSource(t, "multiple", "main")
	ctx := context.Background()
	bogus := "0123456789abcdef0123456789abcdef01234567"
	for name, call := range map[string]func() error{
		"unknown commit": func() error { _, err := OpenGit(ctx, url, "main", bogus, t.TempDir()); return err },
		"unknown branch": func() error { _, err := OpenGit(ctx, url, "nope", "", t.TempDir()); return err },
		"bad source": func() error {
			_, err := OpenGit(ctx, "file://"+filepath.ToSlash(t.TempDir()), "main", "", t.TempDir())
			return err
		},
	} {
		err := call()
		if !errors.Is(err, ErrSourceUnavailable) {
			t.Errorf("%s = %v", name, err)
			continue
		}
		if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "git ") {
			t.Errorf("%s leaks transport detail: %q", name, err)
		}
	}
	// An invalid catalog on a reachable branch is a catalog error, not an outage.
	_, badURL := gitSource(t, "invalid-schema", "main")
	if _, err := OpenGit(ctx, badURL, "main", "", t.TempDir()); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid catalog = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := OpenGit(cancelled, url, "main", "", t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled = %v", err)
	}
}

func TestOpenGitRejectsSubmodules(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	dir, url := gitSource(t, "single", "main")
	sub := t.TempDir()
	gittest.Git(t, sub, "init", "-q", "-b", "main")
	gittest.Git(t, sub, "commit", "-q", "--allow-empty", "-m", "s")
	gittest.Git(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", "file://"+filepath.ToSlash(sub), "vendor")
	gittest.Git(t, dir, "commit", "-q", "-m", "sub")
	if _, err := OpenGit(context.Background(), url, "main", "", t.TempDir()); !errors.Is(err, ErrInvalid) {
		t.Errorf("submodule = %v, want ErrInvalid", err)
	}
	_ = gitx.ErrUnsafeContent
}

func TestGitPackagesRecordExecutableBit(t *testing.T) {
	_, url := gitSource(t, "single", "main")
	snap, err := OpenGit(context.Background(), url, "main", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	local, err := CaptureLocal(context.Background(), marketplaces.Copy(t, "single"), stageDir(t), CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if snap.Offer.Entries[0].PackageDigest != local.Offer.Entries[0].PackageDigest {
		t.Error("the same committed package must digest identically from Git and from a local capture")
	}
}

func TestSnapshotCloseIsIdempotentAndDetachTransfersOwnership(t *testing.T) {
	snap, err := CaptureLocal(context.Background(), marketplaces.Copy(t, "single"), stageDir(t), CaptureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dir := snap.Dir
	snap.Detach()
	snap.Close()
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Close removed a detached directory: %v", err)
	}
	owned, _ := CaptureLocal(context.Background(), marketplaces.Copy(t, "single"), stageDir(t), CaptureOptions{})
	d := owned.Dir
	owned.Close()
	owned.Close()
	if _, err := os.Stat(d); err == nil {
		t.Error("Close left the capture behind")
	}
}
