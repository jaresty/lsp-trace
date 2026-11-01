//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/graphprovenance"
)

type RetainedSelectedReaderPrivateV3 struct {
	state *retainedSelectedReaderStatePrivateV3
}
type retainedSelectedReaderStatePrivateV3 struct {
	self                        *retainedSelectedReaderStatePrivateV3
	owner                       *retainedCheckpointStatePrivateV3
	ordinal, role               int
	checkpoint                  string
	generation, pages           uint64
	raw                         [4][]byte
	descriptor                  retainedObjectIndexPrivateV3
	remaining, count            uint64
	last                        string
	allocated, complete, failed bool
}
type RetainedSelectedProgressPrivateV3 struct {
	Status                  string
	VerifiedPages           uint64
	CompletedRoles, Ordinal int
	Cumulative              PageCostPrivateV3
}
type retainedSelectedSnapshotPrivateV3 struct {
	Schema, Checkpoint string
	Ordinal, Role      int
	Generation, Pages  uint64
	Raw                [4]string
	Descriptor         retainedObjectIndexPrivateV3
	Remaining, Count   uint64
	Last               string
	Allocated          bool
}

func (RetainedSelectedReaderPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (p RetainedPageCheckpointOwnerPrivateV3) SelectedBatchReader(ctx context.Context, ordinal int, exchange PageCostPrivateV3) (RetainedSelectedReaderPrivateV3, error) {
	s := p.state
	if !s.valid() {
		return RetainedSelectedReaderPrivateV3{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil || ctx.Err() != nil || ordinal != s.manifest.NextBatch || ordinal < 0 || uint64(ordinal) >= s.manifest.Revision.BatchDenominator || 7+s.manifest.Members+4*uint64(ordinal+1) > s.manifest.Objects {
		return RetainedSelectedReaderPrivateV3{}, ErrInvalid
	}
	if e := s.ledger.state.reserve(checkpointReadCostPrivateV3(producerPageHeaderPrivateV3), &exchange); e != nil {
		return RetainedSelectedReaderPrivateV3{}, e
	}
	if s.savedSelection != nil {
		st := s.savedSelection
		s.savedSelection = nil
		return RetainedSelectedReaderPrivateV3{st}, nil
	}
	st := &retainedSelectedReaderStatePrivateV3{owner: s, ordinal: ordinal, checkpoint: s.checkpoint}
	st.self = st
	if s.manifest.SelectionSnapshot != "" {
		if e := st.restore(ctx, s.manifest.SelectionSnapshot, &exchange); e != nil {
			return RetainedSelectedReaderPrivateV3{}, e
		}
	}
	return RetainedSelectedReaderPrivateV3{st}, nil
}
func (p RetainedPageCheckpointOwnerPrivateV3) SelectedBatch(ctx context.Context, ordinal int, exchange PageCostPrivateV3) (RetainedSelectedBatchPrivateV3, error) {
	before := p.BudgetSpent()
	r, e := p.SelectedBatchReader(ctx, ordinal, exchange)
	if e != nil {
		return RetainedSelectedBatchPrivateV3{}, e
	}
	exchange, e = RemainingPageExchangePrivateV3(before, p.BudgetSpent(), exchange)
	if e != nil {
		return RetainedSelectedBatchPrivateV3{}, e
	}
	_, b, e := r.Advance(ctx, r.state.generation+1, ^uint64(0), exchange)
	return b, e
}

// Advance preserves the opaque verified suffix across budget/cancel/page pauses.
// maxPages is an exchange scheduling bound, not application coverage or a cap
// waiver. A page-bound pause emits no partial selected batch.
func (r RetainedSelectedReaderPrivateV3) Advance(ctx context.Context, generation, maxPages uint64, exchange PageCostPrivateV3) (RetainedSelectedProgressPrivateV3, RetainedSelectedBatchPrivateV3, error) {
	st := r.state
	if st == nil || st.self != st || !st.owner.valid() {
		return RetainedSelectedProgressPrivateV3{}, RetainedSelectedBatchPrivateV3{}, ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	progress := func(status string) RetainedSelectedProgressPrivateV3 {
		return RetainedSelectedProgressPrivateV3{status, st.pages, st.role, st.ordinal, (RetainedPageCheckpointOwnerPrivateV3{s}).BudgetSpent()}
	}
	if ctx == nil || generation != st.generation+1 || st.generation == ^uint64(0) || st.complete || st.failed || maxPages == 0 {
		return progress("INVALID"), RetainedSelectedBatchPrivateV3{}, ErrInvalid
	}
	st.generation = generation
	fail := func(e error) (RetainedSelectedProgressPrivateV3, RetainedSelectedBatchPrivateV3, error) {
		status := "FAILED"
		if errors.Is(e, ErrPagesPausedPrivateV3) {
			status = "PAUSED_BUDGET"
		} else if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			status = "CANCELLED"
		} else {
			st.failed = true
			st.raw = [4][]byte{}
		}
		return progress(status), RetainedSelectedBatchPrivateV3{}, e
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return fail(e)
	}
	used := uint64(0)
	for st.role < 4 {
		object := 7 + s.manifest.Members + 4*uint64(st.ordinal) + uint64(st.role)
		if !st.allocated {
			d, e := s.findObject(ctx, object, &exchange)
			if e != nil {
				return fail(e)
			}
			if d.Length > s.manifest.Revision.Policy.ComputationUnitBytes {
				return fail(ErrPagesUnitPrivateV3)
			}
			if e = s.ledger.state.reserve(selectedObjectCostPrivateV3(d.Length), &exchange); e != nil {
				return fail(e)
			}
			st.raw[st.role] = make([]byte, d.Length)
			st.descriptor = d
			st.remaining = d.Length
			st.last = d.LastPage
			st.count = d.PageCount
			st.allocated = true
		}
		if used == maxPages {
			return progress("PAUSED_PAGE_BOUND"), RetainedSelectedBatchPrivateV3{}, nil
		}
		if e := st.readPage(ctx, &exchange); e != nil {
			return fail(e)
		}
		used++
		if st.remaining == 0 {
			if Digest(st.raw[st.role]) != st.descriptor.SHA256 {
				return fail(ErrInvalid)
			}
			st.role++
			st.allocated = false
			st.descriptor = retainedObjectIndexPrivateV3{}
			st.last = ""
			st.count = 0
		}
	}
	// Parser/hash/metadata work is admitted before parsing, including failed work.
	if e := s.ledger.state.reserve(s.stageCost(), &exchange); e != nil {
		return fail(e)
	}
	src, e := graphprovenance.ParseSourceWithholdingPrivateV2(st.raw[2])
	if e != nil {
		return fail(e)
	}
	if len(src.Policy.Members) != len(s.frozen.state.files) {
		return fail(ErrInvalid)
	}
	for _, m := range src.Policy.Members {
		found := false
		for _, f := range s.frozen.state.files {
			if m.URI == f.URI && m.SHA256 == f.SHA256 && m.Role == f.Role {
				found = true
			}
		}
		if !found {
			return fail(ErrInvalid)
		}
	}
	if e = s.checkSource(ctx, &exchange); e != nil {
		return fail(e)
	}
	b := &retainedSelectedBatchStatePrivateV3{owner: s, ordinal: st.ordinal, checkpoint: st.checkpoint, raw: st.raw, source: src}
	b.self = b
	st.raw = [4][]byte{}
	st.complete = true
	return progress("ADMITTED_RETAINED_SELECTED_OBSERVATIONS"), RetainedSelectedBatchPrivateV3{b}, nil
}
func (st *retainedSelectedReaderStatePrivateV3) readPage(ctx context.Context, left *PageCostPrivateV3) error {
	s := st.owner
	d := st.descriptor
	raw, e := s.readPair(ctx, st.last, s.manifest.Revision.Policy.PayloadBytes+producerPageHeaderPrivateV3+4, left)
	if e != nil {
		return e
	}
	h, b, e := decodePagePrivateV3(raw)
	if e != nil {
		return e
	}
	if st.count == 0 || h.Index != st.count-1 || h.Schema != ProducerPagesSchemaPrivateV3 || h.Binding != d.Binding || h.Object != d.Object || h.Role != d.Role || h.Member != d.Member || h.Batch != d.Batch || h.Length != d.Length || h.Offset > st.remaining || uint64(len(b)) != st.remaining-h.Offset || uint64(len(b)) > s.manifest.Revision.Policy.PayloadBytes || d.Length > 0 && len(b) == 0 {
		return ErrInvalid
	}
	copy(st.raw[st.role][h.Offset:st.remaining], b)
	st.remaining = h.Offset
	st.last = h.Previous
	st.count--
	st.pages++
	return nil
}

// Checkpoint saves bounded private spools, never promotes them to complete role
// bytes. The manifest binds their exact pins and cursor. A later process verifies
// each saved suffix against the admitted raw page chain before using that cursor.
func (r RetainedSelectedReaderPrivateV3) Checkpoint(ctx context.Context, intent string, exchange PageCostPrivateV3) ([]byte, error) {
	st := r.state
	if st == nil || st.self != st || !st.owner.valid() || st.complete || st.failed || !validPageDigestPrivateV3(intent) {
		return nil, ErrInvalid
	}
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.ordinal != s.manifest.NextBatch || s.manifest.Generation == ^uint64(0) {
		return nil, ErrInvalid
	}
	if e := s.checkSource(ctx, &exchange); e != nil {
		return nil, e
	}
	snap := retainedSelectedSnapshotPrivateV3{Schema: retainedCheckpointSchemaPrivateV3, Checkpoint: st.checkpoint, Ordinal: st.ordinal, Role: st.role, Generation: st.generation, Pages: st.pages, Descriptor: st.descriptor, Remaining: st.remaining, Count: st.count, Last: st.last, Allocated: st.allocated}
	for j, raw := range st.raw {
		if raw == nil {
			continue
		}
		if uint64(len(raw))+1 > s.manifest.Revision.Policy.ComputationUnitBytes {
			return nil, ErrPagesUnitPrivateV3
		}
		if e := s.ledger.state.reserve(pagePairCostPrivateV3(uint64(len(raw))+1), &exchange); e != nil {
			return nil, e
		}
		spool := make([]byte, len(raw)+1)
		copy(spool[1:], raw)
		id := Digest(spool)
		if _, e := retainScopePairBytes(ctx, scopeRetentionObject{id, spool}, id, 0, s.roots.state.outputs[0], s.roots.state.outputs[1], producerPacketRetentionOpsPrivateV2()); e != nil {
			return nil, e
		}
		snap.Raw[j] = id
	}
	if e := s.ledger.state.reserve(pagePairCostPrivateV3(producerPageHeaderPrivateV3), &exchange); e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(snap)
	if len(raw) > producerPageHeaderPrivateV3 {
		return nil, ErrInvalid
	}
	id := Digest(raw)
	if _, e := retainScopePairBytes(ctx, scopeRetentionObject{id, raw}, id, 0, s.roots.state.outputs[0], s.roots.state.outputs[1], producerPacketRetentionOpsPrivateV2()); e != nil {
		return nil, e
	}
	m := s.manifest
	m.Generation++
	m.PreviousCheckpoint = s.checkpoint
	m.Intent = intent
	m.SelectionSnapshot = id
	m.SemanticStage = "PARTIAL_SELECTED_OBSERVATIONS_NOT_COMPLETE"
	out, e := writeRetainedCheckpointPrivateV3(ctx, m, s.ledger, s.roots, s.frozen, &exchange)
	if e != nil {
		return nil, e
	}
	if canonicalRetainedPrivateV3(out, &m) != nil {
		return nil, ErrInvalid
	}
	s.manifest = m
	s.checkpoint = Digest(out)
	return out, nil
}
func (st *retainedSelectedReaderStatePrivateV3) restore(ctx context.Context, id string, left *PageCostPrivateV3) error {
	s := st.owner
	if e := s.checkSource(ctx, left); e != nil {
		return e
	}
	raw, e := s.readPair(ctx, id, producerPageHeaderPrivateV3, left)
	if e != nil {
		return e
	}
	var snap retainedSelectedSnapshotPrivateV3
	if canonicalRetainedPrivateV3(raw, &snap) != nil || snap.Schema != retainedCheckpointSchemaPrivateV3 || !validPageDigestPrivateV3(snap.Checkpoint) || snap.Ordinal != st.ordinal || snap.Role < 0 || snap.Role > 4 || snap.Generation == ^uint64(0) || snap.Pages > s.manifest.Revision.Cursor.PageCount || snap.Role == 4 && snap.Allocated {
		return ErrInvalid
	}
	if e = s.ledger.state.reserve(s.stageCost(), left); e != nil {
		return e
	}
	verified := uint64(0)
	for j, ref := range snap.Raw {
		required := j < snap.Role || j == snap.Role && snap.Allocated
		if !required {
			if ref != "" {
				return ErrInvalid
			}
			continue
		}
		object := 7 + s.manifest.Members + 4*uint64(st.ordinal) + uint64(j)
		d, e := s.findObject(ctx, object, left)
		if e != nil {
			return e
		}
		if d.Length+1 > s.manifest.Revision.Policy.ComputationUnitBytes || d.Length == ^uint64(0) {
			return ErrPagesUnitPrivateV3
		}
		if e = s.ledger.state.reserve(selectedObjectCostPrivateV3(d.Length), left); e != nil {
			return e
		}
		spool, e := s.readPair(ctx, ref, d.Length+1, left)
		if e != nil {
			return e
		}
		if uint64(len(spool)) != d.Length+1 || spool[0] != 0 {
			return ErrInvalid
		}
		st.raw[j] = bytes.Clone(spool[1:])
		// Verify the saved suffix with the actual pages, not just the spool digest.
		probe := retainedSelectedReaderStatePrivateV3{owner: s, role: j, descriptor: d, remaining: d.Length, last: d.LastPage, count: d.PageCount}
		probe.raw[j] = make([]byte, d.Length)
		target := uint64(0)
		if j == snap.Role {
			target = snap.Remaining
			if snap.Descriptor != d || target > d.Length {
				return ErrInvalid
			}
		}
		for probe.remaining > target || d.Length == 0 && probe.pages == 0 {
			if e = probe.readPage(ctx, left); e != nil {
				return e
			}
			if probe.remaining < target {
				return ErrInvalid
			}
		}
		verified += probe.pages
		if !bytes.Equal(probe.raw[j], st.raw[j]) {
			return ErrInvalid
		}
		if j < snap.Role {
			if Digest(st.raw[j]) != d.SHA256 {
				return ErrInvalid
			}
		} else if probe.remaining != snap.Remaining || probe.last != snap.Last || probe.count != snap.Count {
			return ErrInvalid
		}
	}
	if !snap.Allocated && (snap.Remaining != 0 || snap.Count != 0 || snap.Last != "" || snap.Descriptor != (retainedObjectIndexPrivateV3{})) {
		return ErrInvalid
	}
	if verified != snap.Pages {
		return ErrInvalid
	}
	st.role = snap.Role
	st.generation = 0
	st.pages = snap.Pages
	st.descriptor = snap.Descriptor
	st.remaining = snap.Remaining
	st.count = snap.Count
	st.last = snap.Last
	st.allocated = snap.Allocated
	return s.checkSource(ctx, left)
}
