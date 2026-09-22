package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime/trace"
	"sync"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv3"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
	"lsp-trace/sessionruntime"
	"lsp-trace/structuralcontextsymbolops"
)

const (
	structuralContextOperation              operation.Name = "structural_context"
	structuralContextContinuationTTL                       = 5 * time.Minute
	structuralContextContinuationMaxEntries                = 64
	structuralContextContinuationMaxBytes                  = 64 << 20
)

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
	RegexLocator     *struct {
		URI            string `json:"uri"`
		Pattern        string `json:"pattern"`
		MatchIndex     int    `json:"match_index"`
		CaptureGroup   int    `json:"capture_group,omitempty"`
		ExpectedDigest string `json:"expected_document_digest,omitempty"`
		Limits         struct {
			MaxDocumentBytes int `json:"max_document_bytes"`
			MaxMatches       int `json:"max_matches"`
			MaxPatternBytes  int `json:"max_pattern_bytes"`
			MaxWork          int `json:"max_work"`
		} `json:"limits"`
	} `json:"regex_locator,omitempty"`
	Analysis struct {
		Kind      string `json:"kind"`
		Direction string `json:"direction,omitempty"`
		Depth     int    `json:"depth,omitempty"`
	} `json:"analysis"`
}
type structuralContextExecutor struct{ runtime *hostSelectorRuntime }

type structuralContextProjectionInput struct {
	Transient     transientstructural.Result
	WorkspaceRoot string
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
	Paging          *struct {
		Cursor           string `json:"cursor,omitempty"`
		MaxPageBytes     int    `json:"max_page_bytes"`
		MaxPages         int    `json:"max_pages"`
		MaxResponseBytes int    `json:"max_response_bytes"`
	} `json:"paging,omitempty"`
}

func sourceOnlyProjection(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	var mode string
	var occurrences bool
	if json.Unmarshal(fields["mode"], &mode) != nil || mode != "TARGET" {
		return false
	}
	value, present := fields["include_relation_occurrences"]
	return present && json.Unmarshal(value, &occurrences) == nil && !occurrences
}

type structuralContextDelegate interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type structuralContextProjectionPreparer struct {
	manager    *sessionruntime.Manager
	target     *sessionruntime.DocumentSupply
	refreshURI string
}

func (p structuralContextProjectionPreparer) PrepareDocument(ctx context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	if p.target != nil && request.URI == p.target.URI {
		cloned := *p.target
		cloned.Content = append([]byte(nil), p.target.Content...)
		cloned.Params = append([]byte(nil), p.target.Params...)
		return sessionruntime.DocumentResult{URI: request.URI, Version: cloned.DocumentVersion, Supply: &cloned}
	}
	if request.URI == p.refreshURI {
		return p.manager.RefreshDocumentSupply(ctx, request)
	}
	return p.manager.PrepareDocument(ctx, request)
}

type structuralContextContinuation struct {
	requestDigest string
	structural    json.RawMessage
	expires       time.Time
	sequence      uint64
	bytes         int64
	page          func(string) (sourceprojectionv3.Page, error)
}

type unifiedStructuralContextV2Executor struct {
	exact   structuralContextDelegate
	symbol  structuralContextDelegate
	manager *sessionruntime.Manager

	continuationMu         sync.Mutex
	continuations          map[string]structuralContextContinuation
	continuationSequence   uint64
	continuationBytes      int64
	continuationNow        func() time.Time
	continuationTTL        time.Duration
	continuationMaxEntries int
	continuationMaxBytes   int64
}

func newStructuralContextExecutor(r *hostSelectorRuntime) *structuralContextExecutor {
	return &structuralContextExecutor{runtime: r}
}

func newUnifiedStructuralContextV2Executor(r *hostSelectorRuntime, exact *structuralContextExecutor) *unifiedStructuralContextV2Executor {
	return &unifiedStructuralContextV2Executor{exact: exact, symbol: structuralcontextsymbolops.NewUnifiedV2Executor(r, exact), manager: r.Manager,
		continuationTTL: structuralContextContinuationTTL, continuationMaxEntries: structuralContextContinuationMaxEntries, continuationMaxBytes: structuralContextContinuationMaxBytes}
}

func (e *unifiedStructuralContextV2Executor) continuationConfig() (func() time.Time, time.Duration, int, int64) {
	now := e.continuationNow
	if now == nil {
		now = time.Now
	}
	ttl := e.continuationTTL
	if ttl <= 0 || ttl > structuralContextContinuationTTL {
		ttl = structuralContextContinuationTTL
	}
	entries := e.continuationMaxEntries
	if entries <= 0 {
		entries = structuralContextContinuationMaxEntries
	}
	bytes := e.continuationMaxBytes
	if bytes <= 0 {
		bytes = structuralContextContinuationMaxBytes
	}
	return now, ttl, entries, bytes
}

