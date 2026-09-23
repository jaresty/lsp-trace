package adr0011sourcecontext

import (
	"bytes"
	"testing"
)

func fixture() (PointIdentity, SpanIdentity) {
	d := Document{URI: "file:///a.go", Digest: "sha256:abc", ByteLength: 100, CustodyKind: "LIVE", SessionID: "project", Generation: 1, DocumentVersion: "4", Encoding: "utf-16", Privacy: "private"}
	return PointIdentity{Document: d, Evidence: Range{Position{2, 1}, Position{2, 5}}}, SpanIdentity{Document: d, Display: Range{Position{1, 0}, Position{8, 1}}, DisplayReceipt: "documentSymbol:1", ProjectionPolicy: "full-definition", Disposition: "SELECTED", Body: "func f() {\n  call()\n}"}
}
func TestInterningAndPermutation(t *testing.T) {
	p, s := fixture()
	other := p
	other.Evidence.Start.Character++
	input := Input{Points: []PointIdentity{p, p, other}, Spans: []SpanIdentity{s, s}, Bindings: []Binding{
		{ID: "call", Kind: Calls, PointID: PointID(p), SpanID: SpanID(s), Role: "QUERY_OCCURRENCE", MethodReceipt: "callHierarchy:1"},
		{ID: "ref", Kind: ReferencesSymbol, PointID: PointID(p), SpanID: SpanID(s), Role: "REFERENCING_OCCURRENCE", MethodReceipt: "references:1"},
		{ID: "def", Kind: ResolvesToDefinition, PointID: PointID(p), SpanID: SpanID(s), Role: "DEFINITION_TARGET", MethodReceipt: "definition:1"},
		{ID: "other", Kind: ReferencesSymbol, PointID: PointID(other), SpanID: SpanID(s), Role: "REFERENCING_OCCURRENCE", MethodReceipt: "references:2"},
	}}
	got, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts != (Counts{3, 2, 2, 1, 4, len(s.Body)}) {
		t.Fatalf("balanced point/span/occurrence accounting: %+v", got.Counts)
	}
	if len(got.Bindings) != 4 || got.Bindings[0].Kind == got.Bindings[1].Kind && got.Bindings[1].Kind == got.Bindings[2].Kind {
		t.Fatalf("typed occurrence multiplicity: %+v", got.Bindings)
	}
	reverse := input
	reverse.Points = []PointIdentity{other, p, p}
	reverse.Spans = []SpanIdentity{s, s}
	reverse.Bindings = append([]Binding(nil), input.Bindings...)
	for i, j := 0, len(reverse.Bindings)-1; i < j; i, j = i+1, j-1 {
		reverse.Bindings[i], reverse.Bindings[j] = reverse.Bindings[j], reverse.Bindings[i]
	}
	b1, _ := got.Marshal()
	b2snap, err := Build(reverse)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := b2snap.Marshal()
	if !bytes.Equal(b1, b2) {
		t.Fatalf("permutation-stable serialization: %s != %s", b1, b2)
	}
}
func TestIdentityAndFailClosed(t *testing.T) {
	p, s := fixture()
	p2 := p
	p2.Document.DocumentVersion = "5"
	if PointID(p) == PointID(p2) {
		t.Error("version separates point")
	}
	variants := []SpanIdentity{}
	v := s
	v.Document.DocumentVersion = "5"
	variants = append(variants, v)
	v = s
	v.DisplayReceipt = "documentSymbol:2"
	variants = append(variants, v)
	v = s
	v.Document.Privacy = "other"
	variants = append(variants, v)
	v = s
	v.Disposition = "WITHHELD"
	v.Omitted = true
	v.Body = ""
	variants = append(variants, v)
	for _, v := range variants {
		if SpanID(s) == SpanID(v) {
			t.Errorf("ineligible span shared: %+v", v)
		}
	}
	base := Input{Points: []PointIdentity{p}, Spans: []SpanIdentity{s}, Bindings: []Binding{{ID: "one", Kind: Calls, PointID: PointID(p), SpanID: SpanID(s)}}}
	cases := map[string]func(*Input){
		"duplicate binding identity": func(i *Input) { i.Bindings = append(i.Bindings, i.Bindings[0]) },
		"forged span":                func(i *Input) { i.Bindings[0].SpanID = "span:forged" },
		"missing point":              func(i *Input) { i.Bindings[0].PointID = "point:missing" },
		"wrong kind":                 func(i *Input) { i.Bindings[0].Kind = "GUESS" },
		"mismatched custody":         func(i *Input) { i.Spans[0].Document.DocumentVersion = "5"; i.Bindings[0].SpanID = SpanID(i.Spans[0]) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			i := base
			i.Points = append([]PointIdentity(nil), base.Points...)
			i.Spans = append([]SpanIdentity(nil), base.Spans...)
			i.Bindings = append([]Binding(nil), base.Bindings...)
			mutate(&i)
			if _, err := Build(i); err == nil {
				t.Fatal("expected fail-closed rejection")
			}
		})
	}
}
