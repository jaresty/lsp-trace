package sessionruntime

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"lsp-trace/internal/lsp"
)

// Zero is opt-out on discovery construction. The historical directional API
// interprets its own zero value as both directions for compatibility.
type PrivateCallDirections uint8

const (
	PrivateCallsIncoming PrivateCallDirections = 1 << iota
	PrivateCallsOutgoing
)

type privateDiscoveryOwnedCall struct {
	view PrivateDirectionalEvidenceViewV2
}
type privateDiscoveryCallAdmissionKey struct{}
type privateDiscoveryCallAdmission struct {
	window  *privateDiscoveryWindow
	request DirectionalCallHierarchyRequestPrivate
	used    bool
}

func (w *privateDiscoveryWindow) beginOwnedDirectional(ctx context.Context, q DirectionalCallHierarchyRequestPrivate) bool {
	a, _ := ctx.Value(privateDiscoveryCallAdmissionKey{}).(*privateDiscoveryCallAdmission)
	w.mu.Lock()
	defer w.mu.Unlock()
	if a == nil || a.window != w || a.used || w.failed || w.ownedReleased || !reflect.DeepEqual(a.request, q) {
		return false
	}
	a.used = true
	return true
}

// Only the live runtime companion may enter the discovery directional reader.
func (w *privateDiscoveryWindow) ownedDirectionalRequest(ctx context.Context, m *Manager, req RoundTripRequest) bool {
	c, _ := ctx.Value(privateDirectionalEvidenceKeyV2{}).(*privateDirectionalEvidenceV2)
	a, _ := ctx.Value(privateDiscoveryCallAdmissionKey{}).(*privateDiscoveryCallAdmission)
	w.mu.Lock()
	defer w.mu.Unlock()
	if a == nil || a.window != w || !a.used || w.failed || w.ownedReleased || w.ownedMethods == nil || w.ownedMethods.directions == 0 || c == nil || c.manager != m || m != w.manager || c.view.Session.SessionID != req.SessionID || c.view.Session.Generation != req.Generation || c.view.Request.Directions != w.ownedMethods.directions || req.MaxBytes < 1 || req.MaxBytes > PrivateDirectionalEvidenceMaxBytesV2 || req.MaxMessages < 1 || req.MaxMessages > 32 {
		return false
	}
	switch req.Method {
	case "textDocument/prepareCallHierarchy":
		return true
	case "callHierarchy/incomingCalls":
		return c.view.Request.Directions&PrivateCallsIncoming != 0
	case "callHierarchy/outgoingCalls":
		return c.view.Request.Directions&PrivateCallsOutgoing != 0
	}
	return false
}

// The existing directional reader owns raw reservation/decoding; this additional
// quote bounds discovery's decoded trees, observer records and consumer copies.
// Stream bytes still pass through the original charged child reader. No refund.
func (w *privateDiscoveryWindow) quoteDirectionalFrame(c *privateDiscoveryChild, requested int64) (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c == nil || c.window != w || w.child != c || requested < 1 || requested > PrivateDirectionalEvidenceMaxBytesV2 {
		return 0, false
	}
	cap := min(uint64(requested), w.limits.FrameBytes, w.remaining.ResponseBytes, w.remaining.WireBytes)
	if cap == 0 {
		w.failLocked(PrivateDiscoveryRefusal{Stage: PrivateDiscoveryFrame})
		return 0, false
	}
	b := cap + w.limits.HeaderBytes + 4096
	ok := w.reserveLocked(PrivateDiscoveryFrame, privateDiscoveryCharge{&w.remaining.Messages, 1}, privateDiscoveryCharge{&w.remaining.RetainedBytes, 128 * b}, privateDiscoveryCharge{&w.remaining.Work, 512 * b})
	return int64(cap), ok
}

// NativeCallCount is scalar metadata, not admission or permission to process it.
func (o PrivateDiscoveryOwnedMethods) NativeCallCount() int {
	if !o.Available() {
		return 0
	}
	return len(o.state.calls)
}

// TakeNativeCalls transfers an actual three-request runtime original once. The
// caller must separately reserve downstream conversion work; this primitive
// confers no native admission and accepts no caller-created evidence DTO.
func (o PrivateDiscoveryOwnedMethods) TakeNativeCalls(ctx context.Context, index int) (PrivateDirectionalEvidenceViewV2, error) {
	if ctx == nil || ctx.Err() != nil || !o.Available() || index < 0 || index >= len(o.state.calls) || index >= 128 {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
	}
	s := o.state
	s.nativeMu.Lock()
	defer s.nativeMu.Unlock()
	if s.nativeTaken[index] {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
	}
	s.nativeTaken[index] = true // failure cannot revive this transfer
	v := s.calls[index].view
	if v.Request.Directions != PrivateCallsIncoming|PrivateCallsOutgoing || len(v.Requests) != 3 || !v.Request.Deadline.Equal(s.deadline) || !time.Now().Before(s.deadline) || v.Session.SessionID != s.sessionID || v.Session.Generation != s.generation || v.Session.Workspace != s.workspace {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
	}
	for i, method := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		if v.Requests[i].Ordinal != i || v.Requests[i].Method != method || v.Requests[i].RequestID == 0 || v.Requests[i].Generation != s.generation {
			return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
		}
	}
	// Conversion may not widen the prepared/supplied document scope while
	// reading source for provider-returned endpoints.
	var incoming []struct {
		From lsp.CallHierarchyItem `json:"from"`
	}
	var outgoing []struct {
		To lsp.CallHierarchyItem `json:"to"`
	}
	if json.Unmarshal(v.Requests[1].Result, &incoming) != nil || json.Unmarshal(v.Requests[2].Result, &outgoing) != nil {
		return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
	}
	for _, call := range incoming {
		if s.sources[call.From.URI] == nil {
			return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
		}
	}
	for _, call := range outgoing {
		if s.sources[call.To.URI] == nil {
			return PrivateDirectionalEvidenceViewV2{}, ErrPrivateDiscovery
		}
	}
	return v, nil
}

