package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

func TestClosedEntry(t *testing.T) {
	l := Limits{64, 64 << 10, 8 << 20, 256, 2 << 20}
	if _, err := Prepare(context.Background(), []Input{{Projection: derived.Projection{}}}, l); err == nil {
		t.Fatal("zero proof accepted")
	}
	if _, err := Prepare(nil, []Input{{}}, l); err == nil {
		t.Fatal("nil context")
	}
	if _, err := Prepare(context.Background(), nil, l); err == nil {
		t.Fatal("no inputs")
	}
}
func TestOffsets(t *testing.T) {
	src := []byte("// é😀z\r\nfunc F() {}\n")
	for _, tt := range []struct {
		enc  string
		col  uint32
		want int
		ok   bool
	}{{"utf-8", 9, 9, true}, {"utf-16", 6, 9, true}, {"utf-32", 5, 9, true}, {"utf-16", 5, 0, false}, {"utf-8", 6, 0, false}, {"utf-16", 99, 0, false}, {"UNKNOWN", 1, 0, false}} {
		got, ok := positionOffset(src, graph.Position{Character: tt.col}, tt.enc)
		if ok != tt.ok || ok && got != tt.want {
			t.Fatalf("%+v got %d %v", tt, got, ok)
		}
	}
	if _, ok := positionOffset(src, graph.Position{Line: 4}, "utf-16"); ok {
		t.Fatal("absent line")
	}
}
func TestRetainedDeclarationAndReceivers(t *testing.T) {
	for _, tt := range []struct {
		src, name, enc, want string
		kind                 int
		col, end             uint32
	}{
		{"package p\nfunc F() {}", "F", "utf-16", "func F() {}", 12, 5, 11},
		{"package p\ntype T struct{}\nfunc (t *T) M() {}", "(*T).M", "utf-16", "func (t *T) M() {}", 6, 12, 18},
		{"package p\ntype T struct{}\nfunc (t T) M() {}", "T.M", "utf-8", "func (t T) M() {}", 6, 11, 17},
		{"package p\n/*😀*/func F() {}", "F", "utf-16", "func F() {}", 12, 11, 17},
		{"package p\n/*😀*/func F() {}", "F", "utf-8", "func F() {}", 12, 13, 19},
	} {
		src := []byte(tt.src)
		line := uint32(strings.Count(tt.src, "\n"))
		n := graph.Node{ID: "n", Item: graph.Item{Name: tt.name, Kind: tt.kind, URI: "file:///retained.go", Range: graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: tt.end}}, SelectionRange: graph.Range{Start: graph.Position{Line: line, Character: tt.col}, End: graph.Position{Line: line, Character: tt.col + 1}}}}
		d, gap := declaration(src, n, tt.enc)
		if gap != "" || string(d.Bytes) != tt.want || !bytes.Equal(src[d.StartByte:d.EndByte], d.Bytes) {
			t.Fatalf("%+v gap=%s decl=%s", tt, gap, d.Bytes)
		}
		d.Bytes[0] = 'x'
		if bytes.Contains(src, []byte("xunc")) {
			t.Fatal("source alias")
		}
		n.Name = "Wrong"
		if _, gap := declaration(src, n, tt.enc); gap != "MISSING_SOURCE_BINDING" {
			t.Fatal("name mismatch", gap)
		}
	}
}
func TestDeclarationRejectsUnsafeBindings(t *testing.T) {
	src := []byte("package p\nfunc F() {}")
	n := graph.Node{Item: graph.Item{Name: "F", Kind: 12, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 11}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1, Character: 5}, End: graph.Position{Line: 1, Character: 6}}}}
	for _, mutate := range []func(*graph.Node){func(n *graph.Node) { n.Kind = 6 }, func(n *graph.Node) { n.SelectionRange.Start.Character = 6 }, func(n *graph.Node) { n.Range.End.Character = 8 }, func(n *graph.Node) { n.SelectionRange.End.Character = 99 }} {
		bad := n
		mutate(&bad)
		if _, gap := declaration(src, bad, "utf-16"); gap == "" {
			t.Fatal("bad binding accepted")
		}
	}
	if _, gap := declaration(src, n, ""); gap != "UNAVAILABLE_POSITION_ENCODING" {
		t.Fatal(gap)
	}
}
func TestBoundedPartsPreserveEveryRow(t *testing.T) {
	l := Limits{64, 2048, 8 << 20, 256, 2 << 20}
	p := Packet{ID: "packet", InputSHA256: "input", Target: "root", ScopeSHA256: "scope"}
	var rows []Record
	for i := 0; i < 40; i++ {
		rows = append(rows, record("MEMBER", string(rune('A'+i)), strings.Repeat("x", 200)))
	}
	parts, ok := split(p, rows, l, 0, 0)
	if !ok || len(parts) < 2 {
		t.Fatal("not split")
	}
	count := 0
	for i, part := range parts {
		b, _ := json.Marshal(part)
		if len(b) > l.MaxPartBytes || part.Index != i+1 {
			t.Fatal("bad part")
		}
		count += len(part.Records)
	}
	if count != len(rows) {
		t.Fatal("dropped rows")
	}
	again, _ := split(p, rows, l, 0, 0)
	a, _ := json.Marshal(parts)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("not deterministic")
	}
	if _, ok := split(p, []Record{record("RETAINED_SOURCE_AST_DECLARATION", "decl", strings.Repeat("x", 3000))}, l, 0, 0); ok {
		t.Fatal("split indivisible declaration")
	}
	l.MaxParts = 1
	if _, ok := split(p, rows, l, 0, 0); ok {
		t.Fatal("part budget ignored")
	}
	l.MaxParts = 256
	l.MaxTotalBytes = 2048
	if _, ok := split(p, rows, l, 0, 0); ok {
		t.Fatal("total ignored")
	}
}
