package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

// censusMCPRunner is deliberately private: operation 34 has no registry or
// advertisement surface yet. The production censusRuntime satisfies it via run.
type censusMCPRunner interface {
	run(context.Context, operation.Request) censusCompletion
}

type censusBatchTargetsContextKey struct{}

func censusBatchTargetsFromContext(ctx context.Context) int {
	value, _ := ctx.Value(censusBatchTargetsContextKey{}).(int)
	return value
}

type censusContinuationMCPHost interface {
	Fresh(context.Context, censusCompletion, string) (censusContinuationMCPResult, error)
	Resume(context.Context, string, string) (censusContinuationMCPResult, error)
}

type censusContinuationMCPResult struct {
	Census           *censusresult.Result
	CensusSelector   string
	CensusDigest     string
	CensusByteLength uint64
	Descriptor       string
	Status           string
	RequestCount     int
	PreparationCount int
}

type privateCensusMCPBinding struct {
	runtime      censusMCPRunner
	continuation censusContinuationMCPHost
}

type privateCensusMCPResult struct {
	Text       []byte
	Structured []byte
	IsError    bool
}

type censusMCPEnvelope struct {
	EnvelopeVersion  string                            `json:"envelope_version"`
	EnvelopeSchemaID string                            `json:"envelope_schema_id"`
	Tool             string                            `json:"tool"`
	RequestID        string                            `json:"request_id"`
	Outcome          string                            `json:"outcome"`
	OperationStatus  string                            `json:"operation_status"`
	IsError          bool                              `json:"isError"`
	Result           any                               `json:"result,omitempty"`
	Error            *censusresult.Diagnostic          `json:"error,omitempty"`
	Diagnostic       *censusresult.DiscoveryDiagnostic `json:"diagnostic,omitempty"`
	RequestReceipt   *censusrequest.Receipt            `json:"request_receipt,omitempty"`
}

func bindCensusDiagnosticRecorder(server *mcp.Server, recorder *censusdiagnostic.Recorder) error {
	if server == nil {
		return errors.New("private census MCP binding unavailable")
	}
	binding, ok := server.Executors[mcp.CensusExecutorFamily].(*privateCensusMCPBinding)
	if !ok || binding == nil {
		return errors.New("private census MCP binding unavailable")
	}
	runtime, ok := binding.runtime.(*censusRuntime)
	if !ok || runtime == nil {
		return errors.New("private census runtime unavailable")
	}
	runtime.acquisitionDiagnostic = recorder
	return nil
}

