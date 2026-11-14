package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

//go:embed all:work-plugin-builder
var assets embed.FS

func Names() []string { return []string{"work-plugin-builder"} }
func Tree(name string) (fs.FS, error) {
	if name != "work-plugin-builder" {
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
