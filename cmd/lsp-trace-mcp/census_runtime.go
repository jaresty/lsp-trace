package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

const (
	censusRuntimeMaxEntries   = 100000
	censusRuntimeMaxWork      = 200000
	censusRuntimeMaxPathBytes = 4096
	censusRuntimeMaxDepth     = 64
)

type censusRuntimeConfig struct {
	sources, includes, excludes                   []string
	downDepth, upDepth, maxNodes, maxBatchTargets uint64
	timeoutMS, requestTimeoutMS                   uint64
}

func censusRuntimeConfigFromDecoded(value map[string]any) censusRuntimeConfig {
	stringsFor := func(key string) []string {
		items, _ := value[key].([]any)
		out := make([]string, len(items))
		for i := range items {
			out[i], _ = items[i].(string)
		}
		return out
	}
	uintFor := func(key string, fallback uint64) uint64 {
		switch value := value[key].(type) {
		case uint64:
			return value
		case json.Number:
			n, _ := value.Int64()
			return uint64(n)
		default:
			return fallback
		}
	}
	return censusRuntimeConfig{
		sources: stringsFor("sources"), includes: stringsFor("includes"), excludes: stringsFor("excludes"),
		downDepth: uintFor("down_depth", censusrequest.DefaultDownDepth), upDepth: uintFor("up_depth", censusrequest.DefaultUpDepth), maxNodes: uintFor("max_nodes", censusrequest.DefaultMaxNodes), maxBatchTargets: uintFor("batch_targets", censusrequest.DefaultBatchTargets),
		timeoutMS: uintFor("timeout_ms", censusrequest.DefaultTimeoutMS), requestTimeoutMS: uintFor("request_timeout_ms", censusrequest.DefaultRequestTimeoutMS),
	}
}

func cloneCensusRuntimeConfig(in censusRuntimeConfig) censusRuntimeConfig {
	in.sources = append([]string(nil), in.sources...)
	in.includes = append([]string(nil), in.includes...)
	in.excludes = append([]string(nil), in.excludes...)
	return in
}

type censusRuntimeResult struct {
	admitted  censusAdmittedSession
	options   censusRuntimeConfig
	discovery censusacquisition.Discovery
	deadline  time.Time
}

type fixedCensusRuntimeDiscovery struct{ discovery censusacquisition.Discovery }

func (d fixedCensusRuntimeDiscovery) Discover(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return d.discovery, nil
}

type censusPrivateFailure struct {
	category          string
	ordinal           *int
	operationCode     string
	operationCategory string
	failedField       string
	failedInvariant   string
	expectedCount     *int
	observedCount     *int
}

func (e censusPrivateFailure) Error() string { return "census private acquisition failure" }

type censusRuntimeBatchAcquirer struct {
	runtime  *hostSelectorRuntime
	admitted censusAdmittedSession
	limits   acquisitionops.Limits
	execute  func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure)
}

