package operation

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedanalysis"
)

const BoundedRetainedAnalysis Name = "bounded_retained_analysis"

func BoundedRetainedAnalysisHandler(ctx context.Context, request Request) (Result, *Failure) {
	if err := boundedanalysis.Preflight(request.Input, boundedanalysis.MaxBytes); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var input struct {
		Input     json.RawMessage `json:"input"`
		Operation string          `json:"operation"`
		Start     string          `json:"start"`
		End       string          `json:"end"`
		Mode      string          `json:"mode"`
		MaxWork   int             `json:"max_work"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var text string
	if json.Unmarshal(input.Input, &text) == nil {
		input.Input = []byte(text)
	}
	raw, err := boundedanalysis.Analyze(ctx, input.Input, boundedanalysis.Parameters{Operation: input.Operation, Start: input.Start, End: input.End, Mode: input.Mode, MaxWork: input.MaxWork})
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if _, err = boundedanalysis.ValidateFor(raw, boundedanalysis.Family, "v1"); err != nil {
		return Result{}, inputFailure("OUTPUT_VALIDATION_FAILED", err)
	}
	return Result{Artifact: raw}, nil
}