func (e *unifiedStructuralContextV2Executor) removeContinuationLocked(cursor string) {
	if c, ok := e.continuations[cursor]; ok {
		delete(e.continuations, cursor)
		e.continuationBytes -= c.bytes
	}
}

func (e *unifiedStructuralContextV2Executor) cleanupContinuationsLocked(now time.Time) {
	for cursor, c := range e.continuations {
		if !now.Before(c.expires) {
			e.removeContinuationLocked(cursor)
		}
	}
}

func (e *unifiedStructuralContextV2Executor) retainContinuationLocked(cursor string, c structuralContextContinuation, now time.Time) bool {
	_, _, maxEntries, maxBytes := e.continuationConfig()
	e.cleanupContinuationsLocked(now)
	if c.bytes > maxBytes {
		return false
	}
	for len(e.continuations) >= maxEntries || e.continuationBytes+c.bytes > maxBytes {
		oldestCursor := ""
		oldestSequence := ^uint64(0)
		for candidate, retained := range e.continuations {
			if retained.sequence < oldestSequence || retained.sequence == oldestSequence && candidate < oldestCursor {
				oldestCursor, oldestSequence = candidate, retained.sequence
			}
		}
		if oldestCursor == "" {
			return false
		}
		e.removeContinuationLocked(oldestCursor)
	}
	e.continuationSequence++
	c.sequence = e.continuationSequence
	e.continuations[cursor] = c
	e.continuationBytes += c.bytes
	return true
}

func tracedProjectionFailure(ctx context.Context, stage string, err error) (operation.Result, *operation.Failure) {
	if err != nil {
		trace.Logf(ctx, "projection_error", "%s: %v", stage, err)
	}
	trace.Log(ctx, "projection_failure", stage)
	_ = dumpRuntimeTrace(ctx, stage)
	return fail("SOURCE_PROJECTION_FAILED", err)
}

