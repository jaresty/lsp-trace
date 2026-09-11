package programc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func validV5(t *testing.T, nodes []graph.Node, edges []graph.Edge) []byte {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "gopls"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "gopls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}},
		Nodes:         nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: "seed"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return raw
}

func node(name string, line uint32) graph.Node {
	r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: r, SelectionRange: r})
}

func TestAdmissionRejectsMalformedAndUnqualifiedCalls(t *testing.T) {
	if _, err := Compute([]byte(`{"schema_version":"lsp-trace.graph-provenance.v5"`), 7); err == nil || err.Code != CodeInvalidProvenance {
		t.Fatalf("ASSERT_V5_MALFORMED_REJECTED: err=%#v", err)
	}
	a, b := node("a", 0), node("b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 4}, End: graph.Position{Line: 4, Character: 1}}}}})
	var envelope map[string]any
	_ = json.Unmarshal(raw, &envelope)
	native, _ := base64.StdEncoding.DecodeString(envelope["graph_v5"].(string))
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	receipt := doc["evidence_receipt"].(map[string]any)
	receipt["relations"].([]any)[0].(map[string]any)["evidence_class"] = "DISCOVERY_NOMINATION"
	changed, _ := json.Marshal(doc)
	envelope["graph_v5"] = base64.StdEncoding.EncodeToString(changed)
	sum := sha256.Sum256(changed)
	envelope["graph_v5_sha256"] = fmt.Sprintf("sha256:%x", sum)
	bad, _ := json.Marshal(envelope)
	if _, err := Compute(bad, 7); err == nil || err.Code != CodeInvalidProvenance {
		t.Fatalf("ASSERT_CALLS_CUSTODY_EVIDENCE_REJECTED: err=%#v", err)
	}
}

func TestProjectionPreservesOccurrencesLoopsMultiplicityAndStableIDs(t *testing.T) {
	z, a := node("z", 2), node("a", 0)
	sites := []graph.Range{{Start: graph.Position{Line: 3}, End: graph.Position{Line: 3, Character: 1}}, {Start: graph.Position{Line: 5}, End: graph.Position{Line: 5, Character: 1}}}
	raw := validV5(t, []graph.Node{z, a}, []graph.Edge{{CallerNodeID: z.ID, CalleeNodeID: z.ID, CallSites: sites}, {CallerNodeID: z.ID, CalleeNodeID: a.ID, CallSites: sites[:1]}})
	got, failure := Project(raw)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(got.Occurrences) != 3 || got.PairWeights[Pair{From: got.NodeIDs[z.ID], To: got.NodeIDs[z.ID]}] != 2 || got.Occurrences[0].Weight != 1 {
		t.Fatalf("ASSERT_OCCURRENCES_SELF_LOOP_MULTIPLICITY_PRESERVED: projection=%#v", got)
	}
	if got.NodeIDs[a.ID] != 0 || got.NodeIDs[z.ID] != 1 {
		t.Fatalf("ASSERT_STABLE_SORTED_NODE_IDS: ids=%#v", got.NodeIDs)
	}
}

func TestCapsAcceptEqualityRejectExcess(t *testing.T) {
	if failure := enforceCaps(MaxNodes, MaxOccurrences); failure != nil {
		t.Fatalf("ASSERT_CAP_EQUALITY_ACCEPTED_EXCESS_REJECTED: equality=%v", failure)
	}
	for _, tc := range []struct{ n, o int }{{MaxNodes + 1, 0}, {0, MaxOccurrences + 1}} {
		if failure := enforceCaps(tc.n, tc.o); failure == nil || failure.Code != CodeResourceLimit {
			t.Fatalf("ASSERT_CAP_EQUALITY_ACCEPTED_EXCESS_REJECTED: n=%d o=%d failure=%#v", tc.n, tc.o, failure)
		}
	}
}

func TestComputeDeterministicAcrossRepeatAndPermutation(t *testing.T) {
	a, b, c := node("a", 0), node("b", 1), node("c", 2)
	edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}, {CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{{}}}}
	first, failure := Compute(validV5(t, []graph.Node{c, a, b}, edges), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	second, failure := Compute(validV5(t, []graph.Node{b, c, a}, []graph.Edge{edges[1], edges[0]}), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	if first.LogicalDigest != second.LogicalDigest || !equalCommunities(first.Communities, second.Communities) {
		t.Fatalf("ASSERT_REPEAT_PERMUTATION_DETERMINISTIC: first=%#v second=%#v", first, second)
	}
	if first.Algorithm != "gonum.org/v1/gonum/graph/community.Leiden" || first.Resolution != 1 || first.Seed != 19 {
		t.Fatalf("ASSERT_EXACT_GONUM_LEIDEN_KERNEL: %#v", first)
	}
	const wantDigest = "sha256:76d4637884dfb6b1a039b60376768dc8967205272b807ec1ecbcfdfdd0f55b37"
	if first.LogicalDigest != wantDigest {
		t.Fatalf("ASSERT_CANONICAL_LOGICAL_DIGEST: got=%q want=%q", first.LogicalDigest, wantDigest)
	}
	if first.ProfileID != ProfileID || first.ProfileDigest != ProfileDigest || first.ClaimCeiling != ClaimCeiling {
		t.Fatalf("ASSERT_TYPED_CLAIM_CEILING_AND_SURFACE_UNCHANGED: %#v", first)
	}
}
