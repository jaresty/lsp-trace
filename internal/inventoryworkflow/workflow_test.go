package inventoryworkflow

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programctransitionhost"
	"lsp-trace/internal/publication"
)

type fakeRoot struct{ id int }

type workflowDepsFixture struct {
	events []string
	final  censuscontinuation.ProgramCTransitionResult
	runErr error
}

func (f *workflowDepsFixture) deps() workflowDeps {
	next := 0
	return workflowDeps{
		runProgramC: func(context.Context, programctransitionhost.RepeatedRequest) (programctransitionhost.RepeatedResult, error) {
			f.events = append(f.events, "program-c")
			return programctransitionhost.RepeatedResult{Final: f.final}, f.runErr
		},
		open: func() (stateHandle, error) {
			next++
			f.events = append(f.events, "open")
			return stateHandle{private: &fakeRoot{id: next}}, nil
		},
		verifyProgramC: func(context.Context, stateHandle, CompletionRequest) (censuscontinuation.ProgramCTransitionResult, error) {
			f.events = append(f.events, "verify-program-c")
			return f.final, nil
		},
		coordinateSuccessor: func(context.Context, stateHandle, Request) (inventorysuccessor.Result, error) {
			f.events = append(f.events, "successor")
			return inventorysuccessor.Result{Outcome: inventorysuccessor.OutcomeWaitingEvidence, CurrentStage: inventorysuccessor.StageDescribePreparation, LastCompletedStage: inventorysuccessor.StageProgramC}, nil
		},
		buildLedger: func(context.Context, stateHandle, Request) ([]byte, error) {
			f.events = append(f.events, "ledger")
			return []byte("ledger"), nil
		},
		close: func(stateHandle) error {
			f.events = append(f.events, "close")
			return nil
		},
	}
}

func completeProgramC() censuscontinuation.ProgramCTransitionResult {
	return censuscontinuation.ProgramCTransitionResult{Status: censuscontinuation.ProgramCTransitionComplete, CommittedPrefix: 395, ConstituentCount: 395}
}

func validRequest() Request {
	return Request{
		ProgramC:             programctransitionhost.RepeatedRequest{WorkflowKey: "program-c", HeadKey: "program-c-head", MaxTransitions: 396},
		SuccessorWorkflowKey: "successor", SuccessorHeadKey: "successor-head",
		Identity: inventorysuccessor.WorkflowIdentity{Profile: "CALLS_ONLY_FROZEN_CENSUS_V1", Contract: "adr0012-private-successor-v1"},
	}
}

func TestRunReopensBetweenSuccessorAndLedger(t *testing.T) {
	fixture := &workflowDepsFixture{final: completeProgramC()}
	got, err := run(context.Background(), validRequest(), fixture.deps())
	if err != nil || got.Outcome != inventorysuccessor.OutcomeWaitingEvidence || string(got.GapLedger) != "ledger" || got.ProgramCStatus != censuscontinuation.ProgramCTransitionComplete {
		t.Fatalf("ASSERT_WORKFLOW_WAITING_EVIDENCE got=%+v err=%v", got, err)
	}
	want := []string{"program-c", "open", "verify-program-c", "successor", "close", "open", "ledger", "close"}
	if !reflect.DeepEqual(fixture.events, want) {
		t.Fatalf("ASSERT_WORKFLOW_CLOSE_REOPEN events=%v want=%v", fixture.events, want)
	}
}

