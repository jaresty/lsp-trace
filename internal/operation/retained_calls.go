package operation

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/strictjson"
)

// ExportRetainedCallsHandler is the shared offline CLI/MCP export. Input text is
// JSON, never a path; it preserves the exact envelope bytes through map transports.
func ExportRetainedCallsHandler(ctx context.Context, request Request) (Result, *Failure) {
	if err := ctx.Err(); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if err := strictjson.RejectDuplicates(request.Input); err != nil {
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
	raw, err := retainedcalls.Export(input.Input)
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if _, err = retainedcalls.ValidateFor(raw, retainedcalls.Family, "v1"); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	return Result{Artifact: raw}, nil
}
