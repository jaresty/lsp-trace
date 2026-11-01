//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
)

type retainedReportReferencesPrivateV3 struct {
	Schema, Checkpoint, Previous string
	Ordinal                      int
	References                   [3]string
}

// ReadOutput verifies exact existing paired bytes under explicit unit and ledger
// bounds. It does not mint a preparation/source qualification from those bytes.
func (p RetainedPageCheckpointOwnerPrivateV3) ReadOutput(ctx context.Context, id string, max uint64, exchange PageCostPrivateV3) ([]byte, error) {
	s := p.state
	if !s.valid() || max == 0 || max > s.manifest.Revision.Policy.ComputationUnitBytes {
		return nil, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.checkSource(ctx, &exchange); e != nil {
		return nil, e
	}
	return s.readPair(ctx, id, max, &exchange)
}

// RecordPublishedReferences records only verified paired content references. It
// does NOT assert that arbitrary caller-supplied bytes are a prepared seal. The
// separate observational preparation API owns that seal. Linked report metadata
// is bounded, immutable, and does not accumulate an application-total array.
func (p RetainedPageCheckpointOwnerPrivateV3) RecordPublishedReferences(ctx context.Context, b RetainedSelectedBatchPrivateV3, refs [3]string, exchange PageCostPrivateV3) error {
	s := p.state
	if !s.valid() || b.state == nil || b.state.self != b.state || b.state.owner != s {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.state.ordinal != s.manifest.NextBatch || s.pendingReports != "" {
		return ErrInvalid
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return e
	}
	for _, id := range refs {
		if _, e := s.readPair(ctx, id, s.manifest.Revision.Policy.ComputationUnitBytes, &exchange); e != nil {
			return e
		}
	}
	r := retainedReportReferencesPrivateV3{retainedCheckpointSchemaPrivateV3, b.state.checkpoint, s.manifest.Reports, b.state.ordinal, refs}
	if e := s.ledger.state.reserve(pagePairCostPrivateV3(producerPageHeaderPrivateV3), &exchange); e != nil {
		return e
	}
	raw, _ := json.Marshal(r)
	if len(raw) > producerPageHeaderPrivateV3 {
		return ErrInvalid
	}
	id := Digest(raw)
	if _, e := retainScopePairBytes(ctx, scopeRetentionObject{id, raw}, id, 0, s.roots.state.outputs[0], s.roots.state.outputs[1], producerPacketRetentionOpsPrivateV2()); e != nil {
		return e
	}
	s.pendingReports = id
	return ctx.Err()
}
func (s *retainedCheckpointStatePrivateV3) verifyReports(ctx context.Context, left *PageCostPrivateV3) error {
	id := s.manifest.Reports
	previous := s.manifest.NextBatch
	for n := 0; id != ""; n++ {
		if n >= s.manifest.NextBatch {
			return ErrInvalid
		}
		raw, e := s.readPair(ctx, id, producerPageHeaderPrivateV3, left)
		if e != nil {
			return e
		}
		var r retainedReportReferencesPrivateV3
		if canonicalRetainedPrivateV3(raw, &r) != nil || r.Schema != retainedCheckpointSchemaPrivateV3 || !validPageDigestPrivateV3(r.Checkpoint) || r.Ordinal < 0 || r.Ordinal >= previous {
			return ErrInvalid
		}
		for _, ref := range r.References {
			if _, e = s.readPair(ctx, ref, s.manifest.Revision.Policy.ComputationUnitBytes, left); e != nil {
				return e
			}
		}
		previous = r.Ordinal
		id = r.Previous
	}
	return nil
}
