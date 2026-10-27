package programcrepresentative_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/programcrepresentative"
)

func TestPreparedMechanicsV2MatchesLegacyExactly(t *testing.T) {
	for _, seedGroups := range []int{1, 5, programcrepresentative.MaxSeedGroupsV2} {
		t.Run(fmt.Sprint(seedGroups), func(t *testing.T) {
			base := mechanicsV2Input(t, seedGroups)
			common := preparedMechanicsV2Common(base)
			prepared, err := programcrepresentative.PrepareMechanicsV2(common)
			if err != nil {
				t.Fatal(err)
			}
			counts := prepared.SourceCounts()
			if counts.SeedGroups != uint64(seedGroups) || counts.Memberships == 0 || counts.GroupOccurrences == 0 {
				t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_SOURCE_COUNTS got=%+v", counts)
			}

			contexts := preparedMechanicsV2Contexts(base, 4)
			batch, err := programcrepresentative.NewPreparedMechanicsBatchV2(prepared, contexts)
			if err != nil || len(batch) != len(contexts) {
				t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_BATCH_CARDINALITY got=%d want=%d err=%v", len(batch), len(contexts), err)
			}
			for i, context := range contexts {
				legacy, legacyErr := programcrepresentative.NewMechanicsV2(preparedMechanicsV2Legacy(common, context))
				single, singleErr := programcrepresentative.NewPreparedMechanicsV2(prepared, context)
				if legacyErr != nil || singleErr != nil {
					t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_EQUIVALENCE index=%d legacy_err=%v prepared_err=%v", i, legacyErr, singleErr)
				}
				if !reflect.DeepEqual(legacy.DTO(), single.DTO()) || !reflect.DeepEqual(single.DTO(), batch[i].DTO()) || legacy.Identity() != single.Identity() || single.Identity() != batch[i].Identity() || !bytes.Equal(legacy.CanonicalBytes(), single.CanonicalBytes()) || !bytes.Equal(single.CanonicalBytes(), batch[i].CanonicalBytes()) || legacy.Valid() != single.Valid() || single.Valid() != batch[i].Valid() || legacy.Trusted() != single.Trusted() || single.Trusted() != batch[i].Trusted() {
					t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_EQUIVALENCE index=%d", i)
				}
				dto := batch[i].DTO()
				if dto.RunID != context.RunID || dto.CommunityOrdinal != context.CommunityOrdinal || dto.Authority != 0 || dto.Accepted {
					t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_BATCH_ORDER index=%d dto=%+v", i, dto)
				}
			}
		})
	}
}

func TestPreparedMechanicsV2DefensiveIsolation(t *testing.T) {
	base := mechanicsV2Input(t, 5)
	prepared, err := programcrepresentative.PrepareMechanicsV2(preparedMechanicsV2Common(base))
	if err != nil {
		t.Fatal(err)
	}
	contexts := preparedMechanicsV2Contexts(base, 3)
	batch, err := programcrepresentative.NewPreparedMechanicsBatchV2(prepared, contexts)
	if err != nil {
		t.Fatal(err)
	}
	before := make([][]byte, len(batch))
	for i := range batch {
		before[i] = batch[i].CanonicalBytes()
	}

	contexts[0].CommunityMembers[0] = "mutated-input"
	firstDTO := batch[0].DTO()
	firstDTO.CommunityMembers[0] = "mutated-dto"
	firstDTO.Evidence[0].RelationID = "mutated-evidence"
	firstDTO.SelectionContext.CommunityMembers[0] = "mutated-context"
	firstDTO.SelectionContext.SeedGroups[0].Memberships[0].EndpointID = "mutated-membership"
	firstDTO.SelectionContext.SeedGroups[0].OccurrenceIDs[0] = "mutated-occurrence"
	members := batch[0].CommunityMembers()
	members[0] = "mutated-members"
	evidence := batch[0].Evidence()
	evidence[0].RelationID = "mutated-evidence-accessor"
	selection := batch[0].SelectionContext()
	selection.SeedGroups[0].Memberships[0].EndpointID = "mutated-selection"
	canonical := batch[0].CanonicalBytes()
	canonical[0] ^= 0xff

	for i := range batch {
		if !bytes.Equal(batch[i].CanonicalBytes(), before[i]) {
			t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_NO_ALIASES index=%d", i)
		}
		if !batch[i].Valid() || !batch[i].Trusted() {
			t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_NO_ALIASES_INVALID index=%d", i)
		}
	}
	if prepared.SourceCounts().SeedGroups != 5 {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_SCALAR_ACCESSOR got=%+v", prepared.SourceCounts())
	}
}

