package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
	"lsp-trace/sessionruntime"
	"lsp-trace/structuralcontextsymbolops"
)

const structuralContextOperation operation.Name = "structural_context"

type structuralContextInput struct {
	SessionID        string          `json:"session_id"`
	Generation       uint64          `json:"generation"`
	URI              string          `json:"uri"`
	Symbol           string          `json:"symbol,omitempty"`
	Line             *uint32         `json:"line,omitempty"`
	Character        *uint32         `json:"character,omitempty"`
	DownDepth        int             `json:"down_depth"`
	UpDepth          int             `json:"up_depth"`
	MaxNodes         int             `json:"max_nodes"`
	TimeoutMS        int64           `json:"timeout_ms"`
	RequestTimeoutMS int64           `json:"request_timeout_ms"`
	MaxMessages      int             `json:"max_messages,omitempty"`
	MaxBytes         int64           `json:"max_bytes,omitempty"`
	Projection       json.RawMessage `json:"projection,omitempty"`
	Analysis         struct {
		Kind      string `json:"kind"`
		Direction string `json:"direction,omitempty"`
		Depth     int    `json:"depth,omitempty"`
	} `json:"analysis"`
}
type structuralContextExecutor struct{ runtime *hostSelectorRuntime }

type structuralContextProjectionInput struct {
	Transient transientstructural.Result
}

type sourceProjectionRequest struct {
	Mode                       string `json:"mode"`
	Body                       string `json:"body"`
	IncludeRelationOccurrences bool   `json:"include_relation_occurrences"`
	IncludeAncillary           bool   `json:"include_ancillary"`
	Limits                     struct {
		MaxObjects                 int `json:"max_objects"`
		MaxRanges                  int `json:"max_ranges"`
		MaxSourceBytes             int `json:"max_source_bytes"`
		MaxWork                    int `json:"max_work"`
		MaxResponseBytes           int `json:"max_response_bytes"`
		MaxAdditionalDocuments     int `json:"max_additional_documents"`
		MaxDocumentRequests        int `json:"max_document_requests"`
		MaxDocumentBytes           int `json:"max_document_bytes"`
		MaxTotalDocumentBytes      int `json:"max_total_document_bytes"`
		MaxDocumentMessages        int `json:"max_document_messages"`
		MaxDocumentAcquisitionWork int `json:"max_document_acquisition_work"`
		MaxDisplayResolutionWork   int `json:"max_display_resolution_work"`
	} `json:"limits"`
	PrivacyPolicyID string `json:"privacy_policy_id"`
}

type structuralContextDelegate interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type structuralContextProjectionPreparer struct {
	manager *sessionruntime.Manager
	target  *sessionruntime.DocumentSupply
}

func (p structuralContextProjectionPreparer) PrepareDocument(ctx context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	if p.target != nil && request.URI == p.target.URI {
		cloned := *p.target
		cloned.Content = append([]byte(nil), p.target.Content...)
		cloned.Params = append([]byte(nil), p.target.Params...)
		return sessionruntime.DocumentResult{URI: request.URI, Version: cloned.DocumentVersion, Supply: &cloned}
	}
	return p.manager.PrepareDocument(ctx, request)
}

type unifiedStructuralContextV2Executor struct {
	exact   structuralContextDelegate
	symbol  structuralContextDelegate
	manager *sessionruntime.Manager
}

func newStructuralContextExecutor(r *hostSelectorRuntime) *structuralContextExecutor {
	return &structuralContextExecutor{runtime: r}
}

func newUnifiedStructuralContextV2Executor(r *hostSelectorRuntime, exact *structuralContextExecutor) *unifiedStructuralContextV2Executor {
	return &unifiedStructuralContextV2Executor{exact: exact, symbol: structuralcontextsymbolops.NewUnifiedV2Executor(r, exact), manager: r.Manager}
}

