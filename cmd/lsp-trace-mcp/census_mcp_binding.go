package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

// censusMCPRunner is deliberately private: operation 34 has no registry or
// advertisement surface yet. The production censusRuntime satisfies it via run.
type censusMCPRunner interface {
	run(context.Context, operation.Request) censusCompletion
}

type privateCensusMCPBinding struct {
	runtime censusMCPRunner
}

type privateCensusMCPResult struct {
	Text       []byte
	Structured []byte
	IsError    bool
}

type censusMCPEnvelope struct {
	EnvelopeVersion  string                   `json:"envelope_version"`
	EnvelopeSchemaID string                   `json:"envelope_schema_id"`
	Tool             string                   `json:"tool"`
	RequestID        string                   `json:"request_id"`
	Outcome          string                   `json:"outcome"`
	OperationStatus  string                   `json:"operation_status"`
	IsError          bool                     `json:"isError"`
	Result           any                      `json:"result,omitempty"`
	Error            *censusresult.Diagnostic `json:"error,omitempty"`
}

func newPrivateCensusMCPBinding(runtime censusMCPRunner) *privateCensusMCPBinding {
	return &privateCensusMCPBinding{runtime: runtime}
}

func (b *privateCensusMCPBinding) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	if request.Name != operation.Census {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: errors.New("census operation name mismatch")}
	}
	result, err := b.call(ctx, request)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInvalidInput, Err: err}
	}
	return operation.Result{Artifact: append([]byte(nil), result.Structured...)}, nil
}

func (b *privateCensusMCPBinding) callDirect(ctx context.Context, request operation.Request) (privateCensusMCPResult, error) {
	return b.call(ctx, request)
}

func (b *privateCensusMCPBinding) callCanonical(ctx context.Context, request operation.Request) (privateCensusMCPResult, error) {
	return b.call(ctx, request)
}

func (b *privateCensusMCPBinding) call(ctx context.Context, request operation.Request) (privateCensusMCPResult, error) {
	if b == nil || b.runtime == nil || request.RequestID == "" || len(request.RequestID) > 256 {
		return privateCensusMCPResult{}, errors.New("private census MCP binding unavailable")
	}
	decoded, err := mcpcontract.DecodeFutureCensusRequestV1(request.Input)
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	request.Input, err = json.Marshal(decoded)
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	return projectPrivateCensusMCP(request.RequestID, b.runtime.run(ctx, request))
}

func projectPrivateCensusMCP(requestID string, completion censusCompletion) (privateCensusMCPResult, error) {
	if requestID == "" || len(requestID) > 256 || (completion.Result == nil) == (completion.Diagnostic == nil) {
		return privateCensusMCPResult{}, errors.New("invalid private census completion")
	}
	env := censusMCPEnvelope{
		EnvelopeVersion: "1", Tool: mcpcontract.FutureCensusTool, RequestID: requestID,
	}
	if completion.Result != nil {
		if _, err := censusresult.Marshal(*completion.Result); err != nil {
			return privateCensusMCPResult{}, errors.New("invalid private census result")
		}
		env.EnvelopeSchemaID = mcpcontract.FutureCensusSuccessID
		env.Outcome = "COMPLETE"
		env.OperationStatus = "SUCCEEDED"
		env.Result = *completion.Result
	} else {
		if _, err := censusresult.MarshalDiagnostic(*completion.Diagnostic); err != nil {
			return privateCensusMCPResult{}, errors.New("invalid private census diagnostic")
		}
		if completion.Diagnostic.Stage == censusresult.StageCommitted {
			env.EnvelopeSchemaID = mcpcontract.FutureCensusSuccessID
			env.Outcome = "COMMITTED_DEGRADED"
			env.OperationStatus = "SUCCEEDED"
			env.Result = *completion.Diagnostic
		} else {
			env.EnvelopeSchemaID = mcpcontract.FutureCensusDomainErrorID
			env.Outcome = "DOMAIN_ERROR"
			env.OperationStatus = "FAILED"
			env.IsError = true
			env.Error = completion.Diagnostic
		}
	}
	raw, err := json.Marshal(env)
	if err != nil || mcpcontract.ValidateFutureCensusEnvelopeExclusive(raw) != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census envelope")
	}
	return privateCensusMCPResult{Text: append([]byte(nil), raw...), Structured: append([]byte(nil), raw...), IsError: env.IsError}, nil
}
