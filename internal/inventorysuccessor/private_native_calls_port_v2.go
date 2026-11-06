package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
	"net/url"
)

var ErrPrivateNativeConstructorV2 = errors.New("private native constructor refused")

type PrivateNativeCallsRequestV2 struct {
	Session           sessionruntime.SessionLeaseRequest
	Directional       sessionruntime.DirectionalCallHierarchyRequestPrivate
	Revision, Privacy string
}
type PrivateNativeRawJoinV2 = referencegrouping.PrivateNativeRawJoinV2
type PrivateNativeCallsResultV2 struct {
	Observations     []sessionruntime.PrivateDirectionalRequestObservationV2
	ParentSupply     sessionruntime.DocumentSupply
	Views            [2]acquisition.PrivateDirectionalViewV2
	Sources          [2]graphprovenance.EvidenceV2
	Inputs           []programccompose.Input
	Composite        programccompose.Result
	Admission        programcadmission.Result
	RawJoins         []PrivateNativeRawJoinV2
	ParentInvocation graph.Invocation
	nativeEvidence   []byte // fixed producer continuation only; never supplied by caller
}

// A fixed private constructor; no caller DTO, manual Complete, new grant, client
// execution, generic capture engine, or public admission path is accepted here.
func ConstructNativeCallsPrivateV2(ctx context.Context, m *sessionruntime.Manager, q PrivateNativeCallsRequestV2) (PrivateNativeCallsResultV2, error) {
	return constructNativeCallsPrivateV2(ctx, m, q, false)
}

