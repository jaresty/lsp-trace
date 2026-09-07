package acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/retainedpath"
)

// ValidateResult checks the native graph and coordinator foreign keys separately.
// It establishes consistency, not independent authentication of client claims.
func ValidateResult(r Result) error {
	if err := ValidateRequest(r.Request); err != nil {
		return err
	}
	if err := r.Graph.ValidateReferences(); err != nil {
		return err
	}
	if r.Policy != Policy || r.PathProjection != "NATIVE_CALLS_CALLER_TO_CALLEE_UNIT_GROUP_HOPS_V1" {
		return errors.New("unknown coordination/projection policy")
	}
	wanted := append([]Target{r.Request.Root}, r.Request.RequiredTargets...)
	if len(wanted) != len(r.Targets) || len(r.Graph.Seeds) != len(wanted) || len(r.Graph.Invocation.Seeds) != len(wanted) {
		return errors.New("missing target accounting")
	}
	u, l := r.Usage, r.Request.Limits
	if u.Nodes != len(r.Graph.Nodes) || u.Nodes > l.MaxNodes || u.Requests > l.MaxRequests || u.EvidenceBytes > l.MaxEvidenceBytes || u.PathWork > l.MaxPathWork || u.PrepareProbes > MaxPrepareProbes || u.Requests < 0 || u.EvidenceBytes < 0 || u.PathWork < 0 {
		return errors.New("invalid shared usage")
	}
	nodes := map[string]graph.Node{}
	for _, n := range r.Graph.Nodes {
		if err := graph.ValidateItem(n.Item); err != nil {
			return err
		}
		nodes[n.ID] = n
	}
	edges := map[string]graph.Edge{}
	for _, e := range r.Graph.Edges {
		edges[e.RelationID] = e
	}
	records := map[string]RequestRecord{}
	attempts, probes, evidenceBytes := 0, 0, 0
	for i, rec := range r.Requests {
		if rec.ID != fmt.Sprintf("request-%06d", i+1) || rec.Context != r.Request.Context {
			return errors.New("request identity/context mismatch")
		}
		owner := false
		for _, t := range wanted {
			if t.ID == rec.TargetID {
				owner = true
			}
		}
		if !owner {
			return errors.New("unknown request owner")
		}
		if rec.Before.Requests != attempts || rec.Before.PrepareProbes != probes || rec.Before.EvidenceBytes != evidenceBytes || rec.EvidenceBytes < 0 {
			return errors.New("request budget context mismatch")
		}
		evidenceBytes += rec.EvidenceBytes
		if rec.CaptureComplete && rec.EvidenceBytes != len(rec.Params)+len(rec.Response)+2 {
			return errors.New("captured evidence byte accounting mismatch")
		}
		if rec.Attempted {
			attempts++
			if rec.Method == "textDocument/prepareCallHierarchy" {
				probes++
			}
		}
		switch rec.Outcome {
		case "SUCCESS", "FAILED", "UNSUPPORTED":
			if !rec.Attempted || !rec.CaptureComplete || !json.Valid(rec.Response) {
				return errors.New("missing response capture")
			}
		case "CAPTURE_INCOMPLETE", "CAPTURE_FAILED":
			if !rec.Attempted || rec.CaptureComplete {
				return errors.New("false incomplete capture")
			}
		case "BUDGET_BLOCKED":
			if rec.Attempted {
				return errors.New("fabricated budget request")
			}
		default:
			return errors.New("invalid request outcome")
		}
		records[rec.ID] = rec
	}
	if attempts != u.Requests || probes != u.PrepareProbes || evidenceBytes != u.EvidenceBytes {
		return errors.New("request attempt accounting mismatch")
	}
	seedByID := map[string]graph.SeedResult{}
	for _, s := range r.Graph.Seeds {
		seedByID[s.Label] = s
	}
	pathNodes, pathEdges := PathInput(r.Graph, r.Request.Context.ID)
	_ = pathNodes
	work := 0
	complete := true
	for i, t := range r.Targets {
		if !reflect.DeepEqual(t.Requested, wanted[i]) {
			return errors.New("target request substitution")
		}
		if t.Resolution.Status != Resolved || t.Admission != Admitted {
			complete = false
		}
		for _, d := range []DirectionResult{t.Outgoing, t.Incoming} {
			if d.Status != SuccessEmpty && d.Status != SuccessNonempty && d.Status != ExpansionNotApplicable {
				complete = false
			}
		}
		seed, ok := seedByID[t.Requested.ID]
		if !ok {
			return errors.New("missing native seed")
		}
		for _, id := range t.Resolution.RequestIDs {
			if _, ok := records[id]; !ok {
				return errors.New("dangling resolution request")
			}
		}
		switch t.Resolution.Status {
		case Resolved:
			rr := t.Resolution
			if rr.Identity == nil || rr.Prepared == nil || rr.Position == nil || !graph.SameNodeIdentity(*rr.Identity, node(*rr.Prepared)) || !bytes.Equal(rr.Identity.Data, rr.Prepared.Data) || rr.Identity.URI != t.Requested.Locator.URI {
				return errors.New("resolved identity mismatch")
			}
			if t.Admission == Admitted {
				n, ok := nodes[rr.Identity.ID]
				if !ok || !graph.SameNodeIdentity(n, *rr.Identity) || len(seed.PreparedTargetIDs) != 1 || seed.PreparedTargetIDs[0] != n.ID {
					return errors.New("admitted identity missing")
				}
			} else if t.Admission != AdmissionBlocked {
				return errors.New("resolved admission absent")
			}
		case Missing, Ambiguous, ResolutionFailed, ResolutionUnsupported, ResolutionBlocked:
			if t.Resolution.Identity != nil || t.Resolution.Prepared != nil || t.Admission != NotApplicable {
				return errors.New("unresolved identity/admission claim")
			}
		default:
			return errors.New("invalid resolution disposition")
		}
		if t.Admission != Admitted && len(seed.PreparedTargetIDs) != 0 {
			return errors.New("unadmitted native target")
		}
		for _, d := range []struct {
			direction Direction
			result    DirectionResult
		}{{Outgoing, t.Outgoing}, {IncomingDirection, t.Incoming}} {
			out := d.result
			if len(out.Expansions) > 0 && out.Status != aggregate(out.Expansions) {
				return errors.New("expansion aggregate mismatch")
			}
			seen := map[string]bool{}
			for _, e := range out.Expansions {
				if _, ok := nodes[e.NodeID]; !ok || seen[e.NodeID] {
					return errors.New("expansion node missing/duplicate")
				}
				seen[e.NodeID] = true
				if e.Depth < 0 || e.Depth >= len(out.Layers) || !has(out.Layers[e.Depth].NodeIDs, e.NodeID) {
					return errors.New("expansion layer mismatch")
				}
				if e.Status == Frontier {
					if e.RequestID != "" || !has(out.FrontierIDs, e.NodeID) {
						return errors.New("frontier request claim")
					}
					continue
				}
				rec, ok := records[e.RequestID]
				method := "callHierarchy/incomingCalls"
				if d.direction == Outgoing {
					method = "callHierarchy/outgoingCalls"
				}
				if !ok || rec.NodeID != e.NodeID || rec.Method != method {
					return errors.New("expansion request mismatch")
				}
				if (e.Status == SuccessEmpty || e.Status == SuccessNonempty || e.Status == Partial) && rec.Outcome != "SUCCESS" {
					return errors.New("failed request is not successful expansion")
				}
				switch e.Status {
				case SuccessEmpty, SuccessNonempty, Partial:
					var calls []json.RawMessage
					if json.Unmarshal(rec.Response, &calls) != nil || (e.Status == SuccessEmpty && len(calls) != 0) || (e.Status != SuccessEmpty && len(calls) == 0) {
						return errors.New("expansion empty/nonempty mismatch")
					}
				case Failed:
					if rec.Outcome != "FAILED" && rec.Outcome != "CAPTURE_FAILED" {
						return errors.New("false failed expansion")
					}
				case Unsupported:
					if rec.Outcome != "UNSUPPORTED" {
						return errors.New("false unsupported expansion")
					}
				case BudgetBlocked:
					if rec.Outcome != "BUDGET_BLOCKED" && rec.Outcome != "CAPTURE_INCOMPLETE" && !(e.Cached && (e.Reason == "context canceled" || e.Reason == "context deadline exceeded")) {
						return errors.New("false budget-blocked expansion")
					}
				default:
					return errors.New("unknown expansion status")
				}
			}
			for _, ids := range [][]string{out.StartIDs, out.FrontierIDs, out.SuccessfulEmptyIDs} {
				for _, id := range ids {
					if !seen[id] {
						return errors.New("directional set is not expanded/frontier")
					}
				}
			}
			for depth, layer := range out.Layers {
				if layer.Depth != depth {
					return errors.New("noncontiguous layers")
				}
				for _, id := range layer.NodeIDs {
					if !seen[id] {
						return errors.New("unaccounted layer member")
					}
				}
			}
		}
		conn := t.Connection
		work += conn.Work
		evaluable := i > 0 && r.Targets[0].Admission == Admitted && t.Admission == Admitted
		if !evaluable {
			if conn.Status != "NOT_EVALUABLE" || len(conn.Path.Nodes) != 0 || conn.Work != 0 {
				return errors.New("unevaluable endpoint has path claim")
			}
			continue
		}
		from, to := r.Targets[0].Resolution.Identity.ID, t.Resolution.Identity.ID
		if r.Request.Mode == Incoming {
			from, to = to, from
		}
		if conn.From != from || conn.To != to || conn.Direction != "CALLER_TO_CALLEE" || conn.Work < 0 {
			return errors.New("connection endpoint/direction mismatch")
		}
		switch conn.Status {
		case "FOUND", "NOT_FOUND_IN_RETAINED_GRAPH":
			if conn.Status == "NOT_FOUND_IN_RETAINED_GRAPH" && (len(conn.Path.Nodes) != 0 || len(conn.Path.GroupIDs) != 0 || len(conn.Path.OccurrenceIDs) != 0) {
				return errors.New("nonempty not-found witness")
			}
			if err := retainedpath.Prove(pathEdges, from, to, conn.Status, conn.Path); err != nil {
				return err
			}
		case "INCOMPLETE":
			if conn.Reason != "LIMIT" && conn.Reason != "CANCELLED" {
				return errors.New("invalid incomplete reason")
			}
			if len(conn.Path.Nodes) != 0 || len(conn.Path.GroupIDs) != 0 || len(conn.Path.OccurrenceIDs) != 0 {
				return errors.New("partial path leaked")
			}
		default:
			return errors.New("invalid connection disposition")
		}
	}
	if complete != r.AcquisitionComplete {
		return errors.New("acquisition completeness mismatch")
	}
	if work != u.PathWork {
		return errors.New("shared path work mismatch")
	}
	for _, o := range r.EdgeObservations {
		rec, ok := records[o.RequestID]
		if !ok || rec.Outcome != "SUCCESS" || !rec.Attempted || (rec.Method != "callHierarchy/incomingCalls" && rec.Method != "callHierarchy/outgoingCalls") {
			return errors.New("unsupported edge observation")
		}
		edge, ok := edges[o.RelationID]
		if !ok {
			return errors.New("dangling observed relation")
		}
		for _, site := range o.CallSites {
			found := false
			for _, retained := range edge.CallSites {
				if site == retained {
					found = true
				}
			}
			if !found {
				return errors.New("missing observed site")
			}
		}
	}
	if err := validateObservedJoins(r, records); err != nil {
		return err
	}
	if err := validateTargetJoins(r, records); err != nil {
		return err
	}
	if err := validateRecordReplay(r); err != nil {
		return err
	}
	return validatePathAccounting(r)
}
func has(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
