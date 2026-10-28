// Package retainedpartialprogramc is the explicitly private adapter above the
// retained producer and Program C math. Neither lower package imports it.
// It is not ordinary Graph Provenance, Census admission, replay, or publication.
package retainedpartialprogramc

import (
	"bytes"
	"encoding/json"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/programc"
)

const (
	ProfileID = "DERIVED_RETAINED_PARTIAL_CALLS_PRIVATE_V1"
	// SHA256 of the exact ProfileDefinition below; ordinary calls-v1 pins do not change.
	ProfileDefinition = "lsp-trace:DERIVED_RETAINED_PARTIAL_CALLS_PRIVATE_V1:sealed-derived-v1;native-v5-calls-v1;directed-unit-occurrences;leiden-resolution-1;canonical-communities-v1;authority-0;accepted-false;partial;source-UNKNOWN;feature-UNRESOLVED;no-replay;no-original-native-identity"
	ProfileDigest     = "sha256:3b290ca5ca26337c286cda3f5c3d11fc870eee39935e1726ddbfa3230d1a11bb"
	Admission         = "PRIVATE_COMPUTATION_QUALIFIED"
	ClaimCeiling      = "DERIVED retained-partial structural CALLS communities only; authority=0, accepted=false, partial=true, source completeness=UNKNOWN, feature identity=UNRESOLVED, canonical complete=false, replay=false, original NativeV5 identity=false. Typed CALLS support reference counts, not semantic purpose, runtime execution, source completeness, authenticated custody, permission, or production authority."
)

// SourceBinding is private-derived lineage, never ordinary SourceBinding. All
// bytes and manifest collections are exposed only by detached accessors.
type SourceBinding struct {
	InputSHA256, NativeGraphSHA256, OriginalAcquisitionGraphSHA256                        string
	Profile, ClaimCeiling, ComputationAdmission                                           string
	Origin, SourceCompleteness, FeatureIdentity                                           string
	GraphWideCustodyStatus, OrdinaryQualificationReceiptStatus                            string
	Authority                                                                             int
	Accepted, Partial, CanonicalComplete, ReplayAdmitted, OriginalNativeV5IdentityClaimed bool
	SemanticReceipt                                                                       programc.SemanticReceipt
	raw, native, manifest                                                                 []byte
}

func (s SourceBinding) InputBytes() []byte       { return bytes.Clone(s.raw) }
func (s SourceBinding) NativeGraphBytes() []byte { return bytes.Clone(s.native) }
func (s SourceBinding) Manifest() pilot.DerivedRetainedPartialInputManifestPrivate {
	var m pilot.DerivedRetainedPartialInputManifestPrivate
	_ = json.Unmarshal(s.manifest, &m)
	return m
}

// Projection has no public constructor or mutable admission fields. A value copy
// shares only immutable validated state; zero and JSON-decoded values admit none.
type Projection struct{ state *projectionState }
type projectionState struct {
	self   *projectionState
	math   programc.CallsMathPrivate
	source SourceBinding
}

func (p Projection) Math() programc.CallsMathPrivate {
	if p.state == nil || p.state.self != p.state {
		return programc.CallsMathPrivate{}
	}
	return programc.CloneCallsMathPrivate(p.state.math)
}
func (p Projection) Source() SourceBinding {
	if p.state == nil || p.state.self != p.state {
		return SourceBinding{}
	}
	return p.state.source
}

type Outcome struct {
	ProfileID, ProfileDigest, ClaimCeiling, ComputationAdmission string
	Math                                                         programc.PartitionMathPrivate
	Projection                                                   programc.CallsMathPrivate
	Source                                                       SourceBinding
}

func CloneOutcome(in Outcome) Outcome {
	out := in
	out.Math = programc.ClonePartitionMathPrivate(in.Math)
	out.Projection = programc.CloneCallsMathPrivate(in.Projection)
	out.Source.raw = bytes.Clone(in.Source.raw)
	out.Source.native = bytes.Clone(in.Source.native)
	out.Source.manifest = bytes.Clone(in.Source.manifest)
	return out
}

