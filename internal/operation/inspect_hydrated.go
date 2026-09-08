package operation

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/hydratedinspection"
)

const InspectHydrated Name = "inspect_hydrated"

// InspectHydratedHandler accepts JSON bytes only. It has no acquisition or path
// resolver dependencies; the CLI and MCP consume this exact same operation.
func InspectHydratedHandler(_ context.Context, request Request) (Result, *Failure) {
	r, err := hydratedinspection.Decode(request.Input)
	if err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: errors.New("invalid hydrated inspection request")}
	}
	v, err := hydratedinspection.Inspect(r)
	if err != nil {
		return Result{}, &Failure{Code: "INPUT_INVALID", Err: err}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return Result{}, &Failure{Code: FailureInternal, Err: err}
	}
	return Result{Value: v, Artifact: append(raw, '\n')}, nil
}
