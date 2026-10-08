package sessionruntime

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateB4C15RefreshExactMapping(t *testing.T) {
	if os.Getenv("C15_REFRESH_MAPPING") != "1" {
		t.Skip("set C15_REFRESH_MAPPING=1")
	}
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	for ri := range mapping.Rows {
		row := &mapping.Rows[ri]
		source, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(row.Path)))
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, row.Path, source, parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i, stmt := range block.List {
				a, ok := stmt.(*ast.AssignStmt)
				if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
					continue
				}
				lhs, lok := a.Lhs[0].(*ast.Ident)
				rhs, rok := a.Rhs[0].(*ast.Ident)
				if !lok || !rok || lhs.Name != "_" || rhs.Name != privateB4C15ConstName(row.SiteID) {
					continue
				}
				for distance := 1; distance < len(block.List); distance++ {
					for _, index := range []int{i - distance, i + distance} {
						if index < 0 || index >= len(block.List) || statementIsC15Binding(block.List[index]) {
							continue
						}
						row.BindingFunction = enclosingFunction(file, stmt.Pos())
						row.AdjacencyIndex = index - i
						row.AdjacencyRelation = adjacencyRelation(row.AdjacencyIndex)
						row.ASTDigest = astDigest(fset, source, block.List[index])
						found = true
						return false
					}
				}
			}
			return !found
		})
		if !found {
			t.Fatalf("ASSERT_C15_MAPPING_REFRESH_SITE %s", row.SiteID)
		}
	}
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "c15-census-v1", "operation-mapping.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := os.WriteFile(path+".sha256", []byte(sum+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("ASSERT_C15_MAPPING_REFRESH_SHA256 %s", sum)
}

func TestPrivateB4C15ExactBindingRejectsSameFunctionSwap(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	for i := range mapping.Rows {
		for j := i + 1; j < len(mapping.Rows); j++ {
			a, b := mapping.Rows[i], mapping.Rows[j]
			if a.Path == b.Path && a.BindingFunction == b.BindingFunction && a.ASTDigest != b.ASTDigest {
				if privateB4C15ExactBindingMatch(a, b.Path, b.BindingFunction, b.AdjacencyRelation, b.AdjacencyIndex, b.ASTDigest) ||
					privateB4C15ExactBindingMatch(b, a.Path, a.BindingFunction, a.AdjacencyRelation, a.AdjacencyIndex, a.ASTDigest) {
					t.Fatal("ASSERT_C15_ADVERSARIAL_SWAP_ACCEPTED")
				}
				return
			}
		}
	}
	t.Fatal("ASSERT_C15_ADVERSARIAL_SWAP_FIXTURE")
}
