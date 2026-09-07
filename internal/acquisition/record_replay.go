package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

// A replay consumes actual retained operation records, not reported resolution
// or expansion results. The only report input is an explicit cache-cancellation
// boundary: this is a consistency claim, never independent event authority.
type recordReplay struct {
	source Result
	next   int
	err    error
	ctx    *replayContext
	visits map[string]int
}
type replayContext struct {
	context.Context
	cancelled error
}

func (c *replayContext) Err() error { return c.cancelled }
func (c *replayContext) cancel(reason string) {
	if c.cancelled != nil {
		return
	}
	if reason == context.DeadlineExceeded.Error() {
		c.cancelled = context.DeadlineExceeded
	} else {
		c.cancelled = context.Canceled
	}
}

type replaySupplier struct{ Client }

func (replaySupplier) PrepareDocument(context.Context, AcquisitionContext, Locator) (Supply, error) {
	return Supply{}, errors.New("replay must not call a supplier")
}
func sameJSON(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && bytes.Equal(x, y)
}

func validateRecordReplay(r Result) error {
	rc := &replayContext{Context: context.Background()}
	p := &recordReplay{source: r, ctx: rc, visits: map[string]int{}}
	c := &runner{ctx: rc, replay: p, result: Result{Request: r.Request, Policy: Policy, PathProjection: r.PathProjection, AcquisitionComplete: true}, items: map[string]lsp.CallHierarchyItem{}, nodes: map[string]graph.Node{}, resolutions: map[string]Resolution{}, queries: map[string]queryResult{}}
	for _, rec := range r.Requests {
		if rec.Method == "source/prepareDocument" {
			c.client = replaySupplier{}
			break
		}
	}
	err := c.acquireEvidence()
	if p.err != nil {
		return p.err
	}
	if err != nil {
		return fmt.Errorf("record replay: %w", err)
	}
	if p.next != len(r.Requests) {
		return errors.New("unconsumed acquisition request records")
	}
	for i, t := range r.Targets {
		got := c.result.Targets[i]
		if !sameJSON(t.Resolution, got.Resolution) || t.Admission != got.Admission || !sameJSON(t.Outgoing, got.Outgoing) || !sameJSON(t.Incoming, got.Incoming) {
			return fmt.Errorf("target %s differs from retained-request replay", t.Requested.ID)
		}
	}
	if !sameJSON(r.Supplies, c.result.Supplies) {
		return errors.New("supplies differ from actual supply operation results")
	}
	if !sameJSON(r.EdgeObservations, c.result.EdgeObservations) {
		return errors.New("observations differ from admitted response replay")
	}
	if !sameJSON(r.Graph, c.result.Graph) {
		return errors.New("native graph differs from deterministic acquisition replay")
	}
	usage := r.Usage
	usage.PathWork = 0
	if usage != c.result.Usage || r.AcquisitionComplete != c.result.AcquisitionComplete {
		return errors.New("usage/completeness differs from acquisition replay")
	}
	return nil
}
func (p *recordReplay) fail(format string, args ...any) (any, *RequestRecord) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
	return nil, &RequestRecord{ID: "invalid-replay", Outcome: "FAILED", Reason: "INVALID_REPLAY"}
}
func (p *recordReplay) beforeQuery(c *runner, target, id string, d Direction) {
	key := string(d) + ":" + target
	n := p.visits[key]
	p.visits[key]++
	// Frontiers do not call query. Count only actual/query-cache visits.
	for _, t := range p.source.Targets {
		if t.Requested.ID != target {
			continue
		}
		result := t.Incoming
		if d == Outgoing {
			result = t.Outgoing
		}
		index := 0
		for _, e := range result.Expansions {
			if e.Status == Frontier {
				continue
			}
			if index == n {
				if e.NodeID == id && e.Cached && e.Status == BudgetBlocked && cancellationReason(e.Reason) {
					if _, ok := c.queries[string(d)+":"+id]; ok {
						p.ctx.cancel(e.Reason)
					}
				}
				return
			}
			index++
		}
	}
}
func (p *recordReplay) invoke(c *runner, target, method, id string, params any) (any, *RequestRecord) {
	if p.err != nil {
		return p.fail("replay already invalid")
	}
	if p.next >= len(p.source.Requests) {
		return p.fail("missing request for %s %s", target, method)
	}
	rec := p.source.Requests[p.next]
	if rec.TargetID != target || rec.Method != method || rec.NodeID != id || rec.Context != c.result.Request.Context || rec.Before != c.result.Usage {
		return p.fail("request %s owner/operation/pre-usage mismatch", rec.ID)
	}
	l := c.result.Request.Limits
	// A global cancellation boundary can be declared by an unattempted record.
	// Once encountered it remains sticky; subsequent attempts cannot be replayed.
	if rec.Outcome == "BUDGET_BLOCKED" && cancellationReason(rec.Reason) {
		p.ctx.cancel(rec.Reason)
	}
	blocked := ""
	if err := c.ctx.Err(); err != nil {
		blocked = err.Error()
	} else if c.result.Usage.Requests >= l.MaxRequests {
		blocked = "MAX_REQUESTS"
	} else if c.result.Usage.EvidenceBytes >= l.MaxEvidenceBytes {
		blocked = "EVIDENCE_BYTES"
	} else if method == "textDocument/prepareCallHierarchy" && c.result.Usage.PrepareProbes >= MaxPrepareProbes {
		blocked = "MAX_PREPARE_PROBES"
	}
	var raw json.RawMessage
	paramBytes := 0
	if blocked == "" {
		var err error
		raw, paramBytes, err = boundedJSON(params, l.MaxEvidenceBytes-c.result.Usage.EvidenceBytes)
		if err != nil || c.result.Usage.EvidenceBytes+paramBytes >= l.MaxEvidenceBytes {
			blocked = "EVIDENCE_BYTES"
		}
	}
	if !bytes.Equal(raw, rec.Params) {
		return p.fail("request %s parameters differ from scheduled query", rec.ID)
	}
	if blocked != "" {
		if rec.Outcome != "BUDGET_BLOCKED" || rec.Attempted || rec.Reason != blocked || rec.CaptureComplete || len(rec.Response) != 0 || rec.EvidenceBytes != paramBytes {
			return p.fail("request %s false blocked disposition", rec.ID)
		}
	} else if !rec.Attempted || rec.Outcome == "BUDGET_BLOCKED" {
		return p.fail("request %s unmotivated block", rec.ID)
	}
	responseLimit := l.MaxEvidenceBytes - c.result.Usage.EvidenceBytes - paramBytes
	if responseLimit > l.MaxResponseBytes {
		responseLimit = l.MaxResponseBytes
	}
	responseBytes := rec.EvidenceBytes - paramBytes
	if responseBytes < 0 || responseBytes > responseLimit {
		return p.fail("request %s response consumption outside cap", rec.ID)
	}
	var value any
	if blocked == "" {
		switch rec.Outcome {
		case "SUCCESS":
			if !rec.CaptureComplete || rec.Reason != "" {
				return p.fail("request %s false successful capture", rec.ID)
			}
			var dst any
			switch method {
			case "source/prepareDocument":
				dst = &Supply{}
			case "textDocument/documentSymbol":
				dst = &[]lsp.DocumentSymbol{}
			case "textDocument/prepareCallHierarchy":
				dst = &[]lsp.CallHierarchyItem{}
			case "callHierarchy/outgoingCalls":
				dst = &[]lsp.CallHierarchyOutgoingCall{}
			case "callHierarchy/incomingCalls":
				dst = &[]lsp.CallHierarchyIncomingCall{}
			default:
				return p.fail("unknown acquisition operation %s", method)
			}
			decoder := json.NewDecoder(bytes.NewReader(rec.Response))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(dst); err != nil {
				return p.fail("request %s invalid normalized response: %v", rec.ID, err)
			}
			switch v := dst.(type) {
			case *Supply:
				value = *v
			case *[]lsp.DocumentSymbol:
				value = *v
			case *[]lsp.CallHierarchyItem:
				value = *v
			case *[]lsp.CallHierarchyOutgoingCall:
				value = *v
			case *[]lsp.CallHierarchyIncomingCall:
				value = *v
			}
			normalized, n, err := boundedJSON(value, responseLimit)
			if err != nil || !bytes.Equal(normalized, rec.Response) || n != responseBytes {
				return p.fail("request %s noncanonical normalized capture", rec.ID)
			}
		case "FAILED", "UNSUPPORTED":
			capture := struct {
				Error string `json:"error"`
			}{rec.Reason}
			normalized, n, err := boundedJSON(capture, responseLimit)
			outcome := "FAILED"
			if strings.Contains(rec.Reason, "-32601") {
				outcome = "UNSUPPORTED"
			}
			if err != nil || !rec.CaptureComplete || rec.Outcome != outcome || !bytes.Equal(normalized, rec.Response) || n != responseBytes {
				return p.fail("request %s inconsistent failure capture", rec.ID)
			}
		case "CAPTURE_INCOMPLETE":
			if rec.CaptureComplete || len(rec.Response) != 0 || rec.Reason != "EVIDENCE_BYTES" || responseBytes != responseLimit {
				return p.fail("request %s invalid truncation accounting", rec.ID)
			}
		case "CAPTURE_FAILED":
			if rec.CaptureComplete || len(rec.Response) != 0 || !strings.HasPrefix(rec.Reason, "MALFORMED_EVIDENCE: ") {
				return p.fail("request %s invalid malformed capture", rec.ID)
			}
		default:
			return p.fail("request %s invalid outcome", rec.ID)
		}
	}
	p.next++
	c.result.Usage.EvidenceBytes += rec.EvidenceBytes
	if rec.Attempted {
		c.result.Usage.Requests++
		if method == "textDocument/prepareCallHierarchy" {
			c.result.Usage.PrepareProbes++
		}
	}
	c.result.Requests = append(c.result.Requests, rec)
	return value, &c.result.Requests[len(c.result.Requests)-1]
}
