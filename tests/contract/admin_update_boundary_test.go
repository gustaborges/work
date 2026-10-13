package contract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminUpdateRetrievalBoundary(t *testing.T) {
	// Ordinary Work operations and initial inventory/hub inspection cannot
	// reach update discovery through a new call site unnoticed.
	root := filepath.Join("..", "..", "internal", "cli")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if id.Name == "pluginadmin" && (sel.Sel.Name == "Check" || sel.Sel.Name == "PrepareUpdates") && entry.Name() != "plugin_update.go" {
				t.Errorf("implicit update retrieval in %s", entry.Name())
			}
			return true
		})
	}
}
