//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"reflect"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

// This is a NEW private original-owner observation edge. In particular it is not
// ValidatedBatchPrivateV2, ordinary Native, ADR0012 or FULL_DEFINITION admission.
// No JSON DTO can construct it. A future consumer must explicitly opt into this
// family and account its own computation using the same application ledger.
type OriginalPageBatchPrivateV3 struct {
	state *originalPageBatchStatePrivateV3
}
type originalPageBatchStatePrivateV3 struct {
	self     *originalPageBatchStatePrivateV3
	owner    *producerPagesStatePrivateV3
	revision PageRevisionPrivateV3
	ordinal  int
}
type OriginalPageBatchViewPrivateV3 struct {
	Schema                                                     string
	OriginBinding                                              string
	Revision                                                   PageRevisionPrivateV3
	Ordinal                                                    int
	CanonicalSeeds, CaptureOrigin, SourceArtifact, NativeGraph []byte
	Source                                                     graphprovenance.SourceWithholdingPrivateV2
	Runtime                                                    sessionruntime.SessionMetadata
	Files                                                      []ProducerPacketFilePrivateV2
	Qualification                                              string
}

func (OriginalPageBatchPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (p ProducerPagesPrivateV3) Batch(ctx context.Context, r PageRevisionPrivateV3, ordinal int, exchange PageCostPrivateV3) (OriginalPageBatchPrivateV3, error) {
	if p.state == nil {
		return OriginalPageBatchPrivateV3{}, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if ordinal < 0 || ordinal >= len(s.captures) || r.Cursor.Object < uint64(7+len(s.frozen.state.files)+4*(ordinal+1)) {
		return OriginalPageBatchPrivateV3{}, ErrInvalid
	}
	if err := s.verify(ctx, r, &exchange, nil, nil); err != nil {
		return OriginalPageBatchPrivateV3{}, err
	}
	st := &originalPageBatchStatePrivateV3{owner: s, revision: r, ordinal: ordinal}
	st.self = st
	return OriginalPageBatchPrivateV3{st}, nil
}
func (b OriginalPageBatchPrivateV3) Read(ctx context.Context, exchange PageCostPrivateV3) (OriginalPageBatchViewPrivateV3, error) {
	if b.state == nil || b.state.self != b.state || b.state.owner == nil {
		return OriginalPageBatchViewPrivateV3{}, ErrInvalid
	}
	st := b.state
	s := st.owner
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || ctx == nil || ctx.Err() != nil || st.ordinal < 0 || st.ordinal >= len(s.captures) {
		return OriginalPageBatchViewPrivateV3{}, ErrInvalid
	}
	capture := s.captures[st.ordinal]
	if !successfulProducerLineagePrivateV2(s.frozen, s.origin, capture) {
		return OriginalPageBatchViewPrivateV3{}, ErrInvalid
	}
	n := uint64(len(capture.state.artifact))
	if n > s.policy.ComputationUnitBytes {
		return OriginalPageBatchViewPrivateV3{}, ErrPagesUnitPrivateV3
	}
	total := n + uint64(len(capture.state.seeds)+len(capture.state.origin))
	for _, f := range s.frozen.state.files {
		total += uint64(len(f.Bytes))
	}
	sourceCost, err := s.sourceCheckCost()
	if err != nil {
		return OriginalPageBatchViewPrivateV3{}, err
	}
	cost, err := sourceCost.plus(PageCostPrivateV3{LogicalBytes: 8 * total, ReadHashBytes: 32 * total, Work: 16384})
	if err != nil {
		return OriginalPageBatchViewPrivateV3{}, err
	}
	if e := s.reserve(cost, &exchange); e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	if e := s.frozen.revalidate(ctx); e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	// Reopen and compare the admitted prefix before exposing bytes, rather than
	// relying solely on a past hash or a detached report.
	if e := s.verify(ctx, st.revision, &exchange, nil, nil); e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	source, e := graphprovenance.ParseSourceWithholdingPrivateV2(capture.state.artifact)
	if e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	view := OriginalPageBatchViewPrivateV3{Schema: ProducerPagesSchemaPrivateV3, OriginBinding: s.binding, Revision: st.revision, Ordinal: st.ordinal, CanonicalSeeds: capture.state.seeds, CaptureOrigin: capture.state.origin, SourceArtifact: capture.state.artifact, NativeGraph: source.Evidence.GraphBytes, Source: source, Runtime: s.origin.state.adapter.metadata, Qualification: "PRIVATE_PAGED_ORIGINAL_OBSERVATIONS_ONLY_NOT_AST_ADMISSION"}
	for _, f := range s.frozen.state.files {
		view.Files = append(view.Files, ProducerPacketFilePrivateV2{f.scopeManifestFile, f.Bytes})
	}
	cloned, e := cloneValidatedMetadataPrivateV2(reflect.ValueOf(view))
	if e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	if e = ctx.Err(); e != nil {
		return OriginalPageBatchViewPrivateV3{}, e
	}
	return cloned.Interface().(OriginalPageBatchViewPrivateV3), nil
}
