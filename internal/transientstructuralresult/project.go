package transientstructuralresult

import (
	"errors"
	"lsp-trace/internal/transientstructural"
)

// Project converts the live kernel result into the single transport-neutral CLI/MCP result.
func Project(in transientstructural.Result, q transientstructural.Request, transientID string) (Result, error) {
	nodeIDs := make(map[string]string, len(in.Analysis.Nodes))
	nodes := make([]Node, len(in.Analysis.Nodes))
	for i, n := range in.Analysis.Nodes {
		id, e := NodeID(transientID, q.Generation, n.ID)
		if e != nil {
			return Result{}, ErrInvalid
		}
		nodeIDs[n.ID] = id
		nodes[i] = Node{ID: id}
	}
	target, ok := nodeIDs[in.TargetID]
	if !ok && in.Analysis.Kind == transientstructural.AnalysisImpact {
		var e error
		target, e = NodeID(transientID, q.Generation, in.TargetID)
		if e != nil {
			return Result{}, ErrInvalid
		}
		nodeIDs[in.TargetID] = target
		nodes = append([]Node{{ID: target}}, nodes...)
		ok = true
	}
	if !ok {
		return Result{}, ErrInvalid
	}
	edges := make([]Edge, len(in.Analysis.Occurrences))
	for i, e := range in.Analysis.Occurrences {
		caller, cok := nodeIDs[e.CallerID]
		callee, kok := nodeIDs[e.CalleeID]
		id, x := EdgeID(transientID, q.Generation, e.ID, caller, callee)
		if x != nil || !cok || !kok {
			return Result{}, ErrInvalid
		}
		edges[i] = Edge{ID: id, CallerNodeID: caller, CalleeNodeID: callee}
	}
	var incoming, outgoing uint64
	for _, e := range edges {
		if e.CalleeNodeID == target {
			incoming++
		}
		if e.CallerNodeID == target {
			outgoing++
		}
	}
	analysis := any(NeighborhoodResult{RootNodeID: target, Nodes: nodes, Edges: edges, IncomingCount: incoming, OutgoingCount: outgoing})
	if in.Analysis.Kind == transientstructural.AnalysisImpact {
		reachable := []string{}
		for _, n := range nodes {
			if n.ID != target {
				reachable = append(reachable, n.ID)
			}
		}
		w := []string{}
		for _, e := range edges {
			w = append(w, e.ID)
		}
		analysis = ImpactResult{RootNodeID: target, Direction: string(q.Analysis.Direction), Depth: uint64(q.Analysis.MaxDepth), Nodes: nodes, Edges: edges, ReachableNodeIDs: reachable, WitnessEdgeIDs: w}
	}
	a := Accounting{RequestAttempted: uint64(in.Accounting.Requests.Attempted), RequestSucceeded: uint64(in.Accounting.Requests.Succeeded), RequestFailed: uint64(in.Accounting.Requests.Failed), RequestCancelled: uint64(in.Accounting.Requests.Cancelled), PreparedAttempted: uint64(in.Accounting.Preparation.Attempted), PreparedReturned: uint64(in.Accounting.Preparation.Returned), PreparedEmpty: uint64(in.Accounting.Preparation.Empty), PreparedFailed: uint64(in.Accounting.Preparation.Failed), NodeObserved: uint64(in.Accounting.Nodes.Observed), NodeAdmitted: uint64(in.Accounting.Nodes.Admitted), NodeRejected: uint64(in.Accounting.Nodes.Rejected), NodeOmitted: uint64(in.Accounting.Nodes.Omitted), OccurrenceObserved: uint64(in.Accounting.Occurrences.Observed), OccurrenceAdmitted: uint64(in.Accounting.Occurrences.Admitted), OccurrenceRejected: uint64(in.Accounting.Occurrences.Rejected), OccurrenceOmitted: uint64(in.Accounting.Occurrences.Omitted), FrontierObserved: uint64(in.Accounting.Frontier.Observed), FrontierExpanded: uint64(in.Accounting.Frontier.Expanded), FrontierUnexpanded: uint64(in.Accounting.Frontier.Unexpanded), RequestOmissionReasons: EmptyReasonMap(), NodeOmissionReasons: EmptyReasonMap(), OccurrenceOmissionReasons: EmptyReasonMap(), FrontierOmissionReasons: EmptyReasonMap()}
	if a.RequestFailed != 0 || a.RequestCancelled != 0 {
		return Result{}, errors.New("projection failed")
	}
	if a.NodeOmitted > 0 {
		a.NodeOmissionReasons[Deduplication] = a.NodeOmitted
		a.DeduplicatedNodes = a.NodeOmitted
	}
	if a.OccurrenceOmitted > 0 {
		a.OccurrenceOmissionReasons[Deduplication] = a.OccurrenceOmitted
		a.DeduplicatedOccurrences = a.OccurrenceOmitted
	}
	for _, o := range in.Accounting.Omissions {
		if o.Count < 0 {
			return Result{}, ErrInvalid
		}
		reason := OmissionReason(o.Reason)
		count := uint64(o.Count)
		if reason == Deduplication {
			d := a.NodeOmitted + a.OccurrenceOmitted
			if count < d {
				return Result{}, ErrInvalid
			}
			count -= d
		}
		if count > 0 {
			if _, ok := a.FrontierOmissionReasons[reason]; !ok {
				return Result{}, ErrInvalid
			}
			a.FrontierOmissionReasons[reason] += count
		}
	}
	nq := Request{Generation: q.Generation, DownDepth: uint64(q.DownDepth), UpDepth: uint64(q.UpDepth), MaxNodes: uint64(q.MaxNodes), TimeoutMS: uint64(q.TimeoutMS), RequestTimeoutMS: uint64(q.RequestTimeoutMS), MaxMessages: uint64(q.MaxMessages), MaxBytes: uint64(q.MaxBytes), Analysis: AnalysisRequest{Kind: string(q.Analysis.Kind), Direction: string(q.Analysis.Direction), Depth: uint64(q.Analysis.MaxDepth)}}
	out := NewResult(transientID, q.Generation, target, in.Qualification.PositionEncoding, nq, a, analysis)
	if e := out.Validate(); e != nil {
		return Result{}, e
	}
	return out, nil
}
