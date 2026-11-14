package marketplace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func buildFixture(t *testing.T, name string) (*Offer, error) {
	t.Helper()
	return BuildOffer(dirView{root: marketplaces.Copy(t, name)})
}

func TestValidFixtures(t *testing.T) {
	cases := map[string]struct {
		name, display string
		entries       []string
	}{
		"single":        {"solo", "", []string{"sample"}},
		"multiple":      {"demo", "Demo Plugins", []string{"alpha", "beta", "gamma"}},
		"empty":         {"hollow", "", nil},
		"homonym-alpha": {"alpha-market", "", []string{"sample"}},
		"homonym-beta":  {"beta-market", "", []string{"sample"}},
	}
	for fixture, want := range cases {
		o, err := buildFixture(t, fixture)
		if err != nil {
			t.Errorf("%s: %v", fixture, err)
			continue
		}
		if o.Name != want.name || o.DisplayName != want.display || len(o.Entries) != len(want.entries) {
			t.Errorf("%s: offer = %+v", fixture, o)
		}
		for i, e := range o.Entries {
			if e.Name != want.entries[i] || e.Version != "1.0.0" || e.DigestAlgorithm != AlgorithmPackage || !strings.HasPrefix(e.PackageDigest, "sha256:") {
				t.Errorf("%s: entry %d = %+v", fixture, i, e)
			}
			if len(e.Components) != 1 || len(e.Conventions) != 1 {
				t.Errorf("%s: static contributions = %+v / %+v", fixture, e.Components, e.Conventions)
			}
		}
		if o.CatalogDigest == "" || o.CaptureDigest() == o.CatalogDigest {
			t.Errorf("%s: digests not derived", fixture)
		}
	}
}

func TestHomonymousOffersAreDistinguishedOnlyByNamespace(t *testing.T) {
	a, _ := buildFixture(t, "homonym-alpha")
	b, _ := buildFixture(t, "homonym-beta")
	if a.Entries[0].Name != b.Entries[0].Name || a.Entries[0].Components[0] != b.Entries[0].Components[0] {
		t.Fatal("fixtures must share the plugin and contribution names")
	}
	if a.Entries[0].PackageDigest != b.Entries[0].PackageDigest {
		t.Error("equal package content must have equal package digests regardless of marketplace")
	}
	if a.CatalogDigest == b.CatalogDigest {
		t.Error("different namespaces must have different catalog digests")
	}
}

func TestInvalidCatalogsRejectTheWholeOffer(t *testing.T) {
	for _, name := range []string{
		"invalid-schema", "invalid-unknown-field", "invalid-duplicate-key", "invalid-null",
		"invalid-escaping-path", "invalid-root-path", "invalid-absolute-path", "invalid-backslash-path",
		"invalid-reserved-name", "invalid-case-collision", "invalid-overlap", "invalid-case-overlap",
		"invalid-name-mismatch", "invalid-missing-manifest", "no-catalog",
	} {
		o, err := buildFixture(t, name)
		if err == nil || o != nil {
			t.Errorf("%s: accepted (%v)", name, o)
			continue
		}
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v does not classify as ErrInvalid", name, err)
		}
	}
}

func TestRootPluginJSONIsNeverAFallbackCatalog(t *testing.T) {
	dir := marketplaces.Copy(t, "no-catalog")
	if _, err := BuildOffer(dirView{root: dir}); err == nil || !strings.Contains(err.Error(), CatalogFile) {
		t.Fatalf("err = %v, want a missing-catalog error", err)
	}
}

