package censuscontinuation

import (
	"math"
	"strings"
	"testing"

	"lsp-trace/internal/programc"
)

func TestProgramCStage6AdapterV2MultiSeedComplete(t *testing.T) {
	entry := programCQualificationMultiSeedEntryFixture(t, 5)
	got, err := adaptProgramCStage6InventoryEntryV2("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil || got.classification != programCStage6Complete {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_COMPLETE err=%v class=%q", err, got.classification)
	}
	want := 0
	for _, outcome := range entry.outcomes {
		want += len(outcome.Communities)
	}
	if len(got.mechanics) != want {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_ONE_PER_RUN_COMMUNITY got=%d want=%d", len(got.mechanics), want)
	}
	index := 0
	for runOrdinal, outcome := range entry.outcomes {
		for communityOrdinal := range outcome.Communities {
			mechanics := got.mechanics[index]
			dto := mechanics.DTO()
			if !mechanics.Valid() || !mechanics.Trusted() || len(dto.SelectionContext.SeedGroups) != 5 || dto.RunID != "seed-"+formatProgramCV2Seed(outcome.Seed)+"-replay-"+formatProgramCV2Replay(runOrdinal) || dto.CommunityOrdinal != communityOrdinal {
				t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_ORDER index=%d dto=%+v", index, dto)
			}
			index++
		}
	}
}

func TestProgramCStage6AdapterV2EmptyAndResourceProduceNoMechanics(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		entry.value.ProgramCOutcome = "EMPTY"
		for i := range entry.outcomes {
			entry.outcomes[i].Outcome = "EMPTY"
			entry.outcomes[i].Communities = nil
		}
		programCStage6RecanonicalizeEntry(t, &entry)
		got, err := adaptProgramCStage6InventoryEntryV2("workflow", "sha256:"+strings.Repeat("f", 64), entry)
		if err != nil || got.classification != programCStage6Empty || len(got.mechanics) != 0 {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_EMPTY_ZERO_MECHANICS got=%+v err=%v", got, err)
		}
	})
	t.Run("resource", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		entry.value.Disposition = "COMMITTED_INCOMPLETE_MATCHING_RESOURCE"
		entry.value.InstabilitySchema = programc.InstabilityVersionV3
		entry.value.InstabilityClaimCeiling = programc.InstabilityClaimCeilingV3
		programCStage6RecanonicalizeEntry(t, &entry)
		got, err := adaptProgramCStage6InventoryEntryV2("workflow", "sha256:"+strings.Repeat("f", 64), entry)
		if err != nil || got.classification != programCStage6ResourceIncomplete || len(got.mechanics) != 0 {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_RESOURCE_ZERO_MECHANICS got=%+v err=%v", got, err)
		}
	})
}

func TestProgramCStage6AdapterV2LimitFailureHasZeroPartialExposure(t *testing.T) {
	entry := programCQualificationMultiSeedEntryFixture(t, 65)
	got, err := adaptProgramCStage6InventoryEntryV2("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err == nil || got.classification != "" || got.mechanics != nil {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_V2_LIMIT_ZERO_PARTIAL got=%+v err=%v", got, err)
	}
}

func TestProgramCV2InclusiveCardinalityLimits(t *testing.T) {
	boundaries := []programCV2WorkInput{
		{communities: programCV2MaxCommunitiesPerRun},
		{partitionMembers: programCV2MaxTotalPartitionMembersPerRun},
		{candidates: programCV2MaxCandidatesPerCommunity},
		{compositionOccurrences: programCV2MaxCompositionOccurrences},
		{seedGroups: 64},
		{memberships: 2048},
		{groupOccurrences: 2048},
	}
	for i, input := range boundaries {
		if err := programCV2ValidateCardinalities(input); err != nil {
			t.Fatalf("ASSERT_PROGRAM_C_V2_LIMIT_BOUNDARY_%d err=%v", i, err)
		}
	}
	plusOne := []programCV2WorkInput{
		{communities: programCV2MaxCommunitiesPerRun + 1},
		{partitionMembers: programCV2MaxTotalPartitionMembersPerRun + 1},
		{candidates: programCV2MaxCandidatesPerCommunity + 1},
		{compositionOccurrences: programCV2MaxCompositionOccurrences + 1},
		{seedGroups: 65},
		{memberships: 2049},
		{groupOccurrences: 2049},
	}
	for i, input := range plusOne {
		if err := programCV2ValidateCardinalities(input); err == nil {
			t.Fatalf("ASSERT_PROGRAM_C_V2_LIMIT_PLUS_ONE_%d", i)
		}
	}
}

