package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

type lazyMCPContinuationWorker struct {
	once   sync.Once
	build  func() (*describeworker.Runner, error)
	runner *describeworker.Runner
	err    error
}

func (w *lazyMCPContinuationWorker) Run(ctx context.Context, request describerequest.Record, attempt string) (describeworker.RunResult, error) {
	w.once.Do(func() { w.runner, w.err = w.build() })
	if w.err != nil || w.runner == nil {
		return describeworker.RunResult{}, errors.New("catalog worker unavailable")
	}
	return w.runner.Run(ctx, request, attempt)
}

func (w *lazyMCPContinuationWorker) RunV2(ctx context.Context, request describerequest.Record, packet targetpacket.Packet, attempt string) (describeworker.RunResultV2, error) {
	w.once.Do(func() { w.runner, w.err = w.build() })
	if w.err != nil || w.runner == nil {
		return describeworker.RunResultV2{}, errors.New("catalog worker unavailable")
	}
	return w.runner.RunV2(ctx, request, packet, attempt)
}

type continuationConstructionStage string
type continuationConstructionReason string

const (
	constructionStageRootOpen                      continuationConstructionStage  = "ROOT_OPEN"
	constructionStageStorePrivateValidation        continuationConstructionStage  = "STORE_PRIVATE_VALIDATION"
	constructionStageContractConstruction          continuationConstructionStage  = "CONTRACT_CONSTRUCTION"
	constructionStageRuntimeArity                  continuationConstructionStage  = "RUNTIME_ARITY"
	constructionStagePreparationDiagnostic         continuationConstructionStage  = "PREPARATION_DIAGNOSTIC"
	constructionStageWorkerAssembly                continuationConstructionStage  = "WORKER_ASSEMBLY"
	constructionStageHostAssembly                  continuationConstructionStage  = "HOST_ASSEMBLY"
	constructionReasonRootOpenFailed               continuationConstructionReason = "ROOT_OPEN_FAILED"
	constructionReasonStorePrivateValidationFailed continuationConstructionReason = "STORE_PRIVATE_VALIDATION_FAILED"
	constructionReasonContractConstructionFailed   continuationConstructionReason = "CONTRACT_CONSTRUCTION_FAILED"
	constructionReasonRuntimeArityInvalid          continuationConstructionReason = "RUNTIME_ARITY_INVALID"
	constructionReasonPreparationDiagnosticFailed  continuationConstructionReason = "PREPARATION_DIAGNOSTIC_FAILED"
	constructionReasonWorkerAssemblyFailed         continuationConstructionReason = "WORKER_ASSEMBLY_FAILED"
	constructionReasonHostAssemblyFailed           continuationConstructionReason = "HOST_ASSEMBLY_FAILED"
)

type continuationConstructionError struct {
	Stage  continuationConstructionStage
	Reason continuationConstructionReason
	err    error
}

func (e *continuationConstructionError) Error() string {
	return "continuation host construction failed"
}
func (e *continuationConstructionError) Unwrap() error { return e.err }
func constructionFailure(stage continuationConstructionStage, reason continuationConstructionReason, err error) error {
	return &continuationConstructionError{Stage: stage, Reason: reason, err: err}
}

type productionCensusContinuationHost struct {
	root         *publication.Root
	store        *continuationhost.Store
	worker       censuscontinuation.Worker
	workerV2     censuscontinuation.WorkerV2
	contract     censuscontinuation.ContinuationContract
	freshCapture *censuscontinuation.FreshCaptureDependencies
}

func continuationDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type productionCensusContinuationFactory struct {
	openRoot     func(string) (*publication.Root, error)
	newStore     func(*publication.Root, int64) (*continuationhost.Store, error)
	newContract  func(censuscontinuation.ContractInput) (censuscontinuation.ContinuationContract, error)
	newWorker    func(func() (*describeworker.Runner, error)) (censuscontinuation.Worker, censuscontinuation.WorkerV2, error)
	newHost      func(*publication.Root, *continuationhost.Store, censuscontinuation.Worker, censuscontinuation.WorkerV2, censuscontinuation.ContinuationContract, *censuscontinuation.FreshCaptureDependencies) (*productionCensusContinuationHost, error)
	runtimeCount int
	runtime      interface {
		PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
		RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
		WorkspaceRoot(string, uint64) (string, bool)
		SessionLanguageID(string, uint64) (string, bool)
	}
}