// Project requires the concrete producer handle FIRST. Consistent envelopes and
// exported DTOs alone cannot enter. The closed parser revalidates every retained
// record, code/provider/seed/policy binding and native semantic commitment.
func Project(proof pilot.DerivedRetainedPartialNativePrivate) (Projection, *programc.Failure) {
	raw, err := pilot.ReadDerivedRetainedPartialNativePrivate(proof)
	if err != nil {
		return Projection{}, invalid("constructor-sealed derived producer required")
	}
	e, err := pilot.ParseDerivedRetainedPartialNativePrivate(raw)
	if err != nil {
		return Projection{}, invalid(err.Error())
	}
	m := e.Manifest
	if m.Profile != pilot.DerivedRetainedPartialPrivateV1 || m.ProgramCAdmission != "NOT_PROGRAM_C_ADMITTED" || m.Authority != 0 || m.Accepted || !m.Partial || m.CanonicalComplete || m.SourceCompleteness != "UNKNOWN" || m.FeatureIdentity != "UNRESOLVED" || m.ReplayAdmitted || m.OriginalNativeV5IdentityClaimed || m.Targets != 25 || len(m.SourcePolicy.Members) != 6 || m.ProviderIdentityQualification != "ORIGINAL_OBSERVATION_NOT_LIVE_LEASE" || m.CanonicalNativeGraphSHA256 != pilot.Digest(e.NativeGraph) || m.OriginalAcquisitionGraphSHA256 == m.CanonicalNativeGraphSHA256 {
		return Projection{}, invalid("private derived profile ceiling or binding")
	}
	math, failure := programc.ProjectNativeCallsMathPrivate(e.NativeGraph)
	if failure != nil {
		return Projection{}, failure
	}
	if len(math.NodeIdentities) != m.Nodes || len(math.Occurrences) != m.CallSites {
		return Projection{}, invalid("private projection counts")
	}
	relations := make(map[string]bool)
	for _, o := range math.Occurrences {
		relations[o.RelationID] = true
		if o.Provider != m.ProviderCommand {
			return Projection{}, invalid("private observed provider")
		}
	}
	if len(relations) != m.Calls {
		return Projection{}, invalid("private CALLS relation counts")
	}
	var doc struct {
		TraceReceipt struct {
			ReceiptVersion           string `json:"receipt_version"`
			SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
			DigestScope              string `json:"digest_scope"`
		} `json:"trace_receipt"`
	}
	if json.Unmarshal(e.NativeGraph, &doc) != nil || doc.TraceReceipt.SemanticCommitmentDigest == "" {
		return Projection{}, invalid("private native semantic receipt")
	}
	manifest, _ := json.Marshal(m)
	s := SourceBinding{InputSHA256: pilot.Digest(raw), NativeGraphSHA256: m.CanonicalNativeGraphSHA256, OriginalAcquisitionGraphSHA256: m.OriginalAcquisitionGraphSHA256, Profile: ProfileID, ClaimCeiling: ClaimCeiling, ComputationAdmission: Admission, Origin: m.Origin, SourceCompleteness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", GraphWideCustodyStatus: "UNAVAILABLE", OrdinaryQualificationReceiptStatus: "UNAVAILABLE", Partial: true,
		SemanticReceipt: programc.SemanticReceipt{ReceiptVersion: doc.TraceReceipt.ReceiptVersion, SemanticCommitmentDigest: doc.TraceReceipt.SemanticCommitmentDigest, DigestScope: doc.TraceReceipt.DigestScope}, raw: raw, native: bytes.Clone(e.NativeGraph), manifest: manifest}
	state := &projectionState{math: math, source: s}
	state.self = state
	return Projection{state: state}, nil
}

// ComputeProjection only accepts the opaque adapter projection. The exported
// numerical math function below this layer conveys no admission of its own.
func ComputeProjection(p Projection, seed uint64) (Outcome, *programc.Failure) {
	if p.state == nil || p.state.self != p.state {
		return Outcome{}, invalid("validated private projection required")
	}
	math, failure := programc.PartitionCallsMathPrivate(p.state.math, seed)
	if failure != nil {
		return Outcome{}, failure
	}
	return Outcome{ProfileID: ProfileID, ProfileDigest: ProfileDigest, ClaimCeiling: ClaimCeiling, ComputationAdmission: Admission, Math: math, Projection: p.Math(), Source: p.Source()}, nil
}
func Compute(proof pilot.DerivedRetainedPartialNativePrivate, seed uint64) (Outcome, *programc.Failure) {
	p, failure := Project(proof)
	if failure != nil {
		return Outcome{}, failure
	}
	return ComputeProjection(p, seed)
}
func invalid(message string) *programc.Failure {
	return &programc.Failure{Code: programc.CodeInvalidProvenance, Message: fmt.Sprintf("derived retained partial private: %s", message)}
}
