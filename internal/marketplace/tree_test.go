package marketplace

import (
	"errors"
	"testing"
)

func dirNode(p string) Node        { return Node{Path: p, Kind: KindDir} }
func fileNode(p string) Node       { return Node{Path: p, Kind: KindFile} }
func linkNode(p, text string) Node { return Node{Path: p, Kind: KindLink, Link: text} }
func treeOf(nodes ...Node) *Tree   { return &Tree{Nodes: nodes} }

func TestValidateLinks(t *testing.T) {
	cases := []struct {
		name    string
		tree    *Tree
		wantErr error // nil = valid; otherwise errors.Is target (errAny for any error)
	}{
		{"link to sibling file", treeOf(fileNode("b"), linkNode("a", "b")), nil},
		{"dotdot after a real directory", treeOf(fileNode("c"), dirNode("b"), linkNode("a", "b/../c")), nil},
		{"upward link that stays inside", treeOf(dirNode("p"), fileNode("y"), linkNode("p/x", "../y")), nil},
		{"link to the package root", treeOf(dirNode("p"), linkNode("p/up", "..")), nil},
		{"escape at the top level", treeOf(linkNode("a", "../x")), errEscape},
		{"escape from a subdirectory", treeOf(dirNode("p"), linkNode("p/a", "../../x")), errEscape},
		{
			"dotdot after a symlinked directory",
			treeOf(dirNode("p"), linkNode("p/d", "."), linkNode("p/l", "d/../..")),
			errEscape,
		},
		{"dangling", treeOf(linkNode("a", "missing")), errDangling},
		{"cycle", treeOf(linkNode("a", "b"), linkNode("b", "a")), errAny},
		{"absolute", treeOf(linkNode("a", "/etc/passwd")), errAny},
		{"empty", treeOf(linkNode("a", "")), errAny},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.tree.ValidateLinks()
			switch {
			case c.wantErr == nil && err != nil:
				t.Errorf("ValidateLinks() = %v, want nil", err)
			case c.wantErr == errAny && err == nil:
				t.Error("ValidateLinks() = nil, want an error")
			case c.wantErr != nil && c.wantErr != errAny && !errors.Is(err, c.wantErr):
				t.Errorf("ValidateLinks() = %v, want %v", err, c.wantErr)
			}
		})
	}
}

var errAny = errors.New("any error")