// NativeCallsAvailable reports detached actual directional originals, not native
// admission, grouping authority, completeness, or a source confirmation shortcut.
func (o PrivateDiscoveryOwnedMethods) NativeCallsAvailable() bool {
	return o.Available() && len(o.state.calls) > 0
}

func (s *PrivateDiscoverySession) prepareOwnedCalls(ctx context.Context, p lsp.PrepareCallHierarchyParams) (_ []lsp.CallHierarchyItem, err error) {
	diagnostic := s.diagnosticContext(ctx, p.TextDocument.URI)
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.client.w
	defer func() {
		if err != nil {
			w.mu.Lock()
			w.ownedCallFailed = true
			w.mu.Unlock()
		}
	}()
	if s.closed || ctx == nil || ctx.Err() != nil {
		return nil, ErrPrivateDiscovery
	}
	w.mu.Lock()
	owned := w.ownedMethods
	if owned == nil || w.ownedReleased || w.ownedCallFailed || owned.directions == 0 {
		w.mu.Unlock()
		return nil, ErrPrivateDiscovery
	}
	for _, c := range owned.calls {
		q := c.view.Request
		if q.URI == p.TextDocument.URI && *q.Line == p.Position.Line && *q.Character == p.Position.Character {
			w.mu.Unlock()
			var items []lsp.CallHierarchyItem
			if err := w.decodeOwnedCall(c.view.Requests[0].Result, &items); err != nil {
				return nil, err
			}
			return items, nil
		}
	}
	if len(owned.calls) >= 128 || owned.sources[p.TextDocument.URI] == nil {
		w.mu.Unlock()
		return nil, ErrPrivateDiscovery
	}
	// Covers detached wire/result/source bytes, source checks and bounded cache
	// copies BEFORE lease/capture effects, from the original cumulative balances.
	b := uint64(3 * (3*PrivateDirectionalEvidenceMaxBytesV2 + 16384 + 4096))
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 8 * b}, privateDiscoveryCharge{&w.remaining.Work, 64 * b}) {
		w.mu.Unlock()
		return nil, ErrPrivateDiscovery
	}
	session := SessionLeaseRequest{SessionID: owned.sessionID, Generation: owned.generation, Workspace: owned.workspace}
	q := DirectionalCallHierarchyRequestPrivate{Directions: owned.directions, SessionID: owned.sessionID, Generation: owned.generation, URI: p.TextDocument.URI, Line: &p.Position.Line, Character: &p.Position.Character, Deadline: s.client.deadline, MaxMessages: 32, MaxBytes: PrivateDirectionalEvidenceMaxBytesV2}
	for _, member := range s.members {
		if member.URI == q.URI {
			q.LanguageID = member.LanguageID
			break
		}
	}
	w.mu.Unlock()
	var v PrivateDirectionalEvidenceViewV2
	err = s.client.m.WithReadySessionLease(diagnostic, session, func(leased context.Context) error {
		admitted := context.WithValue(leased, privateDiscoveryCallAdmissionKey{}, &privateDiscoveryCallAdmission{window: w, request: q})
		h, err := s.client.m.CaptureDirectionalEvidencePrivateV2(admitted, session, q)
		if err != nil {
			return err
		}
		v, err = s.client.m.TakeDirectionalEvidencePrivateV2(leased, h)
		if err != nil {
			return err
		}
		return s.client.m.CheckTakenDirectionalEvidencePrivateV2(leased, h)
	})
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	if w.failed || w.ownedReleased || !time.Now().Before(owned.deadline) || !reflect.DeepEqual(*owned.sources[q.URI], v.Supply) {
		w.mu.Unlock()
		return nil, ErrPrivateDiscovery
	}
	owned.calls = append(owned.calls, privateDiscoveryOwnedCall{view: v})
	w.mu.Unlock()
	var items []lsp.CallHierarchyItem
	if err := w.decodeOwnedCall(v.Requests[0].Result, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// Semantic consumers receive fresh decodes of the original wire result; their
// mutations cannot alter the retained owner. Cache reads still spend work/bytes.
func (w *privateDiscoveryWindow) decodeOwnedCall(raw json.RawMessage, out any) error {
	w.mu.Lock()
	n := uint64(len(raw)) + 4096
	ok := w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 8 * n}, privateDiscoveryCharge{&w.remaining.Work, 32 * n})
	w.mu.Unlock()
	if !ok || json.Unmarshal(raw, out) != nil {
		return ErrPrivateDiscovery
	}
	return nil
}

func (s *PrivateDiscoverySession) retainedCalls(ctx context.Context, item lsp.CallHierarchyItem, method string, out any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || ctx == nil || ctx.Err() != nil {
		return ErrPrivateDiscovery
	}
	w := s.client.w
	w.mu.Lock()
	if w.ownedMethods == nil || w.ownedReleased {
		w.mu.Unlock()
		return ErrPrivateDiscovery
	}
	calls := w.ownedMethods.calls
	w.mu.Unlock()
	for _, c := range calls {
		var items []lsp.CallHierarchyItem
		if err := w.decodeOwnedCall(c.view.Requests[0].Result, &items); err != nil {
			return err
		}
		if len(items) != 1 || !reflect.DeepEqual(items[0], item) {
			continue
		}
		for _, request := range c.view.Requests {
			if request.Method == method {
				return w.decodeOwnedCall(request.Result, out)
			}
		}
		return ErrPrivateDiscovery // unselected direction is unavailable, never empty
	}
	return ErrPrivateDiscovery
}
