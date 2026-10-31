//go:build darwin || linux

package censuslineagepilot

import "context"

// The verifier's cursor is opaque. Inspection progress cannot restore it. Each
// exchange can verify a bounded suffix and pause without rescanning that suffix.
// It borrows the original owner and root capabilities; no source lease is replayed.
type PagePrefixVerifierPrivateV3 struct{ state *pageVerifyStatePrivateV3 }
type pageVerifyStatePrivateV3 struct {
	self       *pageVerifyStatePrivateV3
	owner      *producerPagesStatePrivateV3
	revision   PageRevisionPrivateV3
	remaining  PageCursorPrivateV3
	last       string
	generation uint64
	complete   bool
}
type PageVerificationProgressPrivateV3 struct {
	Status          string            `json:"status"`
	PagesVerified   uint64            `json:"pages_verified"`
	PageDenominator uint64            `json:"page_denominator"`
	Binding         string            `json:"binding_sha256"`
	Cumulative      PageCostPrivateV3 `json:"cumulative_reserved"`
}

func (PagePrefixVerifierPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (p ProducerPagesPrivateV3) PrefixVerifier(r PageRevisionPrivateV3) (PagePrefixVerifierPrivateV3, error) {
	if p.state == nil {
		return PagePrefixVerifierPrivateV3{}, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || r.Binding != s.binding || r.Policy != s.policy {
		return PagePrefixVerifierPrivateV3{}, ErrInvalid
	}
	st := &pageVerifyStatePrivateV3{owner: s, revision: r, remaining: r.Cursor, last: r.Cursor.Last}
	st.self = st
	return PagePrefixVerifierPrivateV3{st}, nil
}
func (v PagePrefixVerifierPrivateV3) Advance(ctx context.Context, generation uint64, exchange PageCostPrivateV3) (PageVerificationProgressPrivateV3, error) {
	if v.state == nil || v.state.self != v.state || v.state.owner == nil {
		return PageVerificationProgressPrivateV3{}, ErrInvalid
	}
	st := v.state
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.complete || generation != st.generation+1 || ctx == nil || ctx.Err() != nil {
		return PageVerificationProgressPrivateV3{}, ErrInvalid
	}
	st.generation = generation
	err := s.verify(ctx, st.revision, &exchange, nil, st)
	status := "VERIFIED_ORIGINAL_PREFIX"
	if err == ErrPagesPausedPrivateV3 {
		status = "PAUSED_BUDGET"
	} else if err != nil {
		status = "FAILED"
	} else {
		st.complete = true
	}
	return PageVerificationProgressPrivateV3{status, st.revision.Cursor.PageCount - st.remaining.PageCount, st.revision.Cursor.PageCount, s.binding, s.spent()}, err
}
