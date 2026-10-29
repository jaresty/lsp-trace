package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
)

type ProducerPacketQualificationReportPrivateV2 struct {
	CopyObservations        [2]publication.ProducerPacketCopyObservationPrivateV2 `json:"copy_observations"`
	DurabilityQualification string                                                `json:"durability_qualification"`
	ProtectionCoverage      string                                                `json:"protection_coverage"`
	ProfilePolicy           string                                                `json:"profile_policy"`
	PacketByteLimit         uint64                                                `json:"packet_byte_limit"`
	SourceArtifactByteLimit uint64                                                `json:"source_artifact_byte_limit"`
	MetadataByteLimit       uint64                                                `json:"metadata_byte_limit"`
	Profile                 string                                                `json:"profile"`
	PacketSHA256            string                                                `json:"packet_sha256"`
	PacketByteLength        uint64                                                `json:"packet_byte_length"`
	Status                  string                                                `json:"status"`
	Authority               uint8                                                 `json:"authority"`
	Accepted                bool                                                  `json:"accepted"`
	Feature                 string                                                `json:"feature_identity"`
	Completeness            string                                                `json:"source_completeness"`
	Absence                 string                                                `json:"absence_inference"`
	Custody                 string                                                `json:"paired_root_custody"`
	ProducerLineage         string                                                `json:"producer_lineage"`
	SourceRootClaim         string                                                `json:"source_root_claim"`
	History                 string                                                `json:"runtime_history"`
	WholeOriginCoverage     string                                                `json:"whole_origin_batch_coverage"`
	PolicySHA256            string                                                `json:"profile_policy_sha256"`
	Roles                   []ProducerPacketRolePrivateV2                         `json:"ordered_roles"`
	BatchCount              int                                                   `json:"batch_count"`
	SourceSpecSHA256        string                                                `json:"source_spec_sha256"`
	ScopeSHA256             string                                                `json:"scope_sha256"`
	BudgetSpent             ProducerPacketBudgetPrivateV2                         `json:"budget_spent"`
	Limits                  ProducerPacketLimitsPrivateV2                         `json:"verification_limits"`
	Copies                  [2]ProducerPacketCopyReceiptPrivateV2                 `json:"copy_receipts"`
}
type QualifiedProducerPacketPrivateV2 struct {
	state *qualifiedProducerPacketStatePrivateV2
}
type qualifiedProducerPacketStatePrivateV2 struct {
	self                 *qualifiedProducerPacketStatePrivateV2
	pair                 VerifiedProducerPacketPairPrivateV2
	raw, report          []byte
	digest, reportDigest string
}