func newPrivateCensusMCPBinding(runtime censusMCPRunner, continuation ...censusContinuationMCPHost) *privateCensusMCPBinding {
	binding := &privateCensusMCPBinding{runtime: runtime}
	if len(continuation) != 0 {
		binding.continuation = continuation[0]
	}
	return binding
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
	var probe map[string]json.RawMessage
	if json.Unmarshal(request.Input, &probe) == nil {
		if _, continuation := probe["continuation"]; continuation {
			return b.callContinuation(ctx, request)
		}
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

func (b *privateCensusMCPBinding) callContinuation(ctx context.Context, request operation.Request) (privateCensusMCPResult, error) {
	decoded, err := mcpcontract.DecodeFutureCensusRequestV2(request.Input)
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	ctx = withDescriptorTraceCorrelation(ctx, decoded["generation"])
	continuationBoundaryTrace(ctx, "BINDING", "ENTER", "NONE", "NONE")
	continuation := decoded["continuation"].(map[string]any)
	stopAfter, _ := continuation["stop_after"].(string)
	if selector, resume := continuation["resume_selector"].(string); resume {
		if b.continuation == nil {
			return projectCensusContinuationDomainError(request.RequestID)
		}
		continuationBoundaryTrace(ctx, "HOST_RESUME", "ENTER", "NONE", "NONE")
		result, resumeErr := b.continuation.Resume(ctx, selector, stopAfter)
		if resumeErr != nil {
			if diagnostic, ok := censusresult.ContinuationDiagnosticFromError(resumeErr); ok {
				continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "ERROR", "TYPED")
				continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
				mapped, mapErr := projectCensusContinuationFailure(request.RequestID, diagnostic, "")
				continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "TYPED")
				return mapped, mapErr
			}
			continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "ERROR", "UNTYPED")
			diagnostic, _ := censusresult.NewContinuationDiagnostic(censusresult.ContinuationDescriptorUnavailable, censusresult.ContinuationDiagnosticContext{PublicSelector: selector})
			continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
			mapped, mapErr := projectCensusContinuationFailure(request.RequestID, diagnostic, "")
			continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "TYPED")
			return mapped, mapErr
		}
		continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "OK", "NONE")
		continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
		mapped, mapErr := projectCensusContinuationMCP(request.RequestID, result)
		continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "NONE")
		return mapped, mapErr
	}
	fresh := make(map[string]any, len(decoded)-2)
	batchTargets, batchTargetsOK := decoded["batch_targets"].(json.Number)
	if !batchTargetsOK {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	batchTargetsValue, err := batchTargets.Int64()
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	for key, value := range decoded {
		if key != "continuation" {
			fresh[key] = value
		}
	}
	request.Input, err = json.Marshal(fresh)
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid private census request")
	}
	ctx = context.WithValue(ctx, censusBatchTargetsContextKey{}, int(batchTargetsValue))
	completion := b.runtime.run(ctx, request)
	if completion.Result == nil {
		return projectPrivateCensusMCP(request.RequestID, completion)
	}
	if b.continuation == nil {
		diagnostic := censusresult.NewContinuationHostUnavailableDiagnostic()
		return projectPrivateCensusMCP(request.RequestID, censusCompletion{Diagnostic: &diagnostic})
	}
	continuationBoundaryTrace(ctx, "HOST_FRESH", "ENTER", "NONE", "NONE")
	result, continuationErr := b.continuation.Fresh(ctx, completion, stopAfter)
	if continuationErr != nil {
		if diagnostic, ok := censusresult.ContinuationDiagnosticFromError(continuationErr); ok {
			continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "ERROR", "TYPED")
			continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
			mapped, mapErr := projectCensusContinuationFailure(request.RequestID, diagnostic, completion.Result.Publication.Selector)
			continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "TYPED")
			return mapped, mapErr
		}
		continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "ERROR", "UNTYPED")
		diagnostic, _ := censusresult.NewContinuationDiagnostic(censusresult.ContinuationPublicationFailed, censusresult.ContinuationDiagnosticContext{})
		continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
		mapped, mapErr := projectCensusContinuationFailure(request.RequestID, diagnostic, completion.Result.Publication.Selector)
		continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "TYPED")
		return mapped, mapErr
	}
	continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "OK", "NONE")
	continuationBoundaryTrace(ctx, "MAPPING", "ENTER", "NONE", "NONE")
	mapped, mapErr := projectCensusContinuationMCP(request.RequestID, result)
	continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "NONE")
	return mapped, mapErr
}

func projectCensusContinuationFailure(requestID string, diagnostic censusresult.ContinuationDiagnostic, preservedCensusSelector string) (privateCensusMCPResult, error) {
	if requestID == "" {
		return privateCensusMCPResult{}, errors.New("invalid census continuation diagnostic")
	}
	projected, err := censusresult.NewContinuationDiagnosticV2(diagnostic, preservedCensusSelector)
	if err != nil {
		return privateCensusMCPResult{}, errors.New("invalid census continuation diagnostic")
	}
	envelope := censusMCPEnvelope{EnvelopeVersion: "1", EnvelopeSchemaID: mcpcontract.CensusContinuationDiagnosticEnvelopeV2ID, Tool: mcpcontract.FutureCensusTool, RequestID: requestID, Outcome: "COMMITTED_DEGRADED", OperationStatus: "SUCCEEDED", Result: projected}
	raw, err := json.Marshal(envelope)
	if err != nil || mcpcontract.ValidateFutureCensusEnvelopeExclusive(raw) != nil {
		return privateCensusMCPResult{}, errors.New("invalid census continuation diagnostic")
	}
	return privateCensusMCPResult{Text: append([]byte(nil), raw...), Structured: append([]byte(nil), raw...)}, nil
}

func projectCensusContinuationDomainError(requestID string) (privateCensusMCPResult, error) {
	diagnostic, _ := censusresult.NewDiagnostic(censusresult.StageConfig, nil)
	diagnostic = diagnostic.WithDetail("RESUME_NEEDS_WORKSPACE_OR_UNSUPPORTED")
	return projectPrivateCensusMCP(requestID, censusCompletion{Diagnostic: &diagnostic})
}

