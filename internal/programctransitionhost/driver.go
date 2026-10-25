package programctransitionhost

import (
	"context"
	"errors"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

const MaxRepeatedTransitions = 4096

type RepeatedRequest struct {
	Receipt        censuscontinuation.ExactProgramCTransitionInput
	Ledger         censuscontinuation.ExactProgramCTransitionInput
	WorkflowKey    string
	HeadKey        string
	Bindings       censuscontinuation.ProgramCTransitionBindings
	MaxTransitions int
	OpenState      func() (*publication.Root, error)
}

type RepeatedResult struct {
	Transitions []censuscontinuation.ProgramCTransitionResult
	Final       censuscontinuation.ProgramCTransitionResult
}

type repeatedDriverDeps struct {
	open     func() (*publication.Root, error)
	revision func(context.Context, *publication.Root, string) (censussourcecontinuation.HeadRevision, error)
	step     func(context.Context, Request) (censuscontinuation.ProgramCTransitionResult, error)
	close    func(*publication.Root) error
}

func RunRepeated(ctx context.Context, request RepeatedRequest) (RepeatedResult, error) {
	return runRepeated(ctx, request, repeatedDriverDeps{
		open:     request.OpenState,
		revision: currentRevision,
		step:     InvokeOne,
		close:    func(root *publication.Root) error { return root.Close() },
	})
}

func runRepeated(ctx context.Context, request RepeatedRequest, deps repeatedDriverDeps) (RepeatedResult, error) {
	if ctx == nil || request.MaxTransitions < 1 || request.MaxTransitions > MaxRepeatedTransitions || deps.open == nil || deps.revision == nil || deps.step == nil || deps.close == nil {
		return RepeatedResult{}, censuscontinuation.ErrInvalidProgramCTransition
	}
	var result RepeatedResult
	var expected censussourcecontinuation.HeadRevision
	for i := 0; i < request.MaxTransitions; i++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		root, err := deps.open()
		if err != nil {
			return result, err
		}
		if i == 0 {
			expected, err = deps.revision(ctx, root, request.HeadKey)
			if err != nil {
				return result, errors.Join(err, deps.close(root))
			}
		}
		transition, stepErr := deps.step(ctx, Request{
			StateRoot: root, Receipt: request.Receipt, Ledger: request.Ledger,
			WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey,
			Expected: expected, Bindings: request.Bindings,
		})
		result.Transitions = append(result.Transitions, transition)
		result.Final = transition
		closeErr := deps.close(root)
		if stepErr != nil || closeErr != nil {
			return result, errors.Join(stepErr, closeErr)
		}
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if transition.Status != censuscontinuation.ProgramCTransitionCommitted {
			switch transition.Status {
			case censuscontinuation.ProgramCTransitionComplete,
				censuscontinuation.ProgramCTransitionRetained,
				censuscontinuation.ProgramCTransitionConflict,
				censuscontinuation.ProgramCTransitionIndeterminate:
				return result, nil
			default:
				return result, censuscontinuation.ErrInvalidProgramCTransition
			}
		}
		expected = transition.Head
	}
	return result, nil
}

func currentRevision(ctx context.Context, root *publication.Root, headKey string) (censussourcecontinuation.HeadRevision, error) {
	heads, err := censussourceassembly.NewHeadsWithParser(root, headNamespace, censuscontinuation.ParseProgramCTransitionHeadRevision)
	if err != nil {
		return censussourcecontinuation.HeadRevision{}, err
	}
	raw, err := heads.Load(ctx, headKey)
	if err != nil {
		return censussourcecontinuation.HeadRevision{}, err
	}
	return censuscontinuation.ParseProgramCTransitionHeadRevision(raw)
}
