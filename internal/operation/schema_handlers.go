package operation

import (
	"context"
	"encoding/json"
	"strings"

	"lsp-trace/internal/schema"
)

// ValidationResult identifies the schema version admitted by validation.
type ValidationResult struct {
	SchemaVersion string `json:"schema_version"`
}

type schemaRef struct {
	Family  string `json:"family"`
	Version string `json:"version"`
}

type schemaGetInput struct {
	Schema schemaRef `json:"schema"`
}

type validateInput struct {
	Input  json.RawMessage `json:"input"`
	Schema *schemaRef      `json:"schema,omitempty"`
}

// SchemaGetHandler retrieves exact authoritative bytes from the schema core.
func SchemaGetHandler(_ context.Context, request Request) (Result, *Failure) {
	var input schemaGetInput
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	artifact, err := schema.BytesFor(input.Schema.Family, input.Schema.Version)
	if err != nil {
		return Result{}, inputFailure("INPUT_FAMILY_MISMATCH", err)
	}
	return Result{Artifact: artifact}, nil
}

// ValidateHandler validates structure before family semantics and returns the
// exact admitted input value bytes unchanged.
func ValidateHandler(_ context.Context, request Request) (Result, *Failure) {
	var input validateInput
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	family, version := schema.FamilyGraph, ""
	if input.Schema != nil {
		family, version = input.Schema.Family, input.Schema.Version
	}
	detected, err := schema.ValidateFor(input.Input, family, version)
	if err != nil {
		code := "INPUT_INVALID"
		if strings.HasPrefix(err.Error(), "unsupported schema family ") || strings.HasPrefix(err.Error(), "unsupported schema version ") || strings.HasPrefix(err.Error(), "schema version mismatch:") {
			code = "INPUT_FAMILY_MISMATCH"
		}
		return Result{}, inputFailure(code, err)
	}
	return Result{Value: ValidationResult{SchemaVersion: detected}, Artifact: append([]byte(nil), input.Input...)}, nil
}

func inputFailure(code string, err error) *Failure {
	return &Failure{Code: code, Diagnostics: []string{err.Error()}, Err: err}
}
