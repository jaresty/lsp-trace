package privatedescribeprep

import (
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/sourceposition"
)

func TestIdentifierOnlyNativeRangeRetainedAST(t *testing.T) {
	for _, enc := range []string{"utf-8", "utf-16", "utf-32"} {
		t.Run(enc, func(t *testing.T) {
			src := []byte("package p\n/*😀*/func Café() string { return \"é😀\" }\n")
			start := strings.Index(string(src), "Café")
			pos := func(off int) graph.Position {
				p, e := sourceposition.PositionAtOffset(src, enc, off)
				if e != nil {
					t.Fatal(e)
				}
				return graph.Position{Line: p.Line, Character: p.Character}
			}
			sel := graph.Range{Start: pos(start), End: pos(start + len("Café"))}
			n := graph.Node{ID: "n", Item: graph.Item{Name: "Café", Kind: 12, URI: "file:///retained.go", Range: sel, SelectionRange: sel}}
			d, gap := declaration(src, n, enc)
			if gap != "" {
				t.Fatalf("identifier-only endpoint: %s", gap)
			}
			if d.NativeRange != sel || d.SelectionRange != sel || d.Kind != "RETAINED_SOURCE_AST_DECLARATION" {
				t.Fatal("native evidence changed", d)
			}
			want := "func Café() string { return \"é😀\" }"
			if d.Body != want || string(d.Bytes) != want || d.DeclarationRange.Start != pos(strings.Index(string(src), "func")) || d.DeclarationRange.End != pos(len(src)-1) {
				t.Fatal("incorrect AST span", d)
			}
			n.Range = d.DeclarationRange
			if full, g := declaration(src, n, enc); g != "" || full.Body != want || full.NativeRange != n.Range {
				t.Fatal("full-range regression", g)
			}
			for _, mutate := range []func(*graph.Node){
				func(n *graph.Node) { n.Range.End = n.Range.Start },
				func(n *graph.Node) { n.Range.End.Line = 999 },
				func(n *graph.Node) { n.SelectionRange.Start.Character++ },
				func(n *graph.Node) { n.Name = "Other" },
				func(n *graph.Node) { n.Kind = 6 },
				func(n *graph.Node) { n.SelectionRange.End.Line = 999 },
			} {
				bad := n
				mutate(&bad)
				if _, g := declaration(src, bad, enc); g == "" {
					t.Fatal("unsafe binding accepted", bad)
				}
			}
		})
	}
}

func TestQualifiedGenericPointerReceiverRemainsCoordinateBound(t *testing.T) {
	src := []byte("package p\ntype T[A any] struct{}\nfunc (p *T[A]) M() {}\nfunc (p *T[A]) Other() {}")
	sel := graph.Range{Start: graph.Position{Line: 2, Character: 15}, End: graph.Position{Line: 2, Character: 16}}
	n := graph.Node{ID: "m", Item: graph.Item{Name: "(*T).M", Kind: 6, URI: "file:///retained.go", Range: sel, SelectionRange: sel}}
	for _, name := range []string{"(*T).M", "*T.M", "M"} {
		n.Name = name
		if d, g := declaration(src, n, "utf-16"); g != "" || d.Body != "func (p *T[A]) M() {}" {
			t.Fatal("qualified pointer receiver", g)
		}
	}
	for _, name := range []string{"(*Wrong).M", "T.M", "m"} {
		n.Name = name
		if _, g := declaration(src, n, "utf-16"); g != "MISSING_SOURCE_BINDING" {
			t.Fatal("unproven receiver accepted", name, g)
		}
	}
	n.Name = "(*T).M"
	n.SelectionRange.Start.Line = 3
	n.SelectionRange.End.Line = 3
	n.Range = n.SelectionRange
	if _, g := declaration(src, n, "utf-16"); g == "" {
		t.Fatal("wrong identifier accepted")
	}
}

