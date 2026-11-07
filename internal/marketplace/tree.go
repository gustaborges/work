package marketplace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Node kinds of a package tree.
const (
	KindDir  = "dir"
	KindFile = "file"
	KindLink = "link"
)

// Node is one entry of a package tree. Exec is the recorded semantic
// executable bit (from the Git tree mode or the local file mode), which is
// carried even on hosts that cannot represent it.
type Node struct {
	Path string // normalized relative POSIX path
	Kind string
	Exec bool
	Link string // exact link text for KindLink
}

// Tree is a package directory on disk plus the recorded facts about its nodes.
type Tree struct {
	Root  string
	Nodes []Node // sorted by Path; the root itself is excluded

	mem map[string][]byte // file content for in-memory trees (builtin distribution)
}

// MemFile is one regular file of an in-memory tree.
type MemFile struct {
	Content []byte
	Exec    bool
}

// NewMemoryTree builds a tree whose content lives in memory, so projecting the
// embedded distribution never touches the filesystem. Parent directories are
// implied by file paths.
func NewMemoryTree(files map[string]MemFile) *Tree {
	t := &Tree{mem: map[string][]byte{}}
	dirs := map[string]bool{}
	for p, f := range files {
		t.mem[p] = f.Content
		t.Nodes = append(t.Nodes, Node{Path: p, Kind: KindFile, Exec: f.Exec})
		for d := path.Dir(p); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	for d := range dirs {
		t.Nodes = append(t.Nodes, Node{Path: d, Kind: KindDir})
	}
	slices.SortFunc(t.Nodes, func(a, b Node) int { return strings.Compare(a.Path, b.Path) })
	return t
}

// content returns the bytes of the regular file at the tree-relative path.
func (t *Tree) content(rel string) ([]byte, error) {
	if t.mem != nil {
		b, ok := t.mem[rel]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return b, nil
	}
	return os.ReadFile(filepath.Join(t.Root, filepath.FromSlash(rel)))
}

// IsReservedName reports distribution metadata and Work-owned artifacts that
// are never part of distributable package content. Arbitrary author dotfiles
// are not reserved.
func IsReservedName(name string) bool {
	return name == ".git" || name == ".install-meta.json" || strings.HasPrefix(name, ".admin-")
}

// ReadTree enumerates root without following symlinks. Reserved metadata is
// skipped; sockets, devices and other special files are rejected.
func ReadTree(root string) (*Tree, error) {
	t := &Tree{Root: root}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		if IsReservedName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		n := Node{Path: filepath.ToSlash(rel)}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			n.Kind = KindLink
			if n.Link, err = os.Readlink(p); err != nil {
				return err
			}
		case info.IsDir():
			n.Kind = KindDir
		case info.Mode().IsRegular():
			n.Kind = KindFile
			n.Exec = runtime.GOOS != "windows" && info.Mode().Perm()&0o111 != 0
		default:
			return fmt.Errorf("%s: special files are not distributable", n.Path)
		}
		t.Nodes = append(t.Nodes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(t.Nodes, func(a, b Node) int { return strings.Compare(a.Path, b.Path) })
	return t, nil
}

// Entries reads node content from disk and returns the digest input. Links
// contribute their exact link text and are never followed.
func (t *Tree) Entries() ([]PackageEntry, error) {
	out := make([]PackageEntry, 0, len(t.Nodes))
	for _, n := range t.Nodes {
		e := PackageEntry{Path: n.Path, Kind: n.Kind, Exec: n.Exec}
		switch n.Kind {
		case KindFile:
			b, err := t.content(n.Path)
			if err != nil {
				return nil, err
			}
			e.Content = b
		case KindLink:
			e.Content = []byte(n.Link)
		}
		out = append(out, e)
	}
	return out, nil
}

// Digest is the work-package-v1 digest of the tree.
func (t *Tree) Digest() (string, error) {
	entries, err := t.Entries()
	if err != nil {
		return "", err
	}
	return PackageDigest(entries), nil
}

func (t *Tree) index() map[string]Node {
	m := make(map[string]Node, len(t.Nodes))
	for _, n := range t.Nodes {
		m[n.Path] = n
	}
	return m
}

const maxLinkHops = 40

var (
	errEscape   = errors.New("link escapes the package")
	errDangling = errors.New("dangling path")
)

// resolve follows links inside the tree like realpath. Any component that
// would climb above the package root, any dangling target and any cycle is an
// error, so a validated package is fully self-contained.
func resolve(nodes map[string]Node, p string) (Node, error) {
	pending := strings.Split(p, "/")
	var cur []string
	hops := 0
	for len(pending) > 0 {
		c := pending[0]
		pending = pending[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			if len(cur) == 0 {
				return Node{}, errEscape
			}
			cur = cur[:len(cur)-1]
			continue
		}
		cur = append(cur, c)
		n, ok := nodes[strings.Join(cur, "/")]
		if !ok {
			return Node{}, fmt.Errorf("%s: %w", p, errDangling)
		}
		if n.Kind == KindLink {
			if hops++; hops > maxLinkHops {
				return Node{}, fmt.Errorf("%s: link cycle", p)
			}
			cur = cur[:len(cur)-1]
			pending = append(strings.Split(n.Link, "/"), pending...)
			continue
		}
		if n.Kind == KindFile && len(pending) > 0 {
			return Node{}, fmt.Errorf("%s: path continues through a file", p)
		}
	}
	if len(cur) == 0 {
		return Node{Kind: KindDir}, nil
	}
	return nodes[strings.Join(cur, "/")], nil
}

// ValidateLinks proves every link is relative, portable and resolves to an
// existing node inside the package without cycles.
func (t *Tree) ValidateLinks() error {
	idx := t.index()
	for _, n := range t.Nodes {
		if n.Kind != KindLink {
			continue
		}
		if err := validateLinkText(n.Link); err != nil {
			return fmt.Errorf("%s: %w", n.Path, err)
		}
		target := path.Join(path.Dir(n.Path), n.Link)
		if target == ".." || strings.HasPrefix(target, "../") {
			return fmt.Errorf("%s: %w", n.Path, errEscape)
		}
		if _, err := resolve(idx, target); err != nil {
			return fmt.Errorf("%s: %w", n.Path, err)
		}
	}
	return nil
}

func validateLinkText(text string) error {
	switch {
	case text == "":
		return errors.New("empty link target")
	case strings.ContainsAny(text, "\\\x00:") || path.IsAbs(text) || filepath.IsAbs(text):
		return errors.New("absolute or nonportable link target")
	}
	return nil
}

// ResolveFile resolves rel (following in-package links) to a regular file.
func (t *Tree) ResolveFile(rel string) (Node, error) {
	n, err := resolve(t.index(), path.Clean(rel))
	if err != nil {
		return Node{}, err
	}
	if n.Kind != KindFile {
		return Node{}, fmt.Errorf("%s is not a regular file", rel)
	}
	return n, nil
}

// ReadFile returns the bytes of rel after in-package link resolution.
func (t *Tree) ReadFile(rel string) ([]byte, error) {
	n, err := t.ResolveFile(rel)
	if err != nil {
		return nil, err
	}
	return t.content(n.Path)
}

// Equal reports whether two trees describe identical inventories and bytes.
func (t *Tree) Equal(other *Tree) (bool, error) {
	if len(t.Nodes) != len(other.Nodes) {
		return false, nil
	}
	for i, n := range t.Nodes {
		if n != other.Nodes[i] {
			return false, nil
		}
	}
	a, err := t.Digest()
	if err != nil {
		return false, err
	}
	b, err := other.Digest()
	if err != nil {
		return false, err
	}
	return a == b, nil
}

// CopyTo copies the tree's nodes to dst, recreating links and (on hosts that
// support it) executable bits, and omitting reserved metadata and siblings.
// The recorded Exec bit, not the host mode, is the semantic authority.
func (t *Tree) CopyTo(dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, n := range t.Nodes {
		to := filepath.Join(dst, filepath.FromSlash(n.Path))
		switch n.Kind {
		case KindDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
		case KindLink:
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(n.Link, to); err != nil {
				return err
			}
		case KindFile:
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			b, err := t.content(n.Path)
			if err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if n.Exec {
				mode = 0o755
			}
			if err := os.WriteFile(to, b, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
