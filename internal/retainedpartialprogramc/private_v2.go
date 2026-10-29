package retainedpartialprogramc

// V2 is an additive private computation edge. Original V3 acquisition bytes are
// retained separately from the derived canonical V5 serialization. Neither is an
// ordinary admission, authenticated history, AST association, or prepared source.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/programc"
	"lsp-trace/sessionruntime"
)

const ProfilePrivateV2 = "DERIVED_RETAINED_PARTIAL_CALLS_PRIVATE_V2"
const ProfileDefinitionPrivateV2 = "lsp-trace:DERIVED_RETAINED_PARTIAL_CALLS_PRIVATE_V2;sealed-qualified-producer-packet-v2;per-batch-only;original-native-v3;derived-canonical-v5;packet<=16777216;artifact<=4194304;metadata<=131072;logical<=67108864;read-hash<=268435456;work<=1000000;members<=16;nomination=1..8;context=0..8;targets=1..63;batches=1..63;directed-unit-occurrences;typed-zero-site-relations=zero-occurrences;leiden-resolution-1;canonical-communities-v1;authority=0;accepted=false;partial=true;source=UNKNOWN;feature=UNRESOLVED;absence=PROHIBITED;no-replay;no-original-native-v5-identity;ordinary-custody=UNAVAILABLE;history=NOT_AUTHORITY"
const ClaimCeilingPrivateV2 = "Private derived per-batch structural CALLS communities only. One independent source; authority=0, accepted=false, partial=true, source completeness=UNKNOWN, feature identity=UNRESOLVED, absence inference=PROHIBITED, canonical complete=false, replay=false, original NativeV5 identity=false. No feature confidence, semantic purpose, runtime execution, AST/full-definition association, ordinary custody, authenticated history, permission, or production authority."

func ProfileDigestPrivateV2() string { return pilot.Digest([]byte(ProfileDefinitionPrivateV2)) }

// No legacy Manifest, ordinary SourceBinding, or V1 conversion exists here.
type SourceBindingPrivateV2 struct {
	Profile, ProfileDigest, ClaimCeiling, ComputationAdmission                                          string
	PacketSHA256, QualificationReportSHA256                                                             string
	BatchOrdinal                                                                                        int
	CanonicalSeedsSHA256, CaptureOriginSHA256, SourceArtifactSHA256                                     string
	OriginalNativeV3SHA256, DerivedCanonicalV5SHA256                                                    string
	SourcePolicySHA256, RuntimeMetadataSHA256                                                           string
	SnapshotTargetCount, BatchTargetCount, SourceMemberCount, NodeCount, RelationCount, OccurrenceCount int
	WorkspaceURI, SessionID                                                                             string
	Generation                                                                                          uint64
	ProviderName, ProviderVersion, ProviderCommand, PositionEncoding                                    string
	GraphWideCustodyStatus, OrdinaryQualificationReceiptStatus, OriginalHistoryStatus                   string
	SourceCompleteness, FeatureIdentity, AbsenceInference                                               string
	Authority                                                                                           int
	Accepted, Partial, CanonicalComplete, ReplayAdmitted, OriginalNativeV5IdentityClaimed               bool
	OriginalV3SemanticReceipt, DerivedV5SemanticReceipt                                                 programc.SemanticReceipt
	PublicClaims                                                                                        int
}

// Detached evidence is inspection only. Packet includes the exact declaration
// metadata and owned file bodies; SourceArtifact includes actual source capture
// statuses, native identifiers, kinds and ranges, not self-declared AST nodes.
// Reading this DTO cannot construct a projection or any producer capability.
type ProjectionEvidencePrivateV2 struct {
	Source                                                                     SourceBindingPrivateV2
	Packet, QualificationReport, CanonicalSeeds, CaptureOrigin, SourceArtifact []byte
	OriginalNativeV3, DerivedCanonicalV5                                       []byte
}
type ProjectionPrivateV2 struct{ state *projectionStatePrivateV2 }
type projectionStatePrivateV2 struct {
	self     *projectionStatePrivateV2
	math     programc.CallsMathPrivate
	evidence ProjectionEvidencePrivateV2
	pin      string
}

func (p ProjectionPrivateV2) valid() bool {
	return p.state != nil && p.state.self == p.state && p.state.pin == instabilityDigest(p.state.evidence)
}
func cloneProjectionEvidencePrivateV2(e ProjectionEvidencePrivateV2) ProjectionEvidencePrivateV2 {
	e.Packet = bytes.Clone(e.Packet)
	e.QualificationReport = bytes.Clone(e.QualificationReport)
	e.CanonicalSeeds = bytes.Clone(e.CanonicalSeeds)
	e.CaptureOrigin = bytes.Clone(e.CaptureOrigin)
	e.SourceArtifact = bytes.Clone(e.SourceArtifact)
	e.OriginalNativeV3 = bytes.Clone(e.OriginalNativeV3)
	e.DerivedCanonicalV5 = bytes.Clone(e.DerivedCanonicalV5)
	return e
}
func (p ProjectionPrivateV2) Math() programc.CallsMathPrivate {
	if !p.valid() {
		return programc.CallsMathPrivate{}
	}
	return programc.CloneCallsMathPrivate(p.state.math)
}
func (p ProjectionPrivateV2) Source() SourceBindingPrivateV2 {
	if !p.valid() {
		return SourceBindingPrivateV2{}
	}
	return p.state.evidence.Source
}