func defaultProductionCensusContinuationFactory() productionCensusContinuationFactory {
	return productionCensusContinuationFactory{openRoot: publication.OpenRoot, newStore: continuationhost.NewStore, newContract: censuscontinuation.NewContinuationContract,
		newWorker: func(build func() (*describeworker.Runner, error)) (censuscontinuation.Worker, censuscontinuation.WorkerV2, error) {
			w := &lazyMCPContinuationWorker{build: build}
			return w, w, nil
		},
		newHost: func(root *publication.Root, store *continuationhost.Store, worker censuscontinuation.Worker, workerV2 censuscontinuation.WorkerV2, contract censuscontinuation.ContinuationContract, fresh *censuscontinuation.FreshCaptureDependencies) (*productionCensusContinuationHost, error) {
			return &productionCensusContinuationHost{root: root, store: store, worker: worker, workerV2: workerV2, contract: contract, freshCapture: fresh}, nil
		}}
}

func newProductionCensusContinuationHost(config bootstrapContinuationConfig, runtimes ...interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
	WorkspaceRoot(string, uint64) (string, bool)
	SessionLanguageID(string, uint64) (string, bool)
}) (*productionCensusContinuationHost, error) {
	factory := defaultProductionCensusContinuationFactory()
	factory.runtimeCount = len(runtimes)
	if len(runtimes) == 1 {
		factory.runtime = runtimes[0]
	}
	return newProductionCensusContinuationHostWithFactory(config, factory)
}

func newProductionCensusContinuationHostWithFactory(config bootstrapContinuationConfig, factory productionCensusContinuationFactory) (*productionCensusContinuationHost, error) {
	root, err := factory.openRoot(config.PublicationRoot)
	if err != nil {
		return nil, constructionFailure(constructionStageRootOpen, constructionReasonRootOpenFailed, err)
	}
	store, err := factory.newStore(root, config.MaxObjectBytes)
	if err == nil && store != nil {
		store.SetDescriptorTrace(descriptorPublicationTrace)
	}
	if err != nil {
		_ = root.Close()
		return nil, constructionFailure(constructionStageStorePrivateValidation, constructionReasonStorePrivateValidationFailed, err)
	}
	workerConfig := describeworker.Config{ResponseVersion: describeworker.ResponseVersionV2}
	if config.Worker != nil {
		workerConfig = config.Worker.config()
	}
	worker, workerV2, err := factory.newWorker(func() (*describeworker.Runner, error) {
		bundle, buildErr := continuationhost.New(continuationhost.Config{Root: root, MaxObjectBytes: config.MaxObjectBytes, Worker: workerConfig}, continuationhost.Primitives{})
		if buildErr != nil {
			return nil, buildErr
		}
		return bundle.Runner, nil
	})
	if err != nil {
		_ = root.Close()
		return nil, constructionFailure(constructionStageWorkerAssembly, constructionReasonWorkerAssemblyFailed, err)
	}
	contract, err := factory.newContract(censuscontinuation.ContractInput{ContinuationID: continuationDigest("lsp-trace-mcp-census-catalog-v1"), ProfileID: continuationDigest("lsp-trace-mcp-census-catalog-profile-v1"), WorkerPinID: continuationDigest(workerConfig), ProgramCSeed: 9, ResponseVersion: describeworker.ResponseVersionV2, CaptureLimits: censuscontinuation.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "private-census-catalog-v1", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 90_000, RetryPolicy: censuscontinuation.RetryPolicy{MaxAttemptsPerRequest: 2}})
	if err != nil {
		_ = root.Close()
		return nil, constructionFailure(constructionStageContractConstruction, constructionReasonContractConstructionFailed, err)
	}
	if factory.runtimeCount > 1 {
		_ = root.Close()
		return nil, constructionFailure(constructionStageRuntimeArity, constructionReasonRuntimeArityInvalid, errors.New("invalid runtime arity"))
	}
	var recorder censuscontinuation.ManagedPreparationDiagnosticRecorder
	if config.ManagedPreparationDiagnosticPath != "" {
		recorder, err = censuscontinuation.NewManagedPreparationDiagnosticFileRecorder(config.ManagedPreparationDiagnosticPath, censuscontinuation.DefaultManagedPreparationDiagnosticMaxBytes, censuscontinuation.DefaultManagedPreparationDiagnosticMaxRecords)
		if err != nil {
			_ = root.Close()
			return nil, constructionFailure(constructionStagePreparationDiagnostic, constructionReasonPreparationDiagnosticFailed, err)
		}
	}
	var fresh *censuscontinuation.FreshCaptureDependencies
	if factory.runtimeCount == 1 && factory.runtime != nil {
		fresh = &censuscontinuation.FreshCaptureDependencies{Context: context.Background(), Preparer: censuscontinuation.ManagedPreparer{Preparer: factory.runtime, Workspace: factory.runtime, LanguageResolver: factory.runtime, Recorder: recorder}, Resolver: liveprojection.FullDefinitionResolver{Requester: factory.runtime, Limits: liveprojection.DisplayResolutionLimits{MaxWork: 100, MaxMessages: 32, MaxBytes: 1 << 20, RequestTimeout: 15 * time.Second}}, Limits: censuscontinuation.ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 100, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20}}
	}
	host, err := factory.newHost(root, store, worker, workerV2, contract, fresh)
	if err != nil {
		_ = root.Close()
		return nil, constructionFailure(constructionStageHostAssembly, constructionReasonHostAssemblyFailed, err)
	}
	return host, nil
}

