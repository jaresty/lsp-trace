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

type preparedInvocation struct {
	source   *censuscontinuation.AuthoritativeProgramCTransitionPlanSource
	plan     verifiedPlanSource
	bindings censuscontinuation.ProgramCTransitionBindings
}

type verifiedPlanSource struct{ raw []byte }

func (s verifiedPlanSource) Load(context.Context) ([]byte, error) {
	return append([]byte(nil), s.raw...), nil
}

func prepareInvocation(ctx context.Context, receipt, ledger censuscontinuation.ExactProgramCTransitionInput, bindings censuscontinuation.ProgramCTransitionBindings) (preparedInvocation, error) {
	if ctx == nil {
		return preparedInvocation{}, censuscontinuation.ErrInvalidProgramCTransition
	}
	source := &censuscontinuation.AuthoritativeProgramCTransitionPlanSource{Receipt: receipt, Ledger: ledger}
	planRaw, err := source.Load(ctx)
	if err != nil {
		return preparedInvocation{}, err
	}
	bindings.PlanSHA256 = digest(planRaw)
	return preparedInvocation{source: source, plan: verifiedPlanSource{raw: append([]byte(nil), planRaw...)}, bindings: bindings}, nil
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
	prepared, err := prepareInvocation(ctx, request.Receipt, request.Ledger, request.Bindings)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	return invokePrepared(ctx, request.StateRoot, request.WorkflowKey, request.HeadKey, request.Expected, prepared)
}

func invokePrepared(ctx context.Context, root *publication.Root, workflowKey, headKey string, expected censussourcecontinuation.HeadRevision, prepared preparedInvocation) (censuscontinuation.ProgramCTransitionResult, error) {
	if ctx == nil || root == nil || prepared.source == nil || len(prepared.plan.raw) == 0 {
		return censuscontinuation.ProgramCTransitionResult{}, censuscontinuation.ErrInvalidProgramCTransition
	}
	objects, err := censussourceassembly.NewObjects(root)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	heads, err := censussourceassembly.NewHeadsWithParser(root, headNamespace, censuscontinuation.ParseProgramCTransitionHeadRevision)
	if err != nil {
		return censuscontinuation.ProgramCTransitionResult{}, err
	}
	result, err := censuscontinuation.CoordinateProgramCTransition(ctx, censuscontinuation.ProgramCTransitionRequest{
		WorkflowKey: workflowKey, HeadKey: headKey, Expected: expected,
		Bindings: prepared.bindings, PlanSource: prepared.plan, Objects: objects, Heads: heads,
		Runner: censuscontinuation.NewProgramCProductionRunner(prepared.source),
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