func projectCensusContinuationMCP(requestID string, result censusContinuationMCPResult) (privateCensusMCPResult, error) {
	if requestID == "" || result.Descriptor == "" || (result.Census == nil) == (result.CensusSelector == "") {
		return privateCensusMCPResult{}, errors.New("invalid census continuation result")
	}
	catalogStatus, outcome := "COMPLETE", "COMPLETE"
	if result.Status == "PAUSED" {
		catalogStatus, outcome = "PAUSED", "PAUSED"
	} else if result.Status != "COMPLETE" {
		catalogStatus, outcome = "DEGRADED", "COMMITTED_DEGRADED"
	}
	catalog := map[string]any{"kind": "ADR_0007_FEATURE_CATALOG", "checkpoint_selector": result.Descriptor, "composite_selector": result.Descriptor, "catalog_selector": result.Descriptor, "status": catalogStatus, "authority": 0, "accepted": false, "completeness": "UNKNOWN"}
	if catalogStatus == "PAUSED" {
		catalog["request_count"] = result.RequestCount
		catalog["preparation_count"] = result.PreparationCount
		catalog["resume_guidance"] = "Resume with this selector and omit stop_after to continue exactly the remaining work once."
	}
	composite := map[string]any{
		"schema_version": "lsp-trace.census-feature-catalog-result.v2",
		"catalog":        catalog,
	}
	if result.Census != nil {
		composite["census"] = *result.Census
	} else {
		composite["census_identity"] = map[string]any{"selector": result.CensusSelector, "digest": result.CensusDigest, "byte_length": result.CensusByteLength}
	}
	resultRaw, err := json.Marshal(composite)
	if err != nil || mcpcontract.ValidateFutureCensusCompositeResultV2(resultRaw) != nil {
		return privateCensusMCPResult{}, errors.New("invalid census continuation result")
	}
	envelope := censusMCPEnvelope{EnvelopeVersion: "1", EnvelopeSchemaID: mcpcontract.FutureCensusCompositeSuccessID, Tool: mcpcontract.FutureCensusTool, RequestID: requestID, Outcome: outcome, OperationStatus: "SUCCEEDED", Result: composite}
	raw, err := json.Marshal(envelope)
	if err != nil || mcpcontract.ValidateFutureCensusCompositeEnvelopeV2(raw) != nil {
		return privateCensusMCPResult{}, errors.New("invalid census continuation envelope")
	}
	return privateCensusMCPResult{Text: append([]byte(nil), raw...), Structured: append([]byte(nil), raw...)}, nil
}

func projectPrivateCensusMCP(requestID string, completion censusCompletion) (privateCensusMCPResult, error) {
	if requestID == "" || len(requestID) > 256 {
		return privateCensusMCPResult{}, errors.New("invalid private census completion")
	}
	branches := 0
	if completion.Result != nil {
		branches++
	}
	if completion.Diagnostic != nil {
		branches++
	}
	if completion.DiscoveryDiagnostic != nil && completion.RequestReceipt != nil {
		branches++
	}
	if branches != 1 {
		return privateCensusMCPResult{}, errors.New("invalid private census completion")
	}
	env := censusMCPEnvelope{
		EnvelopeVersion: "1", Tool: mcpcontract.FutureCensusTool, RequestID: requestID,
	}
	if completion.DiscoveryDiagnostic != nil {
		if _, err := censusresult.MarshalDiscoveryDiagnostic(*completion.DiscoveryDiagnostic); err != nil {
			return privateCensusMCPResult{}, errors.New("invalid census discovery diagnostic")
		}
		if _, err := censusrequest.MarshalReceipt(*completion.RequestReceipt); err != nil {
			return privateCensusMCPResult{}, errors.New("invalid census request receipt")
		}
		env.EnvelopeSchemaID = mcpcontract.CensusDiscoveryDiagnosticEnvelopeV2ID
		env.Outcome = "DOMAIN_ERROR"
		env.OperationStatus = "FAILED"
		env.IsError = true
		env.Diagnostic = completion.DiscoveryDiagnostic
		env.RequestReceipt = completion.RequestReceipt
	} else if completion.Result != nil {
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
