package sessionruntime

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// discoverPrivateB4C15Candidates independently enumerates the active production
// package. Generated census outputs are deliberately outside its input set.
func discoverPrivateB4C15Candidates(t *testing.T) []privateB4C15Candidate {
	t.Helper()
	dir := packageDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0)
	sources := map[*ast.File][]byte{}
	paths := map[*ast.File]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		match, err := build.Default.MatchFile(dir, name)
		if err != nil {
			t.Fatalf("ASSERT_C15_BUILD_MATCH_%s: %v", name, err)
		}
		if !match {
			continue
		}
		source, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, source, parser.AllErrors)
		if err != nil {
			t.Fatalf("ASSERT_C15_DISCOVERY_PARSE_%s: %v", name, err)
		}
		files = append(files, file)
		sources[file] = source
		paths[file] = name
	}
	sort.Slice(files, func(i, j int) bool { return paths[files[i]] < paths[files[j]] })
	// Run the whole-package checker as an independent semantic pass. The standard
	// importer is not module-aware, so import diagnostics do not alter the AST
	// census; syntax/type information available within this package is still checked.
	conf := types.Config{Importer: importer.Default(), Error: func(error) {}}
	_, _ = conf.Check("github.com/jaresty/lsp-trace/sessionruntime", fset, files, nil)
	var out []privateB4C15Candidate
	counts := map[string]int{}
	for _, file := range files {
		fileKey := strings.TrimSuffix(strings.ReplaceAll(paths[file], "_", "."), ".go")
		ast.Inspect(file, func(n ast.Node) bool {
			var kind, display string
			switch x := n.(type) {
			case *ast.CallExpr:
				display = privateB4C15ExprName(x.Fun)
				kind = "call"
				if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "make" || id.Name == "append") {
					kind = "builtin-" + id.Name
				}
			case *ast.CompositeLit:
				kind, display = "composite-literal", "composite.literal"
			default:
				return true
			}
			fn := strings.ToLower(enclosingFunction(file, n.Pos()))
			base := "alloc." + fileKey + "." + fn + "." + kind + "." + strings.ToLower(strings.ReplaceAll(display, "_", "."))
			counts[base]++
			id := fmt.Sprintf("%s.%d", base, counts[base])
			normalized := normalizedNodeSource(fset, sources[file], n)
			out = append(out, privateB4C15Candidate{ID: id, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(normalized))), Kind: kind, Display: display})
			return true
		})
	}
	return out
}

func privateB4C15ExprName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return privateB4C15ExprName(x.X) + "." + x.Sel.Name
	case *ast.IndexExpr:
		return privateB4C15ExprName(x.X)
	case *ast.IndexListExpr:
		return privateB4C15ExprName(x.X)
	default:
		return "expression"
	}
}
