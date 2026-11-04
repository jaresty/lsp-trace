package programcrepresentative

import (
	"crypto/sha256"
	"fmt"
)

func multiSeedIdentityV2(domain string, raw []byte) string {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write(raw)
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
func multiSeedBindingV2(schema, identity string, raw []byte) ArtifactBindingDTO {
	return ArtifactBindingDTO{SchemaVersion: schema, Identity: identity, SHA256: rawDigest(raw), ByteLength: uint64(len(raw))}
}

func BuildStage6PolicyArtifactV2() (Stage6PolicyV2, ArtifactBindingDTO, []byte, error) {
	dto := Stage6PolicyV2{
		SchemaVersion: Stage6PolicySchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1,
		AdmittedKind: "CALLS", AdmittedDisposition: "INCLUDED", SelfLoopDisposition: "RETAINED_INCLUDED_DERIVED_INELIGIBLE",
		ContextNodeScope: "UNION_PREPARED_TARGET_AND_REACHED_NODE", RelationOccurrenceClosure: "EXACT_ALL_CALLS_OCCURRENCES_PER_SEED_GROUP_RELATION",
		GraphDirection: "CALLER_TO_CALLEE", StronglyConnectedComponents: "DETERMINISTIC_TARJAN_CANONICAL_NODE_ORDER", PreparedRootDistance: "SHORTEST_DIRECTED_CONDENSATION_EDGE_COUNT",
		CandidateScope: "REACHED_COMMUNITY_MEMBER_REACHABLE_FROM_PREPARED_ROOT_SCC", PreparedRootScope: "ALL_SCCS_CONTAINING_PREPARED_TARGETS", SelectedSCCMembersScope: "ALL_ACTIVE_CONTEXT_NODES_IN_SELECTED_SCC", PredecessorScope: "INCLUDED_NON_SELF_CALLS_DIRECTLY_INCOMING_TO_SELECTED_NODE",
		NoCallsOutcome: "NO_CALLS_REPRESENTATIVE", SelectionFailureOutcome: "CALLS_EVIDENCE_INELIGIBLE", SelectedOutcome: "SELECTED", SelectionOrder: []string{"MIN_DISTANCE", "LEXICOGRAPHIC_NODE_ID"}, PredecessorOrder: []string{"RELATION_ID", "OCCURRENCE_ID"}, OuterTerminalRule: "ALL_EXPECTED_GROUPS_QUALIFIED_TERMINAL", GroupMappingRule: "NATIVE_INVOCATION_ORDER_GROUP_LOCAL_SELECTION", NoOuterWinner: true,
	}
	n, err := CanonicalSizeStage6PolicyV2(dto)
	preimage, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6PolicyV2{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-policy:v2", preimage)
	n, err = CanonicalSizeStage6PolicyV2(dto)
	raw, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6PolicyV2{}, ArtifactBindingDTO{}, nil, err
	}
	return dto, multiSeedBindingV2(dto.SchemaVersion, dto.Identity, raw), raw, nil
}
func BuildStage6LimitsArtifactV2() (Stage6LimitsV2, ArtifactBindingDTO, []byte, error) {
	dto := Stage6LimitsV2{SchemaVersion: Stage6LimitsSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, MaxCommunities: 10000, MaxMembers: 10000, MaxOccurrences: 100000, MaxCandidates: 10000, MaxSeedGroups: 64, MaxMemberships: 2048, MaxMembershipsPerGroup: 256, MaxGroupOccurrences: 256, MaxSumGroupOccurrences: 2048, MaxStringBytes: 1048576, MaxOuterWork: 10000, MaxRunSourceWork: 20000, MaxCanonicalBytes: 67108864}
	n, err := CanonicalSizeStage6LimitsV2(dto)
	preimage, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6LimitsV2{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-limits:v2", preimage)
	n, err = CanonicalSizeStage6LimitsV2(dto)
	raw, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6LimitsV2{}, ArtifactBindingDTO{}, nil, err
	}
	return dto, multiSeedBindingV2(dto.SchemaVersion, dto.Identity, raw), raw, nil
}
func BuildStage6FinalizationArtifactV2() (Stage6FinalizationV2, ArtifactBindingDTO, []byte, error) {
	dto := Stage6FinalizationV2{SchemaVersion: Stage6FinalizationSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, CompositionSchema: CompositionSchemaVersionV2, GroupingSchema: GroupingSchemaVersionV2, QualificationSchema: DerivedQualificationSchemaVersionV2, ResultSchema: ResultSchemaVersionV2, MechanicsSchema: MechanicsSchemaVersionV2, ContextSchema: SelectionContextSchemaVersionV2,
		CompositionMappings:   []string{"VERIFIED_M2_EXACT", "RECOMPUTED_V1_CUSTODY_EXACT", "VERIFIED_POLICY_BINDING", "VERIFIED_ENVELOPE_BINDING"},
		GroupingMappings:      []string{"VERIFIED_COMPOSITION_BINDING", "VERIFIED_LIMITS_BINDING", "ONE_COMPLETE_ORDERED_CONTEXT_V2"},
		GroupMappings:         []string{"NATIVE_INVOCATION_ORDINAL_AND_IDENTITY", "EXACT_GROUP_LOCAL_CLOSURE", "UNCHANGED_CALLS_SCC_DISTANCE_SELECTION", "ALL_GROUPS_NO_OUTER_WINNER"},
		QualificationMappings: []string{"EXACT_COMPOSITION_GROUPING_ENVELOPE", "TERMINAL_HEAD_GENERATION_BOTH_EPOCHS", "RECONSTRUCTED_REPOSITORY_VERIFIER"},
		FinalizationOrder:     []string{"COMPOSITION", "GROUPING", "DERIVED_QUALIFICATION", "ALL_GROUP_RECORDS", "OUTER_TERMINAL_RESULT"}}
	n, err := CanonicalSizeStage6FinalizationV2(dto)
	preimage, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6FinalizationV2{}, ArtifactBindingDTO{}, nil, err
	}
	dto.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-finalization:v2", preimage)
	n, err = CanonicalSizeStage6FinalizationV2(dto)
	raw, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return Stage6FinalizationV2{}, ArtifactBindingDTO{}, nil, err
	}
	return dto, multiSeedBindingV2(dto.SchemaVersion, dto.Identity, raw), raw, nil
}