func (a censusRuntimeBatchAcquirer) AcquireV5(ctx context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	execute := a.execute
	if execute == nil {
		execute = executeCensusPlannedBatch
	}
	result, failure := execute(ctx, a.runtime, a.admitted, fmt.Sprintf("census:%s:%06d:%s", request.CensusID, request.Ordinal, request.BatchID), request.AcquisitionManifest(a.limits), append([]byte(nil), request.CanonicalSeedsV2...))
	if failure != nil {
		code := operation.NormalizeFailure(failure)
		category := "ACQUISITION"
		ordinal := new(int)
		*ordinal = request.Ordinal
		switch failure.Code {
		case operation.FailureInvalidInput, "CENSUS_BATCH_INVALID_INPUT":
			category = "INVALID_INPUT"
		case "CENSUS_BATCH_SESSION_DRIFT":
			category = "SESSION_DRIFT"
		case "CANCELLED", "CANCELED", "REQUEST_CANCELLED", "CENSUS_BATCH_CANCELLED":
			category = "CANCELLED"
		case operation.FailureInternal, "CENSUS_BATCH_ACQUISITION_FAILED":
			category = "ACQUISITION"
		case "CENSUS_BATCH_ADMISSION_FAILED":
			category = "ADMISSION"
		default:
			censusAcquisitionBranchTrace(ctx, "UNMAPPED_OPERATION_CODE")
			return censusacquisition.AcquiredV5{}, code
		}
		private := &censusPrivateFailure{category: category, ordinal: ordinal}
		var validation *acquisitionengine.ValidationError
		if errors.As(failure.Err, &validation) {
			private.failedField = validation.Diagnostic.FailedField
			private.failedInvariant = validation.Diagnostic.FailedInvariant
			private.expectedCount = validation.Diagnostic.ExpectedCount
			private.observedCount = validation.Diagnostic.ObservedCount
		}
		if category == "ACQUISITION" {
			private.operationCode = code.Code
			private.operationCategory = "ACQUISITION"
		}
		return censusacquisition.AcquiredV5{}, private
	}
	if !result.BoundedTraversalComplete {
		censusAcquisitionBranchTrace(ctx, "BATCH_RESULT_INCOMPLETE")
		ordinal := request.Ordinal
		return censusacquisition.AcquiredV5{}, &censusPrivateFailure{category: "ACQUISITION", ordinal: &ordinal, operationCode: "CENSUS_BATCH_INCOMPLETE", operationCategory: "ACQUISITION"}
	}
	if result.SessionID != request.Session.SessionID || result.Generation != request.Session.Generation {
		censusAcquisitionBranchTrace(ctx, "BATCH_RESULT_IDENTITY_DRIFT")
		return censusacquisition.AcquiredV5{}, errors.New("census batch execution incomplete or identity drifted")
	}
	return censusacquisition.AcquiredV5{Session: request.Session, Raw: append([]byte(nil), result.RawV5...)}, nil
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

type censusRuntimeFailure = censusAdmissionFailure

type censusRuntime struct {
	runtime               *hostSelectorRuntime
	acquisitionDiagnostic *censusdiagnostic.Recorder
	admit                 func(context.Context, []byte) (censusAdmittedSession, *censusAdmissionFailure, censusFailureReason)
	discover              func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig) (censusacquisition.Discovery, error)
	batchExecute          func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure)
}

func newCensusRuntime(runtime *hostSelectorRuntime) *censusRuntime {
	return &censusRuntime{runtime: runtime, admit: newCensusExecutor(runtime).execute}
}

func executeCensusPlannedBatch(ctx context.Context, runtime *hostSelectorRuntime, admitted censusAdmittedSession, requestID string, manifest acquisitionengine.Manifest, canonicalSeedsV2 []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
	if runtime == nil || runtime.Manager == nil {
		return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("managed runtime required")}
	}
	return acquisitionorchestration.ExecutePlannedBatch(ctx, runtime.Manager, acquisitionorchestration.PlannedBatchRequest{SessionID: admitted.sessionID, Generation: admitted.generation, RequestID: requestID, Manifest: manifest, CanonicalSeedsV2: append([]byte(nil), canonicalSeedsV2...)})
}

func (r *censusRuntime) execute(parent context.Context, request operation.Request) (censusRuntimeResult, *censusRuntimeFailure, censusFailureReason) {
	if r == nil || r.admit == nil {
		return censusRuntimeResult{}, censusConfigFailure(), reasonRuntimeUnprovisioned
	}
	admitted, failure, reason := r.admit(parent, append([]byte(nil), request.Input...))
	if failure != nil {
		return censusRuntimeResult{}, failure, reason
	}
	if request.PublicationRoot == nil || r.runtime == nil || r.runtime.Manager == nil {
		return censusRuntimeResult{}, censusConfigFailure(), reasonPublicationRootRequired
	}
	options := cloneCensusRuntimeConfig(admitted.options)
	if batchTargets := censusBatchTargetsFromContext(parent); batchTargets != 0 {
		options.maxBatchTargets = uint64(batchTargets)
	}
	deadline := effectiveCensusDeadline(parent, time.Now().Add(time.Duration(options.timeoutMS)*time.Millisecond))
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	discover := r.discover
	if discover == nil {
		discover = censusRuntimeDiscover
	}
	discovery, err := discover(ctx, r.runtime.Manager, admitted, options)
	if err != nil {
		return censusRuntimeResult{}, censusDiscoveryFailure(), reasonDiscoveryFailed
	}
	return censusRuntimeResult{admitted: admitted, options: cloneCensusRuntimeConfig(options), discovery: discovery, deadline: deadline}, nil, reasonNone
}

