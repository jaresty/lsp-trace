package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"

	"lsp-trace/internal/publication"
)

// Roots borrow concrete capabilities NONCONCURRENTLY through retention,
// qualification and any derived reads. The caller owns Close. Additional roots
// describe only supplied optional artifact roots, never a complete hidden set.
type ProducerPacketRootsPrivateV2 struct {
	state *producerPacketRootsStatePrivateV2
}
type producerPacketRootsStatePrivateV2 struct {
	self       *producerPacketRootsStatePrivateV2
	outputs    [2]*publication.ContentRetentionRoot
	source     *publication.SourceProtectionRootPrivateV2
	additional []*publication.ObservedRoot
}

func NewProducerPacketRootsPrivateV2(ctx context.Context, outputs [2]*publication.ContentRetentionRoot, source *publication.SourceProtectionRootPrivateV2, additional []*publication.ObservedRoot) (ProducerPacketRootsPrivateV2, error) {
	if err := publication.CheckProducerPacketRetentionTopologyPrivateV2(ctx, outputs, source, additional); err != nil {
		return ProducerPacketRootsPrivateV2{}, err
	}
	s := &producerPacketRootsStatePrivateV2{outputs: outputs, source: source, additional: append([]*publication.ObservedRoot(nil), additional...)}
	s.self = s
	return ProducerPacketRootsPrivateV2{s}, nil
}
func (ProducerPacketRootsPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (r ProducerPacketRootsPrivateV2) check(ctx context.Context, f FrozenScopePrivateV2) error {
	if ctx == nil || r.state == nil || r.state.self != r.state || !f.valid() {
		return ErrInvalid
	}
	path, err := r.state.source.PathPrivateV2()
	if err != nil {
		return err
	}
	dev, ino, err := r.state.source.IdentityPrivateV2()
	if err != nil {
		return err
	}
	if path != f.state.spec.Root || dev != f.state.rootIdentity.Device || ino != f.state.rootIdentity.Inode {
		return ErrInvalid
	}
	return publication.CheckProducerPacketRetentionTopologyPrivateV2(ctx, r.state.outputs, r.state.source, r.state.additional)
}

type ProducerPacketCopyReceiptPrivateV2 struct {
	Outcome string                        `json:"outcome"`
	Receipt *publication.BoundFileReceipt `json:"receipt,omitempty"`
}
type ProducerPacketRetentionOutcomePrivateV2 struct {
	state *producerPacketRetentionOutcomeStatePrivateV2
}
type producerPacketRetentionOutcomeStatePrivateV2 struct {
	self   *producerPacketRetentionOutcomeStatePrivateV2
	phase  string
	copies [2]ProducerPacketCopyReceiptPrivateV2
	budget ProducerPacketBudgetPrivateV2
	pair   VerifiedProducerPacketPairPrivateV2
}

func (ProducerPacketRetentionOutcomePrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }

func (o ProducerPacketRetentionOutcomePrivateV2) Phase() string {
	if o.state == nil || o.state.self != o.state {
		return "NOT_ATTEMPTED"
	}
	return o.state.phase
}
func (o ProducerPacketRetentionOutcomePrivateV2) BudgetSpent() ProducerPacketBudgetPrivateV2 {
	if o.state == nil || o.state.self != o.state {
		return ProducerPacketBudgetPrivateV2{}
	}
	return o.state.budget
}
func (o ProducerPacketRetentionOutcomePrivateV2) CopyReceipts() [2]ProducerPacketCopyReceiptPrivateV2 {
	if o.state == nil || o.state.self != o.state {
		return [2]ProducerPacketCopyReceiptPrivateV2{}
	}
	out := o.state.copies
	for i := range out {
		if out[i].Receipt != nil {
			r := *out[i].Receipt
			out[i].Receipt = &r
		}
	}
	return out
}
func (o ProducerPacketRetentionOutcomePrivateV2) VerifiedPair() VerifiedProducerPacketPairPrivateV2 {
	if o.state == nil || o.state.self != o.state {
		return VerifiedProducerPacketPairPrivateV2{}
	}
	return o.state.pair
}

type VerifiedProducerPacketPairPrivateV2 struct {
	state *verifiedProducerPacketPairStatePrivateV2
}
type verifiedProducerPacketPairStatePrivateV2 struct {
	self     *verifiedProducerPacketPairStatePrivateV2
	raw      []byte
	digest   string
	frozen   FrozenScopePrivateV2
	origin   EnumerationOriginPrivateV2
	captures []OpaqueSuccessfulV2Capture
	roots    ProducerPacketRootsPrivateV2
	receipts [2]ProducerPacketCopyReceiptPrivateV2
}

func (VerifiedProducerPacketPairPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (v VerifiedProducerPacketPairPrivateV2) valid() bool {
	return v.state != nil && v.state.self == v.state && len(v.state.raw) > 0 && len(v.state.raw) <= producerPacketBytesPrivateV2
}
func producerPacketRetentionOpsPrivateV2() scopeRetentionOps {
	return scopeRetentionOps{
		publish: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
			return r.PublishProducerPacketPrivateV2(ctx, id, raw)
		},
		read: func(ctx context.Context, r *publication.ContentRetentionRoot, id string, limit int64) ([]byte, error) {
			return r.ReadProducerPacketPrivateV2(ctx, id, limit)
		},
	}
}
func RetainCompleteScopePrivateV2(ctx context.Context, f FrozenScopePrivateV2, o EnumerationOriginPrivateV2, captures []OpaqueSuccessfulV2Capture, roots ProducerPacketRootsPrivateV2, limits ...ProducerPacketLimitsPrivateV2) (ProducerPacketRetentionOutcomePrivateV2, error) {
	return retainCompleteScopePrivateV2(ctx, f, o, captures, roots, limits, producerPacketRetentionOpsPrivateV2())
}
func retainCompleteScopePrivateV2(ctx context.Context, f FrozenScopePrivateV2, o EnumerationOriginPrivateV2, captures []OpaqueSuccessfulV2Capture, roots ProducerPacketRootsPrivateV2, limits []ProducerPacketLimitsPrivateV2, ops scopeRetentionOps) (out ProducerPacketRetentionOutcomePrivateV2, err error) {
	s := &producerPacketRetentionOutcomeStatePrivateV2{phase: "PREFLIGHT", copies: [2]ProducerPacketCopyReceiptPrivateV2{{Outcome: "NOT_ATTEMPTED"}, {Outcome: "NOT_ATTEMPTED"}}}
	s.self = s
	out.state = s
	budget, err := newProducerPacketBudgetPrivateV2(limits)
	if err != nil {
		if err == ErrProducerPacketOverBoundPrivateV2 {
			s.phase = "NOT_RETAINED_OVER_BOUND"
		}
		return out, err
	}
	defer func() { s.budget = budget.spent }()
	if ctx == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	// Build checks complete actual lineage before content cloning or storage I/O.
	raw, err := buildProducerPacketPrivateV2(ctx, f, o, captures, budget)
	if err != nil {
		if err == ErrProducerPacketOverBoundPrivateV2 {
			s.phase = "NOT_RETAINED_OVER_BOUND"
		}
		return out, err
	}
	// Reserve six shared-algorithm read buffers, two pair publication clones,
	// two publisher expectation clones, four publication verifier read buffers,
	// and two spare verifier snapshots (16 logical passes). The 32 read/hash
	// passes cover disk reads, exact comparisons, verifier scans and commitments.
	// Reservations precede storage, including the conservative limit+1 sentinels.
	// This is cumulative logical work, not peak heap/CPU metering; even a packet
	// below 16 MiB can exhaust the separate 64 MiB logical profile. No increase.
	if err = budget.passes(uint64(len(raw)), 16, 32, producerPacketTopologyWorkPrivateV2+16); err != nil {
		s.phase = "NOT_RETAINED_OVER_BOUND"
		return out, err
	}
	if err = budget.reserve(16, 32, 0); err != nil {
		s.phase = "NOT_RETAINED_OVER_BOUND"
		return out, err
	}
	if err = roots.check(ctx, f); err != nil {
		return out, err
	}
	digest := Digest(raw)
	attempt, err := retainScopePairBytes(ctx, scopeRetentionObject{digest, raw}, digest, 0, roots.state.outputs[0], roots.state.outputs[1], ops)
	s.phase = attempt.phase
	s.copies = [2]ProducerPacketCopyReceiptPrivateV2{{attempt.primary.outcome, attempt.primary.receipt}, {attempt.backup.outcome, attempt.backup.receipt}}
	if err != nil {
		return out, err
	}
	// Do not treat the common algorithm's legacy verification token as this seal.
	if err = publication.CheckProducerPacketObjectsDistinctPrivateV2(ctx, roots.state.outputs, digest); err != nil {
		s.phase = "FINAL_COPY_IDENTITY_REFUSED_PRIVATE_V2"
		return out, err
	}
	if err = roots.check(ctx, f); err != nil {
		s.phase = "FINAL_TOPOLOGY_REFUSED_PRIVATE_V2"
		return out, err
	}
	if err = ctx.Err(); err != nil {
		s.phase = "FINAL_CONTEXT_REFUSED_PRIVATE_V2"
		return out, err
	}
	ownedCaptures := append([]OpaqueSuccessfulV2Capture(nil), captures...)
	st := &verifiedProducerPacketPairStatePrivateV2{raw: raw, digest: digest, frozen: f, origin: o, captures: ownedCaptures, roots: roots, receipts: out.CopyReceipts()}
	st.self = st
	s.pair = VerifiedProducerPacketPairPrivateV2{st}
	s.phase = "COMPLETE_PACKET_PAIR_VERIFIED_PRIVATE_V2"
	return out, nil
}
func packetMatchesProducerLineagePrivateV2(p ProducerPacketInspectionPrivateV2, v VerifiedProducerPacketPairPrivateV2) bool {
	if !v.valid() {
		return false
	}
	s := v.state
	f, o := s.frozen, s.origin
	if !f.valid() || !o.valid() || o.state.frozen.state != f.state || len(p.Batches) != len(s.captures) || len(p.Files) != len(f.state.files) {
		return false
	}
	if !bytes.Equal(p.FrozenSpec, f.state.specBytes) || !bytes.Equal(p.FrozenManifest, f.state.manifest) || !bytes.Equal(p.NominationSelection, f.state.selection) || !bytes.Equal(p.FilePlan, f.state.filePlan) || !bytes.Equal(p.EnumerationOrigin, o.state.raw) {
		return false
	}
	policy, _ := json.Marshal(f.state.spec.EffectivePolicy)
	meta, _ := json.Marshal(o.state.adapter.metadata)
	if !bytes.Equal(p.EffectivePolicy, policy) || !bytes.Equal(p.RuntimeMetadata, meta) {
		return false
	}
	for i, file := range p.Files {
		if file.Record != f.state.files[i].scopeManifestFile || !bytes.Equal(file.Body, f.state.files[i].Bytes) {
			return false
		}
	}
	seen := make([]bool, len(p.Batches))
	for _, capture := range s.captures {
		if !successfulProducerLineagePrivateV2(f, o, capture) {
			return false
		}
		c := capture.state
		if seen[c.ordinal] {
			return false
		}
		seen[c.ordinal] = true
		b := p.Batches[c.ordinal]
		if b.Ordinal != c.ordinal || !bytes.Equal(b.Seeds, c.seeds) || !bytes.Equal(b.Origin, c.origin) || !bytes.Equal(b.SourceArtifact, c.artifact) || !bytes.Equal(b.ManifestInput, o.state.batches[c.ordinal].binding.Input) || !b.AttachedObserved || !b.ReleasedObserved {
			return false
		}
	}
	for _, ok := range seen {
		if !ok {
			return false
		}
	}
	return true
}
