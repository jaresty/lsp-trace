package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/incomingops"
	"lsp-trace/internal/operation"
)

const contextSymbolChurnCaptureOperation operation.Name = "context_symbol_churn_capture"

// buildSymbolChurnFromStructural is the byte-preserving composition seam between
// Structural Context V2 acquisition and symbol-churn construction.
func buildSymbolChurnFromStructural(raw []byte, build func([]byte) (string, error)) (string, error) {
	return build(raw)
}

type contextSymbolChurnCaptureInput struct {
	structuralContextInput
	FromRevision string `json:"from_revision"`
	ToRevision   string `json:"to_revision"`
	Profile      string `json:"profile"`
	LanguageID   string `json:"language_id"`
}

type contextSymbolChurnCaptureOperationExecutor interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type contextSymbolChurnCaptureExecutor struct {
	structural       contextSymbolChurnCaptureOperationExecutor
	churn            contextSymbolChurnCaptureOperationExecutor
	resolveWorkspace func(string, uint64) (string, *operation.Failure)
}

func newContextSymbolChurnCaptureExecutor(structural *structuralContextExecutor, churn *contextSymbolChurnExecutor) *contextSymbolChurnCaptureExecutor {
	return &contextSymbolChurnCaptureExecutor{
		structural: structural,
		churn:      churn,
		resolveWorkspace: func(sessionID string, generation uint64) (string, *operation.Failure) {
			id, resolvedGeneration, state := incomingops.ResolveSession(structural.runtime, sessionID, generation)
			if state != "" {
				return "", &operation.Failure{Code: string(state), Err: errors.New(string(state))}
			}
			workspace, ok := structural.runtime.Manager.WorkspaceRoot(id, resolvedGeneration)
			if !ok {
				return "", &operation.Failure{Code: "WORKSPACE_UNAVAILABLE", Err: errors.New("managed session workspace unavailable")}
			}
			return workspace, nil
		},
	}
}

func preserveStructuralFailure(failure *operation.Failure) (operation.Result, *operation.Failure) {
	return operation.Result{}, failure
}

func (e *contextSymbolChurnCaptureExecutor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.structural == nil || e.churn == nil || op.Name != contextSymbolChurnCaptureOperation {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in contextSymbolChurnCaptureInput
	if err := json.Unmarshal(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	structuralInput, err := json.Marshal(in.structuralContextInput)
	if err != nil {
		return fail(operation.FailureInternal, err)
	}
	acquired, failure := e.structural.Execute(ctx, operation.Request{Name: operation.Name("structural_context_v2"), Input: structuralInput})
	if failure != nil {
		return preserveStructuralFailure(failure)
	}
	if e.resolveWorkspace == nil {
		return fail(operation.FailureInternal, errors.New("workspace resolver unavailable"))
	}
	workspace, workspaceFailure := e.resolveWorkspace(in.SessionID, in.Generation)
	if workspaceFailure != nil {
		return operation.Result{}, workspaceFailure
	}
	var result operation.Result
	_, err = buildSymbolChurnFromStructural(acquired.Artifact, func(raw []byte) (string, error) {
		churnInput, marshalErr := json.Marshal(contextSymbolChurnInput{Input: string(raw), Workspace: workspace, FromRevision: in.FromRevision, ToRevision: in.ToRevision, Profile: in.Profile, LanguageID: in.LanguageID, TimeoutMS: int(in.TimeoutMS), RequestTimeoutMS: int(in.RequestTimeoutMS)})
		if marshalErr != nil {
			return "", marshalErr
		}
		result, failure = e.churn.Execute(ctx, operation.Request{Name: operation.Name("context_symbol_churn"), Input: churnInput})
		if failure != nil {
			return "", failure.Err
		}
		return string(result.Artifact), nil
	})
	if failure != nil {
		return operation.Result{}, failure
	}
	if err != nil {
		return fail("SYMBOL_CHURN_ACQUISITION_FAILED", err)
	}
	return result, nil
}
