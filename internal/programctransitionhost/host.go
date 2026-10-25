// Package programctransitionhost is a private, non-scheduled construction seam
// for invoking exactly one authoritative Program C transition.
package programctransitionhost

import (
	"context"
	"errors"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

const headNamespace = "census-program-c-transition/heads/private/v1"

type CompletionRequest struct {
	StateRoot   *publication.Root
	Receipt     censuscontinuation.ExactProgramCTransitionInput
	Ledger      censuscontinuation.ExactProgramCTransitionInput
	WorkflowKey string
	HeadKey     string
	Bindings    censuscontinuation.ProgramCTransitionBindings
}

type Request struct {
	StateRoot   *publication.Root
	Receipt     censuscontinuation.ExactProgramCTransitionInput
	Ledger      censuscontinuation.ExactProgramCTransitionInput
	WorkflowKey string
	HeadKey     string
	Expected    censussourcecontinuation.HeadRevision
	Bindings    censuscontinuation.ProgramCTransitionBindings
}

// NewCompletionVerificationRequest constructs read-only production authorities
// for proving terminal Program C custody in the private head namespace.
func NewCompletionVerificationRequest(ctx context.Context, request CompletionRequest) (censuscontinuation.ProgramCTransitionVerificationRequest, error) {
	if ctx == nil || request.StateRoot == nil || request.WorkflowKey == "" || request.HeadKey == "" {
		return censuscontinuation.ProgramCTransitionVerificationRequest{}, censuscontinuation.ErrInvalidProgramCTransition
	}
	source := &censuscontinuation.AuthoritativeProgramCTransitionPlanSource{Receipt: request.Receipt, Ledger: request.Ledger}
	planRaw, err := source.Load(ctx)
	if err != nil {
		return censuscontinuation.ProgramCTransitionVerificationRequest{}, err
	}
	request.Bindings.PlanSHA256 = digest(planRaw)
	objects, err := censussourceassembly.NewObjects(request.StateRoot)
	if err != nil {
		return censuscontinuation.ProgramCTransitionVerificationRequest{}, err
	}
	heads, err := censussourceassembly.NewHeadsWithParser(request.StateRoot, headNamespace, censuscontinuation.ParseProgramCTransitionHeadRevision)
	if err != nil {
		return censuscontinuation.ProgramCTransitionVerificationRequest{}, err
	}
	return censuscontinuation.ProgramCTransitionVerificationRequest{
		WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey, Bindings: request.Bindings,
		PlanSource: source, Objects: objects, Heads: heads,
	}, nil
}

func VerifyCompletion(ctx context.Context, request CompletionRequest) (censuscontinuation.ProgramCTransitionResult, error) {
	verification, err := NewCompletionVerificationRequest(ctx, request)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	return censuscontinuation.VerifyProgramCTransitionCompletion(ctx, verification)
}

// InvokeOne constructs all production authorities and advances at most one
// transition. It is intentionally not connected to public command dispatch.
func InvokeOne(ctx context.Context, request Request) (censuscontinuation.ProgramCTransitionResult, error) {
	if ctx == nil || request.StateRoot == nil {
		return censuscontinuation.ProgramCTransitionResult{}, censuscontinuation.ErrInvalidProgramCTransition
	}
	source := &censuscontinuation.AuthoritativeProgramCTransitionPlanSource{Receipt: request.Receipt, Ledger: request.Ledger}
	planRaw, err := source.Load(ctx)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	request.Bindings.PlanSHA256 = digest(planRaw)
	objects, err := censussourceassembly.NewObjects(request.StateRoot)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	heads, err := censussourceassembly.NewHeadsWithParser(request.StateRoot, headNamespace, censuscontinuation.ParseProgramCTransitionHeadRevision)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	result, err := censuscontinuation.CoordinateProgramCTransition(ctx, censuscontinuation.ProgramCTransitionRequest{
		WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey, Expected: request.Expected,
		Bindings: request.Bindings, PlanSource: source, Objects: objects, Heads: heads,
		Runner: censuscontinuation.NewProgramCProductionRunner(source),
	})
	if errors.Is(err, censuscontinuation.ErrInvalidProgramCTransition) {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	return result, err
}

func digest(raw []byte) string {
	// Reuse the immutable-store selector implementation, whose contract is the
	// same canonical sha256:<lowerhex> identity required by transition bindings.
	return censussourceassembly.ObjectSelector(raw)
}