func (h *productionCensusContinuationHost) Close() error {
	if h == nil || h.root == nil {
		return nil
	}
	return h.root.Close()
}

func (h *productionCensusContinuationHost) Fresh(ctx context.Context, completion censusCompletion, publicStop string) (censusContinuationMCPResult, error) {
	continuationBoundaryTrace(ctx, "HOST_FRESH", "ENTER", "NONE", "NONE")
	handoff, err := completion.BuildCommittedHandoff()
	if err != nil {
		cause := completionUnknown
		if completion.Result == nil {
			cause = completionResultMissing
		} else if completion.Diagnostic != nil {
			cause = completionDiagnosticPresent
		} else if completion.continuation == nil {
			cause = custodyMissing
		} else if completion.continuationError != "" {
			cause = completion.continuationError
		} else {
			cause = classifyHandoffError(err)
		}
		continuationPreconditionCauseTrace(ctx, censuscontinuation.PreconditionCauseHandoffBuild, cause)
		stage := handoffFailureStage(err)
		censusHandoffStageTrace(ctx, stage)
		if stage == censusprogramc.StageComposition {
			var failure *censusprogramc.Failure
			if errors.As(err, &failure) && failure != nil {
				censusHandoffBranchTrace(ctx, programccompose.ErrorBranch(failure.Err))
			}
		}
		return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationHandoffBuildFailed, censusresult.ContinuationDiagnosticContext{})
	}
	workspace, err := url.Parse(handoff.WorkspaceIdentity().URI)
	if err != nil || workspace.Scheme != "file" {
		continuationPreconditionCauseTrace(ctx, censuscontinuation.PreconditionCauseInputValidation)
		return censusContinuationMCPResult{}, errors.New("continuation workspace unavailable")
	}
	stopAfter := censuscontinuation.StopAfter("")
	worker := h.worker
	workerV2 := h.workerV2
	if publicStop == "DESCRIBE_REQUESTS" {
		stopAfter = censuscontinuation.StopAfterDescribeRequests
		worker = nil
		workerV2 = nil
	}
	continuationBoundaryTrace(ctx, "CHECKPOINT", "ENTER", "NONE", "NONE")
	result := censuscontinuation.Run(ctx, censuscontinuation.Request{Handoff: handoff, Workspace: workspace.Path, Contract: h.contract, Worker: worker, WorkerV2: workerV2, Store: h.store, FreshCapture: h.freshCapture, StopAfter: stopAfter})
	if result.Err != nil {
		continuationPreconditionCauseTrace(ctx, result.PreconditionCause)
		continuationBoundaryTrace(ctx, "CHECKPOINT", "RETURN", "ERROR", "UNTYPED")
	} else {
		continuationBoundaryTrace(ctx, "CHECKPOINT", "RETURN", "OK", "NONE")
	}
	projected, err := h.publish(ctx, result)
	if completion.Result != nil {
		projected.Census = completion.Result
	}
	if err != nil {
		if _, typed := censusresult.ContinuationDiagnosticFromError(err); typed {
			continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "ERROR", "TYPED")
		} else {
			continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "ERROR", "UNTYPED")
		}
	} else {
		continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "OK", "NONE")
	}
	return projected, err
}

