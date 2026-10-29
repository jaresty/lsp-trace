package privatedescribeprep

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

// Same native relation, different actual response requests and ranges. This is
// intentionally WITHIN one input namespace, not an independent-input test.
func TestOwnedCallResponseRangesAndCachedReferences(t *testing.T) {
	siteA := graph.Range{Start: graph.Position{Line: 10}, End: graph.Position{Line: 10, Character: 1}}
	siteB := graph.Range{Start: graph.Position{Line: 20}, End: graph.Position{Line: 20, Character: 1}}
	edge := graph.Edge{RelationID: "relation", CallerNodeID: "caller", CalleeNodeID: "callee", CallSites: []graph.Range{siteA, siteB}}
	a := acquisition.Result{Requests: []acquisition.RequestRecord{
		{ID: "request-A", TargetID: "A", Method: "callHierarchy/outgoingCalls", NodeID: "caller", Attempted: true, Outcome: "SUCCESS", Response: json.RawMessage(`[{"fromRanges":[{"start":{"line":10,"character":0},"end":{"line":10,"character":1}}]}]`)},
		{ID: "request-B", TargetID: "B", Method: "callHierarchy/incomingCalls", NodeID: "callee", Attempted: true, Outcome: "SUCCESS", Response: json.RawMessage(`[{"fromRanges":[{"start":{"line":20,"character":0},"end":{"line":20,"character":1}}]}]`)},
	}, EdgeObservations: []acquisition.EdgeObservation{{RequestID: "request-A", RelationID: "relation", CallSites: []graph.Range{siteA}}, {RequestID: "request-B", RelationID: "relation", CallSites: []graph.Range{siteB}}}}
	math := programc.CallsMathPrivate{NodeIdentities: []string{"caller", "callee"}, Occurrences: []programc.Occurrence{
		{Identity: "original-native-occurrence-A", RelationID: "relation", Provider: "provider", From: 0, To: 1, CallSite: siteA, Weight: 1},
		{Identity: "original-native-occurrence-B", RelationID: "relation", Provider: "provider", From: 0, To: 1, CallSite: siteB, Weight: 1},
	}}
	targetA := acquisition.TargetResult{Requested: acquisition.Target{ID: "A"}, Outgoing: acquisition.DirectionResult{Expansions: []acquisition.Expansion{{NodeID: "caller", RequestID: "request-A", Status: acquisition.SuccessNonempty}}}}
	targetB := acquisition.TargetResult{Requested: acquisition.Target{ID: "B"}, Incoming: acquisition.DirectionResult{Expansions: []acquisition.Expansion{{NodeID: "callee", RequestID: "request-B", Status: acquisition.SuccessNonempty}}}}
	check := func(target acquisition.TargetResult, want graph.Range, wantRequest, wantOccurrence string, cached bool) {
		t.Helper()
		var rows []Record
		ownedCallRecords("same-input", target, a, edge, math, func(r Record) { rows = append(rows, r) })
		sites, observations, occurrences, refs := 0, 0, 0, 0
		for _, row := range rows {
			switch row.Kind {
			case "DIRECT_CALLSITE_RANGE":
				var got struct {
					CallSite          graph.Range
					RequestReferences []ownedRequestRef
				}
				_ = json.Unmarshal(row.Value, &got)
				if got.CallSite != want || len(got.RequestReferences) != 1 || got.RequestReferences[0].RequestID != wantRequest || got.RequestReferences[0].Cached != cached {
					t.Fatalf("borrowed range %+v", got)
				}
				sites++
			case "REQUEST_OBSERVATION":
				var got struct {
					RequestID       string
					CallSite        graph.Range
					CachedReference bool
				}
				_ = json.Unmarshal(row.Value, &got)
				if got.RequestID != wantRequest || got.CallSite != want || got.CachedReference != cached {
					t.Fatalf("borrowed observation %+v", got)
				}
				observations++
			case "CALL_OCCURRENCE":
				var got struct {
					Occurrence        programc.Occurrence
					RequestReferences []ownedRequestRef
				}
				_ = json.Unmarshal(row.Value, &got)
				if got.Occurrence.Identity != wantOccurrence || got.Occurrence.CallSite != want || len(got.RequestReferences) != 1 || got.RequestReferences[0].RequestID != wantRequest || got.RequestReferences[0].Cached != cached {
					t.Fatalf("borrowed native occurrence %+v", got)
				}
				occurrences++
			case "TARGET_OWNED_REQUEST_REFERENCE":
				var got ownedRequestRef
				_ = json.Unmarshal(row.Value, &got)
				if got.RequestID != wantRequest || got.Cached != cached {
					t.Fatalf("incorrect reference %+v", got)
				}
				refs++
			}
		}
		if sites != 1 || observations != 1 || occurrences != 1 || refs != 1 {
			t.Fatalf("lost owned evidence %d %d %d %d", sites, observations, occurrences, refs)
		}
	}
	check(targetA, siteA, "request-A", "original-native-occurrence-A", false)
	check(targetB, siteB, "request-B", "original-native-occurrence-B", false)
	// Explicit cache lineage in B reuses A's SAME request; never fabricate a B
	// response or count it as an independently acquired observation.
	targetB.Incoming = acquisition.DirectionResult{}
	targetB.Outgoing = targetA.Outgoing
	targetB.Outgoing.Expansions = append([]acquisition.Expansion(nil), targetA.Outgoing.Expansions...)
	targetB.Outgoing.Expansions[0].Cached = true
	check(targetB, siteA, "request-A", "original-native-occurrence-A", true)
	// Two target-owned responses for the SAME native site retain both request
	// references without multiplying or inventing a native occurrence identity.
	a.Requests[1].TargetID = "A"
	a.EdgeObservations[1].CallSites = []graph.Range{siteA}
	targetA.Incoming = acquisition.DirectionResult{Expansions: []acquisition.Expansion{{NodeID: "callee", RequestID: "request-B", Status: acquisition.SuccessNonempty}}}
	var sameSiteRows []Record
	ownedCallRecords("same-input", targetA, a, edge, math, func(r Record) { sameSiteRows = append(sameSiteRows, r) })
	occCount := 0
	for _, r := range sameSiteRows {
		if r.Kind == "CALL_OCCURRENCE" {
			var got struct {
				Occurrence        programc.Occurrence
				RequestReferences []ownedRequestRef
			}
			_ = json.Unmarshal(r.Value, &got)
			if got.Occurrence.Identity != "original-native-occurrence-A" || len(got.RequestReferences) != 2 {
				t.Fatal("same-site references lost or native occurrence fabricated")
			}
			occCount++
		}
	}
	if occCount != 1 {
		t.Fatal("same-site native occurrence duplicated")
	}
	targetB.Outgoing.Expansions[0].Cached = false
	var rows []Record
	ownedCallRecords("same-input", targetB, a, edge, math, func(r Record) { rows = append(rows, r) })
	for _, r := range rows {
		if r.Kind == "CALL_OCCURRENCE" || r.Kind == "DIRECT_CALLSITE_RANGE" || r.Kind == "REQUEST_OBSERVATION" {
			t.Fatal("noncached foreign request admitted")
		}
	}
}