func constructNativeCallsPrivateV2(ctx context.Context, m *sessionruntime.Manager, q PrivateNativeCallsRequestV2, retain bool) (PrivateNativeCallsResultV2, error) {
	var out PrivateNativeCallsResultV2
	if ctx == nil || m == nil || len(q.Revision) == 0 || len(q.Revision) > 4096 || len(q.Privacy) == 0 || len(q.Privacy) > 4096 || len(q.Session.Workspace) > 4096 {
		return out, ErrPrivateNativeConstructorV2
	}
	if q.Directional.Deadline.IsZero() {
		return out, ErrPrivateNativeConstructorV2
	}
	if d, ok := ctx.Deadline(); !ok || d.After(q.Directional.Deadline) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, q.Directional.Deadline)
		defer cancel()
	}
	h, err := m.CaptureDirectionalEvidencePrivateV2(ctx, q.Session, q.Directional)
	if err != nil {
		return out, err
	}
	v, err := m.TakeDirectionalEvidencePrivateV2(ctx, h)
	if err != nil {
		return out, err
	}
	// Fixed synchronous work stays inside the caller's existing active lease.
	// The runtime executes no caller-supplied function and the lease owns the
	// protocol exclusion until this function and its enclosing callback return.
	err = func() error {
		out.Observations = v.Requests
		out.ParentSupply = v.Supply
		if _, ok := privateNativeEncodedSizeV2(uint64(len(v.Supply.Content))+uint64(len(v.Supply.Params)), 8, 65536); !ok {
			return ErrPrivateNativeConstructorV2
		}
		var rawSize uint64
		for _, r := range v.Requests {
			if len(r.Params) > 65536 || len(r.Result) > 65536 {
				return ErrPrivateNativeConstructorV2
			}
			rawSize += uint64(len(r.Params)) + uint64(len(r.Result))
			// Frame detachment admits <=128KiB and <=32 responses per request.
			if len(r.RequestFrame) > 65536 || len(r.ResponseFrames) > 32 {
				return ErrPrivateNativeConstructorV2
			}
			frameBytes := len(r.RequestFrame)
			for _, frame := range r.ResponseFrames {
				if len(frame) > 65536 || len(frame) > 131072-frameBytes {
					return ErrPrivateNativeConstructorV2
				}
				frameBytes += len(frame)
			}
			if retain {
				rawSize += uint64(frameBytes)
			}
		}
		nativeBound, ok := privateNativeEncodedSizeV2(rawSize, 64, 1<<20)
		if !ok || nativeBound > 4<<20 {
			return ErrPrivateNativeConstructorV2
		}
		observation, err := json.Marshal(v.Supply)
		if err != nil {
			return err
		}
		p := acquisition.PrivateDirectionalParentV2{OperationID: v.OperationID, SessionID: v.Session.SessionID, Generation: v.Session.Generation, URI: v.Request.URI, LanguageID: v.Request.LanguageID, Line: v.Request.Line, Character: v.Request.Character, MaxMessages: v.Request.MaxMessages, PositionEncoding: v.Metadata.PositionEncoding, SupplyURI: v.Supply.URI, SupplyObservation: observation}
		for _, r := range v.Requests {
			p.Requests = append(p.Requests, acquisition.PrivateDirectionalObservationV2{Ordinal: r.Ordinal, Method: r.Method, RequestID: r.RequestID, Generation: r.Generation, Params: r.Params, Result: r.Result})
		}
		out.Views, err = acquisition.BuildDirectionalViewsPrivateV2(p)
		if err != nil {
			return err
		}
		// The enclosing invocation is defined ONCE from actual producer custody, not
		// from either view's derived limits, direction selection or request IDs.
		inv := graph.Invocation{WorkspaceURI: (&url.URL{Scheme: "file", Path: v.Session.Workspace}).String(), WorkingDirectory: v.Process.Dir, EffectiveEnvironment: v.Process.Env, Target: graph.Target{URI: v.Request.URI, Line: int(*v.Request.Line), Column: int(*v.Request.Character)}, Server: graph.ServerInvocation{Command: v.Process.Path, Arguments: v.Process.Args}, Limits: graph.Limits{MaxDepth: 1, MaxNodes: 128, TimeoutMS: v.Request.Deadline.Sub(v.StartedAt).Milliseconds()}, RequestTimeoutMS: v.Request.Deadline.Sub(v.StartedAt).Milliseconds(), Concurrency: 1, LanguageID: v.Request.LanguageID, Seeds: out.Views[0].Result.Graph.Invocation.Seeds, Provenance: graph.InvocationProvenance{InvocationID: out.Views[0].Result.Request.Context.ID, Caller: "private-native-constructor", Source: "controlled-native-directions", SourceRevision: q.Revision, ServerVersion: v.Metadata.ProviderVersion, Timestamp: v.StartedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}}
		out.ParentInvocation = inv
		seen := map[string]bool{}
		for _, view := range out.Views {
			for _, n := range view.Result.Graph.Nodes {
				seen[n.ID] = true
			}
		}
		if len(seen) > inv.Limits.MaxNodes {
			return ErrPrivateNativeConstructorV2
		}
		for i := range out.Views {
			out.Sources[i], err = graphprovenance.CaptureNativeConstructorSourcePrivateV2(ctx, out.Views[i].Result, v.Session.Workspace)
			if err != nil {
				return err
			}
			native := out.Views[i].Result.Graph
			native.SchemaVersion = graph.SchemaVersionV5
			native.Invocation = inv
			native.Capabilities.CallHierarchyProvider = v.Metadata.CallHierarchySupport
			native.Canonicalize()
			raw, err := json.Marshal(native)
			if err != nil {
				return err
			}
			// 128 bounded nodes/edges and <=1MiB normalized evidence per view have
			// already been admitted. Source capture has independent existing caps.
			if len(raw) > 4<<20 {
				return ErrPrivateNativeConstructorV2
			}
			envelope, err := graphprovenance.CaptureV5(raw, v.Session.SessionID, v.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &out.Sources[i])
			if err != nil {
				return err
			}
			out.Inputs = append(out.Inputs, programccompose.Input{Bytes: envelope, Identity: fmt.Sprintf("%s:view-%d", inv.Provenance.InvocationID, i), SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(envelope)), ByteLength: len(envelope), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: inv.WorkspaceURI, RevisionCustody: "CALLER_ASSERTED", PositionEncoding: v.Metadata.PositionEncoding, AcquisitionSemantics: "ONE_ACTUAL_DIRECTIONAL_INVOCATION_TWO_VALIDATED_VIEWS", PrivacyPolicy: q.Privacy}})
			joins, err := privateNativeRangeJoinV2(v, i, native)
			if err != nil {
				return err
			}
			out.RawJoins = append(out.RawJoins, joins...)
		}
		out.Composite, err = programccompose.Compose(out.Inputs)
		if err != nil {
			return err
		}
		if _, err = programccompose.Validate(out.Composite.Bytes); err != nil {
			return err
		}
		out.Admission, err = programcadmission.Admit(out.Composite.Bytes)
		if err != nil {
			return err
		}
		if !retain {
			return nil
		}
		proof := referencegrouping.PrivateNativeEvidenceV2{Schema: referencegrouping.PrivateNativeEvidenceSchemaV2, Parent: p, Invocation: inv, Sources: out.Sources, Inputs: out.Inputs, RawJoins: out.RawJoins}
		proof.Parent.Requests = nil
		for _, r := range v.Requests {
			spans, _ := directionalraw.Elements(r.Result)
			status := "OBSERVED_NONEMPTY"
			if len(spans) == 0 {
				status = "OBSERVED_EMPTY"
			}
			proof.Requests = append(proof.Requests, referencegrouping.PrivateNativeRequestV2{Ordinal: r.Ordinal, Method: r.Method, RequestID: r.RequestID, Generation: r.Generation, Params: r.Params, Result: r.Result, RequestFrame: r.RequestFrame, ResponseFrames: r.ResponseFrames, Wire: r.Wire, Observations: r.Observations, Status: status})
		}
		for i := range out.Views {
			proof.OriginalOrdinals[i] = out.Views[i].OriginalOrdinals
		}
		out.nativeEvidence, err = referencegrouping.EncodePrivateNativeEvidenceV2(proof, out.Composite.Bytes)
		return err
	}()
	if err != nil {
		return PrivateNativeCallsResultV2{}, err
	}
	if err := m.CheckTakenDirectionalEvidencePrivateV2(ctx, h); err != nil {
		return PrivateNativeCallsResultV2{}, err
	}
	return out, nil
}

