package privatedescribeprep

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/programc"
)

// A typed, owned test provider only: no server/session/process or source IO.
type overlappingCallsFixture struct {
	caller, callee  lsp.CallHierarchyItem
	outSite, inSite lsp.Range
}

func (f overlappingCallsFixture) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return nil, nil
}
func (f overlappingCallsFixture) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	if p.Position == f.caller.SelectionRange.Start {
		return []lsp.CallHierarchyItem{f.caller}, nil
	}
	return []lsp.CallHierarchyItem{f.callee}, nil
}
func (f overlappingCallsFixture) OutgoingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	if item.Name == f.caller.Name {
		return []lsp.CallHierarchyOutgoingCall{{To: f.callee, FromRanges: []lsp.Range{f.outSite}}}, false, nil
	}
	return nil, true, nil
}
func (f overlappingCallsFixture) IncomingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	if item.Name == f.callee.Name {
		return []lsp.CallHierarchyIncomingCall{{From: f.caller, FromRanges: []lsp.Range{f.inSite}}}, false, nil
	}
	return nil, true, nil
}

func TestOwnedCallsFromActualAcquisitionSharedEdgeAndCache(t *testing.T) {
	item := func(name string, line uint32) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: "file:///fixture.go", Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 40}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 5}, End: lsp.Position{Line: line, Character: 6}}}
	}
	f := overlappingCallsFixture{caller: item("caller", 0), callee: item("callee", 3), outSite: lsp.Range{Start: lsp.Position{Line: 1, Character: 1}, End: lsp.Position{Line: 1, Character: 2}}, inSite: lsp.Range{Start: lsp.Position{Line: 2, Character: 1}, End: lsp.Position{Line: 2, Character: 2}}}
	target := func(id string, i lsp.CallHierarchyItem, down, up int) acquisition.Target {
		line, ch := i.SelectionRange.Start.Line, i.SelectionRange.Start.Character
		return acquisition.Target{ID: id, Locator: acquisition.Locator{URI: i.URI, Line: &line, Character: &ch}, DownDepth: down, UpDepth: up}
	}
	req := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "owned-overlap", SessionID: "typed-fixture", Generation: 1, PositionEncoding: "utf-16"}, Root: target("root", f.caller, 1, 0), RequiredTargets: []acquisition.Target{target("B", f.callee, 0, 1), target("C", f.caller, 1, 0)}, Limits: acquisition.Limits{MaxNodes: 16, MaxRequests: 32, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	a, err := acquisition.Acquire(context.Background(), f, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := acquisition.ValidateResult(a); err != nil {
		t.Fatal("actual acquisition invalid", err)
	}
	if len(a.Graph.Edges) != 1 || len(a.Graph.Edges[0].CallSites) != 2 || len(a.EdgeObservations) != 2 {
		t.Fatalf("shared-edge witness not established edges=%+v observations=%+v", a.Graph.Edges, a.EdgeObservations)
	}
	raw, err := json.Marshal(a.Graph)
	if err != nil {
		t.Fatal(err)
	}
	native, err := graph.ProjectRetainedV3ToV5Private(raw, "file:///", "owned-typed-fixture")
	if err != nil {
		t.Fatal(err)
	}
	math, failure := programc.ProjectNativeCallsMathPrivate(native)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(math.Occurrences) != 2 {
		t.Fatal("original math occurrence witness absent")
	}
	edge := a.Graph.Edges[0]
	for _, target := range a.Targets {
		var rows []Record
		ownedCallRecords("one-acquired-input", target, a, edge, math, func(r Record) { rows = append(rows, r) })
		want := f.outSite
		cached := false
		if target.Requested.ID == "B" {
			want = f.inSite
		}
		if target.Requested.ID == "C" {
			cached = true
		}
		observations, occurrences := 0, 0
		for _, r := range rows {
			switch r.Kind {
			case "REQUEST_OBSERVATION":
				var got struct {
					CallSite        graph.Range
					CachedReference bool
				}
				if err := json.Unmarshal(r.Value, &got); err != nil {
					t.Fatal(err)
				}
				if got.CallSite.Start.Line != want.Start.Line || got.CachedReference != cached {
					t.Fatalf("target %s borrowed site/cache %+v", target.Requested.ID, got)
				}
				observations++
			case "CALL_OCCURRENCE":
				var got struct {
					Occurrence        programc.Occurrence
					RequestReferences []ownedRequestRef
				}
				if err := json.Unmarshal(r.Value, &got); err != nil {
					t.Fatal(err)
				}
				if got.Occurrence.CallSite.Start.Line != want.Start.Line || len(got.RequestReferences) != 1 || got.RequestReferences[0].Cached != cached {
					t.Fatalf("target %s borrowed native occurrence %+v", target.Requested.ID, got)
				}
				if cached && got.RequestReferences[0].ObservedForTarget != "root" {
					t.Fatal("cache fabricated independent producer")
				}
				occurrences++
			}
		}
		if observations != 1 || occurrences != 1 {
			t.Fatalf("target %s lost own evidence %d %d", target.Requested.ID, observations, occurrences)
		}
	}
}
