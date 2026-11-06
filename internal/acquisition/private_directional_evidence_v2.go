package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

var ErrPrivateDirectionalViewV2 = errors.New("private directional parent/view binding refused")
var ErrPrivateDirectionalViewBindingV2 = errors.New("private directional actual parent/selection mismatch")

type PrivateDirectionalViewV2 struct {
	Result Result
	// Original request ordinals are not coordinator request IDs or V3 ordinals.
	OriginalOrdinals []int
}

func privateEqualJSONV2(a, b []byte) bool {
	decode := func(raw []byte) ([]byte, error) {
		var v any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	}
	x, e := decode(a)
	y, f := decode(b)
	return e == nil && f == nil && bytes.Equal(x, y)
}

type PrivateDirectionalObservationV2 struct {
	Ordinal               int
	Method                string
	RequestID, Generation uint64
	Params, Result        json.RawMessage
}
type PrivateDirectionalParentV2 struct {
	OperationID             string
	SessionID               string
	Generation              uint64
	URI, LanguageID, Symbol string
	Line, Character         *uint32
	MaxMessages             int
	PositionEncoding        string
	SupplyURI               string
	SupplyObservation       json.RawMessage
	Requests                []PrivateDirectionalObservationV2
}

// BuildDirectionalViewsPrivateV2 reconstructs two views of ONE actual invocation.
// It is not a producer and grants no custody: its only caller in the constructor
// runs inside Manager.ConsumeDirectionalEvidencePrivateV2.
func BuildDirectionalViewsPrivateV2(v PrivateDirectionalParentV2) ([2]PrivateDirectionalViewV2, error) {
	var out [2]PrivateDirectionalViewV2
	q := v
	if len(v.Requests) != 3 || q.Line == nil || q.Character == nil || q.Symbol != "" || v.OperationID == "" || len(v.OperationID) > 4096 {
		return out, ErrPrivateDirectionalViewV2
	}
	methods := []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"}
	for i, o := range v.Requests {
		if o.Ordinal != i || o.Method != methods[i] || o.Generation != q.Generation || o.RequestID == 0 || len(o.Params) > 65536 || len(o.Result) == 0 || len(o.Result) > 65536 {
			return out, ErrPrivateDirectionalViewV2
		}
	}
	for direction := 0; direction < 2; direction++ {
		indexes := []int{-1, 0, direction + 1} // -1 is actual document-supply event, not an RPC ordinal
		mode, down, up := Incoming, 0, 1
		if direction == 1 {
			mode, down, up = Slice, 1, 0
		}
		req := Request{Mode: mode, Context: AcquisitionContext{ID: v.OperationID, SessionID: q.SessionID, Generation: q.Generation, PositionEncoding: v.PositionEncoding}, Root: Target{ID: "root", Locator: Locator{URI: q.URI, LanguageID: q.LanguageID, Line: q.Line, Character: q.Character}, DownDepth: down, UpDepth: up}, Limits: Limits{MaxNodes: 128, MaxRequests: 3, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1024, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 65536, MaxMessages: q.MaxMessages}}
		if err := ValidateRequest(req); err != nil {
			return out, err
		}
		rc := &replayContext{Context: context.Background()}
		p := &recordReplay{source: Result{Requests: make([]RequestRecord, 3)}, ctx: rc, visits: map[string]int{}}
		p.privateNormalize = func(c *runner, target, method, id string, params any) (RequestRecord, error) {
			if p.next >= len(indexes) {
				return RequestRecord{}, ErrPrivateDirectionalViewV2
			}
			if p.next == 0 {
				if method != "source/prepareDocument" || v.SupplyURI != q.URI || len(v.SupplyObservation) > 65536 {
					return RequestRecord{}, ErrPrivateDirectionalViewV2
				}
				observation := v.SupplyObservation
				encoded, _, err := boundedJSON(params, 65536)
				if err != nil {
					return RequestRecord{}, err
				}
				response, n, err := boundedJSON(Supply{URI: q.URI, LanguageID: q.LanguageID, Observation: observation}, 1<<20)
				if err != nil {
					return RequestRecord{}, err
				}
				return RequestRecord{ID: "request-000001", TargetID: target, Method: method, Context: req.Context, Before: c.result.Usage, Attempted: true, Outcome: "SUCCESS", Params: encoded, Response: response, CaptureComplete: true, EvidenceBytes: len(encoded) + 1 + n}, nil
			}
			actual := v.Requests[indexes[p.next]]
			encoded, _, err := boundedJSON(params, 65536)
			if err != nil || method != actual.Method || !privateEqualJSONV2(encoded, actual.Params) {
				return RequestRecord{}, ErrPrivateDirectionalViewBindingV2
			}
			var dst any
			switch method {
			case methods[0]:
				dst = &[]lsp.CallHierarchyItem{}
			case methods[1]:
				dst = &[]lsp.CallHierarchyIncomingCall{}
			case methods[2]:
				dst = &[]lsp.CallHierarchyOutgoingCall{}
			default:
				return RequestRecord{}, ErrPrivateDirectionalViewV2
			}
			d := json.NewDecoder(bytes.NewReader(actual.Result))
			d.DisallowUnknownFields()
			if err = d.Decode(dst); err != nil {
				return RequestRecord{}, err
			}
			response, n, err := boundedJSON(dst, 65536)
			if err != nil {
				return RequestRecord{}, err
			}
			return RequestRecord{ID: fmt.Sprintf("request-%06d", p.next+1), TargetID: target, Method: method, NodeID: id, Context: req.Context, Before: c.result.Usage, Attempted: true, Outcome: "SUCCESS", Params: encoded, Response: response, CaptureComplete: true, EvidenceBytes: len(encoded) + 1 + n}, nil
		}
		c := &runner{ctx: rc, client: replaySupplier{}, replay: p, result: Result{Request: req, Policy: Policy, PathProjection: "NATIVE_CALLS_CALLER_TO_CALLEE_UNIT_GROUP_HOPS_V1", AcquisitionComplete: true}, items: map[string]lsp.CallHierarchyItem{}, nodes: map[string]graph.Node{}, resolutions: map[string]Resolution{}, queries: map[string]queryResult{}}
		if err := c.acquireEvidence(); err != nil {
			return out, err
		}
		if p.err != nil {
			return out, errors.Join(ErrPrivateDirectionalViewV2, p.err)
		}
		if p.next != 3 {
			return out, ErrPrivateDirectionalViewV2
		}
		if err := c.connect(); err != nil {
			return out, err
		}
		if err := ValidateResult(c.result); err != nil {
			return out, err
		}
		out[direction] = PrivateDirectionalViewV2{Result: c.result, OriginalOrdinals: indexes}
	}
	return out, nil
}
