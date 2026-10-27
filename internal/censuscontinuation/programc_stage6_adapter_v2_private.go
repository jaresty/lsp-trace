package censuscontinuation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
)

const (
	programCV2MaxCommunitiesPerRun           = 10000
	programCV2MaxTotalPartitionMembersPerRun = 10000
	programCV2MaxCandidatesPerCommunity      = 10000
	programCV2MaxCompositionOccurrences      = 100000
	programCV2MaxOuterWork                   = 10000
	programCV2MaxRunSourceWork               = 20000
)

type programCStage6AdaptationV2 struct {
	classification programCStage6Classification
	mechanics      []programcrepresentative.MechanicsV2
	seedGroupCount uint64
}

type programCV2WorkInput struct {
	communities            uint64
	compositionOccurrences uint64
	partitionMembers       uint64
	seedGroups             uint64
	memberships            uint64
	groupOccurrences       uint64
	candidates             uint64
}

func adaptProgramCStage6InventoryEntryV2(workflowKey, terminalHeadSelector string, entry programCTransitionReceiptInventoryEntry) (programCStage6AdaptationV2, error) {
	zero := programCStage6AdaptationV2{}
	outcomes := cloneProgramCOutcomes(entry.outcomes)
	if workflowKey == "" || entry.ordinal < 0 || entry.value.Ordinal != entry.ordinal || entry.value.ReceiptOrdinal <= 0 || entry.selector == "" || transitionSHA(entry.canonicalReceipt) != entry.selector || entry.value.WorkflowKey != workflowKey || entry.value.Outcome != "SUCCEEDED" || len(entry.nativeV5) == 0 || entry.value.ResultSHA256 != transitionSHA(entry.value.ResultBytes) || entry.value.ResultByteLength != len(entry.value.ResultBytes) || len(outcomes) != 6 || !programCV2StringsWithin(reflect.ValueOf(entry)) || !programCV2StringsWithin(reflect.ValueOf(workflowKey)) || !programCV2StringsWithin(reflect.ValueOf(terminalHeadSelector)) {
		return zero, ErrInvalidProgramCTransition
	}
	canonicalReceipt, err := json.Marshal(entry.value)
	if err != nil || !bytes.Equal(canonicalReceipt, entry.canonicalReceipt) || !validProgramCStage6RunSequence(outcomes, entry.value.Bindings.AlgorithmVersion) {
		return zero, ErrInvalidProgramCTransition
	}
	source := outcomes[0].Source
	sourceBytes := source.InputBytes()
	if !bytes.Equal(entry.nativeV5, sourceBytes) || source.InputLength != len(sourceBytes) || source.InputSHA256 != transitionSHA(sourceBytes) {
		return zero, ErrInvalidProgramCTransition
	}
	for _, outcome := range outcomes {
		if !reflect.DeepEqual(outcome.Source, source) || !reflect.DeepEqual(outcome.Projection.Source, source) {
			return zero, ErrInvalidProgramCTransition
		}
	}
	switch {
	case entry.value.ProgramCOutcome == "EMPTY" && entry.value.Disposition == "":
		for _, outcome := range outcomes {
			if outcome.Outcome != "EMPTY" {
				return zero, ErrInvalidProgramCTransition
			}
		}
		return programCStage6AdaptationV2{classification: programCStage6Empty, mechanics: []programcrepresentative.MechanicsV2{}}, nil
	case entry.value.ProgramCOutcome == "COMPLETE" && entry.value.Disposition == "COMMITTED_INCOMPLETE_MATCHING_RESOURCE":
		for _, outcome := range outcomes {
			if outcome.Outcome != "COMPLETE" {
				return zero, ErrInvalidProgramCTransition
			}
		}
		if entry.value.InstabilitySchema != programc.InstabilityVersionV3 {
			return zero, ErrInvalidProgramCTransition
		}
		return programCStage6AdaptationV2{classification: programCStage6ResourceIncomplete, mechanics: []programcrepresentative.MechanicsV2{}}, nil
	case entry.value.ProgramCOutcome != "COMPLETE" || entry.value.Disposition != "":
		return zero, ErrInvalidProgramCTransition
	}
	for _, outcome := range outcomes {
		if outcome.Outcome != "COMPLETE" {
			return zero, ErrInvalidProgramCTransition
		}
	}
	if entry.value.InstabilitySchema != programc.InstabilityVersionV2 || entry.value.InstabilityClaimCeiling != programc.InstabilityClaimCeilingV2 {
		return zero, ErrInvalidProgramCTransition
	}
	identity, pairs, err := decodeCompleteProgramCInstability(entry.value.ResultBytes)
	if err != nil || identity.AdmittedGraphSHA256 != outcomes[0].Source.InputSHA256 || identity.ProjectionSHA256 != programc.ProfileDigest || identity.ProjectionPolicyID != programc.ProfileID || identity.AlgorithmName != "gonum.org/v1/gonum/graph/community.Leiden" || identity.AlgorithmVersion != entry.value.Bindings.AlgorithmVersion || identity.ParametersCanonicalSHA256 != entry.value.Bindings.ParametersSHA256 || identity.ResourcePolicySHA256 != entry.value.Bindings.ResourcePolicySHA256 {
		return zero, ErrInvalidProgramCTransition
	}
	communityDigests, err := verifyProgramCStage6CommunityDigests(outcomes, identity, pairs)
	if err != nil {
		return zero, ErrInvalidProgramCTransition
	}
	prepared, err := programcrepresentative.PrepareMechanicsV2(programcrepresentative.MechanicsCommonInputV2{
		WorkflowKey: workflowKey, TerminalProgramCHeadSelector: terminalHeadSelector,
		ReceiptSelector: entry.selector, ReceiptOrdinal: entry.value.ReceiptOrdinal,
		ConstituentOrdinal: entry.ordinal, PublicationSelector: entry.value.PublicationSelector,
		ConstituentSelector: entry.value.ConstituentSelector, Source: source,
		ProgramCAlgorithmVersion: entry.value.Bindings.AlgorithmVersion,
	})
	if err != nil {
		return zero, ErrInvalidProgramCTransition
	}
	counts := prepared.SourceCounts()
	sourceCounts := programCV2WorkInput{
		seedGroups: counts.SeedGroups, memberships: counts.Memberships,
		groupOccurrences: counts.GroupOccurrences,
	}
	contexts := []programcrepresentative.MechanicsContextInputV2{}
	for runOrdinal, outcome := range outcomes {
		communities := uint64(len(outcome.Communities))
		occurrences := uint64(len(outcome.Projection.Occurrences))
		if communities == 0 {
			return zero, ErrInvalidProgramCTransition
		}
		partitionMembers := uint64(0)
		for _, community := range outcome.Communities {
			if len(community.Members) == 0 {
				return zero, ErrInvalidProgramCTransition
			}
			partitionMembers, err = programCV2CheckedAdd(partitionMembers, uint64(len(community.Members)))
			if err != nil {
				return zero, ErrInvalidProgramCTransition
			}
		}
		if partitionMembers != uint64(len(outcome.Projection.NodeIdentities)) {
			return zero, ErrInvalidProgramCTransition
		}
		replayOrdinal := uint8(runOrdinal%3 + 1)
		runID := fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, replayOrdinal)
		for communityOrdinal, community := range outcome.Communities {
			work := sourceCounts
			work.communities = communities
			work.compositionOccurrences = occurrences
			work.partitionMembers = partitionMembers
			work.candidates = uint64(len(community.Members))
			if _, err = programCV2CheckedOuterWork(work); err != nil {
				return zero, ErrInvalidProgramCTransition
			}
			contexts = append(contexts, programcrepresentative.MechanicsContextInputV2{
				RunID: runID, Seed: outcome.Seed, ReplayOrdinal: replayOrdinal,
				CommunityOrdinal: communityOrdinal, CommunityArtifactSHA256: communityDigests[runOrdinal],
				CommunityMembers: append([]string{}, community.Members...),
			})
		}
		runWork := sourceCounts
		runWork.communities = communities
		runWork.compositionOccurrences = occurrences
		runWork.partitionMembers = partitionMembers
		if _, err = programCV2CheckedRunSourceWork(runWork); err != nil {
			return zero, ErrInvalidProgramCTransition
		}
	}
	mechanics, err := programcrepresentative.NewPreparedMechanicsBatchV2(prepared, contexts)
	if err != nil {
		return zero, ErrInvalidProgramCTransition
	}
	return programCStage6AdaptationV2{
		classification: programCStage6Complete, mechanics: mechanics, seedGroupCount: counts.SeedGroups,
	}, nil
}