func TestProgramCV2CheckedWorkFormulaAndOverflow(t *testing.T) {
	input := programCV2WorkInput{communities: 11, compositionOccurrences: 13, partitionMembers: 17, seedGroups: 19, memberships: 23, groupOccurrences: 29, candidates: 31}
	outer, run, err := programCV2CheckedWork(input)
	if err != nil || outer != 690 || run != 424 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_EXACT_WORK_FORMULA outer=%d run=%d err=%v", outer, run, err)
	}
	if _, err := programCV2CheckedAdd(math.MaxUint64, 1); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_CHECKED_ADD_OVERFLOW")
	}
	if _, err := programCV2CheckedMultiply(math.MaxUint64, 2); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_CHECKED_MULTIPLY_OVERFLOW")
	}
	if _, _, err := programCV2CheckedWork(programCV2WorkInput{seedGroups: 64, partitionMembers: 4096}); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_RUN_SOURCE_WORK_LIMIT")
	}
	if _, _, err := programCV2CheckedWork(programCV2WorkInput{compositionOccurrences: programCV2MaxOuterWork, partitionMembers: 1}); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_OUTER_WORK_LIMIT")
	}
}

func TestProgramCV2WorkCapsAndObservedMaxima(t *testing.T) {
	if got, err := programCV2CheckedOuterWork(programCV2WorkInput{compositionOccurrences: programCV2MaxOuterWork}); err != nil || got != programCV2MaxOuterWork {
		t.Fatalf("ASSERT_PROGRAM_C_V2_OUTER_WORK_EQUALITY got=%d err=%v", got, err)
	}
	if _, err := programCV2CheckedOuterWork(programCV2WorkInput{compositionOccurrences: programCV2MaxOuterWork + 1}); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_OUTER_WORK_PLUS_ONE")
	}
	if got, err := programCV2CheckedRunSourceWork(programCV2WorkInput{compositionOccurrences: programCV2MaxRunSourceWork}); err != nil || got != programCV2MaxRunSourceWork {
		t.Fatalf("ASSERT_PROGRAM_C_V2_RUN_WORK_EQUALITY got=%d err=%v", got, err)
	}
	if _, err := programCV2CheckedRunSourceWork(programCV2WorkInput{compositionOccurrences: programCV2MaxRunSourceWork + 1}); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_V2_RUN_WORK_PLUS_ONE")
	}
	if got, err := programCV2CheckedOuterWork(programCV2WorkInput{seedGroups: 1, candidates: 7185}); err != nil || got != 7186 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_OBSERVED_OUTER_MAX got=%d err=%v", got, err)
	}
	if got, err := programCV2CheckedRunSourceWork(programCV2WorkInput{seedGroups: 1, partitionMembers: 8010}); err != nil || got != 16021 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_OBSERVED_RUN_MAX got=%d err=%v", got, err)
	}
	workLimitedAtIndividualEquality := []programCV2WorkInput{
		{communities: programCV2MaxCommunitiesPerRun, partitionMembers: programCV2MaxCommunitiesPerRun, seedGroups: 1, memberships: 1},
		{communities: 1, partitionMembers: programCV2MaxTotalPartitionMembersPerRun, seedGroups: 1, memberships: 1},
		{communities: 1, partitionMembers: 1, seedGroups: 1, memberships: 1, candidates: programCV2MaxCandidatesPerCommunity},
		{communities: 1, partitionMembers: 1, seedGroups: 1, memberships: 1, compositionOccurrences: programCV2MaxCompositionOccurrences},
	}
	for i, input := range workLimitedAtIndividualEquality {
		if err := programCV2ValidateCardinalities(input); err != nil {
			t.Fatalf("ASSERT_PROGRAM_C_V2_INDIVIDUAL_EQUALITY_CARDINALITY_%d err=%v", i, err)
		}
		if _, _, err := programCV2CheckedWork(input); err == nil {
			t.Fatalf("ASSERT_PROGRAM_C_V2_INDIVIDUAL_EQUALITY_STILL_WORK_LIMITED_%d", i)
		}
	}
}

func formatProgramCV2Seed(seed uint64) string { return leftPadProgramCV2(seed, 20) }
func formatProgramCV2Replay(runOrdinal int) string {
	return leftPadProgramCV2(uint64(runOrdinal%3+1), 1)
}
func leftPadProgramCV2(value uint64, width int) string {
	digits := ""
	if value == 0 {
		digits = "0"
	}
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return strings.Repeat("0", width-len(digits)) + digits
}
