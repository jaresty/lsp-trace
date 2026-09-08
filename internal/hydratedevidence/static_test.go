package hydratedevidence

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestOfflineProductionSurface(t *testing.T) {
	allowed := map[string]bool{}
	for _, p := range []string{"bytes", "crypto/sha256", "encoding/json", "encoding/base64", "embed", "errors", "fmt", "io", "sort", "strconv", "strings", "sync", "unicode/utf8", "lsp-trace/internal/graphprovenance", "lsp-trace/internal/source", "github.com/santhosh-tekuri/jsonschema/v6"} {
		allowed[p] = true
	}
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, e := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !allowed[p] {
				t.Fatalf("offline surface gained unreviewed dependency: %s", p)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
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
			if id.Name == "graphprovenance" && sel.Sel.Name != "ValidateFor" {
				t.Fatalf("non-admission graphprovenance call: %s", sel.Sel.Name)
			}
			if id.Name == "source" {
				t.Fatalf("source acquisition call: %s", sel.Sel.Name)
			}
			return true
		})
	}
	var doc any
	if e = json.Unmarshal(Schema(), &doc); e != nil {
		t.Fatal(e)
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "$ref" {
					s, ok := v.(string)
					if !ok || !strings.HasPrefix(s, "#/") {
						t.Fatal("schema has external acquisition reference")
					}
				}
				walk(v)
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	walk(doc)
}
