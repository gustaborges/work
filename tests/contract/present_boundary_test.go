// Package contract also enforces the architectural boundary from
// specs/003-terminal-ux-revamp/contracts/presentation-boundary.md: the
// interactive presentation package must not know Work domain concepts. This is a
// build-time dependency check, not a review convention (ADR-0020, FR-031).
package contract

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const modulePrefix = "github.com/gustaborges/work/"

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// deps returns the full transitive dependency set of the given package pattern.
func deps(t *testing.T, pattern string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", pattern)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pattern, err, out)
	}
	return strings.Fields(string(out))
}

// TestPresentImportsNoDomainPackage fails if internal/present (or a subpackage)
// transitively imports any github.com/gustaborges/work/internal/* package other
// than internal/diag, the shared domain-free error vocabulary.
func TestPresentImportsNoDomainPackage(t *testing.T) {
	const allowed = modulePrefix + "internal/diag"
	for _, dep := range deps(t, "./internal/present/...") {
		if !strings.HasPrefix(dep, modulePrefix+"internal/") {
			continue
		}
		if dep == allowed || strings.HasPrefix(dep, modulePrefix+"internal/present") {
			continue
		}
		t.Errorf("internal/present imports a forbidden domain package: %s", dep)
	}
}

// TestDiagImportsNoInternalPackage keeps internal/diag a leaf: it is the only
// internal dependency present is allowed, so it must itself stay domain-free.
func TestDiagImportsNoInternalPackage(t *testing.T) {
	for _, dep := range deps(t, "./internal/diag") {
		if strings.HasPrefix(dep, modulePrefix+"internal/") && dep != modulePrefix+"internal/diag" {
			t.Errorf("internal/diag imports an internal package: %s", dep)
		}
	}
}
