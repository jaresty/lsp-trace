package integratedconformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceAuthorityProductionAPIIsConcreteAndPrivate(t *testing.T) {
	root := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, "traceops")); !os.IsNotExist(err) {
		t.Fatal("ASSERT_NO_PUBLIC_TRACE_RUNTIME_EXECUTOR_AUTHORITY_CONSTRUCTOR: public traceops package exists")
	}

	path := filepath.Join(root, "cmd", "lsp-trace-mcp", "trace_executor.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var privateConcreteConstructor bool
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := strings.ToLower(d.Name.Name)
			if ast.IsExported(d.Name.Name) && (strings.Contains(name, "trace") || strings.Contains(name, "executor") || strings.Contains(name, "runtime") || strings.Contains(name, "authority") || strings.Contains(name, "source") || strings.Contains(name, "assembl")) {
				t.Fatalf("ASSERT_NO_PUBLIC_TRACE_ACQUISITION_SOURCE_REUSE_ASSEMBLY_CAPABILITY: exported function %s", d.Name.Name)
			}
			if d.Name.Name == "newTraceExecutor" && d.Type.Params != nil && len(d.Type.Params.List) == 1 {
				star, ok := d.Type.Params.List[0].Type.(*ast.StarExpr)
				if ok {
					ident, named := star.X.(*ast.Ident)
					privateConcreteConstructor = named && ident.Name == "hostSelectorRuntime"
				}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !ast.IsExported(typeSpec.Name.Name) {
					continue
				}
				name := strings.ToLower(typeSpec.Name.Name)
				if strings.Contains(name, "trace") || strings.Contains(name, "executor") || strings.Contains(name, "runtime") || strings.Contains(name, "authority") || strings.Contains(name, "source") || strings.Contains(name, "assembl") {
					t.Fatalf("ASSERT_NO_PUBLIC_TRACE_RUNTIME_EXECUTOR_AUTHORITY_CONSTRUCTOR: exported type %s", typeSpec.Name.Name)
				}
			}
		}
	}
	if !privateConcreteConstructor {
		t.Fatal("ASSERT_TRACE_EXECUTOR_CONSTRUCTION_REQUIRES_CONCRETE_MANAGED_AUTHORITY")
	}
}
