package programc

import (
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestPrivateV2MathTypedZeroSitesAndDuplicateWeights(t *testing.T) {
	a, b, c := node("a", 0), node("b", 1), node("c", 2)
	site := graph.Range{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}
	raw := validV5(t, []graph.Node{c, b, a}, []graph.Edge{
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{}},
		{CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{site, site}},
		{CallerNodeID: c.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{site}},
	})
	_, nativeBytes := decodeV5(t, raw)
	m, f := ProjectNativeCallsMathPrivateV2(nativeBytes)
	if f != nil {
		t.Fatal(f)
	}
	if _, f = ProjectNativeCallsMathPrivate(nativeBytes); f == nil {
		t.Fatal("ASSERT_V1_ZERO_SITE_STAYS_REFUSED")
	}
	if _, f = Project(raw); f == nil {
		t.Fatal("ASSERT_ORDINARY_ZERO_SITE_STAYS_REFUSED")
	}
	var doc nativeGraphV2Fixture
	if json.Unmarshal(nativeBytes, &doc) != nil {
		t.Fatal("fixture decode")
	}
	count := 0
	for _, edge := range doc.Edges {
		count += len(edge.CallSites)
	}
	if len(m.NodeIdentities) != 3 || len(m.Occurrences) != count || count != 2 || len(m.PairWeights) != 2 {
		t.Fatal("ASSERT_V2_ZERO_SITE_DUPLICATE_ARITHMETIC", m, count)
	}
	weights := map[Pair]float64{}
	for _, o := range m.Occurrences {
		if o.Weight != 1 {
			t.Fatal("unit")
		}
		weights[Pair{o.From, o.To}]++
	}
	if !reflect.DeepEqual(weights, m.PairWeights) {
		t.Fatal("ASSERT_V2_DIRECTED_WEIGHTS")
	}
	// Typed relation validation is still required even when it has no call sites.
	var typed native
	if json.Unmarshal(nativeBytes, &typed) != nil {
		t.Fatal("typed decode")
	}
	// The native producer canonicalizes duplicate sites. The numerical driver
	// itself must still count every occurrence, not deduplicate a typed list.
	for i := range typed.Edges {
		if len(typed.Edges[i].CallSites) > 0 {
			typed.Edges[i].CallSites = append(typed.Edges[i].CallSites, typed.Edges[i].CallSites[0])
			break
		}
	}
	duplicateMath, failure := projectNativeCallsPolicyPrivate(typed, true)
	if failure != nil || len(duplicateMath.Occurrences) != 3 {
		t.Fatal("ASSERT_V2_NUMERIC_DUPLICATES_ARE_UNITS", failure)
	}
	typed.EvidenceReceipt.Relations[0].EvidenceRole = "DISCOVERY_ONLY"
	if _, f := projectNativeCallsPolicyPrivate(typed, true); f == nil {
		t.Fatal("ASSERT_V2_ZERO_SITE_TYPED_RECEIPT_REQUIRED")
	}
	typed.EvidenceReceipt = nil
	if _, f := projectNativeCallsPolicyPrivate(typed, true); f == nil {
		t.Fatal("ASSERT_V2_ZERO_SITE_RECEIPTS_REQUIRED")
	}
}

type nativeGraphV2Fixture struct {
	Edges []graph.Edge `json:"edges"`
}

func TestPrivateV2MathEdgelessAndNativeBounds(t *testing.T) {
	for _, nodes := range [][]graph.Node{nil, {node("a", 0)}, {node("a", 0), node("b", 1)}} {
		raw := validV5(t, nodes, nil)
		_, native := decodeV5(t, raw)
		m, f := ProjectNativeCallsMathPrivateV2(native)
		if f != nil {
			t.Fatal(f)
		}
		out, f := PartitionCallsMathPrivate(m, 42)
		if f != nil || out.Outcome != "EMPTY" || len(m.Occurrences) != 0 || len(out.Communities) != len(nodes) {
			t.Fatal("ASSERT_V2_EDGELESS_EMPTY", f)
		}
	}
	for _, raw := range [][]byte{nil, []byte(`{}`), make([]byte, (8<<20)+1)} {
		if _, f := ProjectNativeCallsMathPrivateV2(raw); f == nil {
			t.Fatal("ASSERT_V2_NATIVE_BOUNDS")
		}
	}
}