func structuralContextRequestDigest(raw json.RawMessage) (string, error) {
	var request map[string]any
	if err := json.Unmarshal(raw, &request); err != nil {
		return "", err
	}
	if projection, ok := request["projection"].(map[string]any); ok {
		if paging, ok := projection["paging"].(map[string]any); ok {
			delete(paging, "cursor")
		}
	}
	canonical, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func structuralContextContinuationMetadata(v2 any, page sourceprojectionv3.Page, expires time.Time, pageLimit uint64) (*sourceprojectionv3.Continuation, error) {
	if page.Complete || page.NextCursor == "" {
		return nil, nil
	}
	raw, err := json.Marshal(v2)
	if err != nil {
		return nil, err
	}
	snapshotHash := sha256.Sum256(raw)
	var identity struct {
		PhysicalProjectionID string          `json:"physical_projection_id"`
		CustodyBinding       json.RawMessage `json:"custody_binding"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil || identity.PhysicalProjectionID == "" || len(identity.CustodyBinding) == 0 {
		return nil, sourceprojectionv3.ErrInvalidRequest
	}
	custodyHash := sha256.Sum256(identity.CustodyBinding)
	return &sourceprojectionv3.Continuation{
		SnapshotID: "sha256:" + hex.EncodeToString(snapshotHash[:]), CustodyID: "sha256:" + hex.EncodeToString(custodyHash[:]),
		Page: page.Accounting.Pages, PageLimit: pageLimit, ExpiresAt: expires.UTC().Format(time.RFC3339Nano),
		NextRequest: map[string]any{"projection": map[string]any{"paging": map[string]any{"cursor": page.NextCursor}}},
	}, nil
}

func unifiedStructuralContextArtifact(structural json.RawMessage, projection any) ([]byte, error) {
	return json.Marshal(struct {
		SchemaVersion string          `json:"schema_version"`
		Structural    json.RawMessage `json:"structural"`
		Projection    any             `json:"projection"`
	}{SchemaVersion: "lsp-trace.unified-structural-context-result.v3", Structural: structural, Projection: projection})
}

func (e *unifiedStructuralContextV2Executor) resume(cursor, requestDigest string, maxResponseBytes int) (operation.Result, *operation.Failure, bool) {
	if cursor == "" {
		return operation.Result{}, nil, false
	}
	e.continuationMu.Lock()
	defer e.continuationMu.Unlock()
	now, _, _, _ := e.continuationConfig()
	e.cleanupContinuationsLocked(now())
	continued, ok := e.continuations[cursor]
	if !ok || continued.requestDigest != requestDigest {
		return operation.Result{}, &operation.Failure{Code: "SOURCE_PROJECTION_FAILED"}, true
	}
	page, err := continued.page(cursor)
	if err != nil {
		return operation.Result{}, &operation.Failure{Code: "SOURCE_PROJECTION_FAILED"}, true
	}
	artifact, err := unifiedStructuralContextArtifact(continued.structural, page)
	if err != nil || maxResponseBytes > 0 && len(artifact) > maxResponseBytes {
		return operation.Result{}, &operation.Failure{Code: "SOURCE_PROJECTION_FAILED"}, true
	}
	e.removeContinuationLocked(cursor)
	if page.NextCursor != "" && !e.retainContinuationLocked(page.NextCursor, continued, now()) {
		return operation.Result{}, &operation.Failure{Code: "SOURCE_PROJECTION_FAILED"}, true
	}
	return operation.Result{Artifact: artifact}, nil, true
}

func (e *unifiedStructuralContextV2Executor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.exact == nil || e.symbol == nil || op.Name != operation.Name("structural_context_v2") {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var target struct {
		URI          string          `json:"uri"`
		Symbol       string          `json:"symbol"`
		Line         *uint32         `json:"line"`
		Character    *uint32         `json:"character"`
		UpDepth      int             `json:"up_depth"`
		DownDepth    int             `json:"down_depth"`
		RegexLocator json.RawMessage `json:"regex_locator"`
		Projection   json.RawMessage `json:"projection"`
	}
	if err := json.Unmarshal(op.Input, &target); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	var projectionRequest sourceProjectionRequest
	requestDigest := ""
	if len(target.Projection) != 0 {
		if err := json.Unmarshal(target.Projection, &projectionRequest); err != nil {
			return fail(operation.FailureInvalidInput, err)
		}
		var err error
		requestDigest, err = structuralContextRequestDigest(op.Input)
		if err != nil {
			return fail(operation.FailureInvalidInput, err)
		}
		if projectionRequest.Paging != nil && projectionRequest.Paging.Cursor != "" {
			if result, failure, handled := e.resume(projectionRequest.Paging.Cursor, requestDigest, projectionRequest.Paging.MaxResponseBytes); handled {
				return result, failure
			}
		}
	}
	if len(target.Projection) != 0 {
		var task *trace.Task
		ctx, task = trace.NewTask(ctx, "structural_context_projection")
		defer task.End()
		trace.Log(ctx, "projection_stage", "delegate_start")
	}
	sourceOnlyTarget := sourceOnlyProjection(target.Projection) && target.UpDepth == 0 && target.DownDepth == 0 && target.URI != "" && ((target.Line != nil && target.Character != nil) || target.Symbol != "" || len(target.RegexLocator) != 0)
	delegate := e.exact
	if target.Symbol != "" && !sourceOnlyTarget {
		delegate = e.symbol
	}
	result, failure := delegate.Execute(ctx, op)
	if failure != nil {
		return operation.Result{}, failure
	}
	if !json.Valid(result.Artifact) {
		if len(target.Projection) != 0 {
			return tracedProjectionFailure(ctx, "delegate_artifact_invalid", nil)
		}
		return fail("INVALID_SERVER_RESPONSE", nil)
	}
	var projection any
	maxResponseBytes := 0
	if len(target.Projection) != 0 {
		request := projectionRequest
		maxResponseBytes = request.Limits.MaxResponseBytes
		trace.Log(ctx, "projection_stage", "delegate_complete")
		payload, ok := result.Value.(structuralContextProjectionInput)
		if !ok {
			return tracedProjectionFailure(ctx, "projection_input_missing", nil)
		}
		candidates, err := sourceprojection.DeriveWorkspaceCandidates(payload.Transient, request.Mode, request.IncludeRelationOccurrences, payload.WorkspaceRoot)
		if err != nil {
			return tracedProjectionFailure(ctx, "candidate_derivation", err)
		}
		trace.Logf(ctx, "projection_stage", "candidates_complete count=%d", len(candidates))
		targetURI := ""
		for _, candidate := range candidates {
			if candidate.GraphSubjectID == payload.Transient.TargetID {
				targetURI = candidate.LogicalSourceID
				break
			}
		}
		if targetURI == "" {
			return tracedProjectionFailure(ctx, "target_candidate_missing", nil)
		}
		refreshURI := ""
		if payload.Transient.SourceSupply == nil {
			refreshURI = targetURI
		}
		plan := liveprojection.PlanDocuments(candidates, targetURI)
		maxDocuments := 1 + request.Limits.MaxAdditionalDocuments
		if request.Limits.MaxDocumentRequests < maxDocuments {
			maxDocuments = request.Limits.MaxDocumentRequests
		}
		trace.Logf(ctx, "projection_stage", "prepare_start documents=%d refresh_target=%t", len(plan), refreshURI != "")
		prepared := liveprojection.Prepare(ctx, structuralContextProjectionPreparer{manager: e.manager, target: payload.Transient.SourceSupply, refreshURI: refreshURI}, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, "", plan, liveprojection.PreparationLimits{MaxDocuments: maxDocuments, MaxBytes: request.Limits.MaxTotalDocumentBytes, MaxDocumentBytes: request.Limits.MaxDocumentBytes, MaxMessages: request.Limits.MaxDocumentMessages, MaxWork: request.Limits.MaxDocumentAcquisitionWork})
		if prepared.Status != liveprojection.PreparationComplete {
			trace.Logf(ctx, "projection_status", "prepare_incomplete status=%s", prepared.Status)
			return tracedProjectionFailure(ctx, "document_preparation", nil)
		}
		trace.Log(ctx, "projection_stage", "prepare_complete")
		if e.manager != nil {
			candidates, err = liveprojection.ResolveDisplayRanges(ctx, e.manager, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, candidates, liveprojection.DisplayResolutionLimits{
				MaxWork: request.Limits.MaxDisplayResolutionWork, MaxMessages: request.Limits.MaxDocumentMessages,
				MaxBytes: int64(request.Limits.MaxDocumentBytes), RequestTimeout: 30 * time.Second,
			})
			if err != nil {
				return tracedProjectionFailure(ctx, "display_range_resolution", err)
			}
		}
		trace.Log(ctx, "projection_stage", "display_ranges_complete")
		policy := sourceprojection.Policy{PolicyID: request.PrivacyPolicyID, BodyRequested: request.Body == "INCLUDE", MaxBytes: request.Limits.MaxSourceBytes, MaxRanges: request.Limits.MaxRanges, MaxObjects: request.Limits.MaxObjects, MaxWork: request.Limits.MaxWork, EnforceLimits: true}
		composed := liveprojection.Compose(prepared, payload.Transient.Qualification.SessionID, payload.Transient.Qualification.Generation, candidates, policy)
		if composed.Status != liveprojection.CompositionComplete {
			trace.Logf(ctx, "projection_status", "composition_incomplete status=%s", composed.Status)
			return tracedProjectionFailure(ctx, "composition", nil)
		}
		trace.Log(ctx, "projection_stage", "composition_complete")
		if request.Paging != nil && (request.Paging.MaxPageBytes <= 0 || request.Paging.MaxPages <= 0 || request.Paging.MaxResponseBytes <= 0) {
			return fail("SOURCE_PROJECTION_FAILED", nil)
		}
		identityRequest := request
		if request.Paging != nil {
			identityPaging := *request.Paging
			identityPaging.Cursor = ""
			identityRequest.Paging = &identityPaging
		}
		policyBytes, err := json.Marshal(identityRequest)
		if err != nil {
			return fail("SOURCE_PROJECTION_FAILED", err)
		}
		sum := sha256.Sum256(policyBytes)
		policyID := "sha256:" + hex.EncodeToString(sum[:])
		assemblyLimit := request.Limits.MaxResponseBytes
		if request.Paging != nil {
			assemblyLimit = request.Paging.MaxResponseBytes
		}
		v2, err := liveprojection.AssembleV2Bounded(composed, prepared, targetURI, plan, policyID, assemblyLimit)
		if err != nil {
			return tracedProjectionFailure(ctx, "assembly", err)
		}
		trace.Log(ctx, "projection_stage", "assembly_complete")
		projection = v2
		if request.Paging != nil {
			pagingLimits := sourceprojectionv3.Limits{
				MaxPageBytes: uint64(request.Paging.MaxPageBytes), MaxPages: uint64(request.Paging.MaxPages), MaxResponseBytes: uint64(request.Paging.MaxResponseBytes),
				MaxObjects: uint64(request.Limits.MaxObjects), MaxRanges: uint64(request.Limits.MaxRanges), MaxSourceBytes: uint64(request.Limits.MaxSourceBytes), MaxWork: uint64(request.Limits.MaxWork),
			}
			page, pageErr := sourceprojectionv3.PaginateV2(v2, policyID, pagingLimits, "")
			if pageErr != nil {
				return fail("SOURCE_PROJECTION_FAILED", pageErr)
			}
			now, ttl, _, _ := e.continuationConfig()
			expires := now().Add(ttl)
			page.Continuation, pageErr = structuralContextContinuationMetadata(v2, page, expires, pagingLimits.MaxPages)
			if pageErr != nil {
				return fail("SOURCE_PROJECTION_FAILED", pageErr)
			}
			projection = page
			if page.NextCursor != "" {
				retainedV2, marshalErr := json.Marshal(v2)
				if marshalErr != nil {
					return fail("SOURCE_PROJECTION_FAILED", marshalErr)
				}
				continued := structuralContextContinuation{
					requestDigest: requestDigest,
					structural:    append(json.RawMessage(nil), result.Artifact...),
					expires:       expires,
					bytes:         int64(len(result.Artifact) + len(retainedV2)),
					page: func(cursor string) (sourceprojectionv3.Page, error) {
						next, err := sourceprojectionv3.PaginateV2(v2, policyID, pagingLimits, cursor)
						if err != nil {
							return sourceprojectionv3.Page{}, err
						}
						next.Continuation, err = structuralContextContinuationMetadata(v2, next, expires, pagingLimits.MaxPages)
						return next, err
					},
				}
				e.continuationMu.Lock()
				if e.continuations == nil {
					e.continuations = make(map[string]structuralContextContinuation)
				}
				retained := e.retainContinuationLocked(page.NextCursor, continued, now())
				e.continuationMu.Unlock()
				if !retained {
					return fail("SOURCE_PROJECTION_FAILED", sourceprojectionv3.ErrResourceLimit)
				}
			}
		}
	}
	schemaVersion := "lsp-trace.unified-structural-context-result.v2"
	if target.Projection != nil {
		var probe struct {
			Paging json.RawMessage `json:"paging"`
		}
		if json.Unmarshal(target.Projection, &probe) == nil && len(probe.Paging) != 0 {
			schemaVersion = "lsp-trace.unified-structural-context-result.v3"
		}
	}
	envelope := struct {
		SchemaVersion string          `json:"schema_version"`
		Structural    json.RawMessage `json:"structural"`
		Projection    any             `json:"projection,omitempty"`
	}{SchemaVersion: schemaVersion, Structural: json.RawMessage(result.Artifact), Projection: projection}
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
	target := transientstructural.Target{URI: in.URI, Symbol: in.Symbol, Line: in.Line, Character: in.Character}
	if in.RegexLocator != nil {
		target.URI = in.RegexLocator.URI
		target.Regex = &transientstructural.RegexLocator{Pattern: in.RegexLocator.Pattern, MatchIndex: in.RegexLocator.MatchIndex, CaptureGroup: in.RegexLocator.CaptureGroup, ExpectedDigest: in.RegexLocator.ExpectedDigest, MaxDocumentBytes: in.RegexLocator.Limits.MaxDocumentBytes, MaxMatches: in.RegexLocator.Limits.MaxMatches, MaxPatternBytes: in.RegexLocator.Limits.MaxPatternBytes, MaxWork: in.RegexLocator.Limits.MaxWork}
	}
	var projectionRequest sourceProjectionRequest
	if len(in.Projection) != 0 {
		if err := json.Unmarshal(in.Projection, &projectionRequest); err != nil {
			return fail(operation.FailureInvalidInput, err)
		}
	}
	sourceOnlyTarget := sourceOnlyProjection(in.Projection) && in.UpDepth == 0 && in.DownDepth == 0 && target.URI != "" && ((target.Line != nil && target.Character != nil) || target.Symbol != "" || target.Regex != nil)
	q := transientstructural.Request{SessionID: id, Generation: generation, Target: target, DownDepth: in.DownDepth, UpDepth: in.UpDepth, MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, CaptureSupply: len(in.Projection) != 0 || in.RegexLocator != nil, SourceOnlyTarget: sourceOnlyTarget, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisKind(in.Analysis.Kind), Direction: transientstructural.Direction(in.Analysis.Direction), MaxDepth: in.Analysis.Depth}}
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
	var root string
	if op.Name == operation.Name("structural_context_v2") {
		var ok bool
		root, ok = e.runtime.Manager.WorkspaceRoot(id, generation)
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
		result.Value = structuralContextProjectionInput{Transient: got, WorkspaceRoot: root}
	}
	return result, nil
}
