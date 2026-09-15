package transientstructuraldelta

import (
	"encoding/json"
	"errors"
	"lsp-trace/internal/graph"
	tsr "lsp-trace/internal/transientstructuralresult"
	"testing"
)

func fixture(id, path, name string) tsr.LocatorResultV2 {
	return tsr.LocatorResultV2{SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 0, SourceGraphComplete: "UNKNOWN", PositionEncoding: "utf-16", TargetID: id, Nodes: []tsr.LocatorNodeV2{{ID: id, Name: name, Kind: 12, Path: path, DeclarationRange: graph.Range{}}}, AnalyticsScope: "BOUNDED_LOCAL", Coupling: []tsr.CouplingV2{{NodeID: id}}, StrongComponents: []tsr.StrongComponentV2{{Nodes: []string{id}}}, WeakProjection: "UNDIRECTED_SIMPLE", PageRankDamping: .85, AnalyticsTolerance: 1e-12, PageRank: []tsr.NodeScoreV2{{NodeID: id, Score: 1}}, HITS: []tsr.HubAuthorityV2{{NodeID: id}}}
}
func TestShiftedIDsAndRangesMatch(t *testing.T) {
	b := fixture("tn_0123456789abcdef0123456789abcdef", "src/a.go", "A")
	a := fixture("tn_fedcba9876543210fedcba9876543210", "src/a.go", "A")
	a.Nodes[0].DeclarationRange.Start.Line = 99
	r, e := Compare(Input{b, a})
	if e != nil || len(r.MatchedSymbols) != 1 || len(r.AddedSymbols) != 0 || !r.Qualification.NodeUniverseEqual {
		t.Fatalf("ASSERT_DELTA_CANONICAL_MATCH: r=%+v err=%v", r, e)
	}
}
func TestAmbiguityMismatchAndConfinement(t *testing.T) {
	b := fixture("tn_0123456789abcdef0123456789abcdef", "src/a.go", "A")
	a := b
	a.Nodes = append(a.Nodes, a.Nodes[0])
	if _, e := Compare(Input{b, a}); code(e) != CodeAmbiguousSymbolKey {
		t.Fatalf("ASSERT_DELTA_AMBIGUITY: %v", e)
	}
	a = b
	a.PositionEncoding = "utf-8"
	if _, e := Compare(Input{b, a}); code(e) != CodePositionEncodingMismatch {
		t.Fatalf("ASSERT_DELTA_ENCODING: %v", e)
	}
	a = b
	a.Nodes = append(a.Nodes, tsr.LocatorNodeV2{ID: "tn_11111111111111111111111111111111", Name: "B", Kind: 12, Path: "src/b.go"})
	a.TargetID = a.Nodes[1].ID
	if _, e := Compare(Input{b, a}); code(e) != CodeTargetMismatch {
		t.Fatalf("ASSERT_DELTA_TARGET: %v", e)
	}
	a = b
	a.Nodes[0].Path = "../a.go"
	if _, e := Compare(Input{b, a}); code(e) != CodeInvalidInput {
		t.Fatalf("ASSERT_DELTA_CONFINEMENT: %v", e)
	}
}
func TestCallsAnalyticsQualification(t *testing.T) {
	b := fixture("tn_0123456789abcdef0123456789abcdef", "src/a.go", "A")
	b.Nodes = append(b.Nodes, tsr.LocatorNodeV2{ID: "tn_11111111111111111111111111111111", Name: "B", Kind: 12, Path: "src/b.go"})
	b.Coupling = append(b.Coupling, tsr.CouplingV2{NodeID: b.Nodes[1].ID})
	b.StrongComponents = []tsr.StrongComponentV2{{Nodes: []string{b.TargetID, b.Nodes[1].ID}}}
	b.PageRank = append(b.PageRank, tsr.NodeScoreV2{NodeID: b.Nodes[1].ID})
	b.HITS = append(b.HITS, tsr.HubAuthorityV2{NodeID: b.Nodes[1].ID})
	b.Calls = []tsr.LocatorCallV2{{CallerID: b.TargetID, CalleeID: b.Nodes[1].ID, Path: "src/a.go"}}
	a := b
	a.Calls = append(a.Calls, a.Calls[0])
	a.Coupling[0].Ce = 1
	a.Coupling[0].Instability = 1
	a.StrongComponents[0].Cyclic = true
	a.ArticulationPoints = []string{a.TargetID}
	a.PageRank[0].Score = .5
	a.HITS[0].Hub = .7
	a.HITS[0].Authority = .2
	r, e := Compare(Input{b, a})
	if e != nil || len(r.Calls.CountChanged) != 1 || r.Calls.CountChanged[0].Before != 1 || r.Calls.CountChanged[0].After != 2 || len(r.Analytics) != 1 || r.Qualification.ComparisonScope != "TWO_BOUNDED_LOCAL_RESULTS" {
		t.Fatalf("ASSERT_DELTA_CALL_ANALYTICS: r=%+v err=%v", r, e)
	}
}
func TestStrictDecode(t *testing.T) {
	b := fixture("tn_0123456789abcdef0123456789abcdef", "src/a.go", "A")
	raw, _ := json.Marshal(Input{b, b})
	raw = append(raw[:len(raw)-1], []byte(`,"publication_selector":{}}`)...)
	if _, e := Decode(raw); code(e) != CodeInvalidInput {
		t.Fatalf("ASSERT_DELTA_STRICT_INPUT: %v", e)
	}
}
func code(e error) Code {
	var d *DomainError
	if errors.As(e, &d) {
		return d.Code
	}
	return ""
}