func handoffFailureStage(err error) censusprogramc.Stage {
	var failure *censusprogramc.Failure
	if !errors.As(err, &failure) || failure == nil {
		return ""
	}
	switch failure.Stage {
	case censusprogramc.StagePublication, censusprogramc.StageReconciliation, censusprogramc.StageComposition,
		censusprogramc.StageAdmission, censusprogramc.StageComputation, censusprogramc.StageRepresentative:
		return failure.Stage
	default:
		return ""
	}
}

func classifyHandoffError(err error) completionSubcause {
	switch censuscontinuation.ClassifyValidationCause(err) {
	case censuscontinuation.ValidationCauseProjection:
		return handoffProjection
	case censuscontinuation.ValidationCauseWorkspace:
		return handoffWorkspace
	case censuscontinuation.ValidationCausePositionEncoding:
		return handoffPositionEncoding
	case censuscontinuation.ValidationCauseCensusResult:
		return handoffResult
	case censuscontinuation.ValidationCauseIdentityReconciliation:
		return handoffIdentityReconciliation
	case censuscontinuation.ValidationCausePublicationReconciliation:
		return handoffPublicationReconciliation
	case censuscontinuation.ValidationCauseManifest:
		return handoffManifest
	case censuscontinuation.ValidationCauseCompose:
		return handoffCompose
	case censuscontinuation.ValidationCauseIdentity:
		return handoffIdentity
	default:
		return completionUnknown
	}
}

func (h *productionCensusContinuationHost) Resume(ctx context.Context, selector, publicStop string) (censusContinuationMCPResult, error) {
	continuationBoundaryTrace(ctx, "HOST_RESUME", "ENTER", "NONE", "NONE")
	descriptor, err := h.store.ResolveDescriptor(ctx, selector)
	if err != nil {
		return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationDescriptorUnavailable, censusresult.ContinuationDiagnosticContext{PublicSelector: selector})
	}
	stopAfter := censuscontinuation.StopAfter("")
	worker := h.worker
	workerV2 := h.workerV2
	if publicStop == "DESCRIBE_REQUESTS" {
		stopAfter = censuscontinuation.StopAfterDescribeRequests
		worker = nil
		workerV2 = nil
	}
	continuationBoundaryTrace(ctx, "CHECKPOINT", "ENTER", "NONE", "NONE")
	result := censuscontinuation.Resume(ctx, censuscontinuation.ResumeRequest{Selector: descriptor.CheckpointSelector, UseCommittedWorkspace: true, Store: h.store, Worker: worker, WorkerV2: workerV2, FreshCapture: h.freshCapture, StopAfter: stopAfter})
	if result.Err != nil {
		continuationBoundaryTrace(ctx, "CHECKPOINT", "RETURN", "ERROR", "UNTYPED")
	} else {
		continuationBoundaryTrace(ctx, "CHECKPOINT", "RETURN", "OK", "NONE")
	}
	if result.Err != nil && result.CheckpointID == "" {
		if result.Err.Error() == "stop checkpoint mismatch" {
			return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationStopCheckpointMismatch, censusresult.ContinuationDiagnosticContext{PublicSelector: selector, ExpectedStage: string(censuscontinuation.StageRequestsRendered), ExpectedStatus: string(censuscontinuation.StatusRunning), ObservedStage: result.InvocationStage, ObservedStatus: result.InvocationStatus})
		}
		return censusContinuationMCPResult{}, errors.New("resume needs workspace or is unsupported")
	}
	projected, err := h.publish(ctx, result)
	if err != nil {
		if _, typed := censusresult.ContinuationDiagnosticFromError(err); typed {
			continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "ERROR", "TYPED")
		} else {
			continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "ERROR", "UNTYPED")
		}
		return censusContinuationMCPResult{}, err
	}
	continuationBoundaryTrace(ctx, "HOST_RESUME", "RETURN", "OK", "NONE")
	projected.CensusSelector = projected.Descriptor
	projected.CensusDigest = continuationhost.Digest([]byte(projected.Descriptor))
	projected.CensusByteLength = uint64(len(projected.Descriptor))
	return projected, nil
}