func (QualifiedProducerPacketPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (q QualifiedProducerPacketPrivateV2) valid() bool {
	return q.state != nil && q.state.self == q.state && q.state.pair.valid() && q.state.digest == q.state.pair.state.digest && len(q.state.raw) > 0 && len(q.state.raw) <= producerPacketBytesPrivateV2 && len(q.state.report) > 0 && len(q.state.report) <= scopeMetadataPrivateV2
}

func checkQualifiedPacketBindingPrivateV2(q QualifiedProducerPacketPrivateV2, b *producerPacketBudgetPrivateV2) error {
	if !q.valid() {
		return ErrInvalid
	}
	if err := b.passes(uint64(len(q.state.raw)), 0, 3, 1); err != nil {
		return err
	}
	if err := b.passes(uint64(len(q.state.report)), 0, 1, 1); err != nil {
		return err
	}
	if !bytes.Equal(q.state.raw, q.state.pair.state.raw) || Digest(q.state.raw) != q.state.digest || Digest(q.state.report) != q.state.reportDigest {
		return ErrInvalid
	}
	return nil
}

// Independent qualification reopens BOTH pinned roots. It never consults a live
// Manager or source body, and never replays acquisition after historical expiry.
func QualifyProducerPacketPrivateV2(ctx context.Context, pair VerifiedProducerPacketPairPrivateV2) (QualifiedProducerPacketPrivateV2, error) {
	return qualifyProducerPacketPrivateV2(ctx, pair, producerPacketRetentionOpsPrivateV2())
}
func qualifyProducerPacketPrivateV2(ctx context.Context, pair VerifiedProducerPacketPairPrivateV2, ops scopeRetentionOps) (QualifiedProducerPacketPrivateV2, error) {
	if ctx == nil || !pair.valid() || ops.read == nil {
		return QualifiedProducerPacketPrivateV2{}, ErrInvalid
	}
	s := pair.state
	if err := ctx.Err(); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	if err := b.passes(uint64(len(s.raw)), 2, 8, producerPacketTopologyWorkPrivateV2+8); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	if err := s.roots.check(ctx, s.frozen); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	var read [2][]byte
	for i, r := range s.roots.state.outputs {
		raw, err := ops.read(ctx, r, s.digest, int64(len(s.raw)))
		if err != nil {
			return QualifiedProducerPacketPrivateV2{}, err
		}
		if len(raw) != len(s.raw) || Digest(raw) != s.digest || !bytes.Equal(raw, s.raw) {
			return QualifiedProducerPacketPrivateV2{}, ErrInvalid
		}
		read[i] = raw
	}
	if !bytes.Equal(read[0], read[1]) {
		return QualifiedProducerPacketPrivateV2{}, ErrInvalid
	}
	custody, err := publication.ObserveProducerPacketPairCustodyPrivateV2(ctx, s.roots.state.outputs, s.digest)
	if err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	for _, copy := range custody {
		if copy.ObjectByteLength != uint64(len(s.raw)) || copy.Selector != s.digest {
			return QualifiedProducerPacketPrivateV2{}, ErrInvalid
		}
	}
	p, err := parseProducerPacketPrivateV2(read[0], b)
	if err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	if !packetMatchesProducerLineagePrivateV2(p, pair) {
		return QualifiedProducerPacketPrivateV2{}, ErrInvalid
	}
	if err = b.passes(scopeMetadataPrivateV2, 2, 4, 256); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	report := ProducerPacketQualificationReportPrivateV2{CopyObservations: custody, DurabilityQualification: "DIRECTORY_SYNC_RECEIPTS_ONLY_NO_POWER_LOSS_GUARANTEE_NO_ATOMIC_PAIR", ProtectionCoverage: "REQUIRED_FROZEN_SOURCE_PLUS_SUPPLIED_OPTIONAL_ROOTS_NOT_COMPREHENSIVE_MONITORING", ProfilePolicy: producerPacketPolicyPrivateV2, PacketByteLimit: producerPacketBytesPrivateV2, SourceArtifactByteLimit: producerPacketArtifactBytesPrivateV2, MetadataByteLimit: scopeMetadataPrivateV2, Profile: ProducerPacketProfilePrivateV2, PacketSHA256: s.digest, PacketByteLength: uint64(len(s.raw)), Status: "PRIVATE_PACKET_QUALIFIED_ONLY", Feature: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Custody: "TWO_PINNED_ROOTS_FRESH_REOPEN_DISTINCT_DIRECTORIES_AND_OBJECT_INODES", ProducerLineage: "ORIGINAL_OPAQUE_FROZEN_ENUMERATION_AND_SUCCESS_STATES", SourceRootClaim: sourceRootNoAtomicSnapshotPrivateV2, History: "NOT_AUTHENTICATED_NO_ANALYZED_SOURCE_CONFIDENCE", WholeOriginCoverage: "ALL_ORIGIN_BATCHES_NOT_SOURCE_COMPLETENESS", PolicySHA256: p.PolicySHA256, Roles: append([]ProducerPacketRolePrivateV2(nil), p.Roles...), BatchCount: len(p.Batches), SourceSpecSHA256: Digest(p.FrozenSpec), ScopeSHA256: Digest(p.FrozenManifest), BudgetSpent: b.spent, Limits: ProducerPacketLimitsPrivateV2{producerPacketLogicalPrivateV2, producerPacketReadHashPrivateV2, producerPacketWorkPrivateV2}, Copies: copyProducerPacketReceiptsPrivateV2(s.receipts)}
	reportRaw, err := json.Marshal(report)
	if err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	if len(reportRaw) > scopeMetadataPrivateV2 {
		return QualifiedProducerPacketPrivateV2{}, ErrProducerPacketOverBoundPrivateV2
	}
	finalCustody, err := publication.ObserveProducerPacketPairCustodyPrivateV2(ctx, s.roots.state.outputs, s.digest)
	if err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	if finalCustody != custody {
		return QualifiedProducerPacketPrivateV2{}, ErrInvalid
	}
	if err = s.roots.check(ctx, s.frozen); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	if err = ctx.Err(); err != nil {
		return QualifiedProducerPacketPrivateV2{}, err
	}
	st := &qualifiedProducerPacketStatePrivateV2{pair: pair, raw: read[0], report: reportRaw, digest: s.digest, reportDigest: Digest(reportRaw)}
	st.self = st
	return QualifiedProducerPacketPrivateV2{st}, nil
}
func copyProducerPacketReceiptsPrivateV2(in [2]ProducerPacketCopyReceiptPrivateV2) [2]ProducerPacketCopyReceiptPrivateV2 {
	for i := range in {
		if in[i].Receipt != nil {
			r := *in[i].Receipt
			in[i].Receipt = &r
		}
	}
	return in
}

type DerivedProducerPacketPrivateV2 struct {
	state *derivedProducerPacketStatePrivateV2
}
type derivedProducerPacketStatePrivateV2 struct {
	self      *derivedProducerPacketStatePrivateV2
	qualified QualifiedProducerPacketPrivateV2
	batches   []ProducerPacketBatchEvidencePrivateV2
}

// Detached per-batch evidence for the FUTURE ProjectV2 owner. No graph merge,
// ordinary NativeV5 admission, feature acceptance or source-completeness claim.
type ProducerPacketBatchEvidencePrivateV2 struct {
	Ordinal                                                    int
	CanonicalSeeds, CaptureOrigin, SourceArtifact, NativeGraph []byte
}
type ProducerPacketDerivedViewPrivateV2 struct {
	Packet, QualificationReport []byte
	PacketSHA256, ReportSHA256  string
	Batches                     []ProducerPacketBatchEvidencePrivateV2
}

func (DerivedProducerPacketPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func DeriveProducerPacketPrivateV2(ctx context.Context, q QualifiedProducerPacketPrivateV2) (DerivedProducerPacketPrivateV2, error) {
	if ctx == nil || !q.valid() {
		return DerivedProducerPacketPrivateV2{}, ErrInvalid
	}
	pair := q.state.pair
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	if err := b.reserve(0, 0, producerPacketTopologyWorkPrivateV2); err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	if err := checkQualifiedPacketBindingPrivateV2(q, b); err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	p, err := parseProducerPacketPrivateV2(q.state.raw, b)
	if err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	if Digest(q.state.raw) != q.state.digest || !packetMatchesProducerLineagePrivateV2(p, pair) {
		return DerivedProducerPacketPrivateV2{}, ErrInvalid
	}
	st := &derivedProducerPacketStatePrivateV2{qualified: q}
	for _, batch := range p.Batches {
		if err = b.passes(uint64(len(batch.SourceArtifact)), 2, 24, 4096); err != nil {
			return DerivedProducerPacketPrivateV2{}, err
		}
		evidence, e := graphprovenance.ParseSourceWithholdingPrivateV2(batch.SourceArtifact)
		if e != nil {
			return DerivedProducerPacketPrivateV2{}, e
		}
		st.batches = append(st.batches, ProducerPacketBatchEvidencePrivateV2{batch.Ordinal, batch.Seeds, batch.Origin, batch.SourceArtifact, evidence.Evidence.GraphBytes})
	}
	if err = pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	if err = ctx.Err(); err != nil {
		return DerivedProducerPacketPrivateV2{}, err
	}
	st.self = st
	return DerivedProducerPacketPrivateV2{st}, nil
}
func ReadDerivedProducerPacketPrivateV2(ctx context.Context, d DerivedProducerPacketPrivateV2) (ProducerPacketDerivedViewPrivateV2, error) {
	var out ProducerPacketDerivedViewPrivateV2
	if ctx == nil || d.state == nil || d.state.self != d.state || !d.state.qualified.valid() {
		return out, ErrInvalid
	}
	q := d.state.qualified
	pair := q.state.pair
	budget, _ := newProducerPacketBudgetPrivateV2(nil)
	if err := budget.reserve(0, 0, producerPacketTopologyWorkPrivateV2); err != nil {
		return out, err
	}
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return out, err
	}
	if err := checkQualifiedPacketBindingPrivateV2(q, budget); err != nil {
		return out, err
	}
	p, err := parseProducerPacketPrivateV2(q.state.raw, budget)
	if err != nil {
		return out, err
	}
	if Digest(q.state.raw) != q.state.digest || !packetMatchesProducerLineagePrivateV2(p, pair) || len(p.Batches) != len(d.state.batches) {
		return out, ErrInvalid
	}
	if err = budget.passes(uint64(len(q.state.raw)), 1, 1, 1); err != nil {
		return out, err
	}
	if err = budget.passes(uint64(len(q.state.report)), 1, 0, 1); err != nil {
		return out, err
	}
	out.Packet = bytes.Clone(q.state.raw)
	out.QualificationReport = bytes.Clone(q.state.report)
	out.PacketSHA256 = q.state.digest
	out.ReportSHA256 = q.state.reportDigest
	for i, batch := range d.state.batches {
		if batch.Ordinal != i || !bytes.Equal(batch.SourceArtifact, p.Batches[i].SourceArtifact) || !bytes.Equal(batch.CanonicalSeeds, p.Batches[i].Seeds) || !bytes.Equal(batch.CaptureOrigin, p.Batches[i].Origin) {
			return ProducerPacketDerivedViewPrivateV2{}, ErrInvalid
		}
		for _, raw := range [][]byte{batch.CanonicalSeeds, batch.CaptureOrigin, batch.SourceArtifact, batch.NativeGraph} {
			if err = budget.passes(uint64(len(raw)), 1, 1, 1); err != nil {
				return ProducerPacketDerivedViewPrivateV2{}, err
			}
		}
		out.Batches = append(out.Batches, ProducerPacketBatchEvidencePrivateV2{batch.Ordinal, bytes.Clone(batch.CanonicalSeeds), bytes.Clone(batch.CaptureOrigin), bytes.Clone(batch.SourceArtifact), bytes.Clone(batch.NativeGraph)})
	}
	if err = pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return ProducerPacketDerivedViewPrivateV2{}, err
	}
	if err = ctx.Err(); err != nil {
		return ProducerPacketDerivedViewPrivateV2{}, err
	}
	return out, nil
}
