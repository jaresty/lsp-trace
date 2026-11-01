//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
)

const SelectedAdmissionProfilePrivateV3 = "PRIVATE_PAGED_ORIGINAL_SELECTED_BATCH_V3"

// This sibling proof is never a ValidatedBatchPrivateV2. Its self/owner and
// original batch pointers cannot be recovered from JSON or a retained report.
type AdmittedPageBatchPrivateV3 struct {
	state *admittedPageBatchStatePrivateV3
}
type admittedPageBatchStatePrivateV3 struct {
	self            *admittedPageBatchStatePrivateV3
	batch           OriginalPageBatchPrivateV3
	source          graphprovenance.SourceWithholdingPrivateV2
	fullTargetCount int
	targets         []censusacquisition.PreparedTarget
	pins            [4]string
}
type SelectedPageBatchReaderPrivateV3 struct {
	state *selectedPageBatchReaderStatePrivateV3
}
type selectedPageBatchReaderStatePrivateV3 struct {
	self                        *selectedPageBatchReaderStatePrivateV3
	batch                       OriginalPageBatchPrivateV3
	proof                       pageVerifyStatePrivateV3
	generation                  uint64
	raw                         [4][]byte
	assembled                   [4]uint64
	allocated, failed, complete bool
}
type AdmittedPageBatchViewPrivateV3 struct {
	Profile, Binding, RevisionSHA256, ScopeSHA256, OriginSHA256, RuntimeSHA256 string
	Revision                                                                   PageRevisionPrivateV3
	Ordinal, FullBatchCount, FullTargetCount                                   int
	ObjectSHA256                                                               [4]string
	Source                                                                     graphprovenance.SourceWithholdingPrivateV2
	BatchTargets                                                               []censusacquisition.PreparedTarget
	CanonicalSeeds, CaptureOrigin, SourceArtifact, OriginalNativeV3            []byte
	RuntimeMetadata                                                            []byte
}

func (SelectedPageBatchReaderPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (AdmittedPageBatchPrivateV3) MarshalJSON() ([]byte, error)       { return nil, ErrInvalid }

// Only one prefix walk is needed for all selected role objects. The saved
// verifier cursor is fed directly into the parser admission; no second walk is
// done on successful completion or on a later budget/cancel continuation.
func (p ProducerPagesPrivateV3) SelectedBatchReader(ctx context.Context, r PageRevisionPrivateV3, ordinal int, exchange PageCostPrivateV3) (SelectedPageBatchReaderPrivateV3, error) {
	if p.state == nil {
		return SelectedPageBatchReaderPrivateV3{}, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		return SelectedPageBatchReaderPrivateV3{}, ErrInvalid
	}
	if e := s.reserve(PageCostPrivateV3{LogicalBytes: 4 * producerPageHeaderPrivateV3, Work: 32}, &exchange); e != nil {
		return SelectedPageBatchReaderPrivateV3{}, e
	}
	if !s.valid() || ordinal < 0 || ordinal >= len(s.captures) || r.Binding != s.binding || r.Policy != s.policy || r.Cursor.Object < uint64(7+len(s.frozen.state.files)+4*(ordinal+1)) {
		return SelectedPageBatchReaderPrivateV3{}, ErrInvalid
	}
	base := 7 + len(s.frozen.state.files) + 4*ordinal
	for j := 0; j < 4; j++ {
		if uint64(len(s.objects[base+j].raw)) > s.policy.ComputationUnitBytes {
			return SelectedPageBatchReaderPrivateV3{}, ErrPagesUnitPrivateV3
		}
	}
	b := &originalPageBatchStatePrivateV3{owner: s, revision: r, ordinal: ordinal}
	b.self = b
	st := &selectedPageBatchReaderStatePrivateV3{batch: OriginalPageBatchPrivateV3{b}, proof: pageVerifyStatePrivateV3{owner: s, revision: r, remaining: r.Cursor, last: r.Cursor.Last}}
	st.self = st
	return SelectedPageBatchReaderPrivateV3{st}, nil
}
func (v SelectedPageBatchReaderPrivateV3) Advance(ctx context.Context, generation uint64, exchange PageCostPrivateV3) (OriginalPageObjectProgressPrivateV3, AdmittedPageBatchPrivateV3, error) {
	if v.state == nil || v.state.self != v.state || v.state.batch.state == nil {
		return OriginalPageObjectProgressPrivateV3{}, AdmittedPageBatchPrivateV3{}, ErrInvalid
	}
	st := v.state
	b := st.batch.state
	s := b.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || ctx == nil || ctx.Err() != nil || st.complete || st.failed || generation != st.generation+1 {
		return OriginalPageObjectProgressPrivateV3{}, AdmittedPageBatchPrivateV3{}, ErrInvalid
	}
	st.generation = generation
	report := func(status string) OriginalPageObjectProgressPrivateV3 {
		var n uint64
		for _, v := range st.assembled {
			n += v
		}
		return OriginalPageObjectProgressPrivateV3{Status: status, PagesVerified: b.revision.Cursor.PageCount - st.proof.remaining.PageCount, PageDenominator: b.revision.Cursor.PageCount, SelectedBytes: n, Binding: s.binding, Cumulative: s.spent()}
	}
	fail := func(e error) (OriginalPageObjectProgressPrivateV3, AdmittedPageBatchPrivateV3, error) {
		status := "FAILED"
		if errors.Is(e, ErrPagesPausedPrivateV3) {
			status = "PAUSED_BUDGET"
		} else if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			status = "CANCELLED"
		} else {
			st.failed = true
			st.raw = [4][]byte{}
		}
		return report(status), AdmittedPageBatchPrivateV3{}, e
	}
	base := 7 + len(s.frozen.state.files) + 4*b.ordinal
	if !st.allocated {
		var n uint64
		for j := 0; j < 4; j++ {
			n += uint64(len(s.objects[base+j].raw))
		}
		if e := s.reserve(selectedObjectCostPrivateV3(n), &exchange); e != nil {
			return fail(e)
		}
		for j := 0; j < 4; j++ {
			st.raw[j] = make([]byte, len(s.objects[base+j].raw))
		}
		st.allocated = true
	}
	if e := s.verify(ctx, b.revision, &exchange, func(h pageHeaderPrivateV3, raw []byte) error {
		if h.Object >= uint64(base) && h.Object < uint64(base+4) {
			j := int(h.Object) - base
			copy(st.raw[j][h.Offset:], raw)
			st.assembled[j] += uint64(len(raw))
		}
		return nil
	}, &st.proof); e != nil {
		return fail(e)
	}
	// Parsing and all metadata/hash/clone work are reserved BEFORE they occur. A
	// pause here retains the fully verified cursor and selected buffers.
	if e := s.reserve(s.selectedStageCost(), &exchange); e != nil {
		return fail(e)
	}
	for j := 0; j < 4; j++ {
		if !bytes.Equal(st.raw[j], s.objects[base+j].raw) || st.assembled[j] != uint64(len(st.raw[j])) {
			return fail(ErrInvalid)
		}
	}
	c := s.captures[b.ordinal]
	if !successfulProducerLineagePrivateV2(s.frozen, s.origin, c) || !bytes.Equal(st.raw[0], c.state.seeds) || !bytes.Equal(st.raw[1], c.state.origin) || !bytes.Equal(st.raw[2], c.state.artifact) || !bytes.Equal(st.raw[3], s.origin.state.batches[b.ordinal].binding.Input) {
		return fail(ErrInvalid)
	}
	src, e := graphprovenance.ParseSourceWithholdingPrivateV2(st.raw[2])
	if e != nil {
		return fail(e)
	}
	// Decode only the enumeration, not all encoded planned batches/captures.
	var frame struct {
		Enumeration json.RawMessage `json:"actual_enumeration"`
	}
	var d censusacquisition.Discovery
	if json.Unmarshal(s.origin.state.raw, &frame) != nil || json.Unmarshal(frame.Enumeration, &d) != nil {
		return fail(ErrInvalid)
	}
	req, e := validateScopedBatchBindingPrivate(s.origin.state.adapter, s.origin.state.batches[b.ordinal].binding)
	if e != nil {
		return fail(e)
	}
	requested := append([]acquisition.Target{req.Root}, req.RequiredTargets...)
	if len(src.Policy.Members) != len(s.frozen.state.files) || len(src.Evidence.Acquisition.Targets) != len(requested) || src.Policy.SessionID != s.origin.state.identity.SessionID || src.Policy.Generation != s.origin.state.identity.Generation {
		return fail(ErrInvalid)
	}
	ac := src.Evidence.Acquisition.Request.Context
	if ac.SessionID != s.origin.state.identity.SessionID || ac.Generation != s.origin.state.identity.Generation || ac.PositionEncoding != s.origin.state.adapter.metadata.PositionEncoding || src.Policy.WorkspaceURI != (&url.URL{Scheme: "file", Path: s.frozen.state.spec.Root}).String() {
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
	var targets []censusacquisition.PreparedTarget
	for i, t := range requested {
		if !reflect.DeepEqual(src.Evidence.Acquisition.Targets[i].Requested, t) {
			return fail(ErrInvalid)
		}
		found := false
		for _, dt := range d.Targets {
			if t.Locator.URI == dt.URI && t.Locator.Line != nil && t.Locator.Character != nil && *t.Locator.Line == dt.SelectionRange.Start.Line && *t.Locator.Character == dt.SelectionRange.Start.Character {
				targets = append(targets, dt)
				found = true
				break
			}
		}
		if !found {
			return fail(ErrInvalid)
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	a := &admittedPageBatchStatePrivateV3{batch: st.batch, source: src, fullTargetCount: len(d.Targets), targets: targets}
	for j := 0; j < 4; j++ {
		a.pins[j] = Digest(st.raw[j])
	}
	a.self = a
	st.complete = true
	st.raw = [4][]byte{}
	return report("ADMITTED_SELECTED_BATCH_PRIVATE_V3"), AdmittedPageBatchPrivateV3{a}, nil
}

// Fixed private stage accounting, not a new caller-selectable allowance. Includes
// parsing, math, metadata clones/hashes and bounded result serialization. It is a
// conservative logical reservation, NOT a practical scalability measurement.
func (s *producerPagesStatePrivateV3) selectedStageCost() PageCostPrivateV3 {
	n := uint64(scopeMetadataPrivateV2)
	for _, f := range s.frozen.state.files {
		n += uint64(len(f.Bytes))
	}
	// Largest unit bounds each stage without materializing all captures.
	n += 4 * s.policy.ComputationUnitBytes
	return PageCostPrivateV3{LogicalBytes: 256 * n, ReadHashBytes: 256 * n, Work: 1000000}
}
func (b AdmittedPageBatchPrivateV3) valid() bool {
	return b.state != nil && b.state.self == b.state && b.state.batch.state != nil && b.state.batch.state.self == b.state.batch.state && b.state.batch.state.owner.valid()
}

// ReadStage reserves every attempt, rechecks the exact retained revision and
// original source, and yields only detached metadata. Typed consumers keep b,
// never reconstruct a qualification from the view. No prefix replay is needed.
func (b AdmittedPageBatchPrivateV3) ReadStage(ctx context.Context, exchange PageCostPrivateV3) (AdmittedPageBatchViewPrivateV3, error) {
	if !b.valid() || ctx == nil {
		return AdmittedPageBatchViewPrivateV3{}, ErrInvalid
	}
	a := b.state
	bs := a.batch.state
	s := bs.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	cost, e := s.sourceCheckCost()
	if e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	cost, e = cost.plus(s.selectedStageCost())
	if e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = s.reserve(cost, &exchange); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = ctx.Err(); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = s.frozen.revalidate(ctx); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = s.verifyRevision(ctx, bs.revision, &exchange); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	c := s.captures[bs.ordinal]
	if !successfulProducerLineagePrivateV2(s.frozen, s.origin, c) {
		return AdmittedPageBatchViewPrivateV3{}, ErrInvalid
	}
	meta, _ := json.Marshal(s.origin.state.adapter.metadata)
	v := AdmittedPageBatchViewPrivateV3{Profile: SelectedAdmissionProfilePrivateV3, Binding: s.binding, RevisionSHA256: Digest(mustPageJSONPrivateV3(bs.revision)), ScopeSHA256: Digest(s.frozen.state.manifest), OriginSHA256: Digest(s.origin.state.raw), RuntimeSHA256: Digest(meta), Revision: bs.revision, Ordinal: bs.ordinal, FullBatchCount: len(s.captures), FullTargetCount: a.fullTargetCount, ObjectSHA256: a.pins, Source: a.source, BatchTargets: a.targets, CanonicalSeeds: c.state.seeds, CaptureOrigin: c.state.origin, SourceArtifact: c.state.artifact, OriginalNativeV3: a.source.Evidence.GraphBytes, RuntimeMetadata: meta}
	clone, e := cloneValidatedMetadataPrivateV2(reflect.ValueOf(v))
	if e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = ctx.Err(); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = s.frozen.revalidate(ctx); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	if e = s.roots.check(ctx, s.frozen); e != nil {
		return AdmittedPageBatchViewPrivateV3{}, e
	}
	return clone.Interface().(AdmittedPageBatchViewPrivateV3), nil
}
func mustPageJSONPrivateV3(v any) []byte { raw, _ := json.Marshal(v); return raw }
func (b AdmittedPageBatchPrivateV3) BudgetSpent() PageCostPrivateV3 {
	if !b.valid() {
		return PageCostPrivateV3{}
	}
	return (ProducerPagesPrivateV3{b.state.batch.state.owner}).BudgetSpent()
}

// Carry the exchange remainder across nested stage calls. Concurrent ledger work
// can only make this conservative; it cannot replenish the exchange.
func RemainingPageExchangePrivateV3(before, after, left PageCostPrivateV3) (PageCostPrivateV3, error) {
	if !before.fits(after) {
		return PageCostPrivateV3{}, ErrInvalid
	}
	delta := after.minus(before)
	if !delta.fits(left) {
		return PageCostPrivateV3{}, ErrPagesPausedPrivateV3
	}
	return left.minus(delta), nil
}

// OwnSource is never a path/source fallback: only a readable receipt whose exact
// bytes match the original frozen owned member may expose a source body.
func (b AdmittedPageBatchPrivateV3) OwnSource(ctx context.Context, uri string, exchange PageCostPrivateV3) ([]byte, error) {
	v, e := b.ReadStage(ctx, exchange)
	if e != nil {
		return nil, e
	}
	s := b.state.batch.state.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.frozen.state.files {
		if f.URI == uri && f.Role == "NOMINATION" {
			count := 0
			for _, r := range v.Source.Evidence.Captures {
				if r.URI == uri {
					if r.Status != "READABLE" || !bytes.Equal(r.Content, f.Bytes) {
						return nil, ErrInvalid
					}
					count++
				}
			}
			if count != 1 {
				return nil, ErrInvalid
			}
			return bytes.Clone(f.Bytes), nil
		}
	}
	return nil, ErrInvalid
}

// This low-level pair sink does not mint a prepared owner. The opaque prepared
// consumer owns qualification; this method returns merely a content reference.
func (b AdmittedPageBatchPrivateV3) RetainOutput(ctx context.Context, raw []byte, exchange PageCostPrivateV3) (string, error) {
	if !b.valid() || ctx == nil {
		return "", ErrInvalid
	}
	s := b.state.batch.state.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if uint64(len(raw)) > s.policy.ComputationUnitBytes {
		return "", ErrPagesUnitPrivateV3
	}
	cost, e := s.sourceCheckCost()
	if e != nil {
		return "", e
	}
	cost, e = cost.plus(pagePairCostPrivateV3(uint64(len(raw))))
	if e != nil {
		return "", e
	}
	if e = s.reserve(cost, &exchange); e != nil {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	if e = s.frozen.revalidate(ctx); e != nil {
		return "", e
	}
	if e = s.verifyRevision(ctx, b.state.batch.state.revision, &exchange); e != nil {
		return "", e
	}
	id, e := s.retain(ctx, raw)
	if e != nil {
		return "", e
	}
	if e = s.frozen.revalidate(ctx); e != nil {
		return "", e
	}
	return id, ctx.Err()
}
