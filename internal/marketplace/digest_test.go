package marketplace

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type vectorEntry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable"`
	ContentHex string `json:"content_hex"`
}

type vector struct {
	Name         string        `json:"name"`
	Algorithm    string        `json:"algorithm"`
	Entries      []vectorEntry `json:"entries"`
	Fields       []string      `json:"fields"`
	CanonicalHex string        `json:"canonical_hex"`
	Digest       string        `json:"digest"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "specs", "009-marketplace-distribution", "contracts", "digest-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema  int      `json:"schema"`
		Vectors []vector `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Schema != 1 || len(doc.Vectors) == 0 {
		t.Fatalf("vectors: %v", err)
	}
	return doc.Vectors
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCanonicalVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			var canonical *bytes.Buffer
			var digest string
			switch v.Algorithm {
			case AlgorithmPackage:
				entries := []PackageEntry{}
				for _, e := range v.Entries {
					entries = append(entries, PackageEntry{Path: e.Path, Kind: e.Kind, Exec: e.Executable, Content: decodeHex(t, e.ContentHex)})
				}
				canonical, digest = packageCanonical(entries), PackageDigest(entries)
			case AlgorithmCatalog:
				f := v.Fields
				// fields: schema, name, display, description, count, then name/path/description/digest per entry.
				entries := []OfferEntry{{Name: f[5], Path: f[6], Description: f[7], PackageDigest: f[8]}}
				canonical, digest = catalogCanonical(f[1], f[2], f[3], entries), CatalogDigest(f[1], f[2], f[3], entries)
			case AlgorithmCapture:
				f := v.Fields
				entries := []OfferEntry{{Name: f[2], Path: f[3], PackageDigest: f[4]}}
				canonical, digest = captureCanonical(f[0], entries), CaptureDigest(f[0], entries)
			default:
				t.Fatalf("unknown algorithm %q", v.Algorithm)
			}
			if got := hex.EncodeToString(canonical.Bytes()); got != v.CanonicalHex {
				t.Errorf("canonical bytes\n got %s\nwant %s", got, v.CanonicalHex)
			}
			if digest != v.Digest {
				t.Errorf("digest = %s, want %s", digest, v.Digest)
			}
		})
	}
}

func TestPackageDigestIsOrderIndependentAndContentSensitive(t *testing.T) {
	a := PackageEntry{Path: "a", Kind: "file", Content: []byte("1")}
	b := PackageEntry{Path: "b", Kind: "file", Content: []byte("2")}
	if PackageDigest([]PackageEntry{a, b}) != PackageDigest([]PackageEntry{b, a}) {
		t.Error("entry order changed the digest")
	}
	exec := b
	exec.Exec = true
	if PackageDigest([]PackageEntry{a, b}) == PackageDigest([]PackageEntry{a, exec}) {
		t.Error("executable bit ignored")
	}
	// Length framing: moving a byte across a field boundary must change the digest.
	x := PackageDigest([]PackageEntry{{Path: "ab", Kind: "file", Content: []byte("c")}})
	y := PackageDigest([]PackageEntry{{Path: "a", Kind: "file", Content: []byte("bc")}})
	if x == y {
		t.Error("field boundaries are ambiguous")
	}
}

func TestCatalogDigestSemantics(t *testing.T) {
	entries := []OfferEntry{{Name: "b", Path: "p/b", PackageDigest: "sha256:2"}, {Name: "a", Path: "p/a", PackageDigest: "sha256:1"}}
	rev := []OfferEntry{entries[1], entries[0]}
	base := CatalogDigest("team", "Team", "d", entries)
	if base != CatalogDigest("team", "Team", "d", rev) {
		t.Error("entry order changed the catalog digest")
	}
	if base != CatalogDigest("team", "  Team  ", "d", entries) {
		t.Error("display name is trimmed before hashing")
	}
	if base == CatalogDigest("team", "Other", "d", entries) || base == CatalogDigest("team", "Team", "e", entries) {
		t.Error("display metadata must affect the digest")
	}
	moved := []OfferEntry{entries[0], {Name: "a", Path: "q/a", PackageDigest: "sha256:1"}}
	if base == CatalogDigest("team", "Team", "d", moved) {
		t.Error("a path move must affect the catalog digest")
	}
	if CaptureDigest(base, entries) != CaptureDigest(base, rev) {
		t.Error("capture digest is order dependent")
	}
	if CaptureDigest(base, entries) == CaptureDigest(CatalogDigest("team", "Other", "d", entries), entries) {
		t.Error("capture digest ignores the catalog digest")
	}
}

func TestHexPayload(t *testing.T) {
	if h, ok := HexPayload("sha256:abc"); !ok || h != "abc" {
		t.Errorf("HexPayload = %q, %v", h, ok)
	}
	if _, ok := HexPayload("md5:abc"); ok {
		t.Error("foreign prefix accepted")
	}
}
