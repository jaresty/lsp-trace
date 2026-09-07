package operation

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/boundedmetrics"
)

const BoundedRetainedMetrics Name = "bounded_retained_metrics"

func BoundedRetainedMetricsHandler(ctx context.Context, request Request) (Result, *Failure) {
	if err := boundedanalysis.Preflight(request.Input, boundedmetrics.MaxBytes); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var input struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var text string
	if json.Unmarshal(input.Input, &text) == nil {
		input.Input = []byte(text)
	}
	raw, err := boundedmetrics.Analyze(ctx, input.Input)
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if _, err = boundedmetrics.ValidateFor(raw, boundedmetrics.Family, "v1"); err != nil {
		return Result{}, inputFailure("OUTPUT_VALIDATION_FAILED", err)
	}
	if err = ctx.Err(); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	return Result{Artifact: raw}, nil
}
