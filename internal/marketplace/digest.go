// Package marketplace defines the strict, passive marketplace model: the
// published catalog schema, fixed source views (local capture, exact Git
// commit, builtin), canonical digests and the stored discovery offer. It never
// executes plugin code and never installs anything; installation consumes the
// immutable entries it produces.
package marketplace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// Digest algorithm names persisted beside every digest they produced.
const (
	AlgorithmPackage = "work-package-v1"
	AlgorithmCatalog = "work-catalog-v1"
	AlgorithmCapture = "work-capture-v1"
)

const digestPrefix = "sha256:"

// framed appends F(s): the ASCII decimal byte length, ':' and the bytes. Length
// framing keeps every field boundary unambiguous without any escaping.
func framed(b *bytes.Buffer, s []byte) {
	b.WriteString(strconv.Itoa(len(s)))
	b.WriteByte(':')
	b.Write(s)
}

func framedString(b *bytes.Buffer, s string) { framed(b, []byte(s)) }

func sum(b *bytes.Buffer) string {
	h := sha256.Sum256(b.Bytes())
	return digestPrefix + hex.EncodeToString(h[:])
}

// HexPayload strips the algorithm prefix from a public digest, producing the
// only form permitted as a capture directory name.
func HexPayload(digest string) (string, bool) {
	return strings.CutPrefix(digest, digestPrefix)
}

// PackageEntry is one distributable node of a package: a normalized relative
// POSIX path, its kind (dir, file or link), the recorded executable bit and its
// content (file bytes or exact link text; empty for directories).
type PackageEntry struct {
	Path    string
	Kind    string
	Exec    bool
	Content []byte
}

func packageCanonical(entries []PackageEntry) *bytes.Buffer {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(a, b PackageEntry) int { return strings.Compare(a.Path, b.Path) })
	var b bytes.Buffer
	b.WriteString(AlgorithmPackage + "\n")
	for _, e := range sorted {
		framedString(&b, e.Path)
		framedString(&b, e.Kind)
		if e.Exec {
			framedString(&b, "1")
		} else {
			framedString(&b, "0")
		}
		framed(&b, e.Content)
	}
	return &b
}

// PackageDigest is the work-package-v1 digest. It covers a package's own
// content only: never siblings, marketplace metadata, provenance or config.
func PackageDigest(entries []PackageEntry) string { return sum(packageCanonical(entries)) }

// OfferEntry is the digest-relevant part of one catalog entry.
type OfferEntry struct {
	Name          string
	Path          string
	Description   string
	PackageDigest string
}

func sortedByName(entries []OfferEntry) []OfferEntry {
	out := slices.Clone(entries)
	slices.SortFunc(out, func(a, b OfferEntry) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func catalogCanonical(name, displayName, description string, entries []OfferEntry) *bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(AlgorithmCatalog + "\n")
	framedString(&b, "1")
	framedString(&b, name)
	framedString(&b, strings.TrimSpace(displayName))
	framedString(&b, description)
	framedString(&b, strconv.Itoa(len(entries)))
	for _, e := range sortedByName(entries) {
		framedString(&b, e.Name)
		framedString(&b, e.Path)
		framedString(&b, e.Description)
		framedString(&b, e.PackageDigest)
	}
	return &b
}

// CatalogDigest is the work-catalog-v1 digest. Meaningful display metadata and
// every package digest participate; object-key and entry ordering do not.
func CatalogDigest(name, displayName, description string, entries []OfferEntry) string {
	return sum(catalogCanonical(name, displayName, description, entries))
}

func captureCanonical(catalogDigest string, entries []OfferEntry) *bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(AlgorithmCapture + "\n")
	framedString(&b, catalogDigest)
	framedString(&b, strconv.Itoa(len(entries)))
	for _, e := range sortedByName(entries) {
		framedString(&b, e.Name)
		framedString(&b, e.Path)
		framedString(&b, e.PackageDigest)
	}
	return &b
}

// CaptureDigest is the work-capture-v1 digest: the fixed catalog plus every
// distributable package, without source, config or transport detail.
func CaptureDigest(catalogDigest string, entries []OfferEntry) string {
	return sum(captureCanonical(catalogDigest, entries))
}