func effectiveCensusDeadline(parent context.Context, configured time.Time) time.Time {
	if deadline, ok := parent.Deadline(); ok && deadline.Before(configured) {
		return deadline
	}
	return configured
}

func censusRuntimeAcquisitionLimits(options censusRuntimeConfig) acquisitionops.Limits {
	maxNodes, maxRequests, maxEvidenceBytes, maxPathWork := int(options.maxNodes), 1000, 4<<20, 100000
	timeoutMS, requestTimeoutMS := int(options.timeoutMS), int(options.requestTimeoutMS)
	maxResponseBytes, maxMessages := 4<<20, 64
	return acquisitionops.Limits{MaxNodes: &maxNodes, MaxRequests: &maxRequests, MaxEvidenceBytes: &maxEvidenceBytes, MaxPathWork: &maxPathWork, TimeoutMS: &timeoutMS, RequestTimeoutMS: &requestTimeoutMS, MaxResponseBytes: &maxResponseBytes, MaxMessages: &maxMessages}
}

func (r *censusRuntime) acquire(parent context.Context, result censusRuntimeResult) (censusacquisition.Projection, *censusRuntimeFailure, censusFailureReason) {
	if r == nil || r.runtime == nil || r.runtime.Manager == nil || result.admitted.sessionID == "" || result.admitted.generation == 0 || result.deadline.IsZero() {
		return censusacquisition.Projection{}, censusConfigFailure(), reasonRuntimeUnprovisioned
	}
	options := cloneCensusRuntimeConfig(result.options)
	ctx, cancel := context.WithDeadline(parent, result.deadline)
	defer cancel()
	limits := censusRuntimeAcquisitionLimits(options)
	core := censusacquisition.Core{
		Discoverer: fixedCensusRuntimeDiscovery{discovery: result.discovery},
		Acquirer:   censusRuntimeBatchAcquirer{runtime: r.runtime, admitted: result.admitted, limits: limits, execute: r.batchExecute},
		Planning:   &censusacquisition.PlanningConfig{DownDepth: int(options.downDepth), UpDepth: int(options.upDepth), MaxBatchTargets: int(options.maxBatchTargets)},
		OnErrorPhase: func(phase censusacquisition.Phase) {
			censusAcquisitionBranchTrace(ctx, "CORE_ERROR_"+string(phase))
		},
	}
	censusAcquisitionBranchTrace(ctx, "BEFORE_ACQUIRE")
	projection, err := core.Run(ctx, censusacquisition.SessionIdentity{SessionID: result.admitted.sessionID, Generation: result.admitted.generation})
	if err != nil {
		var typed *censusPrivateFailure
		if errors.As(err, &typed) {
			if r.acquisitionDiagnostic == nil {
				censusAcquisitionBranchTrace(ctx, "RECORDER_ABSENT")
			} else {
				recordErr := r.acquisitionDiagnostic.Record(censusdiagnostic.Record{Fingerprint: result.admitted.requestReceipt.Fingerprint, Category: typed.category, Ordinal: typed.ordinal, OperationCode: typed.operationCode, OperationCategory: typed.operationCategory, FailedField: typed.failedField, FailedInvariant: typed.failedInvariant, ExpectedCount: typed.expectedCount, ObservedCount: typed.observedCount})
				if recordErr != nil {
					censusAcquisitionBranchTrace(ctx, "RECORDER_REJECTED")
				} else {
					censusAcquisitionBranchTrace(ctx, "RECORD_ACCEPTED")
				}
			}
		}
		if errors.Is(err, censusacquisition.ErrDiscoveryIncomplete) {
			return censusacquisition.Projection{}, censusDiscoveryFailure(), reasonDiscoveryIncomplete
		}
		return censusacquisition.Projection{}, censusAcquisitionFailure(), reasonAcquisitionFailed
	}
	return projection, nil, reasonNone
}

