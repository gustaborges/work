package contract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAdministrativeWriterBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			return e
		}
		imports := map[string]string{}
		for _, i := range f.Imports {
			name, _ := strconv.Unquote(i.Path.Value)
			key := filepath.Base(name)
			if i.Name != nil {
				key = i.Name.Name
			}
			imports[key] = name
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Save" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			imp := imports[id.Name]
			if strings.HasSuffix(imp, "/config") || strings.HasSuffix(imp, "/registry") || strings.HasSuffix(imp, "/repoconv") {
				t.Errorf("unserialized administrative Save in %s", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestAdministrationPresentationBoundary(t *testing.T) {
	for _, dir := range []string{"adminstate", "pluginadmin", "plugininstall"} {
		entries, err := os.ReadDir(filepath.Join("..", "..", "internal", dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				b, err := os.ReadFile(filepath.Join("..", "..", "internal", dir, e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(b), `"github.com/gustaborges/work/internal/present`) {
					t.Errorf("presentation import in %s/%s", dir, e.Name())
				}
			}
		}
	}
}
