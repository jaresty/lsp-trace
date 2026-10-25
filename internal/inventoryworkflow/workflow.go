// Package inventoryworkflow composes the qualified private Program C and
// inventory-successor boundaries without exposing transport or stage controls.
package inventoryworkflow

import (
	"bytes"
	"context"
	"errors"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programctransitionhost"
	"lsp-trace/internal/publication"
)

type CompletionRequest = programctransitionhost.CompletionRequest

type Request struct {
	ProgramC             programctransitionhost.RepeatedRequest
	SuccessorWorkflowKey string
	SuccessorHeadKey     string
	Identity             inventorysuccessor.WorkflowIdentity
}

type Result struct {
	Outcome                inventorysuccessor.Outcome
	CurrentStage           inventorysuccessor.Stage
	LastCompletedStage     inventorysuccessor.Stage
	ProgramCStatus         censuscontinuation.ProgramCTransitionStatus
	ProgramCTransitions    int
	Successor              inventorysuccessor.Result
	GapLedger              []byte
	Authority              uint8
	Accepted               bool
	FeatureIdentity        string
	Completeness           string
	Effects                string
	ImplementationDecision string
	ProductionAdmission    string
	SourceGraphComplete    string
	AbsenceInference       string
}

type stateHandle struct {
	root    *publication.Root
	private any
}

type workflowDeps struct {
	runProgramC         func(context.Context, programctransitionhost.RepeatedRequest) (programctransitionhost.RepeatedResult, error)
	open                func() (stateHandle, error)
	verifyProgramC      func(context.Context, stateHandle, CompletionRequest) (censuscontinuation.ProgramCTransitionResult, error)
	coordinateSuccessor func(context.Context, stateHandle, Request) (inventorysuccessor.Result, error)
	buildLedger         func(context.Context, stateHandle, Request) ([]byte, error)
	close               func(stateHandle) error
}

func baseResult(outcome inventorysuccessor.Outcome) Result {
	return Result{
		Outcome: outcome, Authority: 0, Accepted: false, FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN",
		Effects: "NONE", ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE",
		SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED",
	}
}

func Run(ctx context.Context, request Request) (Result, error) {
	if request.ProgramC.OpenState == nil {
		return Result{}, inventorysuccessor.ErrInvalid
	}
	return run(ctx, request, workflowDeps{
		runProgramC: programctransitionhost.RunRepeated,
		open: func() (stateHandle, error) {
			root, err := request.ProgramC.OpenState()
			return stateHandle{root: root}, err
		},
		verifyProgramC: func(ctx context.Context, state stateHandle, completion CompletionRequest) (censuscontinuation.ProgramCTransitionResult, error) {
			completion.StateRoot = state.root
			return programctransitionhost.VerifyCompletion(ctx, completion)
		},
		coordinateSuccessor: coordinateSuccessor,
		buildLedger:         buildLedger,
		close: func(state stateHandle) error {
			if state.root == nil {
				return inventorysuccessor.ErrInvalid
			}
			return state.root.Close()
		},
	})
}

func run(ctx context.Context, request Request, deps workflowDeps) (Result, error) {
	if ctx == nil || request.ProgramC.WorkflowKey == "" || request.ProgramC.HeadKey == "" || request.SuccessorWorkflowKey == "" || request.SuccessorHeadKey == "" ||
		deps.runProgramC == nil || deps.open == nil || deps.verifyProgramC == nil || deps.coordinateSuccessor == nil || deps.buildLedger == nil || deps.close == nil {
		return Result{}, inventorysuccessor.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return baseResult(inventorysuccessor.OutcomeCancelled), nil
	}
	programC, err := deps.runProgramC(ctx, request.ProgramC)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return baseResult(inventorysuccessor.OutcomeCancelled), nil
		}
		return Result{}, err
	}
	result := baseResult(inventorysuccessor.OutcomeAdvanced)
	result.ProgramCStatus = programC.Final.Status
	result.ProgramCTransitions = len(programC.Transitions)
	result.CurrentStage = inventorysuccessor.StageProgramC
	switch programC.Final.Status {
	case censuscontinuation.ProgramCTransitionCommitted:
		return result, nil
	case censuscontinuation.ProgramCTransitionRetained:
		result.Outcome = inventorysuccessor.OutcomeRetainedFailure
		return result, nil
	case censuscontinuation.ProgramCTransitionConflict:
		result.Outcome = inventorysuccessor.OutcomeConflict
		return result, nil
	case censuscontinuation.ProgramCTransitionIndeterminate:
		result.Outcome = inventorysuccessor.OutcomeIndeterminateExecution
		return result, nil
	case censuscontinuation.ProgramCTransitionComplete:
	default:
		return Result{}, inventorysuccessor.ErrInvalid
	}
	completion := CompletionRequest{
		Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger,
		WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings,
	}
	first, err := deps.open()
	if err != nil {
		return Result{}, err
	}
	verified, verifyErr := deps.verifyProgramC(ctx, first, completion)
	if verifyErr == nil && (verified.Status != censuscontinuation.ProgramCTransitionComplete || verified.CommittedPrefix != verified.ConstituentCount || verified.ConstituentCount <= 0) {
		verifyErr = inventorysuccessor.ErrInvalid
	}
	var successor inventorysuccessor.Result
	if verifyErr == nil {
		successor, verifyErr = deps.coordinateSuccessor(ctx, first, request)
	}
	closeErr := deps.close(first)
	if verifyErr != nil || closeErr != nil {
		return Result{}, errors.Join(verifyErr, closeErr)
	}
	result.Successor = successor
	result.Outcome = successor.Outcome
	result.CurrentStage = successor.CurrentStage
	result.LastCompletedStage = successor.LastCompletedStage
	if successor.Outcome != inventorysuccessor.OutcomeWaitingEvidence {
		return result, nil
	}
	second, err := deps.open()
	if err != nil {
		return Result{}, err
	}
	ledger, ledgerErr := deps.buildLedger(ctx, second, request)
	closeErr = deps.close(second)
	if ledgerErr != nil || closeErr != nil {
		return Result{}, errors.Join(ledgerErr, closeErr)
	}
	result.GapLedger = bytes.Clone(ledger)
	return result, nil
}

func coordinateSuccessor(ctx context.Context, state stateHandle, request Request) (inventorysuccessor.Result, error) {
	if state.root == nil {
		return inventorysuccessor.Result{}, inventorysuccessor.ErrInvalid
	}
	verification, err := programctransitionhost.NewCompletionVerificationRequest(ctx, CompletionRequest{
		StateRoot: state.root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger,
		WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings,
	})
	if err != nil {
		return inventorysuccessor.Result{}, err
	}
	heads, err := inventorysuccessor.NewHeads(state.root)
	if err != nil {
		return inventorysuccessor.Result{}, err
	}
	return inventorysuccessor.CoordinateNext(ctx, inventorysuccessor.Request{
		WorkflowKey: request.SuccessorWorkflowKey, HeadKey: request.SuccessorHeadKey,
		Identity: request.Identity, ProgramC: verification, Heads: heads,
	})
}

func buildLedger(ctx context.Context, state stateHandle, request Request) ([]byte, error) {
	if state.root == nil {
		return nil, inventorysuccessor.ErrInvalid
	}
	heads, err := inventorysuccessor.NewHeads(state.root)
	if err != nil {
		return nil, err
	}
	return inventorysuccessor.BuildDescribePreparationGapLedger(ctx, inventorysuccessor.GapLedgerRequest{
		WorkflowKey: request.SuccessorWorkflowKey, HeadKey: request.SuccessorHeadKey, Identity: request.Identity, Heads: heads,
	})
}
