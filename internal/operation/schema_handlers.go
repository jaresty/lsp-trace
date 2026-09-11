package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"lsp-trace/internal/boundedmetrics"
	"lsp-trace/internal/boundedranking"
	"lsp-trace/internal/retainedrelations"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/v5sourcesnapshot"
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
	Detail string    `json:"detail,omitempty"`
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
	result := Result{Artifact: artifact}
	if input.Detail == "compact" {
		var parsed any
		if err := json.Unmarshal(artifact, &parsed); err != nil {
			return Result{}, inputFailure("OUTPUT_VALIDATION_FAILED", err)
		}
		schemaID := ""
		if object, ok := parsed.(map[string]any); ok {
			schemaID, _ = object["$id"].(string)
		}
		result.Value = map[string]any{"schema_id": schemaID, "schema": parsed}
	}
	return result, nil
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
	// New-family inline text preserves exact artifact bytes through transports
	// which historically decode object arguments into maps. It is never a path.
	if family == schema.FamilyOperationalCustody || family == schema.FamilyGraphProvenance || family == schema.FamilyRetainedCalls || family == schema.FamilyRetainedRelations || family == schema.FamilyGraphV5SourceSnapshot || family == schema.FamilyBoundedAnalysis || family == boundedmetrics.Family || family == boundedranking.Family || family == schema.FamilyBoundedGraphV2 || family == schema.FamilyBoundedAnalysisV2 || family == schema.FamilyBoundedMetricsV2 || family == schema.FamilyBoundedRankingV2 {
		var text string
		if json.Unmarshal(input.Input, &text) == nil {
			input.Input = []byte(text)
		}
	}
	var detected string
	var err error
	switch family {
	case schema.FamilyRetainedRelations:
		detected, err = retainedrelations.ValidateFor(input.Input, family, version)
	case schema.FamilyGraphV5SourceSnapshot:
		if version != "v1" && version != v5sourcesnapshot.Version {
			err = fmt.Errorf("unsupported schema version %q for family %q", version, family)
		} else {
			detected, err = v5sourcesnapshot.Validate(input.Input)
		}
	default:
		detected, err = boundedranking.ValidateFor(input.Input, family, version)
	}
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
