package operation

import (
	"context"
	"encoding/json"
	"fmt"
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
		Input   json.RawMessage `json:"input"`
		Version string          `json:"version"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var text string
	if json.Unmarshal(input.Input, &text) == nil {
		input.Input = []byte(text)
	}
	version := input.Version
	if version == "" {
		version = "v1"
	}
	var raw []byte
	var err error
	switch version {
	case "v1":
		raw, err = retainedcalls.Export(input.Input)
	case "v2":
		raw, err = retainedcalls.ExportV2(input.Input)
	default:
		return Result{}, inputFailure("INPUT_INVALID", fmt.Errorf("unsupported retained-calls version %q", version))
	}
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if _, err = retainedcalls.ValidateFor(raw, retainedcalls.Family, version); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	return Result{Artifact: raw}, nil
}
