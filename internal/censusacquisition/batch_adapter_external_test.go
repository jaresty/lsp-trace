package censusacquisition_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
}

func parseProductionPackage(t *testing.T, dir string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

func TestExternalPackageHasNoCensusAssemblyMintingPath(t *testing.T) {
	root := repositoryRoot(t)
	for _, rel := range []string{"internal/acquisitionorchestration", "internal/censusacquisition"} {
		for _, file := range parseProductionPackage(t, filepath.Join(root, rel)) {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if !ast.IsExported(d.Name.Name) {
						continue
					}
					if d.Name.Name == "ExecutePlannedBatch" && (fieldListContainsAny(d.Type.Params, "Binding", "SeedAuthority", "CustodyMode", "Workspace", "PublicationRoot", "ArtifactStore", "Callback") || fieldListContainsAny(d.Type.Results, "Assembly", "PublicationCapability", "Projection", "SeedAuthority")) {
						t.Fatalf("ASSERT_NO_EXPORTED_ARBITRARY_INPUT_TO_CENSUS_AUTHORITY: %s exports %s", rel, d.Name.Name)
					}
					if fieldListContainsAny(d.Type.Results, "Assembly", "PublicationCapability") {
						t.Fatalf("ASSERT_NO_EXPORTED_ARBITRARY_INPUT_TO_CENSUS_AUTHORITY: %s exports %s", rel, d.Name.Name)
					}
					if strings.Contains(strings.ToLower(d.Name.Name), "census") && (strings.Contains(d.Name.Name, "Capability") || strings.HasPrefix(d.Name.Name, "Execute") || strings.HasPrefix(d.Name.Name, "Acquire") || strings.HasPrefix(d.Name.Name, "New")) {
						t.Fatalf("ASSERT_NO_EXPORTED_INTERNAL_CENSUS_AUTHORITY: %s exports %s", rel, d.Name.Name)
					}
					if fieldListContainsAny(d.Type.Params, "Runtime", "Acquirer", "AcquiredV5", "RawMessage") && fieldListContainsAny(d.Type.Results, "BatchResult", "AcquiredV5", "Assembly", "PublicationCapability") {
						t.Fatalf("ASSERT_NO_ARBITRARY_INPUT_TO_CENSUS_AUTHORITY: %s exports %s", rel, d.Name.Name)
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						named, ok := spec.(*ast.TypeSpec)
						if !ok || !ast.IsExported(named.Name.Name) {
							continue
						}
						if named.Name.Name == "BatchAdapter" || named.Name.Name == "BatchResult" || named.Name.Name == "Assembly" || named.Name.Name == "PublicationCapability" {
							t.Fatalf("ASSERT_NO_EXPORTED_INTERNAL_CENSUS_AUTHORITY: %s exports %s", rel, named.Name.Name)
						}
					}
				}
			}
		}
	}
}

func TestPackageMainOwnsConcreteUnexportedCensusAcquirer(t *testing.T) {
	root := repositoryRoot(t)
	files := parseProductionPackage(t, filepath.Join(root, "cmd/lsp-trace"))
	var acquirer, constructor, assembly, publicationCapability, orchestrator bool
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.TypeSpec:
				if ast.IsExported(n.Name.Name) && (strings.Contains(n.Name.Name, "Assembly") || strings.Contains(n.Name.Name, "PublicationCapability")) {
					t.Fatalf("ASSERT_NO_EXPORTED_PACKAGE_MAIN_CENSUS_AUTHORITY: %s", n.Name.Name)
				}
				if n.Name.Name == "censusBatchAcquirer" && !ast.IsExported(n.Name.Name) {
					acquirer = true
				}
				if n.Name.Name == "censusAssembly" && !ast.IsExported(n.Name.Name) {
					assembly = true
				}
				if n.Name.Name == "censusPublicationCapability" && !ast.IsExported(n.Name.Name) {
					publicationCapability = true
				}
			case *ast.FuncDecl:
				if n.Name.Name == "newInitializedCensusBatchAcquirer" && !ast.IsExported(n.Name.Name) {
					constructor = firstParameterIsPointerTo(n, "initializedAcquisitionRuntime")
				}
				if n.Name.Name == "runInitializedCensusAcquisition" && !ast.IsExported(n.Name.Name) {
					orchestrator = firstParameterAfterContextIsPointerTo(n, "initializedAcquisitionRuntime") && fieldListContainsIdent(n.Type.Results, "censusAssembly")
				}
			}
			return true
		})
	}
	if !acquirer || !constructor || !assembly || !publicationCapability || !orchestrator {
		t.Fatalf("ASSERT_PACKAGE_MAIN_PRIVATE_CONCRETE_CENSUS_AUTHORITY: acquirer=%v constructor=%v assembly=%v publication=%v orchestrator=%v", acquirer, constructor, assembly, publicationCapability, orchestrator)
	}
	found := false
	for _, file := range parseProductionPackage(t, filepath.Join(root, "internal/acquisitionorchestration")) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "ExecutePlannedBatch" && ast.IsExported(fn.Name.Name) {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("ASSERT_SHARED_FIXED_POLICY_PLANNED_BATCH_EXECUTOR")
	}
}

func fieldListContainsAny(fields *ast.FieldList, names ...string) bool {
	for _, name := range names {
		if fieldListContainsIdent(fields, name) {
			return true
		}
	}
	return false
}

func fieldListContainsIdent(fields *ast.FieldList, name string) bool {
	if fields == nil {
		return false
	}
	found := false
	ast.Inspect(fields, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func firstParameterIsPointerTo(fn *ast.FuncDecl, name string) bool {
	return fn.Type.Params != nil && len(fn.Type.Params.List) > 0 && pointerIsNamed(fn.Type.Params.List[0].Type, name)
}

func firstParameterAfterContextIsPointerTo(fn *ast.FuncDecl, name string) bool {
	return fn.Type.Params != nil && len(fn.Type.Params.List) > 1 && pointerIsNamed(fn.Type.Params.List[1].Type, name)
}

func pointerIsNamed(expr ast.Expr, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	return ok && ident.Name == name
}
