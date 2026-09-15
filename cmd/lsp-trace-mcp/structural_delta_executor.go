package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/transientstructuraldelta"
)

type structuralDeltaExecutor struct{}

func (structuralDeltaExecutor) Execute(_ context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if op.Name != operation.Name("structural_delta") {
		return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
	}
	result, err := transientstructuraldelta.Execute(op.Input)
	if err != nil {
		var d *transientstructuraldelta.DomainError
		if errors.As(err, &d) {
			return operation.Result{}, &operation.Failure{Code: string(d.Code), Err: d}
		}
		return operation.Result{}, &operation.Failure{Code: string(transientstructuraldelta.CodeInvalidInput), Err: err}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: "CANCELLED", Err: err}
	}
	return operation.Result{Artifact: raw}, nil
}
