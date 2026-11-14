// Package skills owns the release-bundled, vendor-neutral authoring assets.
package skills

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const Algorithm = "work-skill-tree-v1"

// TreeDigest hashes portable paths and content, excluding only the root marker.
func TreeDigest(tree fs.FS) (string, error) {
	type record struct {
		name string
		dir  bool
		data []byte
	}
	records := map[string]record{}
	err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if !portablePath(p) {
			return fmt.Errorf("unsafe asset path %q", p)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe asset node %q", p)
		}
		if d.IsDir() {
			return nil
		}
		if p == ".work-skill.json" {
			return nil
		}
		data, err := fs.ReadFile(tree, p)
		if err != nil {
			return err
		}
		records[p] = record{p, false, data}
		for parent := path.Dir(p); parent != "."; parent = path.Dir(parent) {
			records[parent] = record{parent, true, nil}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(records))
	for n := range records {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	h.Write([]byte(Algorithm + "\x00"))
	var size [8]byte
	for _, n := range names {
		r := records[n]
		binary.BigEndian.PutUint64(size[:], uint64(len(n)))
		h.Write(size[:])
		h.Write([]byte(n))
		kind := byte('f')
		if r.dir {
			kind = 'd'
		}
		h.Write([]byte{kind})
		binary.BigEndian.PutUint64(size[:], uint64(len(r.data)))
		h.Write(size[:])
		h.Write(r.data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// portablePath rejects path spellings that alias or cannot exist on Windows.
func portablePath(p string) bool {
	if !fs.ValidPath(p) || !utf8.ValidString(p) || strings.ContainsAny(p, "\\:\x00<>\"|?*") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return false
		}
		for _, r := range segment {
			if r < 32 {
				return false
			}
		}
		stem := strings.ToLower(strings.SplitN(segment, ".", 2)[0])
		if stem == "con" || stem == "prn" || stem == "aux" || stem == "nul" {
			return false
		}
		if len(stem) == 4 && (strings.HasPrefix(stem, "com") || strings.HasPrefix(stem, "lpt")) && stem[3] >= '1' && stem[3] <= '9' {
			return false
		}
	}
	return true
}
