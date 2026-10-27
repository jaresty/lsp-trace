package censuscontinuation

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/programc"
)

func TestProgramCV2ImmutableHeadPreflightJointNearLimitSuccess(t *testing.T) {
	planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 64)
	objectSnapshot := cloneProgramCV2ObjectMap(objects.m)
	headSnapshot := bytes.Clone(heads.raw)
	puts, swaps, loads := objects.puts, heads.cas, heads.loads

	got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
	if err != nil || got.transition.Status != ProgramCTransitionComplete || len(got.adaptations) != 1 || got.accounting.completeConstituents != 1 || got.accounting.emptyConstituents != 0 || got.accounting.resourceIncompleteConstituents != 0 || got.accounting.eligibleRunCommunityResultsD != expected.eligibleRunCommunityResultsD || got.accounting.seedGroupResultsG != expected.seedGroupResultsG {
		t.Fatalf("ASSERT_PROGRAM_C_V2_IMMUTABLE_PREFLIGHT_SUCCESS got=%+v err=%v", got, err)
	}
	if objects.puts != puts || heads.cas != swaps || heads.loads != loads+1 || !reflect.DeepEqual(objects.m, objectSnapshot) || !bytes.Equal(heads.raw, headSnapshot) {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_READ_ONLY puts=%d/%d cas=%d/%d loads=%d/%d", objects.puts, puts, heads.cas, swaps, heads.loads, loads+1)
	}
	adaptation := got.adaptations[0]
	if adaptation.classification != programCStage6Complete || len(adaptation.mechanics) == 0 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_NEAR_LIMIT_ADAPTATION got=%+v", adaptation)
	}
	for i, mechanics := range adaptation.mechanics {
		if !mechanics.Valid() || !mechanics.Trusted() || len(mechanics.SelectionContext().SeedGroups) != 64 {
			t.Fatalf("ASSERT_PROGRAM_C_V2_NEAR_LIMIT_MECHANICS index=%d", i)
		}
	}
	measurement, err := measureProgramCV2Source(programCQualificationMultiSeedEntryFixture(t, 64))
	if err != nil || len(measurement.Groups) != 64 || measurement.MembershipCount == 0 || measurement.SumGroupOccurrences == 0 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_NEAR_LIMIT_SOURCE measurement=%+v err=%v", measurement, err)
	}
}

func TestProgramCV2ImmutableHeadPreflightInjectedFailuresHaveZeroExposure(t *testing.T) {
	assertZero := func(t *testing.T, got programCV2PreflightResult, err error) {
		t.Helper()
		if err == nil || got.transition != (ProgramCTransitionResult{}) || got.adaptations != nil || got.accounting != (programCV2PreflightAccounting{}) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_INJECTED_FAILURE_ZERO_EXPOSURE got=%+v err=%v", got, err)
		}
	}
	assertReadOnly := func(t *testing.T, objects *transitionObjects, heads *transitionHeads, objectSnapshot map[string][]byte, headSnapshot []byte, puts, swaps int) {
		t.Helper()
		if objects.puts != puts || heads.cas != swaps || !reflect.DeepEqual(objects.m, objectSnapshot) || !bytes.Equal(heads.raw, headSnapshot) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_INJECTED_FAILURE_READ_ONLY puts=%d/%d cas=%d/%d", objects.puts, puts, heads.cas, swaps)
		}
	}
	t.Run("head snapshot", func(t *testing.T) {
		planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 2)
		sentinel := errors.New("injected head snapshot failure")
		heads.loadErr = sentinel
		objectSnapshot, headSnapshot, puts, swaps := cloneProgramCV2ObjectMap(objects.m), bytes.Clone(heads.raw), objects.puts, heads.cas
		got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
		assertZero(t, got, err)
		assertReadOnly(t, objects, heads, objectSnapshot, headSnapshot, puts, swaps)
		if !errors.Is(err, sentinel) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_HEAD_FAILURE_TYPED err=%v", err)
		}
	})
	t.Run("source snapshot generation", func(t *testing.T) {
		planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 2)
		runner.resolveErr = errors.New("injected source snapshot failure")
		objectSnapshot, headSnapshot, puts, swaps := cloneProgramCV2ObjectMap(objects.m), bytes.Clone(heads.raw), objects.puts, heads.cas
		got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
		assertZero(t, got, err)
		assertReadOnly(t, objects, heads, objectSnapshot, headSnapshot, puts, swaps)
		if !errors.Is(err, ErrInvalidProgramCTransition) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_SOURCE_FAILURE_TYPED err=%v", err)
		}
	})
	t.Run("census invariant", func(t *testing.T) {
		planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 2)
		for selector, raw := range objects.m {
			corrupt := bytes.Clone(raw)
			corrupt[len(corrupt)/2] ^= 1
			objects.m[selector] = corrupt
			break
		}
		objectSnapshot, headSnapshot, puts, swaps := cloneProgramCV2ObjectMap(objects.m), bytes.Clone(heads.raw), objects.puts, heads.cas
		got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
		assertZero(t, got, err)
		assertReadOnly(t, objects, heads, objectSnapshot, headSnapshot, puts, swaps)
		if !errors.Is(err, ErrInvalidProgramCTransition) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_INVARIANT_FAILURE_TYPED err=%v", err)
		}
	})
	t.Run("returned entries", func(t *testing.T) {
		planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 2)
		expected.eligibleRunCommunityResultsD++
		objectSnapshot, headSnapshot, puts, swaps := cloneProgramCV2ObjectMap(objects.m), bytes.Clone(heads.raw), objects.puts, heads.cas
		got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
		assertZero(t, got, err)
		assertReadOnly(t, objects, heads, objectSnapshot, headSnapshot, puts, swaps)
		if !errors.Is(err, ErrInvalidProgramCTransition) {
			t.Fatalf("ASSERT_PROGRAM_C_V2_RETURNED_ENTRY_FAILURE_TYPED err=%v", err)
		}
	})
	t.Run("commit write", func(t *testing.T) {
		planRaw := transitionPlan(t, records2()[:1])
		entry := programCQualificationMultiSeedEntryFixture(t, 2)
		runner := &programCV2PreflightFixtureRunner{entry: entry}
		objects, heads := &transitionObjects{}, &transitionHeads{}
		sentinel := errors.New("injected immutable write failure")
		objects.putErr = sentinel
		request := transitionRequest(planRaw, objects, heads, runner)
		got, err := CoordinateProgramCTransition(context.Background(), request)
		if !errors.Is(err, sentinel) || got != (ProgramCTransitionResult{}) || len(objects.m) != 0 {
			t.Fatalf("ASSERT_PROGRAM_C_V2_WRITE_FAILURE_ZERO_EXPOSURE got=%+v objects=%d err=%v", got, len(objects.m), err)
		}
	})
}