func programCV2ValidateCardinalities(in programCV2WorkInput) error {
	if in.communities > programCV2MaxCommunitiesPerRun || in.partitionMembers > programCV2MaxTotalPartitionMembersPerRun || in.candidates > programCV2MaxCandidatesPerCommunity || in.compositionOccurrences > programCV2MaxCompositionOccurrences || in.seedGroups > programcrepresentative.MaxSeedGroupsV2 || in.memberships > programcrepresentative.MaxMembershipsV2 || in.groupOccurrences > programcrepresentative.MaxSumGroupOccurrencesV2 {
		return ErrInvalidProgramCTransition
	}
	return nil
}

func programCV2CheckedAdd(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, ErrInvalidProgramCTransition
	}
	return a + b, nil
}

func programCV2CheckedMultiply(a, b uint64) (uint64, error) {
	if a != 0 && b > math.MaxUint64/a {
		return 0, ErrInvalidProgramCTransition
	}
	return a * b, nil
}

func programCV2CheckedOuterWork(in programCV2WorkInput) (uint64, error) {
	if err := programCV2ValidateCardinalities(in); err != nil {
		return 0, err
	}
	candidates, err := programCV2CheckedMultiply(in.seedGroups, in.candidates)
	if err != nil {
		return 0, err
	}
	work, err := programCV2CheckedSum(in.compositionOccurrences, in.partitionMembers, in.seedGroups, in.memberships, in.groupOccurrences, candidates)
	if err != nil || work > programCV2MaxOuterWork {
		return 0, ErrInvalidProgramCTransition
	}
	return work, nil
}

