package censusacquisition_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func adapterSource(t *testing.T) *ast.File {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(here), "batch_adapter.go"), nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestExternalBatchAuthoritySurfaceIsClosed(t *testing.T) {
	file := adapterSource(t)
	forbidden := map[string]bool{"BatchSession": true, "NewBatchAdapter": true}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if named, ok := spec.(*ast.TypeSpec); ok && forbidden[named.Name.Name] {
					t.Fatalf("ASSERT_EXTERNAL_BATCH_AUTHORITY_CLOSED: exported %s", named.Name.Name)
				}
			}
		case *ast.FuncDecl:
			if forbidden[d.Name.Name] {
				t.Fatalf("ASSERT_EXTERNAL_BATCH_AUTHORITY_CLOSED: exported %s", d.Name.Name)
			}
		}
	}
}

func TestBatchAdapterForbiddenDependencies(t *testing.T) {
	file := adapterSource(t)
	var imports []string
	for _, imp := range file.Imports {
		imports = append(imports, imp.Path.Value)
	}
	sort.Strings(imports)
	for _, forbidden := range []string{"cmd/", "internal/mcp", "internal/publication", "internal/registry", "internal/runtimeprofile", "internal/managedprocess"} {
		for _, path := range imports {
			if len(path) >= 2 && contains(path, forbidden) {
				t.Fatalf("ASSERT_BATCH_FORBIDDEN_DEPENDENCY: %s", path)
			}
		}
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