func TestPreparedMechanicsV2RejectsInvalidAndReturnsNoPartialBatch(t *testing.T) {
	base := mechanicsV2Input(t, 1)
	common := preparedMechanicsV2Common(base)
	invalidCommon := common
	invalidCommon.WorkflowKey = ""
	if got, err := programcrepresentative.PrepareMechanicsV2(invalidCommon); err == nil || got.SourceCounts() != (programcrepresentative.MechanicsSourceCountsV2{}) {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_INVALID_COMMON got=%+v err=%v", got.SourceCounts(), err)
	}

	context := preparedMechanicsV2Contexts(base, 1)[0]
	if got, err := programcrepresentative.NewPreparedMechanicsV2(programcrepresentative.PreparedMechanicsV2{}, context); err == nil || got.Valid() {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_ZERO_PREPARED err=%v", err)
	}
	prepared, err := programcrepresentative.PrepareMechanicsV2(common)
	if err != nil {
		t.Fatal(err)
	}
	invalidContext := context
	invalidContext.CommunityOrdinal = -1
	invalidContext.ReplayOrdinal = 0
	if got, err := programcrepresentative.NewPreparedMechanicsV2(prepared, invalidContext); err == nil || got.Valid() || err.Error() != "invalid v2 mechanics input" {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_INVALID_CONTEXT_PRECEDENCE got=%+v err=%v", got, err)
	}
	batch, err := programcrepresentative.NewPreparedMechanicsBatchV2(prepared, []programcrepresentative.MechanicsContextInputV2{context, invalidContext})
	if err == nil || batch != nil {
		t.Fatalf("ASSERT_PREPARED_MECHANICS_V2_BATCH_ZERO_PARTIAL batch=%v err=%v", batch, err)
	}

	multiplyInvalid := base
	multiplyInvalid.WorkflowKey = ""
	multiplyInvalid.ReplayOrdinal = 0
	if _, err := programcrepresentative.NewMechanicsV2(multiplyInvalid); err == nil || err.Error() != "invalid v2 mechanics input" {
		t.Fatalf("ASSERT_MECHANICS_V2_LEGACY_ERROR_PRECEDENCE err=%v", err)
	}
	runInvalid := base
	runInvalid.ReplayOrdinal = 0
	if _, err := programcrepresentative.NewMechanicsV2(runInvalid); err == nil || err.Error() != "invalid v2 mechanics run identity" {
		t.Fatalf("ASSERT_MECHANICS_V2_LEGACY_RUN_ERROR_PRECEDENCE err=%v", err)
	}
}

func preparedMechanicsV2Common(in programcrepresentative.MechanicsInputV2) programcrepresentative.MechanicsCommonInputV2 {
	return programcrepresentative.MechanicsCommonInputV2{
		WorkflowKey: in.WorkflowKey, TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector,
		ReceiptSelector: in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal,
		ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector,
		ConstituentSelector: in.ConstituentSelector, Source: in.Source,
		ProgramCAlgorithmVersion: in.ProgramCAlgorithmVersion,
	}
}

func preparedMechanicsV2Contexts(base programcrepresentative.MechanicsInputV2, count int) []programcrepresentative.MechanicsContextInputV2 {
	out := make([]programcrepresentative.MechanicsContextInputV2, count)
	for i := range out {
		seed := uint64(i%2 + 1)
		replay := uint8(i%3 + 1)
		out[i] = programcrepresentative.MechanicsContextInputV2{
			RunID: fmt.Sprintf("seed-%020d-replay-%d", seed, replay), Seed: seed, ReplayOrdinal: replay,
			CommunityOrdinal: i, CommunityArtifactSHA256: "sha256:" + strings.Repeat(string("5678"[i%4]), 64),
			CommunityMembers: append([]string(nil), base.CommunityMembers...),
		}
	}
	return out
}

func preparedMechanicsV2Legacy(common programcrepresentative.MechanicsCommonInputV2, context programcrepresentative.MechanicsContextInputV2) programcrepresentative.MechanicsInputV2 {
	return programcrepresentative.MechanicsInputV2{
		WorkflowKey: common.WorkflowKey, TerminalProgramCHeadSelector: common.TerminalProgramCHeadSelector,
		ReceiptSelector: common.ReceiptSelector, ReceiptOrdinal: common.ReceiptOrdinal,
		ConstituentOrdinal: common.ConstituentOrdinal, PublicationSelector: common.PublicationSelector,
		ConstituentSelector: common.ConstituentSelector, Source: common.Source,
		ProgramCAlgorithmVersion: common.ProgramCAlgorithmVersion,
		RunID:                    context.RunID, Seed: context.Seed, ReplayOrdinal: context.ReplayOrdinal,
		CommunityOrdinal: context.CommunityOrdinal, CommunityArtifactSHA256: context.CommunityArtifactSHA256,
		CommunityMembers: append([]string(nil), context.CommunityMembers...),
	}
}
