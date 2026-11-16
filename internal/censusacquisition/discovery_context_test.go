package censusacquisition

import (
	"context"
	"lsp-trace/internal/lsp"
	"reflect"
	"testing"
)

type context80SourceFiles []SourceFile

func (f context80SourceFiles) Enumerate(context.Context) ([]SourceFile, error) { return f, nil }

type context80Supplier struct{}

func (context80Supplier) Supply(context.Context, SourceFile) error { return nil }

type context80AdapterKey struct{}
type context80AdapterClient struct {
	documents, prepares [][2]int
	positions           []lsp.Position
}

func (*context80AdapterClient) SupportsDocumentSymbols() bool { return true }
func (*context80AdapterClient) SupportsCallHierarchy() bool   { return true }
func (*context80AdapterClient) DiscoveryDiagnosticContext(ctx context.Context, file, target int) context.Context {
	return context.WithValue(ctx, context80AdapterKey{}, [2]int{file, target})
}
func (c *context80AdapterClient) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	v, _ := ctx.Value(context80AdapterKey{}).([2]int)
	c.documents = append(c.documents, v)
	count := 1
	if v[0] == 0 {
		count = 2
	}
	out := make([]lsp.DocumentSymbol, count)
	for i := range out {
		pos := lsp.Position{Line: uint32(i + 3), Character: 7}
		out[i] = lsp.DocumentSymbol{Name: "callable", Kind: 12, Range: lsp.Range{Start: pos, End: pos}, SelectionRange: lsp.Range{Start: pos, End: pos}}
	}
	return out, nil
}
func (c *context80AdapterClient) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	v, _ := ctx.Value(context80AdapterKey{}).([2]int)
	c.prepares = append(c.prepares, v)
	c.positions = append(c.positions, p.Position)
	return nil, nil
}
func TestContext80SourceAdapterOrdinals(t *testing.T) {
	c := &context80AdapterClient{}
	a := DiscoveryAdapter{Workspace: "/offline-context80", Files: context80SourceFiles{{Path: "b.go", URI: "file:///offline-context80/b.go", LanguageID: "go"}, {Path: "a.go", URI: "file:///offline-context80/a.go", LanguageID: "go"}}, Supplier: context80Supplier{}, Client: c}
	d, _ := a.Discover(context.Background(), SessionIdentity{"inert", 1})
	if !reflect.DeepEqual(c.documents, [][2]int{{0, -1}, {1, -1}}) || !reflect.DeepEqual(c.prepares, [][2]int{{0, 0}, {0, 1}, {1, 2}}) {
		t.Fatal("ASSERT80_SOURCE_ORDINALS", c.documents, c.prepares)
	}
	if len(d.EnumerationSymbols) != 3 {
		t.Fatal("enumeration")
	}
	for i, s := range d.EnumerationSymbols {
		if c.prepares[i] != [2]int{s.FileOrdinal, s.Ordinal} || c.positions[i] != s.SelectionStart {
			t.Fatal("ASSERT80_SOURCE_POSITION_UNCHANGED", i)
		}
	}
}
