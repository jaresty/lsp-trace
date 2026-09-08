package hydratedevidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

// Separate synthetic V1 cases; the committed original FR20 fixture is never
// rewritten or replaced. Capture uses only a disposable test file, no provider.
func focusV1(t *testing.T, missing bool, sites []graph.Range) (Input, string, string) {
	t.Helper()
	input, _, path := nativeFixture(t)
	var env graphprovenance.Evidence
	if e := json.Unmarshal(input.Artifact, &env); e != nil {
		t.Fatal(e)
	}
	g, decodeErr := graph.DecodeNativeV3(env.GraphBytes)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	id := g.Nodes[0].ID
	g.Edges = graph.MergeEdge(nil, graph.Edge{CallerNodeID: id, CalleeNodeID: id, CallSites: sites})
	g.Seeds[0].ReachedRelationIDs = []string{g.Edges[0].RelationID}
	g.Slice.OutgoingRelationIDs = []string{g.Edges[0].RelationID}
	g.Canonicalize()
	if missing {
		if e := os.Remove(path); e != nil {
			t.Fatal(e)
		}
	}
	raw, e := graphprovenance.Capture(context.Background(), encoded(t, g), filepath.Dir(path), g.Nodes[0].URI, "s", 1, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !missing {
		if e := os.Remove(path); e != nil {
			t.Fatal(e)
		}
	}
	return Input{Artifact: raw}, id, g.Edges[0].RelationID
}
func TestFocusedV1ZeroSitesAndMissing(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "readable", true: "missing"}[missing], func(t *testing.T) {
			in, node, edge := focusV1(t, missing, nil)
			f := DefaultFocusRequest()
			f.RelationIDs = []string{edge}
			f.IncludeBodies = true
			f.PositionEncoding = "utf-8"
			r := focusedRun(t, in, f)
			if len(r.Manifest.Origins) != 1 || r.Manifest.Origins[0].Status != "NO_CALL_SITES" || r.Manifest.Origins[0].CallSiteCount != 0 || len(r.Bundle.Origins) != 0 || r.Bundle.Complete {
				t.Fatal("ASSERT_NATIVE_EMPTY: zero-sites is explicit, not resolved context")
			}
			if e := ValidateFocused(in, f, r); e != nil {
				t.Fatal(e)
			}
			f.NodeIDs = []string{node}
			r = focusedRun(t, in, f)
			if len(r.Manifest.Origins[0].Sites) != 1 || r.Manifest.Origins[0].Sites[0].Pointer != "/nodes/0/range" || len(r.Bundle.Origins) != 1 {
				t.Fatal("ASSERT_NATIVE_EMPTY: V1 exact node join")
			}
			want := "EXPORTED"
			if missing {
				want = "MISSING_BYTES"
			}
			if r.Bundle.Origins[0].Status != want {
				t.Fatalf("ASSERT_NATIVE_EMPTY: got %s want %s", r.Bundle.Origins[0].Status, want)
			}
			if e := ValidateFocused(in, f, r); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestFocusedV1DirectSitesInvalidAndSPAN(t *testing.T) {
	in, _, edge := focusV1(t, false, []graph.Range{{Start: graph.Position{Character: 0}, End: graph.Position{Character: 1}}, {Start: graph.Position{Character: 90}, End: graph.Position{Character: 91}}})
	f := DefaultFocusRequest()
	f.RelationIDs = []string{edge}
	f.IncludeBodies = true
	f.PositionEncoding = "utf-8"
	r := focusedRun(t, in, f)
	if len(r.Manifest.Origins) != 1 || len(r.Manifest.Origins[0].Sites) != 2 || r.Manifest.Origins[0].Sites[1].Pointer != "/edges/0/call_sites/1" || len(r.Bundle.Origins) != 2 {
		t.Fatal("ASSERT_NATIVE_COORDINATES: exact second callsite")
	}
	if r.Bundle.Origins[0].Status != "EXPORTED" || r.Bundle.Origins[1].Status != "INVALID_COORDINATES" {
		t.Fatal("ASSERT_NATIVE_COORDINATES: invalid range upgraded")
	}
	if e := ValidateFocused(in, f, r); e != nil {
		t.Fatal(e)
	}
	req := r.Request
	req.Selections = append([]Selection{}, req.Selections[:1]...)
	req.Selections[0].Mode = "SPAN"
	req.Selections[0].Bytes = &Interval{0, 1}
	b, e := Hydrate(in, req)
	if e != nil {
		t.Fatal(e)
	}
	if b.Origins[0].Record.Authority != Native || b.Origins[0].CoordinateAuthority != Caller {
		t.Fatal("ASSERT_NATIVE_COORDINATES: caller SPAN authority upgraded")
	}
	if e = Validate(in, req, b); e != nil {
		t.Fatal(e)
	}
}
func TestFocusedIndependentCoverage(t *testing.T) {
	in := focusedFixture(t)
	f := DefaultFocusRequest()
	f.RelationIDs = []string{focusEdge}
	r := focusedRun(t, in, f)
	if e := auditNativeFocus(in, f, r); e != nil {
		t.Fatal(e)
	}
	r.Manifest.Origins[0].Sites = nil
	r.Manifest.Origins[0].CallSiteCount = 0
	r.Request.Selections = nil
	if auditNativeFocus(in, f, r) == nil {
		t.Fatal("ASSERT_INDEPENDENT_COVERAGE: coherently omitted native callsite accepted")
	}
}
func TestFocusedLimitsAndPolicyBinding(t *testing.T) {
	in := focusedFixture(t)
	f := DefaultFocusRequest()
	f.RelationIDs = []string{focusEdge}
	f.CorePolicy.IncludeBodies = true
	r := focusedRun(t, in, f)
	if len(r.Bundle.Spans) != 0 {
		t.Fatal("ASSERT_FOCUS_LIMITS: core policy bypassed body opt-in")
	}
	changed := f
	changed.WholeFile = true
	if ValidateFocused(in, changed, r) == nil {
		t.Fatal("ASSERT_FOCUS_LIMITS: external selection changed")
	}
	f.CorePolicy.MaxOrigins = 1
	if _, e := HydrateFocused(in, f); e == nil {
		t.Fatal("ASSERT_FOCUS_LIMITS: expanded receipt budget bypass")
	}
	f.CorePolicy = DefaultPolicy()
	f.CorePolicy.MaxOutputBytes = 1
	if _, e := HydrateFocused(in, f); e == nil {
		t.Fatal("ASSERT_FOCUS_LIMITS: output budget bypass")
	}
}