func TestProgramCV2PreflightFinalizesOnlyExactAccounting(t *testing.T) {
	entry := programCQualificationMultiSeedEntryFixture(t, 5)
	var d uint64
	for _, outcome := range entry.outcomes {
		d += uint64(len(outcome.Communities))
	}
	expected := programCV2ExpectedAccounting{eligibleRunCommunityResultsD: d, seedGroupResultsG: d * 5}
	transition := ProgramCTransitionResult{Status: ProgramCTransitionComplete, Head: censussourcecontinuation.HeadRevision{Generation: 1, Selector: "sha256:" + strings.Repeat("f", 64)}}
	got, err := finalizeProgramCMultiSeedV2Preflight(context.Background(), transition, []programCTransitionReceiptInventoryEntry{entry}, expected)
	if err != nil || got.transition != transition || len(got.adaptations) != 1 || got.accounting.completeConstituents != 1 || got.accounting.eligibleRunCommunityResultsD != d || got.accounting.seedGroupResultsG != d*5 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_EXACT_ACCOUNTING got=%+v err=%v", got, err)
	}
	if got.adaptations[0].classification != programCStage6Complete || uint64(len(got.adaptations[0].mechanics)) != d {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_COMPLETE_ADAPTATION got=%+v", got.adaptations[0])
	}
	mismatch, err := finalizeProgramCMultiSeedV2Preflight(context.Background(), transition, []programCTransitionReceiptInventoryEntry{entry}, programCV2ExpectedAccounting{eligibleRunCommunityResultsD: d + 1, seedGroupResultsG: d * 5})
	if err == nil || mismatch.transition != (ProgramCTransitionResult{}) || mismatch.adaptations != nil || mismatch.accounting != (programCV2PreflightAccounting{}) {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_ACCOUNTING_MISMATCH_ZERO_EXPOSURE got=%+v err=%v", mismatch, err)
	}
}

