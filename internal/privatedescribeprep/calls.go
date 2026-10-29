package privatedescribeprep

import (
	"fmt"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

type ownedRequestRef struct {
	RequestID, Direction, ObservedForTarget string
	Cached                                  bool
	ExpansionOrdinal                        int
}

// ownedCallRecords never uses merged edge ranges as observation ownership.
// Only this target's explicit expansion -> request -> edge-observation chain
// supplies ranges. Cached expansion references reuse the original observation;
// they are not additional responses or independent support.
func ownedCallRecords(id string, t acquisition.TargetResult, a acquisition.Result, e graph.Edge, math programc.CallsMathPrivate, add func(Record)) {
	var refs []ownedRequestRef
	for _, d := range []struct {
		Name, Method, Node string
		Result             acquisition.DirectionResult
	}{
		{"OUTGOING", "callHierarchy/outgoingCalls", e.CallerNodeID, t.Outgoing},
		{"INCOMING", "callHierarchy/incomingCalls", e.CalleeNodeID, t.Incoming},
	} {
		for ordinal, exp := range d.Result.Expansions {
			if exp.NodeID != d.Node || exp.RequestID == "" || (exp.Status != acquisition.SuccessNonempty && exp.Status != acquisition.Partial) {
				continue
			}
			for _, req := range a.Requests {
				if req.ID != exp.RequestID || req.NodeID != exp.NodeID || req.Method != d.Method || req.Outcome != "SUCCESS" || !req.Attempted {
					continue
				}
				if !exp.Cached && req.TargetID != t.Requested.ID {
					continue
				}
				refs = append(refs, ownedRequestRef{req.ID, d.Name, req.TargetID, exp.Cached, ordinal})
			}
		}
	}
	ownedSites := map[graph.Range]bool{}
	siteRefs := map[graph.Range][]ownedRequestRef{}
	callerURI := ""
	for _, node := range a.Graph.Nodes {
		if node.ID == e.CallerNodeID {
			callerURI = node.URI
		}
	}
	ownedObservations, excludedObservations := 0, 0
	for obsOrdinal, obs := range a.EdgeObservations {
		if obs.RelationID != e.RelationID {
			continue
		}
		linked := false
		for _, ref := range refs {
			if ref.RequestID != obs.RequestID {
				continue
			}
			linked = true
			add(record("TARGET_OWNED_REQUEST_REFERENCE", fmt.Sprintf("%s/seed/%s/%s/expansion/%d", id, t.Requested.ID, ref.Direction, ref.ExpansionOrdinal), ref))
			for siteOrdinal, site := range obs.CallSites {
				ownedSites[site] = true
				siteRefs[site] = append(siteRefs[site], ref)
				// Source is always the caller, including incoming observations.
				add(record("REQUEST_OBSERVATION", fmt.Sprintf("%s/seed/%s/request/%s/relation/%s/observation/%d/site/%d", id, t.Requested.ID, obs.RequestID, e.RelationID, obsOrdinal, siteOrdinal), struct {
					RequestID, RelationID, AcquisitionDirection, CallerNodeID, CalleeNodeID string
					CachedReference                                                         bool
					CallSite                                                                graph.Range
				}{obs.RequestID, e.RelationID, ref.Direction, e.CallerNodeID, e.CalleeNodeID, ref.Cached, site}))
			}
		}
		if linked {
			ownedObservations++
		} else {
			excludedObservations++
		}
	}
	// Iterate original merged order only to order the owned subset, not to admit it.
	ownedSiteCount := 0
	for _, site := range e.CallSites {
		if ownedSites[site] {
			ownedSiteCount++
			add(record("DIRECT_CALLSITE_RANGE", fmt.Sprintf("%s/seed/%s/relation/%s/owned-site/%d", id, t.Requested.ID, e.RelationID, ownedSiteCount-1), struct {
				CallSite                                        graph.Range
				CallerNodeID, CallerSourceURI, PositionEncoding string
				RequestReferences                               []ownedRequestRef
			}{site, e.CallerNodeID, callerURI, a.Request.Context.PositionEncoding, siteRefs[site]}))
		}
	}
	add(record("DIRECT_CALLS", id+"/seed/"+t.Requested.ID+"/relation/"+e.RelationID, struct {
		RelationID, CallerNodeID, CalleeNodeID, EndpointMeaning      string
		OwnedCallSites, OwnedObservations, ExcludedInputObservations int
	}{e.RelationID, e.CallerNodeID, e.CalleeNodeID, "UNKNOWN", ownedSiteCount, ownedObservations, excludedObservations}))
	for _, o := range math.Occurrences {
		if o.RelationID != e.RelationID || o.From < 0 || o.To < 0 || o.From >= int64(len(math.NodeIdentities)) || o.To >= int64(len(math.NodeIdentities)) || math.NodeIdentities[o.From] != e.CallerNodeID || math.NodeIdentities[o.To] != e.CalleeNodeID {
			continue
		}
		site := graph.Range{Start: graph.Position{Line: o.CallSite.Start.Line, Character: o.CallSite.Start.Character}, End: graph.Position{Line: o.CallSite.End.Line, Character: o.CallSite.End.Character}}
		if !ownedSites[site] {
			continue
		}
		// Original native math occurrence identity/provider fields are preserved.
		add(record("CALL_OCCURRENCE", id+"/seed/"+t.Requested.ID+"/occurrence/"+o.Identity, struct {
			Occurrence                        programc.Occurrence
			CallerSourceURI, PositionEncoding string
			RequestReferences                 []ownedRequestRef
		}{o, callerURI, a.Request.Context.PositionEncoding, siteRefs[site]}))
	}
	if excludedObservations > 0 || ownedObservations == 0 {
		add(record("GAP", id+"/seed/"+t.Requested.ID+"/relation/"+e.RelationID+"/ownership", struct {
			Reason               string
			ExcludedObservations int
		}{"INPUT_GLOBAL_OBSERVATIONS_NOT_TARGET_OWNED", excludedObservations}))
	}
}