func TestMethodBindingDoesNotResolveAmbiguousSource(t *testing.T) {
	// Even identical retained bytes do not disambiguate duplicate URI bindings.
	const uri = "file:///retained.go"
	body := []byte("package p\ntype T struct{}\nfunc (p T) M() {}")
	c := graphprovenance.SourceWithholdingPrivateV1{Evidence: graphprovenance.EvidenceV2{Captures: []graphprovenance.Receipt{
		{ID: "receipt-a", URI: uri, Status: "READABLE", Content: body},
		{ID: "receipt-b", URI: uri, Status: "READABLE", Content: body},
	}}}
	if s := sourceFor(uri, sources(c)); s.Status != "AMBIGUOUS_SOURCE_BINDING" || len(s.ReceiptIDs) != 2 {
		t.Fatal("duplicate source URI accepted", s)
	}
}

func TestNativeRangeRejectsSplitSurrogate(t *testing.T) {
	src := []byte("package p\n/*😀*/func F() {}")
	sel := graph.Range{Start: graph.Position{Line: 1, Character: 11}, End: graph.Position{Line: 1, Character: 12}}
	n := graph.Node{Item: graph.Item{Name: "F", Kind: 12, Range: graph.Range{Start: graph.Position{Line: 1, Character: 3}, End: sel.End}, SelectionRange: sel}}
	if _, g := declaration(src, n, "utf-16"); g != "INVALID_ENDPOINT_RANGE" {
		t.Fatal("split surrogate accepted", g)
	}
	n.Range = sel
	n.SelectionRange.Start.Character = 3
	if _, g := declaration(src, n, "utf-16"); g != "INVALID_SELECTION_POSITION" {
		t.Fatal("split selection surrogate accepted", g)
	}
}

// Actual retained provider labels are bare MarshalJSON/kind 12 at these exact
// coordinates. The receiver belongs to the AST source, not the provider label.
func TestObservedBareMethodCallableKindRetainedAST(t *testing.T) {
	for _, tt := range []struct {
		recv      string
		line, col uint32
	}{
		{"StringPartition", 15, 25}, {"IndexPartition", 25, 24},
	} {
		t.Run(tt.recv, func(t *testing.T) {
			header := "package p\ntype " + tt.recv + " struct{}\n"
			prefix := header + strings.Repeat("\n", int(tt.line)-2)
			body := "func (p " + tt.recv + ") MarshalJSON() ([]byte, error) { return nil, nil }"
			src := []byte(prefix + body)
			sel := graph.Range{Start: graph.Position{Line: tt.line, Character: tt.col}, End: graph.Position{Line: tt.line, Character: tt.col + 11}}
			for _, kind := range []int{12, 6} {
				for _, full := range []bool{false, true} {
					n := graph.Node{ID: "method", Item: graph.Item{Name: "MarshalJSON", Kind: kind, URI: "file:///retained.go", Range: sel, SelectionRange: sel}}
					if full {
						n.Range = graph.Range{Start: graph.Position{Line: tt.line}, End: graph.Position{Line: tt.line, Character: uint32(len(body))}}
					}
					original := n
					d, g := declaration(src, n, "utf-16")
					if g != "" || d.Body != body || d.Kind != "RETAINED_SOURCE_AST_DECLARATION" {
						t.Fatalf("kind=%d full=%v gap=%s", kind, full, g)
					}
					if !reflect.DeepEqual(n, original) || d.NativeRange != original.Range || d.SelectionRange != original.SelectionRange {
						t.Fatal("provider label/range rewritten")
					}
					if d.DeclarationRange.Start.Character != 0 || d.DeclarationRange.End.Character != uint32(len(body)) {
						t.Fatal("wrong derived span")
					}
					for _, mutate := range []func(*graph.Node){
						func(n *graph.Node) { n.Name = "Wrong" },
						func(n *graph.Node) { n.Name = "marshalJSON" },
						func(n *graph.Node) { n.Name = "Other.MarshalJSON" },
						func(n *graph.Node) { n.Name = "(*" + tt.recv + ").MarshalJSON" },
						func(n *graph.Node) { n.SelectionRange.Start.Character++ },
						func(n *graph.Node) { n.SelectionRange.Start.Line--; n.SelectionRange.End.Line-- },
						func(n *graph.Node) { n.SelectionRange.End.Character++ },
						func(n *graph.Node) { n.Kind = 13 },
					} {
						bad := n
						mutate(&bad)
						if _, gap := declaration(src, bad, "utf-16"); gap == "" {
							t.Fatal("unsafe association accepted", bad)
						}
					}
				}
			}
		})
	}
}
