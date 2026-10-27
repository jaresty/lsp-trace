package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
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

const programCV2PreflightQualificationOptInEnv = "LSP_TRACE_ADR0014_V2_PREFLIGHT_QUALIFY"

func TestADR0014FrozenCorpusMultiSeedV2PreflightQualification(t *testing.T) {
	if os.Getenv(programCV2PreflightQualificationOptInEnv) != "1" {
		t.Skip("set LSP_TRACE_ADR0014_V2_PREFLIGHT_QUALIFY=1 with the exact receipt, ledger, and retained state")
	}
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	stateDir := os.Getenv("LSP_TRACE_PROGRAM_C_RETAINED_STATE")
	if receiptPath == "" || ledgerPath == "" || stateDir == "" {
		t.Fatal("ASSERT_ADR0014_V2_PREFLIGHT_EXACT_INPUT missing exact frozen input")
	}

	receipt, err := programCV2ExactInput(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := programCV2ExactInput(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SHA256 != "sha256:ecc4a9280e1b3896de6c9799485bc642111e4140ccfc0f458e03e6e25441a751" || receipt.ByteLength != 201291 || ledger.SHA256 != "sha256:2fe7d6d4713e29b85dfeb73b5d9bdf5b7edec39d2cc317f9588d414a0c372bee" || ledger.ByteLength != 236389 {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_EXACT_INPUT receipt=%s/%d ledger=%s/%d", receipt.SHA256, receipt.ByteLength, ledger.SHA256, ledger.ByteLength)
	}
	stateBefore, err := snapshotProgramCV2RetainedState(stateDir)
	if err != nil {
		t.Fatal(err)
	}

	source := &AuthoritativeProgramCTransitionPlanSource{Receipt: receipt, Ledger: ledger}
	planRaw, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bindings := ProgramCTransitionBindings{
		RepositoryCommit: strings.Repeat("a", 40), RepositoryTree: strings.Repeat("b", 40),
		FrozenCorpusSHA256: "sha256:" + strings.Repeat("c", 64), CensusReceiptSHA256: receipt.SHA256, LedgerSHA256: ledger.SHA256,
		AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3,
		ParametersSHA256: "sha256:" + strings.Repeat("d", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("e", 64),
		PlanSHA256: transitionSHA(planRaw),
	}
	root, err := publication.OpenRoot(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := censussourceassembly.NewObjects(root)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	heads, err := censussourceassembly.NewHeadsWithParser(root, programCV2MeasurementHeadNS, ParseProgramCTransitionHeadRevision)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	readOnlyObjects := &programCV2ReadOnlyObjects{inner: objects}
	readOnlyHeads := &programCV2ReadOnlyHeads{inner: heads}

	started := time.Now()
	got, preflightErr := preflightFrozenProgramCMultiSeedV2(context.Background(), ProgramCTransitionVerificationRequest{
		WorkflowKey: "production-workflow", HeadKey: "production-workflow", Bindings: bindings,
		PlanSource: source, Objects: readOnlyObjects, Heads: readOnlyHeads,
	})
	runtime := time.Since(started).Round(time.Millisecond)
	closeErr := root.Close()
	stateAfter, snapshotErr := snapshotProgramCV2RetainedState(stateDir)
	if preflightErr != nil {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_SUCCESS err=%v", preflightErr)
	}
	if closeErr != nil {
		t.Fatalf("close retained state: %v", closeErr)
	}
	if snapshotErr != nil {
		t.Fatal(snapshotErr)
	}

	if got.transition.Status != ProgramCTransitionComplete || got.transition.CommittedPrefix != 395 || got.transition.ConstituentCount != 395 || got.transition.Head.Selector == "" {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_TERMINAL_RECONSTRUCTION transition=%+v", got.transition)
	}
	if len(got.adaptations) != 395 || got.accounting.completeConstituents != 268 || got.accounting.emptyConstituents != 2 || got.accounting.resourceIncompleteConstituents != 125 || got.accounting.eligibleRunCommunityResultsD != 15537 || got.accounting.seedGroupResultsG != 530232 {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_ACCOUNTING adaptations=%d accounting=%+v", len(got.adaptations), got.accounting)
	}
	var mechanicsCount uint64
	for ordinal, adaptation := range got.adaptations {
		switch adaptation.classification {
		case programCStage6Complete:
			if len(adaptation.mechanics) == 0 {
				t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_COMPLETE_MECHANICS ordinal=%d empty", ordinal)
			}
			pairs := make(map[string]struct{}, len(adaptation.mechanics))
			for mechanicsOrdinal, mechanics := range adaptation.mechanics {
				dto := mechanics.DTO()
				pair := fmt.Sprintf("%s/%d", dto.RunID, dto.CommunityOrdinal)
				if !mechanics.Valid() || !mechanics.Trusted() || dto.RunID == "" {
					t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_COMPLETE_MECHANICS ordinal=%d mechanics=%d dto=%+v", ordinal, mechanicsOrdinal, dto)
				}
				if _, duplicate := pairs[pair]; duplicate {
					t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_RUN_COMMUNITY_BIJECTION ordinal=%d pair=%s", ordinal, pair)
				}
				pairs[pair] = struct{}{}
			}
			mechanicsCount += uint64(len(adaptation.mechanics))
		case programCStage6Empty, programCStage6ResourceIncomplete:
			if len(adaptation.mechanics) != 0 {
				t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_EXCLUDED_MECHANICS ordinal=%d classification=%s mechanics=%d", ordinal, adaptation.classification, len(adaptation.mechanics))
			}
		default:
			t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_CLASSIFICATION ordinal=%d classification=%s", ordinal, adaptation.classification)
		}
	}
	if mechanicsCount != 15537 {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_MECHANICS_D got=%d", mechanicsCount)
	}
	if readOnlyHeads.loads != 1 || readOnlyObjects.puts != 0 || readOnlyHeads.cas != 0 {
		t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_READ_ONLY loads=%d puts=%d cas=%d", readOnlyHeads.loads, readOnlyObjects.puts, readOnlyHeads.cas)
	}
	if !reflect.DeepEqual(stateBefore, stateAfter) {
		t.Fatal("ASSERT_ADR0014_V2_PREFLIGHT_STATE_BYTES_UNCHANGED mismatch")
	}
	resultType := reflect.TypeOf(got)
	for i := 0; i < resultType.NumField(); i++ {
		name := strings.ToLower(resultType.Field(i).Name)
		if strings.Contains(name, "subject") || strings.Contains(name, "artifact") || strings.Contains(name, "evidence") {
			t.Fatalf("ASSERT_ADR0014_V2_PREFLIGHT_PREQUALIFICATION_ONLY field=%s", resultType.Field(i).Name)
		}
	}

	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_EXACT_INPUT PASS")
	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_TERMINAL_RECONSTRUCTION PASS")
	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_RUN_COMMUNITY_BIJECTION PASS")
	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_READ_ONLY PASS")
	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_STATE_BYTES_UNCHANGED PASS")
	t.Log("ASSERT_ADR0014_V2_PREFLIGHT_PREQUALIFICATION_ONLY PASS")
	t.Logf("ADR0014_V2_PREFLIGHT_QUALIFICATION runtime=%s adaptations=%d complete=%d empty=%d resource_incomplete=%d D=%d G=%d loads=%d puts=%d cas=%d", runtime, len(got.adaptations), got.accounting.completeConstituents, got.accounting.emptyConstituents, got.accounting.resourceIncompleteConstituents, got.accounting.eligibleRunCommunityResultsD, got.accounting.seedGroupResultsG, readOnlyHeads.loads, readOnlyObjects.puts, readOnlyHeads.cas)
}

type programCV2RetainedStateFile struct {
	ByteLength int64
	SHA256     [sha256.Size]byte
}

func snapshotProgramCV2RetainedState(root string) (map[string]programCV2RetainedStateFile, error) {
	snapshot := make(map[string]programCV2RetainedStateFile)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("retained state contains non-regular file: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = programCV2RetainedStateFile{ByteLength: int64(len(raw)), SHA256: sha256.Sum256(raw)}
		return nil
	})
	return snapshot, err
}

func TestProgramCV2DiagnosticFailuresRemainZeroAndTyped(t *testing.T) {
	t.Run("mechanics", func(t *testing.T) {
		err := validateProgramCV2Mechanics(17, []programcrepresentative.MechanicsV2{{}})
		if err == nil || !errors.Is(err, ErrInvalidProgramCTransition) || !strings.Contains(err.Error(), "category=mechanics-valid ordinal=17 mechanics_index=0") {
			t.Fatalf("ASSERT_PROGRAM_C_V2_MECHANICS_DIAGNOSTIC err=%v", err)
		}
	})
	accounting := programCV2PreflightAccounting{
		completeConstituents: 1, eligibleRunCommunityResultsD: 7, seedGroupResultsG: 11,
	}
	tests := []struct {
		name     string
		expected programCV2ExpectedAccounting
		want     string
	}{
		{"eligible run communities", programCV2ExpectedAccounting{eligibleRunCommunityResultsD: 8, seedGroupResultsG: 11}, "category=eligible-run-community-count observed=7 expected=8"},
		{"seed groups", programCV2ExpectedAccounting{eligibleRunCommunityResultsD: 7, seedGroupResultsG: 12}, "category=seed-group-count observed=11 expected=12"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProgramCV2FinalAccounting(accounting, 1, tc.expected)
			if err == nil || !errors.Is(err, ErrInvalidProgramCTransition) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ASSERT_PROGRAM_C_V2_ACCOUNTING_DIAGNOSTIC err=%v", err)
			}
			got := programCV2PreflightResult{}
			if got.transition != (ProgramCTransitionResult{}) || got.adaptations != nil || got.accounting != (programCV2PreflightAccounting{}) {
				t.Fatalf("ASSERT_PROGRAM_C_V2_DIAGNOSTIC_ZERO_RESULT got=%+v", got)
			}
		})
	}
}

func TestProgramCV2FrozenAccountingConstants(t *testing.T) {
	if programCV2FrozenEligibleRunCommunityResultsD != 15537 || programCV2FrozenSeedGroupResultsG != 530232 {
		t.Fatalf("ASSERT_PROGRAM_C_V2_FROZEN_D_G D=%d G=%d", programCV2FrozenEligibleRunCommunityResultsD, programCV2FrozenSeedGroupResultsG)
	}
}