func TestProgramCV2PreflightCancellationIsOneLoadReadOnlyAndZeroExposure(t *testing.T) {
	planRaw := transitionPlan(t, records2()[:1])
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	if _, err := CoordinateProgramCTransition(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	heads.loads = 0
	puts, swaps := objects.puts, heads.cas
	got, err := preflightProgramCMultiSeedV2WithRunner(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, programCV2ExpectedAccounting{})
	if !errors.Is(err, context.Canceled) || heads.loads != 1 || objects.puts != puts || heads.cas != swaps || got.transition != (ProgramCTransitionResult{}) || got.adaptations != nil || got.accounting != (programCV2PreflightAccounting{}) {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_CANCEL_ZERO_EXPOSURE got=%+v loads=%d puts=%d/%d cas=%d/%d err=%v", got, heads.loads, objects.puts, puts, heads.cas, swaps, err)
	}
}

func TestProgramCV2PreflightTerminalMismatchIsOneLoadReadOnlyAndZeroExposure(t *testing.T) {
	planRaw := transitionPlan(t, records2()[:1])
	objects, heads := &transitionObjects{}, &transitionHeads{}
	runner := &transitionRunner{outcome: "COMPLETE", result: transitionResultBytes("STABLE")}
	request := transitionRequest(planRaw, objects, heads, runner)
	if _, err := CoordinateProgramCTransition(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	heads.raw = append([]byte{}, heads.raw...)
	heads.raw[len(heads.raw)-2] ^= 1
	heads.loads = 0
	puts, swaps := objects.puts, heads.cas
	got, err := preflightProgramCMultiSeedV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, programCV2ExpectedAccounting{})
	if err == nil || heads.loads != 1 || objects.puts != puts || heads.cas != swaps || got.transition != (ProgramCTransitionResult{}) || got.adaptations != nil || got.accounting != (programCV2PreflightAccounting{}) {
		t.Fatalf("ASSERT_PROGRAM_C_V2_PREFLIGHT_TERMINAL_MISMATCH_ZERO_EXPOSURE got=%+v loads=%d puts=%d/%d cas=%d/%d err=%v", got, heads.loads, objects.puts, puts, heads.cas, swaps, err)
	}
}

type programCV2PreflightFixtureRunner struct {
	entry      programCTransitionReceiptInventoryEntry
	index      int
	resolveErr error
}

func (r *programCV2PreflightFixtureRunner) Resolve(context.Context, string, string) ([]byte, error) {
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}
	r.index = 0
	return bytes.Clone(r.entry.nativeV5), nil
}

func (r *programCV2PreflightFixtureRunner) Compute(_ []byte, seed uint64) (programc.Outcome, *programc.Failure) {
	if r.index >= len(r.entry.outcomes) || r.entry.outcomes[r.index].Seed != seed {
		return programc.Outcome{}, &programc.Failure{Code: programc.CodeComputation, Message: "fixture sequence mismatch"}
	}
	out := cloneProgramCOutcomes(r.entry.outcomes[r.index : r.index+1])[0]
	r.index++
	return out, nil
}

func (r *programCV2PreflightFixtureRunner) Instability([]programc.Outcome, ProgramCTransitionBindings) ([]byte, error) {
	return bytes.Clone(r.entry.value.ResultBytes), nil
}

func (r *programCV2PreflightFixtureRunner) ValidateInstability(raw []byte, _ []programc.Outcome, _ ProgramCTransitionBindings) error {
	if !bytes.Equal(raw, r.entry.value.ResultBytes) || transitionValidateResult(raw) != nil {
		return ErrInvalidProgramCTransition
	}
	return nil
}

func programCV2CommittedFixture(t *testing.T, seedCount int) ([]byte, *transitionObjects, *transitionHeads, *programCV2PreflightFixtureRunner, ProgramCTransitionRequest, programCV2ExpectedAccounting) {
	t.Helper()
	planRaw := transitionPlan(t, records2()[:1])
	entry := programCQualificationMultiSeedEntryFixture(t, seedCount)
	runner := &programCV2PreflightFixtureRunner{entry: entry}
	objects, heads := &transitionObjects{}, &transitionHeads{}
	request := transitionRequest(planRaw, objects, heads, runner)
	committed, err := CoordinateProgramCTransition(context.Background(), request)
	if err != nil || committed.Status != ProgramCTransitionCommitted {
		t.Fatalf("commit v2 preflight fixture: got=%+v err=%v", committed, err)
	}
	var communities uint64
	for _, outcome := range entry.outcomes {
		communities += uint64(len(outcome.Communities))
	}
	return planRaw, objects, heads, runner, request, programCV2ExpectedAccounting{eligibleRunCommunityResultsD: communities, seedGroupResultsG: communities * uint64(seedCount)}
}

func cloneProgramCV2ObjectMap(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for selector, raw := range in {
		out[selector] = bytes.Clone(raw)
	}
	return out
}

func TestProgramCV2FrozenAccountingConstants(t *testing.T) {
	if programCV2FrozenEligibleRunCommunityResultsD != 15537 || programCV2FrozenSeedGroupResultsG != 530232 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_FROZEN_D_G D=%d G=%d", programCV2FrozenEligibleRunCommunityResultsD, programCV2FrozenSeedGroupResultsG)
	}
}
