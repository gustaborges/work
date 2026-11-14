package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

//go:embed all:work-*
var assets embed.FS

// Names is the release inventory, in deterministic name order.
func Names() []string {
	entries, _ := assets.ReadDir(".")
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}
func Tree(name string) (fs.FS, error) {
	if !slices.Contains(Names(), name) {
		return nil, fmt.Errorf("unknown skill %q", name)
	}
	tree, err := fs.Sub(assets, name)
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if path.Base(p) == "go.mod" {
				return fmt.Errorf("nested module")
			}
			b, e := fs.ReadFile(tree, p)
			if e != nil {
				return e
			}
			if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
				return fmt.Errorf("non-text asset %s", p)
			}
		}
		return nil
	})
	return tree, err
}
func Digest(name string) (string, error) {
	tree, err := Tree(name)
	if err != nil {
		return "", err
	}
	return TreeDigest(tree)
}
