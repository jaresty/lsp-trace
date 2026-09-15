package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"lsp-trace/internal/operation"
	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssidecar"
)

type contextChurnExecutor struct{}

type contextChurnInput struct {
	Input        string `json:"input"`
	Workspace    string `json:"workspace"`
	FromRevision string `json:"from_revision"`
	ToRevision   string `json:"to_revision"`
	TimeoutMS    int    `json:"timeout_ms,omitempty"`
}

func (contextChurnExecutor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if op.Name != operation.Name("context_churn") {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	if err := ctx.Err(); err != nil {
		return operation.Result{}, &operation.Failure{Code: "TIMEOUT", Err: err}
	}
	var input contextChurnInput
	if err := json.Unmarshal(op.Input, &input); err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	raw := []byte(input.Input)
	artifact, err := tsr.DecodeV2Artifact(raw)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	timeout := 30 * time.Second
	if input.TimeoutMS > 0 {
		timeout = time.Duration(input.TimeoutMS) * time.Millisecond
	}
	result, err := vcssidecar.Build(raw, artifact, vcssidecar.Request{Workspace: input.Workspace, FromRevision: input.FromRevision, ToRevision: input.ToRevision}, vcssidecar.GitHistory{Timeout: timeout})
	if err != nil {
		code := "GIT_FAILED"
		switch {
		case strings.Contains(err.Error(), "timeout"):
			code = "TIMEOUT"
		case strings.Contains(err.Error(), "exceeds limit"):
			code = "RESOURCE_LIMIT"
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			code = "TIMEOUT"
		}
		return operation.Result{}, &operation.Failure{Code: code, Err: err}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: "CANCELLED", Err: err}
	}
	return operation.Result{Artifact: encoded}, nil
}