func programCV2CheckedRunSourceWork(in programCV2WorkInput) (uint64, error) {
	if err := programCV2ValidateCardinalities(in); err != nil {
		return 0, err
	}
	candidates, err := programCV2CheckedMultiply(in.seedGroups, in.partitionMembers)
	if err != nil {
		return 0, err
	}
	work, err := programCV2CheckedSum(in.compositionOccurrences, in.partitionMembers, in.seedGroups, in.memberships, in.groupOccurrences, candidates)
	if err != nil || work > programCV2MaxRunSourceWork {
		return 0, ErrInvalidProgramCTransition
	}
	return work, nil
}

func programCV2CheckedWork(in programCV2WorkInput) (uint64, uint64, error) {
	outer, err := programCV2CheckedOuterWork(in)
	if err != nil {
		return 0, 0, err
	}
	run, err := programCV2CheckedRunSourceWork(in)
	if err != nil {
		return 0, 0, err
	}
	return outer, run, nil
}

func programCV2CheckedSum(values ...uint64) (uint64, error) {
	var total uint64
	var err error
	for _, value := range values {
		total, err = programCV2CheckedAdd(total, value)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func programCV2StringsWithin(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return true
		}
		return programCV2StringsWithin(value.Elem())
	case reflect.String:
		return value.Len() <= programcrepresentative.MaxStringBytesV2
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !programCV2StringsWithin(value.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if !programCV2StringsWithin(value.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if !programCV2StringsWithin(iterator.Key()) || !programCV2StringsWithin(iterator.Value()) {
				return false
			}
		}
	}
	return true
}