type censusRuntimeEnumerator struct {
	workspace string
	roots     []string
	filters   censusacquisition.Filters
	maxFiles  int
}

func (e censusRuntimeEnumerator) Enumerate(ctx context.Context) ([]censusacquisition.SourceFile, error) {
	roots := append([]string(nil), e.roots...)
	sort.Slice(roots, func(i, j int) bool { return filepath.ToSlash(roots[i]) < filepath.ToSlash(roots[j]) })
	return censusacquisition.EnumerateWorkspaceContext(ctx, e.workspace, roots, source.LanguageID, e.filters, source.Limits{
		MaxEntries: censusRuntimeMaxEntries, MaxAccepted: e.maxFiles, MaxWork: censusRuntimeMaxWork,
		MaxPathBytes: censusRuntimeMaxPathBytes, MaxDepth: censusRuntimeMaxDepth,
	})
}

type censusRuntimeSupplier struct {
	manager    *sessionruntime.Manager
	sessionID  string
	generation uint64
}

func (s censusRuntimeSupplier) Supply(ctx context.Context, file censusacquisition.SourceFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.manager.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: s.sessionID, Generation: s.generation, URI: file.URI, LanguageID: file.LanguageID, CaptureSupply: true})
	if result.Failure != "" || result.URI != file.URI || result.LanguageID != file.LanguageID {
		return errors.New("document supply failed")
	}
	if result.Supply != nil && (result.Supply.URI != file.URI || result.Supply.SessionID != s.sessionID || result.Supply.Generation != s.generation) {
		return errors.New("document supply identity mismatch")
	}
	return nil
}

type censusRuntimeDiscoveryClient struct {
	manager        *sessionruntime.Manager
	sessionID      string
	generation     uint64
	requestTimeout time.Duration
}

func (c censusRuntimeDiscoveryClient) SupportsDocumentSymbols() bool { return true }
func (c censusRuntimeDiscoveryClient) SupportsCallHierarchy() bool   { return true }
func (c censusRuntimeDiscoveryClient) call(ctx context.Context, method string, params, output any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(c.requestTimeout)
	if parentDeadline, ok := ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	result := c.manager.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: c.sessionID, Generation: c.generation, Method: method, Params: raw, Deadline: deadline, MaxMessages: 64, MaxBytes: 4 << 20})
	if result.Failure != "" || result.ServerError != nil {
		return errors.New("census discovery request failed")
	}
	if len(result.Result) == 0 || string(result.Result) == "null" {
		return nil
	}
	return json.Unmarshal(result.Result, output)
}
func (c censusRuntimeDiscoveryClient) DocumentSymbols(ctx context.Context, params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var out []lsp.DocumentSymbol
	return out, c.call(ctx, "textDocument/documentSymbol", params, &out)
}
func (c censusRuntimeDiscoveryClient) PrepareCallHierarchy(ctx context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var out []lsp.CallHierarchyItem
	return out, c.call(ctx, "textDocument/prepareCallHierarchy", params, &out)
}

func censusRuntimeDiscover(ctx context.Context, manager *sessionruntime.Manager, admitted censusAdmittedSession, options censusRuntimeConfig) (censusacquisition.Discovery, error) {
	filters := censusacquisition.Filters{Includes: append([]string(nil), options.includes...), Excludes: append([]string(nil), options.excludes...)}
	adapter := censusacquisition.DiscoveryAdapter{
		Workspace: admitted.workspace,
		Filters:   filters,
		Files:     censusRuntimeEnumerator{workspace: admitted.workspace, roots: append([]string(nil), options.sources...), filters: filters, maxFiles: int(options.maxNodes)},
		Supplier:  censusRuntimeSupplier{manager: manager, sessionID: admitted.sessionID, generation: admitted.generation},
		Client:    censusRuntimeDiscoveryClient{manager: manager, sessionID: admitted.sessionID, generation: admitted.generation, requestTimeout: time.Duration(options.requestTimeoutMS) * time.Millisecond},
		Limits:    censusacquisition.DiscoveryLimits{MaxFiles: int(options.maxNodes), MaxSymbols: int(options.maxNodes)},
	}
	return adapter.Discover(ctx, censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation})
}
