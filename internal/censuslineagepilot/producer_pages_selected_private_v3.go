//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
)

// OriginalPageObjectReaderPrivateV3 is original-owner-only continuation, not a
// checkpoint-restored source, Native owner, or AST qualification.
type OriginalPageObjectReaderPrivateV3 struct {
	state *originalPageObjectReadStatePrivateV3
}
type originalPageObjectReadStatePrivateV3 struct {
	self                        *originalPageObjectReadStatePrivateV3
	owner                       *producerPagesStatePrivateV3
	proof                       pageVerifyStatePrivateV3
	object                      uint64
	generation                  uint64
	assembled                   uint64
	out                         []byte
	allocated, complete, failed bool
}
type OriginalPageObjectProgressPrivateV3 struct {
	Status                                        string
	PagesVerified, PageDenominator, SelectedBytes uint64
	Binding                                       string
	Cumulative                                    PageCostPrivateV3
}

// Covers allocation, selected copies, final comparison and digest scans; object
// size is bounded by the existing explicit computation-unit policy.
func selectedObjectCostPrivateV3(n uint64) PageCostPrivateV3 {
	return PageCostPrivateV3{LogicalBytes: 4 * n, ReadHashBytes: 2 * n, Work: 1}
}
func (OriginalPageObjectReaderPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (p ProducerPagesPrivateV3) ObjectReader(ctx context.Context, r PageRevisionPrivateV3, object uint64, exchange PageCostPrivateV3) (OriginalPageObjectReaderPrivateV3, error) {
	if p.state == nil {
		return OriginalPageObjectReaderPrivateV3{}, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		return OriginalPageObjectReaderPrivateV3{}, ErrInvalid
	}
	// Reserve bounded handle/proof/progress metadata before allocating the handle,
	// including failed admissions. This is independent of selected payload cost.
	if err := s.reserve(PageCostPrivateV3{LogicalBytes: 2 * producerPageHeaderPrivateV3, Work: 16}, &exchange); err != nil {
		return OriginalPageObjectReaderPrivateV3{}, err
	}
	if !s.valid() || r.Binding != s.binding || r.Policy != s.policy || object >= uint64(len(s.objects)) || object >= r.Cursor.Object {
		return OriginalPageObjectReaderPrivateV3{}, ErrInvalid
	}
	if uint64(len(s.objects[object].raw)) > s.policy.ComputationUnitBytes {
		return OriginalPageObjectReaderPrivateV3{}, ErrPagesUnitPrivateV3
	}
	st := &originalPageObjectReadStatePrivateV3{owner: s, object: object, proof: pageVerifyStatePrivateV3{owner: s, revision: r, remaining: r.Cursor, last: r.Cursor.Last}}
	st.self = st
	return OriginalPageObjectReaderPrivateV3{st}, nil
}
func (v OriginalPageObjectReaderPrivateV3) Advance(ctx context.Context, generation uint64, exchange PageCostPrivateV3) (OriginalPageObjectProgressPrivateV3, []byte, error) {
	if v.state == nil || v.state.self != v.state || v.state.owner == nil {
		return OriginalPageObjectProgressPrivateV3{}, nil, ErrInvalid
	}
	st := v.state
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || st.complete || st.failed || generation != st.generation+1 || ctx == nil || ctx.Err() != nil {
		return OriginalPageObjectProgressPrivateV3{}, nil, ErrInvalid
	}
	st.generation = generation
	report := func(status string) OriginalPageObjectProgressPrivateV3 {
		return OriginalPageObjectProgressPrivateV3{Status: status, PagesVerified: st.proof.revision.Cursor.PageCount - st.proof.remaining.PageCount, PageDenominator: st.proof.revision.Cursor.PageCount, SelectedBytes: st.assembled, Binding: s.binding, Cumulative: s.spent()}
	}
	if !st.allocated {
		n := uint64(len(s.objects[st.object].raw))
		if err := s.reserve(selectedObjectCostPrivateV3(n), &exchange); err != nil {
			if errors.Is(err, ErrPagesPausedPrivateV3) {
				return report("PAUSED_BUDGET"), nil, err
			}
			st.failed = true
			return report("FAILED"), nil, err
		}
		st.out = make([]byte, n)
		st.allocated = true
	}
	err := s.verify(ctx, st.proof.revision, &exchange, func(h pageHeaderPrivateV3, b []byte) error {
		if h.Object == st.object {
			copy(st.out[h.Offset:], b)
			st.assembled += uint64(len(b))
		}
		return nil
	}, &st.proof)
	if err != nil {
		status := "FAILED"
		if errors.Is(err, ErrPagesPausedPrivateV3) {
			status = "PAUSED_BUDGET"
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = "CANCELLED"
		} else {
			st.failed = true
			st.out = nil
		}
		return report(status), nil, err
	}
	// verify compared every page with its original-owner bytes. This final bounded
	// comparison guards assembly independently, without re-reading retained pages.
	if !bytes.Equal(st.out, s.objects[st.object].raw) {
		st.failed = true
		st.out = nil
		return report("FAILED"), nil, ErrInvalid
	}
	st.complete = true
	progress := report("COMPLETE_SELECTED_ORIGINAL_OBJECT")
	out := st.out
	st.out = nil // transfer ownership; no uncharged final clone or reread
	return progress, out, nil
}
