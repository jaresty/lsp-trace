package sessionruntime

import (
	"context"
	"time"
)

// PrivateGroupingSourceEvidence retains original producer observations, not a
// borrowed view. It establishes neither provider authentication, feature identity,
// source-universe completeness, nor permission to invoke a semantic worker.
// Live custody is issued by TakePrivateGroupingSourceEvidence. Detached
// discovery custody is issued only by ConsumeDiscoveryGroupingOwned from the
// runtime's retained originals, never from a borrowed view or reconstructed lease.
type PrivateGroupingSourceEvidence struct {
	capture   *privateGroupingCapture
	discovery *privateDiscoveryGroupingOriginal
}

func (e PrivateGroupingSourceEvidence) originalView() (PrivateGroupingOwnedView, bool) {
	if e.capture != nil && e.discovery == nil {
		return e.capture.view, true
	}
	if e.capture == nil && e.discovery != nil {
		return e.discovery.view, true
	}
	return PrivateGroupingOwnedView{}, false
}

// TakePrivateGroupingSourceEvidence is a one-time, pointer-only detachment of
// existing retained observations. It does not capture, clone, publish or invoke
// a caller callback. After bounding, it reuses the managed-document validator. The caller must still be inside the EXACT lease
// that successfully completed ConsumePrivateGroupingOwned. Failed callbacks,
// cancellation and lease invalidation cannot produce source evidence.
func (m *Manager) TakePrivateGroupingSourceEvidence(ctx context.Context, h PrivateGroupingOwnedHandoff) (PrivateGroupingSourceEvidence, error) {
	fail := func() (PrivateGroupingSourceEvidence, error) {
		return PrivateGroupingSourceEvidence{}, ErrPrivateGroupingOwned
	}
	if m == nil || ctx == nil || h.capture == nil {
		return fail()
	}
	c := h.capture
	m.mu.Lock()
	defer m.mu.Unlock()
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	r := m.sessions[c.request.Session.SessionID]
	if c.manager != m || !c.consumed || !c.sourceConsumedSuccessfully || c.sourceTaken || c.token == nil || token != c.token || r == nil || m.closed || r.lifecycleOwned || ctx.Err() != nil || !time.Now().Before(c.request.Deadline) || r.record.Generation != c.request.Session.Generation || !m.validSessionLeaseLocked(ctx, r) {
		return fail()
	}
	if _, ok := PrivateGroupingQueryEncodedBound(c.view); !ok {
		return fail()
	}
	matches := func(pair OwnedMethodPair) bool {
		return matchesOwnedDocument(RoundTripRequest{SessionID: pair.SessionID, Generation: pair.Generation, Method: pair.Method, Params: pair.Params, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: pair.Source, MaxMessages: 4, MaxBytes: 65536}, r.documents)
	}
	if !matches(c.view.Symbol) {
		return fail()
	}
	for _, pair := range c.view.Methods {
		if !matches(pair) {
			return fail()
		}
	}
	c.sourceTaken = true
	return PrivateGroupingSourceEvidence{capture: c}, nil
}

// EncodedBound is metadata-only: it neither copies nor hashes retained payloads.
// A positive result describes this closed private encoding, not resource funding.
func (e PrivateGroupingSourceEvidence) EncodedBound() (uint64, bool) {
	v, ok := e.originalView()
	if !ok {
		return 0, false
	}
	return PrivateGroupingQueryEncodedBound(v)
}

// SourceByteLength is bounded scalar metadata, not a source issuance DTO.
func (e PrivateGroupingSourceEvidence) SourceByteLength() int {
	v, ok := e.originalView()
	if !ok {
		return 0
	}
	return len(v.Source)
}

// Snapshot returns defensive copies ONLY after a finite caller ceiling has been
// checked against the existing closed encoding ceiling. Source-stage callers
// must additionally account aggregate objects/work/bytes before requesting it.
// The returned DTO is diagnostic source data, never an issuance token.
func (e PrivateGroupingSourceEvidence) Snapshot(maxEncodedBytes uint64) (PrivateGroupingOwnedView, error) {
	bound, ok := e.EncodedBound()
	if !ok || maxEncodedBytes == 0 || maxEncodedBytes > PrivateGroupingQueryMaxBytes || bound > maxEncodedBytes {
		return PrivateGroupingOwnedView{}, ErrPrivateGroupingOwned
	}
	original, _ := e.originalView()
	v := original
	v.Source = append([]byte(nil), v.Source...)
	v.Supply.Content = append([]byte(nil), v.Supply.Content...)
	v.Supply.Params = append([]byte(nil), v.Supply.Params...)
	v.Symbol = clonePrivateGroupingPair(v.Symbol)
	v.Unavailable = append([]string(nil), v.Unavailable...)
	v.Methods = make([]OwnedMethodPair, len(original.Methods))
	for i, pair := range original.Methods {
		v.Methods[i] = clonePrivateGroupingPair(pair)
	}
	return v, nil
}