// PreparationEvidencePrivateV2 is only a read-only future-owner handoff. It does
// not prepare, acquire documents, use a Manager/provider, or inspect any root.
func (p ProjectionPrivateV2) PreparationEvidencePrivateV2() (ProjectionEvidencePrivateV2, error) {
	if !p.valid() {
		return ProjectionEvidencePrivateV2{}, fmt.Errorf("sealed V2 projection required")
	}
	return cloneProjectionEvidencePrivateV2(p.state.evidence), nil
}
func (ProjectionPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque V2 projection")
}

// Concrete producer proof FIRST. DTOs, raw packets and lower math do not enter.
func ProjectPrivateV2(ctx context.Context, proof pilot.DerivedProducerPacketPrivateV2, batchOrdinal int) (ProjectionPrivateV2, *programc.Failure) {
	view, err := pilot.ReadDerivedProducerPacketPrivateV2(ctx, proof)
	if err != nil {
		return ProjectionPrivateV2{}, invalid("V2 sealed qualified packet required: " + err.Error())
	}
	packet, err := pilot.PureParseProducerPacketPrivateV2(view.Packet)
	if err != nil {
		return ProjectionPrivateV2{}, invalid(err.Error())
	}
	if batchOrdinal < 0 || batchOrdinal > 63 || batchOrdinal >= len(view.Batches) || len(view.Batches) != len(packet.Batches) || view.PacketSHA256 != pilot.Digest(view.Packet) || view.ReportSHA256 != pilot.Digest(view.QualificationReport) {
		return ProjectionPrivateV2{}, invalid("V2 packet/ordinal pins")
	}
	var report pilot.ProducerPacketQualificationReportPrivateV2
	if json.Unmarshal(view.QualificationReport, &report) != nil || report.PacketSHA256 != view.PacketSHA256 || report.PacketByteLength != uint64(len(view.Packet)) || report.BatchCount != len(packet.Batches) || report.Status != "PRIVATE_PACKET_QUALIFIED_ONLY" || report.Authority != 0 || report.Accepted || report.Feature != "UNRESOLVED" || report.Completeness != "UNKNOWN" || report.Absence != "PROHIBITED" {
		return ProjectionPrivateV2{}, invalid("V2 qualification report pins")
	}
	batch := view.Batches[batchOrdinal]
	own := packet.Batches[batchOrdinal]
	if batch.Ordinal != batchOrdinal || own.Ordinal != batchOrdinal || !bytes.Equal(batch.CanonicalSeeds, own.Seeds) || !bytes.Equal(batch.CaptureOrigin, own.Origin) || !bytes.Equal(batch.SourceArtifact, own.SourceArtifact) {
		return ProjectionPrivateV2{}, invalid("V2 own batch pins")
	}
	source, err := graphprovenance.ParseSourceWithholdingPrivateV2(batch.SourceArtifact)
	if err != nil || !bytes.Equal(batch.NativeGraph, source.Evidence.GraphBytes) {
		return ProjectionPrivateV2{}, invalid("V2 original source/graph pins")
	}
	var meta sessionruntime.SessionMetadata
	if json.Unmarshal(packet.RuntimeMetadata, &meta) != nil || meta.ServerCommand == "" {
		return ProjectionPrivateV2{}, invalid("V2 original provider metadata")
	}
	// This neutral helper is used only AFTER new sealed V2 qualification. It
	// preserves validated native process metadata and establishes a NEW receipt.
	derived, err := graph.ProjectRetainedV3ToV5Private(batch.NativeGraph, source.Policy.WorkspaceURI, meta.ServerCommand)
	if err != nil {
		return ProjectionPrivateV2{}, invalid(err.Error())
	}
	math, f := programc.ProjectNativeCallsMathPrivateV2(derived)
	if f != nil {
		return ProjectionPrivateV2{}, f
	}
	for _, o := range math.Occurrences {
		if o.Provider != meta.ServerCommand {
			return ProjectionPrivateV2{}, invalid("V2 occurrence provider")
		}
	}
	// SemanticReceipt uses exported field names, while native receipts use wire names.
	var native, originalNative struct {
		TraceReceipt struct {
			ReceiptVersion           string `json:"receipt_version"`
			SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
			DigestScope              string `json:"digest_scope"`
		} `json:"trace_receipt"`
	}
	if json.Unmarshal(derived, &native) != nil || json.Unmarshal(batch.NativeGraph, &originalNative) != nil || native.TraceReceipt.SemanticCommitmentDigest == "" || originalNative.TraceReceipt.SemanticCommitmentDigest == "" || originalNative.TraceReceipt.SemanticCommitmentDigest == native.TraceReceipt.SemanticCommitmentDigest {
		return ProjectionPrivateV2{}, invalid("V2 derived semantic receipt")
	}
	receipt := programc.SemanticReceipt{ReceiptVersion: native.TraceReceipt.ReceiptVersion, SemanticCommitmentDigest: native.TraceReceipt.SemanticCommitmentDigest, DigestScope: native.TraceReceipt.DigestScope}
	originalReceipt := programc.SemanticReceipt{ReceiptVersion: originalNative.TraceReceipt.ReceiptVersion, SemanticCommitmentDigest: originalNative.TraceReceipt.SemanticCommitmentDigest, DigestScope: originalNative.TraceReceipt.DigestScope}
	var origin struct {
		Enumeration json.RawMessage `json:"actual_enumeration"`
	}
	var discovery censusacquisition.Discovery
	if json.Unmarshal(packet.EnumerationOrigin, &origin) != nil || json.Unmarshal(origin.Enumeration, &discovery) != nil {
		return ProjectionPrivateV2{}, invalid("V2 enumeration metadata")
	}
	policy, _ := json.Marshal(source.Policy)
	s := SourceBindingPrivateV2{
		Profile: ProfilePrivateV2, ProfileDigest: ProfileDigestPrivateV2(),
		ClaimCeiling: ClaimCeilingPrivateV2, ComputationAdmission: Admission,
		PacketSHA256: view.PacketSHA256, QualificationReportSHA256: view.ReportSHA256,
		BatchOrdinal:         batchOrdinal,
		CanonicalSeedsSHA256: pilot.Digest(batch.CanonicalSeeds), CaptureOriginSHA256: pilot.Digest(batch.CaptureOrigin),
		SourceArtifactSHA256: pilot.Digest(batch.SourceArtifact), OriginalNativeV3SHA256: pilot.Digest(batch.NativeGraph),
		DerivedCanonicalV5SHA256: pilot.Digest(derived), SourcePolicySHA256: pilot.Digest(policy),
		RuntimeMetadataSHA256: pilot.Digest(packet.RuntimeMetadata),
		SnapshotTargetCount:   len(discovery.Targets), BatchTargetCount: len(source.Evidence.Acquisition.Targets),
		SourceMemberCount: len(source.Policy.Members), NodeCount: len(math.NodeIdentities),
		RelationCount: len(source.Evidence.Acquisition.Graph.Edges), OccurrenceCount: len(math.Occurrences),
		WorkspaceURI: source.Policy.WorkspaceURI, SessionID: source.Policy.SessionID, Generation: source.Policy.Generation,
		ProviderName: meta.ProviderName, ProviderVersion: meta.ProviderVersion,
		ProviderCommand: meta.ServerCommand, PositionEncoding: meta.PositionEncoding,
		GraphWideCustodyStatus: "UNAVAILABLE", OrdinaryQualificationReceiptStatus: "UNAVAILABLE",
		OriginalHistoryStatus: "ORIGINAL_OBSERVATION_NOT_AUTHORITY",
		SourceCompleteness:    "UNKNOWN", FeatureIdentity: "UNRESOLVED", AbsenceInference: "PROHIBITED", Partial: true,
		OriginalV3SemanticReceipt: originalReceipt, DerivedV5SemanticReceipt: receipt,
	}
	e := cloneProjectionEvidencePrivateV2(ProjectionEvidencePrivateV2{s, view.Packet, view.QualificationReport, batch.CanonicalSeeds, batch.CaptureOrigin, batch.SourceArtifact, batch.NativeGraph, derived})
	if err = ctx.Err(); err != nil {
		return ProjectionPrivateV2{}, invalid(err.Error())
	}
	st := &projectionStatePrivateV2{math: programc.CloneCallsMathPrivate(math), evidence: e, pin: instabilityDigest(e)}
	st.self = st
	return ProjectionPrivateV2{st}, nil
}

type OutcomePrivateV2 struct {
	ProfileID, ProfileDigest, ClaimCeiling, ComputationAdmission string
	Math                                                         programc.PartitionMathPrivate
	Projection                                                   programc.CallsMathPrivate
	Source                                                       SourceBindingPrivateV2
}

func CloneOutcomePrivateV2(o OutcomePrivateV2) OutcomePrivateV2 {
	o.Math = programc.ClonePartitionMathPrivate(o.Math)
	o.Projection = programc.CloneCallsMathPrivate(o.Projection)
	return o
}
func ComputeProjectionPrivateV2(p ProjectionPrivateV2, seed uint64) (OutcomePrivateV2, *programc.Failure) {
	if !p.valid() {
		return OutcomePrivateV2{}, invalid("sealed V2 projection required")
	}
	m, f := programc.PartitionCallsMathPrivate(p.state.math, seed)
	if f != nil {
		return OutcomePrivateV2{}, f
	}
	return OutcomePrivateV2{ProfilePrivateV2, ProfileDigestPrivateV2(), ClaimCeilingPrivateV2, Admission, m, p.Math(), p.Source()}, nil
}