func privateNativeRangeJoinV2(v sessionruntime.PrivateDirectionalEvidenceViewV2, direction int, g graph.Result) ([]PrivateNativeRawJoinV2, error) {
	var prepared []lsp.CallHierarchyItem
	if json.Unmarshal(v.Requests[0].Result, &prepared) != nil || len(prepared) != 1 {
		return nil, ErrPrivateNativeConstructorV2
	}
	node := func(i lsp.CallHierarchyItem) graph.Node {
		return graph.NewNode(graph.Item{Name: i.Name, Kind: i.Kind, Detail: i.Detail, URI: i.URI, Range: privateNativeRangeV2(i.Range), SelectionRange: privateNativeRangeV2(i.SelectionRange), Data: i.Data})
	}
	root := node(prepared[0])
	var joins []PrivateNativeRawJoinV2
	add := func(element int, other lsp.CallHierarchyItem, ranges []lsp.Range) error {
		caller, callee := node(other).ID, root.ID
		label := "INCOMING"
		if direction == 1 {
			caller, callee = root.ID, node(other).ID
			label = "OUTGOING"
		}
		for ordinal, r := range ranges {
			found := false
			for _, edge := range g.Edges {
				if edge.CallerNodeID != caller || edge.CalleeNodeID != callee {
					continue
				}
				for local, site := range edge.CallSites {
					if site == privateNativeRangeV2(r) {
						joins = append(joins, PrivateNativeRawJoinV2{OriginalRequestOrdinal: direction + 1, ElementOrdinal: element, RangeOrdinal: ordinal, ViewIndex: direction, EdgeLocalOrdinal: local, Direction: label, RelationID: edge.RelationID, Range: site})
						found = true
						break
					}
				}
			}
			if !found {
				return fmt.Errorf("%w: unmatched actual raw range %d/%d/%d", ErrPrivateNativeConstructorV2, direction+1, element, ordinal)
			}
		}
		return nil
	}
	if direction == 0 {
		var calls []lsp.CallHierarchyIncomingCall
		if err := json.Unmarshal(v.Requests[1].Result, &calls); err != nil {
			return nil, err
		}
		for i, c := range calls {
			if err := add(i, c.From, c.FromRanges); err != nil {
				return nil, err
			}
		}
	} else {
		var calls []lsp.CallHierarchyOutgoingCall
		if err := json.Unmarshal(v.Requests[2].Result, &calls); err != nil {
			return nil, err
		}
		for i, c := range calls {
			if err := add(i, c.To, c.FromRanges); err != nil {
				return nil, err
			}
		}
	}
	return joins, nil
}

// Closed checked bound used BEFORE constructor marshal/copy/base64 work. This
// is byte-size admission, not a financial ledger or a physical allocation claim.
func privateNativeEncodedSizeV2(n, scale, base uint64) (uint64, bool) {
	if scale != 0 && n > (^uint64(0)-base)/scale {
		return 0, false
	}
	total := base + n*scale
	return total, total <= uint64(graphprovenance.MaxEnvelopeBytesV5)
}

func privateNativeRangeV2(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