func (e *unifiedStructuralContextV2Executor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.exact == nil || e.symbol == nil || op.Name != operation.Name("structural_context_v2") {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var target struct {
		Symbol     string          `json:"symbol"`
		Projection json.RawMessage `json:"projection"`
	}
	if err := json.Unmarshal(op.Input, &target); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	delegate := e.exact
	if target.Symbol != "" {
		delegate = e.symbol
	}
	result, failure := delegate.Execute(ctx, op)
	if failure != nil {
		return operation.Result{}, failure
	}
	if !json.Valid(result.Artifact) {
		return fail("INVALID_SERVER_RESPONSE", nil)
	}
	var projection any
	maxResponseBytes := 0
	if len(target.Projection) != 0 {
		var request sourceProjectionRequest
		if err := json.Unmarshal(target.Projection, &request); err != nil {
			return fail(operation.FailureInvalidInput, err)
		}
		maxResponseBytes = request.Limits.MaxResponseBytes
		payload, ok := result.Value.(structuralContextProjectionInput)
		if !ok || payload.Transient.SourceSupply == nil {
			return fail("SOURCE_PROJECTION_FAILED", nil)
		}
		candidates, err := sourceprojection.DeriveCandidates(payload.Transient, request.Mode, request.IncludeRelationOccurrences)
		if err != nil {
			return fail("SOURCE_PROJECTION_FAILED", err)
		}
		plan := liveprojection.PlanDocuments(candidates, payload.Transient.SourceSupply.URI)
		maxDocuments := 1 + request.Limits.MaxAdditionalDocuments
		if request.Limits.MaxDocumentRequests < maxDocuments {
			maxDocuments = request.Limits.MaxDocumentRequests
		}
		prepared := liveprojection.Prepare(ctx, structuralContextProjectionPreparer{manager: e.manager, target: payload.Transient.SourceSupply}, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, "", plan, liveprojection.PreparationLimits{MaxDocuments: maxDocuments, MaxBytes: request.Limits.MaxTotalDocumentBytes, MaxDocumentBytes: request.Limits.MaxDocumentBytes, MaxMessages: request.Limits.MaxDocumentMessages, MaxWork: request.Limits.MaxDocumentAcquisitionWork})
		if prepared.Status != liveprojection.PreparationComplete {
			return fail("SOURCE_PROJECTION_FAILED", nil)
		}
		if e.manager != nil {
			candidates, err = liveprojection.ResolveDisplayRanges(ctx, e.manager, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, candidates, liveprojection.DisplayResolutionLimits{
				MaxWork: request.Limits.MaxDisplayResolutionWork, MaxMessages: request.Limits.MaxDocumentMessages,
				MaxBytes: int64(request.Limits.MaxDocumentBytes), RequestTimeout: 30 * time.Second,
			})
			if err != nil {
				return fail("SOURCE_PROJECTION_FAILED", err)
			}
		}
		policy := sourceprojection.Policy{PolicyID: request.PrivacyPolicyID, BodyRequested: request.Body == "INCLUDE", MaxBytes: request.Limits.MaxSourceBytes, MaxRanges: request.Limits.MaxRanges, MaxObjects: request.Limits.MaxObjects, MaxWork: request.Limits.MaxWork, EnforceLimits: true}
		composed := liveprojection.Compose(prepared, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, candidates, policy)
		if composed.Status != liveprojection.CompositionComplete {
			return fail("SOURCE_PROJECTION_FAILED", nil)
		}
		policyBytes, err := json.Marshal(request)
		if err != nil {
			return fail("SOURCE_PROJECTION_FAILED", err)
		}
		sum := sha256.Sum256(policyBytes)
		policyID := "sha256:" + hex.EncodeToString(sum[:])
		projection, err = liveprojection.AssembleV2Bounded(composed, prepared, payload.Transient.SourceSupply.URI, plan, policyID, request.Limits.MaxResponseBytes)
		if err != nil {
			return fail("SOURCE_PROJECTION_FAILED", err)
		}
	}
	envelope := struct {
		SchemaVersion string          `json:"schema_version"`
		Structural    json.RawMessage `json:"structural"`
		Projection    any             `json:"projection,omitempty"`
	}{SchemaVersion: "lsp-trace.unified-structural-context-result.v2", Structural: json.RawMessage(result.Artifact), Projection: projection}
	artifact, err := json.Marshal(envelope)
	if err != nil {
		return fail("INVALID_SERVER_RESPONSE", err)
	}
	if maxResponseBytes > 0 && len(artifact) > maxResponseBytes {
		return fail("SOURCE_PROJECTION_FAILED", nil)
	}
	result.Artifact = artifact
	result.Value = nil
	return result, nil
}
func (e *structuralContextExecutor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || (op.Name != structuralContextOperation && op.Name != operation.Name("structural_context_v2")) {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in structuralContextInput
	if err := decode(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if in.MaxMessages == 0 {
		in.MaxMessages = int(transientstructuralresult.DefaultMaxMessages)
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = int64(transientstructuralresult.DefaultMaxBytes)
	}
	id, generation, sf := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(in.TimeoutMS)*time.Millisecond)
	defer cancel()
	q := transientstructural.Request{SessionID: id, Generation: generation, Target: transientstructural.Target{URI: in.URI, Symbol: in.Symbol, Line: in.Line, Character: in.Character}, DownDepth: in.DownDepth, UpDepth: in.UpDepth, MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, CaptureSupply: len(in.Projection) != 0, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisKind(in.Analysis.Kind), Direction: transientstructural.Direction(in.Analysis.Direction), MaxDepth: in.Analysis.Depth}}
	got, domain := transientstructural.Execute(ctx, e.runtime.Manager, q)
	if domain != nil {
		return fail(string(domain.State), domain)
	}
	managerID, sf := e.runtime.TransientSessionIdentity(id, generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	var projected any
	var err error
	if op.Name == operation.Name("structural_context_v2") {
		root, ok := e.runtime.Manager.WorkspaceRoot(id, generation)
		if !ok {
			return fail("INVALID_SERVER_RESPONSE", nil)
		}
		projected, err = transientstructuralresult.ProjectV2(got, q, managerID, root)
	} else {
		projected, err = transientstructuralresult.Project(got, q, managerID)
	}
	if err != nil {
		return fail("INVALID_SERVER_RESPONSE", err)
	}
	raw, err := json.Marshal(projected)
	if err != nil {
		return fail("INVALID_SERVER_RESPONSE", err)
	}
	result := operation.Result{Artifact: raw}
	if op.Name == operation.Name("structural_context_v2") {
		result.Value = structuralContextProjectionInput{Transient: got}
	}
	return result, nil
}
