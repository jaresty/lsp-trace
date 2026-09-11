package ancillaryinspection

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/inspection"
)

const (
	assertExactReassembly = "P_ANCILLARY_EXACT_REASSEMBLY"
	assertManifestCounts  = "P_ANCILLARY_MANIFEST_TOTALS_AND_RETURNED"
	assertInvalidPages    = "P_ANCILLARY_INVALID_CONTINUATIONS_REJECTED"
	assertBounded         = "P_ANCILLARY_PAGE_BOUNDED"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b := inspection.Bundle{SchemaVersion: graph.SchemaVersionV3, Invocation: graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "s"}}}, Nodes: []graph.Node{}, Edges: []graph.Edge{}, DispatchRelationships: []graph.DispatchRelationship{}, SiblingCandidates: []graph.SiblingCandidate{}, Terminals: []graph.Boundary{{Reason: graph.Reason("DEPTH_LIMIT")}}, Frontier: []graph.Boundary{{Reason: graph.Reason("FRONTIER")}}, SeedMemberships: []graph.SeedMembership{}, Seeds: []graph.SeedResult{{Label: "s", PreparedTargetIDs: []string{}, ReachedNodeIDs: []string{}, ReachedRelationIDs: []string{}}}}
	for i := 0; i < 30; i++ {
		b.Diagnostics = append(b.Diagnostics, graph.Diagnostic{Phase: "test", Message: strings.Repeat(string(rune('a'+i%26)), 300)})
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func request(raw []byte) Request {
	encoded, _ := json.Marshal(string(raw))
	r := Request{Input: encoded, Ancillary: true, Page: true, Policy: Policy{MaxPageBytes: 4096, MaxPages: 100, MaxOutputBytes: 1 << 20}}
	r.Selector.AllSeeds = true
	return r
}

func TestAncillaryCanonicalPagesReassembleExactlyAndReportCounts(t *testing.T) {
	r := request(fixture(t))
	views := []View{}
	cursor := ""
	for {
		r.Cursor = cursor
		v, err := Inspect(r)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(v)
		if len(raw) > r.Policy.MaxPageBytes || v.Page == nil {
			t.Fatalf("%s: bytes=%d limit=%d", assertBounded, len(raw), r.Policy.MaxPageBytes)
		}
		totalReturned := v.Manifest.Seeds.Returned + v.Manifest.SeedMemberships.Returned + v.Manifest.Frontier.Returned + v.Manifest.Terminals.Returned + v.Manifest.Diagnostics.Returned
		if totalReturned != len(v.Page.Entries) {
			t.Fatalf("%s: returned=%d entries=%d", assertManifestCounts, totalReturned, len(v.Page.Entries))
		}
		views = append(views, v)
		cursor = v.NextCursor
		if cursor == "" {
			break
		}
	}
	r.Cursor = ""
	got, err := Reassemble(r, views)
	if err != nil {
		t.Fatalf("%s: %v", assertExactReassembly, err)
	}
	raw, _ := InputBytes(r.Input)
	want, _ := inspection.ProjectAllSeeds(raw)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: projection mismatch", assertExactReassembly)
	}
	m := views[0].Manifest
	if m.Seeds.Total != len(want.Seeds) || m.Diagnostics.Total != len(want.Records.Diagnostics) || m.Frontier.Total != len(want.Records.Frontier) || m.Terminals.Total != len(want.Records.Terminals) {
		t.Fatalf("%s: %#v", assertManifestCounts, m)
	}
}

func TestAncillaryRejectsTamperedSkippedDuplicateAndNoncanonicalPages(t *testing.T) {
	r := request(fixture(t))
	first, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	bad := first.NextCursor[:len(first.NextCursor)-1] + "A"
	r.Cursor = bad
	if _, err = Inspect(r); err == nil {
		t.Fatalf("%s: tampered cursor accepted", assertInvalidPages)
	}
	r.Cursor = first.NextCursor
	second, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	r.Cursor = ""
	if _, err = Reassemble(r, []View{first, first}); err == nil {
		t.Fatalf("%s: duplicate accepted", assertInvalidPages)
	}
	if _, err = Reassemble(r, []View{first, second}); err == nil && !(second.NextCursor == "") {
		t.Fatalf("%s: skipped/incomplete accepted", assertInvalidPages)
	}
	mutated := first
	mutated.Page = &Page{}
	*mutated.Page = *first.Page
	mutated.Page.Offset++
	if _, err = Reassemble(r, []View{mutated}); err == nil {
		t.Fatalf("%s: noncanonical accepted", assertInvalidPages)
	}
}
