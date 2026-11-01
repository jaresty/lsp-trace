//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"lsp-trace/internal/publication"
)

// Read one token at a time from the already-owned original enumeration. No
// decoded whole-origin object, capture array, or target array is allocated.
func countRetainedEnumerationTargetsPrivateV3(ctx context.Context, raw []byte) (uint64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	work := 1000000
	found := false
	var walk func(int, int) (uint64, error)
	walk = func(mode, depth int) (uint64, error) {
		work--
		if depth > 64 || work < 0 || ctx.Err() != nil {
			return 0, ErrInvalid
		}
		tok, e := dec.Token()
		if e != nil {
			return 0, e
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return 0, nil
		}
		var count uint64
		switch d {
		case '{':
			for dec.More() {
				k, e := dec.Token()
				if e != nil {
					return 0, e
				}
				key, ok := k.(string)
				if !ok {
					return 0, ErrInvalid
				}
				next := 3
				if mode == 0 && key == "actual_enumeration" {
					next = 1
				}
				if mode == 1 && key == "Targets" {
					if found {
						return 0, ErrInvalid
					}
					found = true
					next = 2
				}
				n, e := walk(next, depth+1)
				if e != nil {
					return 0, e
				}
				count += n
			}
			end, e := dec.Token()
			if e != nil || end != json.Delim('}') {
				return 0, ErrInvalid
			}
		case '[':
			for dec.More() {
				n, e := walk(3, depth+1)
				if e != nil {
					return 0, e
				}
				if mode == 2 {
					count++
				} else {
					count += n
				}
			}
			end, e := dec.Token()
			if e != nil || end != json.Delim(']') {
				return 0, ErrInvalid
			}
		default:
			return 0, ErrInvalid
		}
		return count, nil
	}
	n, e := walk(0, 0)
	if e != nil || !found || n > 10000 {
		return 0, ErrInvalid
	}
	if _, e = dec.Token(); e != io.EOF {
		return 0, ErrInvalid
	}
	return n, nil
}
func (b RetainedSelectedBatchPrivateV3) BudgetSpent() PageCostPrivateV3 {
	if b.state == nil || b.state.self != b.state {
		return PageCostPrivateV3{}
	}
	return (RetainedPageCheckpointOwnerPrivateV3{b.state.owner}).BudgetSpent()
}

// This is a paired byte sink, not original capture qualification or a prepared
// seal. Only the separate observational preparation family can mint its seal.
func (b RetainedSelectedBatchPrivateV3) RetainOutput(ctx context.Context, raw []byte, exchange PageCostPrivateV3) (string, error) {
	st := b.state
	if st == nil || st.self != st || !st.owner.valid() {
		return "", ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if uint64(len(raw)) > s.manifest.Revision.Policy.ComputationUnitBytes {
		return "", ErrPagesUnitPrivateV3
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return "", e
	}
	if e := s.ledger.state.reserve(pagePairCostPrivateV3(uint64(len(raw))), &exchange); e != nil {
		return "", e
	}
	id := Digest(raw)
	if _, e := retainScopePairBytes(ctx, scopeRetentionObject{id, raw}, id, 0, s.roots.state.outputs[0], s.roots.state.outputs[1], producerPacketRetentionOpsPrivateV2()); e != nil {
		return "", e
	}
	if e := publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, s.roots.state.outputs, id); e != nil {
		return "", e
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return "", e
	}
	return id, nil
}