func TestParseCatalogDetails(t *testing.T) {
	ok := func(doc string) *Catalog {
		t.Helper()
		c, err := ParseCatalog([]byte(doc))
		if err != nil {
			t.Fatalf("%s: %v", doc, err)
		}
		return c
	}
	bad := func(why, doc string) {
		t.Helper()
		if _, err := ParseCatalog([]byte(doc)); err == nil || !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: accepted or unclassified (%v)", why, err)
		}
	}
	if c := ok(`{"schema":1,"name":"t","display_name":"  Team  ","plugins":[]}`); c.DisplayName != "Team" || len(c.Plugins) != 0 {
		t.Errorf("display name not trimmed: %+v", c)
	}
	if c := ok(`{"plugins":[{"path":"a/b","name":"x","description":"d"}],"name":"t","schema":1}`); c.Plugins[0].Path != "a/b" || c.Plugins[0].Description != "d" {
		t.Errorf("key order must not matter: %+v", c)
	}
	bad("blank display name", `{"schema":1,"name":"t","display_name":"   ","plugins":[]}`)
	bad("missing plugins", `{"schema":1,"name":"t"}`)
	bad("plugins not an array", `{"schema":1,"name":"t","plugins":{}}`)
	bad("entry not an object", `{"schema":1,"name":"t","plugins":["x"]}`)
	bad("entry unknown field", `{"schema":1,"name":"t","plugins":[{"name":"a","path":"p","version":"1"}]}`)
	bad("entry missing path", `{"schema":1,"name":"t","plugins":[{"name":"a"}]}`)
	bad("non-string description", `{"schema":1,"name":"t","plugins":[{"name":"a","path":"p","description":5}]}`)
	bad("schema string", `{"schema":"1","name":"t","plugins":[]}`)
	bad("schema float", `{"schema":1.0,"name":"t","plugins":[]}`)
	bad("nested duplicate key", `{"schema":1,"name":"t","plugins":[{"name":"a","path":"p","path":"q"}]}`)
	bad("duplicate entries", `{"schema":1,"name":"t","plugins":[{"name":"a","path":"p"},{"name":"a","path":"q"}]}`)
	bad("not an object", `[]`)
	bad("trailing content", `{"schema":1,"name":"t","plugins":[]} {}`)
	bad("stray closing brace", `{"schema":1,"name":"t","plugins":[]}}`)
	bad("stray closing bracket", `{"schema":1,"name":"t","plugins":[]}]`)
	bad("empty", ``)
	bad("truncated", `{"schema":1,`)
	bad("bad namespace", `{"schema":1,"name":"a/b","plugins":[]}`)
	// A marketplace may publish the reserved name only through the code-owned provider; parsing is neutral.
	ok(`{"schema":1,"name":"builtin","plugins":[]}`)
}

func TestEntryPathRules(t *testing.T) {
	for _, p := range []string{"plugins/a", "a", "a/b/c", "My_Plugin.v2/x-y"} {
		if err := ValidateEntryPath(p); err != nil {
			t.Errorf("ValidateEntryPath(%q) = %v", p, err)
		}
	}
	for _, p := range []string{
		"", ".", "..", "../x", "a/../b", "a/./b", "/abs", "a/", "a//b", `a\b`, "C:/x", "c:x", "//host/share", "file:///x",
		"a\x00b", "a\nb", ".git", "a/.git", ".install-meta.json", ".admin-stage-x", "a/.hidden", "con", "a/NUL.txt", "a b",
	} {
		if err := ValidateEntryPath(p); err == nil {
			t.Errorf("ValidateEntryPath(%q) accepted", p)
		}
	}
}

// writeTree creates files (and, for "->" values, symlinks) below dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if target, ok := strings.CutPrefix(content, "->"); ok {
			if runtime.GOOS == "windows" {
				t.Skip("symlinks need privileges on Windows")
			}
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const minimalManifest = `{"name":"p","version":"1","components":[],"conventions":[]}`

func offerFor(t *testing.T, files map[string]string) (*Offer, error) {
	t.Helper()
	dir := t.TempDir()
	files[CatalogFile] = `{"schema":1,"name":"m","plugins":[{"name":"p","path":"plugins/p"}]}`
	writeTree(t, dir, files)
	return BuildOffer(dirView{root: dir})
}

func TestPackageContainment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{"plugins/p/plugin.json": minimalManifest}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	if _, err := offerFor(t, base(map[string]string{"plugins/p/data/real": "x", "plugins/p/alias": "->data/real"})); err != nil {
		t.Errorf("a contained relative link was rejected: %v", err)
	}
	for name, extra := range map[string]map[string]string{
		"escaping link":       {"plugins/p/out": "->../../outside"},
		"absolute link":       {"plugins/p/abs": "->/etc/passwd"},
		"dangling link":       {"plugins/p/dangling": "->nowhere"},
		"link cycle":          {"plugins/p/a": "->b", "plugins/p/b": "->a"},
		"self link":           {"plugins/p/self": "->self"},
		"escape via dir link": {"plugins/p/up": "->..", "plugins/p/leak": "->up/up/x"},
	} {
		if _, err := offerFor(t, base(extra)); err == nil || !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	// A symlinked manifest that leaves the package, or an entrypoint link that does.
	if _, err := offerFor(t, map[string]string{"plugins/p/plugin.json": "->../../manifest.json", "manifest.json": minimalManifest}); err == nil {
		t.Error("manifest link escaping the package accepted")
	}
	linked := `{"name":"p","version":"1","components":[{"name":"c","role":"starter","entrypoint":"bin"}],"conventions":[]}`
	if _, err := offerFor(t, map[string]string{"plugins/p/plugin.json": linked, "plugins/p/bin": "->../../tool", "tool": "x"}); err == nil {
		t.Error("entrypoint link escaping the package accepted")
	}
}

func TestAncestorsAndRootMustNotBeLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		CatalogFile:          `{"schema":1,"name":"m","plugins":[{"name":"p","path":"plugins/p"}]}`,
		"real/p/plugin.json": minimalManifest,
		"plugins":            "->real",
	})
	if _, err := BuildOffer(dirView{root: dir}); err == nil || !errors.Is(err, ErrInvalid) {
		t.Errorf("a linked ancestor directory was accepted: %v", err)
	}
	other := t.TempDir()
	writeTree(t, other, map[string]string{CatalogFile: `{"schema":1,"name":"m","plugins":[]}`})
	if err := os.Remove(filepath.Join(other, CatalogFile)); err != nil {
		t.Fatal(err)
	}
	writeTree(t, other, map[string]string{"real.json": `{"schema":1,"name":"m","plugins":[]}`, CatalogFile: "->real.json"})
	if _, err := BuildOffer(dirView{root: other}); err == nil {
		t.Error("a linked marketplace.json was accepted")
	}
}

func TestMissingEntrypointDoesNotBlockDiscovery(t *testing.T) {
	manifest := `{"name":"p","version":"1","components":[{"name":"c","role":"starter","entrypoint":"bin/absent","runtime":"nonexistent-interpreter"}],"conventions":[]}`
	o, err := offerFor(t, map[string]string{"plugins/p/plugin.json": manifest})
	if err != nil || len(o.Entries) != 1 {
		t.Fatalf("discovery must not require the runtime or entrypoint to exist: %v", err)
	}
	for _, bad := range []string{"../x", "/abs", `a\b`, "./a", "a/../b"} {
		escaping := strings.Replace(manifest, "bin/absent", strings.ReplaceAll(bad, `\`, `\\`), 1)
		if _, err := offerFor(t, map[string]string{"plugins/p/plugin.json": escaping}); err == nil {
			t.Errorf("entrypoint %q accepted", bad)
		}
	}
}

func TestSiblingsAndMetadataDoNotAffectPackageDigest(t *testing.T) {
	files := map[string]string{"plugins/p/plugin.json": minimalManifest, "plugins/p/data": "x"}
	a, err := offerFor(t, files)
	if err != nil {
		t.Fatal(err)
	}
	files["plugins/sibling/other.txt"] = "unrelated"
	files["plugins/p/.install-meta.json"] = "{}"
	files["plugins/p/.admin-stage-zzz/leftover"] = "x"
	files["plugins/p/.git/config"] = "x"
	b, err := offerFor(t, files)
	if err != nil {
		t.Fatal(err)
	}
	if a.Entries[0].PackageDigest != b.Entries[0].PackageDigest {
		t.Error("siblings or reserved metadata changed the package digest")
	}
	files["plugins/p/.hidden"] = "author dotfile"
	c, _ := offerFor(t, files)
	if c.Entries[0].PackageDigest == a.Entries[0].PackageDigest {
		t.Error("an arbitrary author dotfile must be part of the package content")
	}
}

func TestExecutableBitAffectsDigest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modes need Unix")
	}
	dir := marketplaces.Copy(t, "single")
	before, err := BuildOffer(dirView{root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "plugins", "sample", "bin", "tool"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := BuildOffer(dirView{root: dir})
	if before.Entries[0].PackageDigest == after.Entries[0].PackageDigest {
		t.Error("the recorded executable bit is not part of the digest")
	}
}

func TestMemoryTreeMatchesDiskTree(t *testing.T) {
	disk := t.TempDir()
	writeTree(t, disk, map[string]string{"plugin.json": minimalManifest, "bin/run": "#!x"})
	if err := os.Chmod(filepath.Join(disk, "bin", "run"), 0o755); err != nil && runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	dt, err := ReadTree(disk)
	if err != nil {
		t.Fatal(err)
	}
	mt := NewMemoryTree(map[string]MemFile{"plugin.json": {Content: []byte(minimalManifest)}, "bin/run": {Content: []byte("#!x"), Exec: runtime.GOOS != "windows"}})
	dd, _ := dt.Digest()
	md, _ := mt.Digest()
	if dd != md {
		t.Errorf("memory and disk trees disagree: %s vs %s", md, dd)
	}
	if err := mt.CopyTo(filepath.Join(t.TempDir(), "out")); err != nil {
		t.Errorf("CopyTo from memory: %v", err)
	}
}
