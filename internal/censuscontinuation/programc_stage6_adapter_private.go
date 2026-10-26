package censuscontinuation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
)

type programCStage6Classification string

const (
	programCStage6Complete           programCStage6Classification = "COMPLETE"
	programCStage6Empty              programCStage6Classification = "EMPTY"
	programCStage6ResourceIncomplete programCStage6Classification = "COMMITTED_INCOMPLETE_MATCHING_RESOURCE"
)

type programCStage6Adaptation struct {
	classification programCStage6Classification
	mechanics      []programcrepresentative.Mechanics
}

func adaptProgramCStage6InventoryEntry(workflowKey, terminalHeadSelector string, entry programCTransitionReceiptInventoryEntry) (programCStage6Adaptation, error) {
	zero := programCStage6Adaptation{}
	outcomes := cloneProgramCOutcomes(entry.outcomes)
	if workflowKey == "" || entry.ordinal < 0 || entry.value.Ordinal != entry.ordinal || entry.value.ReceiptOrdinal <= 0 || entry.selector == "" || transitionSHA(entry.canonicalReceipt) != entry.selector || entry.value.WorkflowKey != workflowKey || entry.value.Outcome != "SUCCEEDED" || len(entry.nativeV5) == 0 || entry.value.ResultSHA256 != transitionSHA(entry.value.ResultBytes) || entry.value.ResultByteLength != len(entry.value.ResultBytes) || len(outcomes) != 6 {
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
		return programCStage6Adaptation{classification: programCStage6Empty, mechanics: []programcrepresentative.Mechanics{}}, nil
	case entry.value.ProgramCOutcome == "COMPLETE" && entry.value.Disposition == "COMMITTED_INCOMPLETE_MATCHING_RESOURCE":
		for _, outcome := range outcomes {
			if outcome.Outcome != "COMPLETE" {
				return zero, ErrInvalidProgramCTransition
			}
		}
		if entry.value.InstabilitySchema != programc.InstabilityVersionV3 {
			return zero, ErrInvalidProgramCTransition
		}
		return programCStage6Adaptation{classification: programCStage6ResourceIncomplete, mechanics: []programcrepresentative.Mechanics{}}, nil
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
	mechanics := make([]programcrepresentative.Mechanics, 0)
	for runOrdinal, outcome := range outcomes {
		replayOrdinal := uint8(runOrdinal%3 + 1)
		runID := fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, replayOrdinal)
		for communityOrdinal, community := range outcome.Communities {
			item, buildErr := programcrepresentative.NewSingletonMechanics(programcrepresentative.SingletonMechanicsInput{
				WorkflowKey: workflowKey, TerminalProgramCHeadSelector: terminalHeadSelector,
				ReceiptSelector: entry.selector, ReceiptOrdinal: entry.value.ReceiptOrdinal,
				ConstituentOrdinal: entry.ordinal, PublicationSelector: entry.value.PublicationSelector,
				ConstituentSelector: entry.value.ConstituentSelector, Source: source,
				ProgramCAlgorithmVersion: entry.value.Bindings.AlgorithmVersion,
				RunID:                    runID, Seed: outcome.Seed, ReplayOrdinal: replayOrdinal,
				CommunityOrdinal: communityOrdinal, CommunityArtifactSHA256: communityDigests[runOrdinal],
				CommunityMembers: append([]string(nil), community.Members...),
			})
			if buildErr != nil {
				return zero, ErrInvalidProgramCTransition
			}
			mechanics = append(mechanics, item)
		}
	}
	return programCStage6Adaptation{classification: programCStage6Complete, mechanics: mechanics}, nil
}

func cloneProgramCOutcomes(in []programc.Outcome) []programc.Outcome {
	out := make([]programc.Outcome, len(in))
	for i := range in {
		out[i] = programc.CloneOutcome(in[i])
	}
	return out
}

func validProgramCStage6RunSequence(outcomes []programc.Outcome, algorithmVersion string) bool {
	if len(outcomes) != 6 || algorithmVersion == "" {
		return false
	}
	seeds := [...]uint64{1, 1, 1, 2, 2, 2}
	for i, outcome := range outcomes {
		if outcome.Seed != seeds[i] || outcome.ProfileID != programc.ProfileID || outcome.ProfileDigest != programc.ProfileDigest || outcome.Algorithm != "gonum.org/v1/gonum/graph/community.Leiden" {
			return false
		}
	}
	return true
}

func decodeCompleteProgramCInstability(raw []byte) (programc.InstabilityIdentity, []programc.InstabilityPair, error) {
	var artifact programc.InstabilityArtifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return programc.InstabilityIdentity{}, nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return programc.InstabilityIdentity{}, nil, ErrInvalidProgramCTransition
	}
	canonical, err := json.Marshal(artifact)
	if err != nil || !bytes.Equal(canonical, raw) || artifact.SchemaVersion != programc.InstabilityVersionV2 || (artifact.Outcome != "STABLE" && artifact.Outcome != "UNSTABLE") || artifact.RunAccounting.DeclaredSeedCount != 2 || artifact.RunAccounting.DeclaredRunCount != 6 || artifact.RunAccounting.CompletedRunCount != 6 || artifact.RunAccounting.FailedRunCount != 0 || artifact.RunAccounting.IncompleteRunCount != 0 || artifact.RunAccounting.ExpectedPairCount != 15 || artifact.RunAccounting.CompletedPairCount != 15 || artifact.RunAccounting.BlockedPairCount != 0 || len(artifact.PairwiseComparisons) != 15 {
		return programc.InstabilityIdentity{}, nil, ErrInvalidProgramCTransition
	}
	return artifact.ComparisonIdentity, append([]programc.InstabilityPair(nil), artifact.PairwiseComparisons...), nil
}

func verifyProgramCStage6CommunityDigests(outcomes []programc.Outcome, identity programc.InstabilityIdentity, pairs []programc.InstabilityPair) ([]string, error) {
	runIDs := make([]string, len(outcomes))
	digests := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		runIDs[i] = fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, i%3+1)
		artifact, err := programc.BuildCommunityArtifact(outcome, identity)
		if err != nil {
			return nil, err
		}
		digests[i] = transitionSHA(artifact)
	}
	incidents := make([]int, len(outcomes))
	pairOrdinal := 0
	for left := 0; left < len(outcomes); left++ {
		for right := left + 1; right < len(outcomes); right++ {
			if pairOrdinal >= len(pairs) {
				return nil, ErrInvalidProgramCTransition
			}
			pair := pairs[pairOrdinal]
			if pair.LeftRunID != runIDs[left] || pair.RightRunID != runIDs[right] || pair.LeftCommunityArtifactSHA256 != digests[left] || pair.RightCommunityArtifactSHA256 != digests[right] {
				return nil, ErrInvalidProgramCTransition
			}
			incidents[left]++
			incidents[right]++
			pairOrdinal++
		}
	}
	if pairOrdinal != 15 || pairOrdinal != len(pairs) || !reflect.DeepEqual(incidents, []int{5, 5, 5, 5, 5, 5}) {
		return nil, ErrInvalidProgramCTransition
	}
	return digests, nil
}
