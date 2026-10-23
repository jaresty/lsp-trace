package censussourcedriver

import (
	"context"
	"errors"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

// Discover performs the bounded discovery selected by one continuation step.
type Discover func(context.Context, int, []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error)

// StepRequest contains only the package-neutral custody needed for one source step.
type StepRequest struct {
	Plan          censussourceplan.Plan
	PartitionSize int
	WorkflowKey   string
	HeadKey       string
	Objects       censussourcecontinuation.ImmutableStore
	Heads         censussourcecontinuation.HeadStore
	Discover      Discover

	// CoordinateForTest is an internal integration-test seam. Production callers
	// leave it nil and use censussourcecontinuation.CoordinateNext.
	CoordinateForTest func(context.Context, censussourcecontinuation.Request) (censussourcecontinuation.Result, error)
}

// StepResult pairs the authoritative continuation result with the discovery
// selection made by this invocation.
type StepResult struct {
	Continuation    censussourcecontinuation.Result
	SelectedOrdinal int
	SelectedOutcome censussourcecontinuation.Outcome
}

// ErrStep reports invalid input, non-committed custody, or disagreement between
// the invocation-local selection and the authoritative continuation result.
var ErrStep = errors.New("census source step failed closed")

type coordinator func(context.Context, censussourcecontinuation.Request) (censussourcecontinuation.Result, error)

// Step performs exactly one authoritative source continuation step.
func Step(ctx context.Context, request StepRequest) (StepResult, error) {
	coordinate := coordinator(censussourcecontinuation.CoordinateNext)
	if request.CoordinateForTest != nil {
		coordinate = request.CoordinateForTest
	}
	return step(ctx, request, coordinate)
}

func step(ctx context.Context, request StepRequest, coordinate coordinator) (StepResult, error) {
	if ctx == nil || request.Heads == nil || request.Objects == nil || request.Discover == nil || request.WorkflowKey == "" || request.HeadKey == "" || coordinate == nil || censussourceplan.Validate(request.Plan, request.PartitionSize) != nil {
		return StepResult{}, ErrStep
	}
	rawHead, err := request.Heads.Load(ctx, request.HeadKey)
	if err != nil {
		return StepResult{}, ErrStep
	}
	expected, err := censussourcecontinuation.ParseHeadRevision(rawHead)
	if err != nil {
		return StepResult{}, ErrStep
	}

	selectedOrdinal := -1
	selectedOutcome := censussourcecontinuation.Outcome("")
	discover := func(callbackCtx context.Context, ordinal int, files []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
		terminal, discoverErr := request.Discover(callbackCtx, ordinal, append([]censusacquisition.SourceFile(nil), files...))
		if discoverErr == nil {
			selectedOrdinal = ordinal
			selectedOutcome = terminal.Outcome
		}
		return terminal, discoverErr
	}

	continuationRequest := censussourcecontinuation.Request{
		Plan: request.Plan, PartitionSize: request.PartitionSize,
		WorkflowKey: request.WorkflowKey, HeadKey: request.HeadKey, Expected: expected,
		Objects: request.Objects, Heads: request.Heads,
	}
	var discoverErr error
	if len(rawHead) == 0 {
		var terminal censussourcecontinuation.Terminal
		terminal, discoverErr = discover(ctx, 0, request.Plan.Partitions[0].Members)
		if discoverErr == nil {
			continuationRequest.ExistingOrdinalZero = &terminal
		}
	} else {
		continuationRequest.Discover = discover
	}

	continuation, err := coordinate(ctx, continuationRequest)
	result := StepResult{Continuation: continuation, SelectedOrdinal: selectedOrdinal, SelectedOutcome: selectedOutcome}
	if discoverErr != nil || err != nil || continuation.Status != censussourcecontinuation.StatusCommitted || selectedOrdinal < 0 || selectedOutcome == "" || continuation.Ordinal != selectedOrdinal || continuation.Outcome != selectedOutcome {
		return result, ErrStep
	}
	return result, nil
}