func TestRunStopsAtProgramCTerminalsWithoutOpeningSuccessor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status censuscontinuation.ProgramCTransitionStatus
		want   inventorysuccessor.Outcome
	}{
		{name: "advanced", status: censuscontinuation.ProgramCTransitionCommitted, want: inventorysuccessor.OutcomeAdvanced},
		{name: "retained", status: censuscontinuation.ProgramCTransitionRetained, want: inventorysuccessor.OutcomeRetainedFailure},
		{name: "conflict", status: censuscontinuation.ProgramCTransitionConflict, want: inventorysuccessor.OutcomeConflict},
		{name: "indeterminate", status: censuscontinuation.ProgramCTransitionIndeterminate, want: inventorysuccessor.OutcomeIndeterminateExecution},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &workflowDepsFixture{final: censuscontinuation.ProgramCTransitionResult{Status: tc.status}}
			got, err := run(context.Background(), validRequest(), fixture.deps())
			if err != nil || got.Outcome != tc.want || !reflect.DeepEqual(fixture.events, []string{"program-c"}) {
				t.Fatalf("ASSERT_WORKFLOW_PROGRAM_C_STOP got=%+v events=%v err=%v", got, fixture.events, err)
			}
		})
	}
}

func TestRunCancellationIsTypedBeforeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fixture := &workflowDepsFixture{final: completeProgramC(), runErr: context.Canceled}
	got, err := run(ctx, validRequest(), fixture.deps())
	if err != nil || got.Outcome != inventorysuccessor.OutcomeCancelled || len(fixture.events) != 0 {
		t.Fatalf("ASSERT_WORKFLOW_CANCELLED got=%+v events=%v err=%v", got, fixture.events, err)
	}
}

func TestRunRetainedFailureExposesOnlyNonCustodialDiagnostics(t *testing.T) {
	fixture := &workflowDepsFixture{final: censuscontinuation.ProgramCTransitionResult{
		Status: censuscontinuation.ProgramCTransitionRetained, ProgramCOutcome: "COMPLETE",
		FailureStage: censuscontinuation.ProgramCTransitionFailureInstabilityBuild, Failure: "matching policy work limit",
	}}
	got, err := run(context.Background(), validRequest(), fixture.deps())
	if err != nil || got.Outcome != inventorysuccessor.OutcomeRetainedFailure || got.ProgramCOutcome != "COMPLETE" || got.ProgramCFailureStage != censuscontinuation.ProgramCTransitionFailureInstabilityBuild || got.ProgramCFailure != "matching policy work limit" || got.Successor.Head.Selector != "" || got.GapLedger != nil {
		t.Fatalf("ASSERT_WORKFLOW_RETAINED_DIAGNOSTICS got=%+v err=%v", got, err)
	}
}

func TestRunStopsOnSuccessorConflictBeforeLedgerReopen(t *testing.T) {
	fixture := &workflowDepsFixture{final: completeProgramC()}
	deps := fixture.deps()
	deps.coordinateSuccessor = func(context.Context, stateHandle, Request) (inventorysuccessor.Result, error) {
		fixture.events = append(fixture.events, "successor")
		return inventorysuccessor.Result{Outcome: inventorysuccessor.OutcomeConflict}, nil
	}
	got, err := run(context.Background(), validRequest(), deps)
	want := []string{"program-c", "open", "verify-program-c", "successor", "close"}
	if err != nil || got.Outcome != inventorysuccessor.OutcomeConflict || !reflect.DeepEqual(fixture.events, want) || got.GapLedger != nil {
		t.Fatalf("ASSERT_WORKFLOW_SUCCESSOR_CONFLICT_STOP got=%+v events=%v err=%v", got, fixture.events, err)
	}
}

func TestRequestHasNoCallerSequencingOrSemanticControls(t *testing.T) {
	typ := reflect.TypeOf(Request{})
	for _, field := range []string{"Ordinal", "Selector", "Expected", "CASHead", "RawPlan", "Processed", "RetryPolicy", "SkipPolicy", "Stage", "Outcome", "Candidate", "Group", "Acceptance"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("ASSERT_WORKFLOW_NO_CALLER_CONTROL field=%s", field)
		}
	}
}

func TestRunPreservesSuccessorCloseFailure(t *testing.T) {
	fixture := &workflowDepsFixture{final: completeProgramC()}
	deps := fixture.deps()
	deps.close = func(stateHandle) error { return errors.New("close failed") }
	if _, err := run(context.Background(), validRequest(), deps); err == nil {
		t.Fatal("ASSERT_WORKFLOW_CLOSE_FAILURE_RETAINED")
	}
}

var _ *publication.Root