func (h *productionCensusContinuationHost) publish(ctx context.Context, result censuscontinuation.Result) (censusContinuationMCPResult, error) {
	var terminalPersistErr *censuscontinuation.TerminalCheckpointPersistError
	if errors.As(result.Err, &terminalPersistErr) {
		return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationPublicationFailed, censusresult.ContinuationDiagnosticContext{})
	}
	if d, ok := result.CaptureFailureDiagnostic(); ok {
		return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationCaptureFailed, censusresult.ContinuationDiagnosticContext{ObservedStage: string(d.Stage), ObservedStatus: string(censuscontinuation.StatusFailedCapture), ResourceCategory: d.Category, ResourceObserved: d.Observed, ResourceLimit: d.Limit, ResourceField: d.FailedField, CaptureInvariant: d.Invariant, CaptureCallerAction: d.CallerAction, CaptureRecovery: d.Recovery})
	}
	if h == nil || h.store == nil || result.CheckpointID == "" {
		return censusContinuationMCPResult{}, errors.New("continuation checkpoint unavailable")
	}
	catalogSelector := result.CatalogSelector
	if catalogSelector == "" && result.Status == censuscontinuation.StatusPaused {
		catalogSelector = result.CheckpointID
	}
	if catalogSelector == "" {
		return censusContinuationMCPResult{}, errors.New("continuation checkpoint unavailable")
	}
	compositeSelector := result.CheckpointID
	if result.Err == nil {
		if raw, err := result.Composite.Bytes(); err == nil {
			if selector, putErr := h.store.Put(ctx, raw); putErr == nil {
				compositeSelector = selector
			}
		}
	}
	continuationBoundaryTrace(ctx, "DESCRIPTOR", "ENTER", "NONE", "NONE")
	descriptor, err := h.store.PublishDescriptor(ctx, continuationhost.DescriptorInput{CatalogSelector: catalogSelector, CheckpointSelector: result.CheckpointID, CompositeSelector: compositeSelector})
	if err != nil {
		continuationBoundaryTrace(ctx, "DESCRIPTOR", "RETURN", "ERROR", "TYPED")
		var publicationErr *continuationhost.DescriptorPublicationError
		if errors.As(err, &publicationErr) {
			fmt.Fprintln(os.Stderr, "continuation descriptor publication cause="+string(publicationErr.Cause()))
		}
		_ = dumpRuntimeTrace(ctx, "descriptor_publication")
		return censusContinuationMCPResult{}, censusresult.NewContinuationFailure(censusresult.ContinuationPublicationFailed, censusresult.ContinuationDiagnosticContext{})
	}
	continuationBoundaryTrace(ctx, "DESCRIPTOR", "RETURN", "OK", "NONE")
	status := "COMPLETE"
	if result.Status == censuscontinuation.StatusPaused && result.Err == nil {
		status = "PAUSED"
	} else if result.Err != nil || result.Status != censuscontinuation.StatusComplete {
		status = "DEGRADED"
	}
	return censusContinuationMCPResult{Descriptor: descriptor, Status: status, RequestCount: result.RequestCount, PreparationCount: result.PreparationCount}, nil
}
