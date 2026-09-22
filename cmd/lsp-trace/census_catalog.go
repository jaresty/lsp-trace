package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

type censusCatalogHostConfig struct {
	Version      int             `json:"version"`
	Processes    json.RawMessage `json:"processes,omitempty"`
	Providers    json.RawMessage `json:"providers,omitempty"`
	Continuation *struct {
		PublicationRoot string `json:"publication_root"`
		MaxObjectBytes  int64  `json:"max_object_bytes"`
		Worker          struct {
			Worker            describeworker.FilePin `json:"worker"`
			Model             describeworker.FilePin `json:"model"`
			Library           describeworker.FilePin `json:"library"`
			SandboxExecutable describeworker.FilePin `json:"sandbox_executable"`
			SandboxProfile    describeworker.FilePin `json:"sandbox_profile"`
			Grammar           describeworker.FilePin `json:"grammar"`
			RuntimeIdentity   string                 `json:"runtime_identity"`
			AdapterIdentity   string                 `json:"adapter_identity"`
			ModelIdentity     string                 `json:"model_identity"`
			Limits            describeworker.Limits  `json:"limits"`
		} `json:"worker"`
	} `json:"continuation"`
}

func loadCensusCatalogConfig(configPath, publicationRoot string) (censusCatalogHostConfig, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil || len(raw) > 4<<20 {
		return censusCatalogHostConfig{}, errors.New("catalog host configuration unavailable")
	}
	var cfg censusCatalogHostConfig
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if dec.Decode(&cfg) != nil || cfg.Version != 1 || cfg.Continuation == nil || (publicationRoot != "" && cfg.Continuation.PublicationRoot != publicationRoot) || cfg.Continuation.MaxObjectBytes < 1 {
		return censusCatalogHostConfig{}, errors.New("catalog host configuration invalid")
	}
	return cfg, nil
}
func openCensusCatalogHost(configPath, publicationRoot string) (*continuationhost.Bundle, error) {
	cfg, err := loadCensusCatalogConfig(configPath, publicationRoot)
	if err != nil {
		return nil, err
	}
	root, err := publication.OpenRoot(publicationRoot)
	if err != nil {
		return nil, errors.New("catalog host configuration unavailable")
	}
	store, err := continuationhost.NewStore(root, cfg.Continuation.MaxObjectBytes)
	if err != nil {
		return nil, errors.New("catalog host configuration unavailable")
	}
	return &continuationhost.Bundle{Store: store}, nil
}

type lazyCensusCatalogWorker struct {
	once   sync.Once
	build  func() (*describeworker.Runner, error)
	runner *describeworker.Runner
	err    error
}

func (w *lazyCensusCatalogWorker) Run(ctx context.Context, r describerequest.Record, attempt string) (describeworker.RunResult, error) {
	w.once.Do(func() { w.runner, w.err = w.build() })
	if w.err != nil || w.runner == nil {
		return describeworker.RunResult{}, errors.New("catalog worker unavailable")
	}
	return w.runner.Run(ctx, r, attempt)
}

func (w *lazyCensusCatalogWorker) RunV2(ctx context.Context, r describerequest.Record, packet targetpacket.Packet, attempt string) (describeworker.RunResultV2, error) {
	w.once.Do(func() { w.runner, w.err = w.build() })
	if w.err != nil || w.runner == nil {
		return describeworker.RunResultV2{}, errors.New("catalog worker unavailable")
	}
	return w.runner.RunV2(ctx, r, packet, attempt)
}

func newLazyCensusCatalogWorker(configPath, publicationRoot string) censuscontinuation.WorkerV2 {
	return &lazyCensusCatalogWorker{build: func() (*describeworker.Runner, error) {
		cfg, err := loadCensusCatalogConfig(configPath, publicationRoot)
		if err != nil {
			return nil, err
		}
		root, err := publication.OpenRoot(publicationRoot)
		if err != nil {
			return nil, err
		}
		w := cfg.Continuation.Worker
		bundle, err := continuationhost.New(continuationhost.Config{Root: root, MaxObjectBytes: cfg.Continuation.MaxObjectBytes, Worker: describeworker.Config{Worker: w.Worker, Model: w.Model, Library: w.Library, SandboxExecutable: w.SandboxExecutable, SandboxProfile: w.SandboxProfile, Grammar: w.Grammar, RuntimeIdentity: w.RuntimeIdentity, AdapterIdentity: w.AdapterIdentity, ModelIdentity: w.ModelIdentity, ResponseVersion: describeworker.ResponseVersionV2, Limits: w.Limits}}, continuationhost.Primitives{})
		if err != nil {
			return nil, err
		}
		return bundle.Runner, nil
	}}
}

func digestCatalog(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

func censusCatalogContract(configPath string) (censuscontinuation.ContinuationContract, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return censuscontinuation.ContinuationContract{}, errors.New("catalog host unavailable")
	}
	var cfg censusCatalogHostConfig
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if dec.Decode(&cfg) != nil || cfg.Continuation == nil {
		return censuscontinuation.ContinuationContract{}, errors.New("catalog host unavailable")
	}
	return censuscontinuation.NewContinuationContract(censuscontinuation.ContractInput{
		ContinuationID: digestCatalog("lsp-trace-cli-census-catalog-v1"), ProfileID: digestCatalog("lsp-trace-cli-census-catalog-profile-v1"), WorkerPinID: digestCatalog(cfg.Continuation.Worker), ProgramCSeed: 9, ResponseVersion: describeworker.ResponseVersionV2,
		CaptureLimits:       censuscontinuation.ProductionCaptureLimits(),
		PacketSourcePolicy:  sourceprojection.Policy{PolicyID: "private-census-catalog-v1", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true},
		PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 90_000, RetryPolicy: censuscontinuation.RetryPolicy{MaxAttemptsPerRequest: 2},
	})
}

func resolveCensusCatalogDescriptor(ctx context.Context, bundle *continuationhost.Bundle, selector string) (continuationhost.DescriptorInput, error) {
	if bundle == nil || bundle.Store == nil {
		return continuationhost.DescriptorInput{}, errors.New("catalog host unavailable")
	}
	return bundle.Store.ResolveDescriptor(ctx, selector)
}

func publishCensusCatalogDescriptor(ctx context.Context, bundle *continuationhost.Bundle, in continuationhost.DescriptorInput) (string, error) {
	if bundle == nil || bundle.Store == nil {
		return "", errors.New("catalog host unavailable")
	}
	return bundle.Store.PublishDescriptor(ctx, in)
}

func censusFreshCaptureDependencies(ctx context.Context, runtime interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}, requestTimeout time.Duration) *censuscontinuation.FreshCaptureDependencies {
	if runtime == nil {
		return nil
	}
	if requestTimeout <= 0 {
		requestTimeout = 15 * time.Second
	}
	return &censuscontinuation.FreshCaptureDependencies{
		Context:  ctx,
		Preparer: censuscontinuation.ManagedPreparer{Preparer: runtime},
		Resolver: liveprojection.FullDefinitionResolver{Requester: runtime, Limits: liveprojection.DisplayResolutionLimits{MaxWork: 100, MaxMessages: 32, MaxBytes: 1 << 20, RequestTimeout: requestTimeout}},
		Limits:   censuscontinuation.ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 100, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20},
	}
}

func runCensusCatalogFresh(o censusCLIOptions, stdout, stderr io.Writer, census censusPublicationOutcome, runtime *initializedAcquisitionRuntime, deps censusRunnerDependencies) int {
	if os.Getenv("LSP_TRACE_PRIVATE_TEST_DIAGNOSTIC") == "1" {
		verification, directorySync, closeStatus := "ABSENT", "ABSENT", "ABSENT"
		if census.Result != nil {
			verification = census.Result.Publication.VerificationStatus
			directorySync = census.Result.Publication.DirectorySyncStatus
			closeStatus = census.Result.Publication.CloseStatus
		}
		_, _ = fmt.Fprintf(stderr, "PRIVATE_CENSUS_OUTCOME result=%t diagnostic=%t custody=%t verification=%s directory_sync=%s close=%s\n", census.Result != nil, census.Diagnostic != nil, census.continuation != nil, verification, directorySync, closeStatus)
	}
	if census.Result == nil || census.Diagnostic != nil {
		return writeCensusOutcome(stdout, stderr, o.Machine, census)
	}
	host, err := deps.openCatalogHost(o.CatalogConfigPath, o.PublicationRoot)
	if err != nil {
		writePrivateCatalogStage(stderr, "HOST_OPEN_FAILED")
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationHostConstructionFailed, censusresult.ContinuationDiagnosticContext{})
	}
	handoff, err := census.BuildCommittedHandoff()
	if err != nil {
		writePrivateCatalogStage(stderr, "HANDOFF_FAILED: "+err.Error())
		return writeCensusCatalogOutcome(stdout, stderr, o.Machine, census, censuscontinuation.Result{Status: censuscontinuation.StatusFailedCatalog, CensusID: census.Result.CensusID, Err: err}, "")
	}
	contract, err := deps.buildCatalogContract(o.CatalogConfigPath)
	if err != nil {
		writePrivateCatalogStage(stderr, "CONTRACT_FAILED")
		return writeCensusCatalogOutcome(stdout, stderr, o.Machine, census, censuscontinuation.Result{Status: censuscontinuation.StatusFailedCatalog, CensusID: census.Result.CensusID, Err: err}, "")
	}
	if os.Getenv("LSP_TRACE_PRIVATE_TEST_DIAGNOSTIC") == "1" {
		writePrivateHandoffPreflight(stderr, handoff, host.Store.MaxObjectBytes())
	}
	ctx := context.Background()
	if runtime != nil && runtime.ctx != nil {
		ctx = runtime.ctx
	}
	var worker censuscontinuation.WorkerV2
	stopAfter := censuscontinuation.StopAfter("")
	if o.StopAfter == "describe-requests" {
		stopAfter = censuscontinuation.StopAfterDescribeRequests
	} else {
		worker = deps.newCatalogWorker(o.CatalogConfigPath, o.PublicationRoot)
	}
	result := deps.runCatalog(ctx, censuscontinuation.Request{Handoff: handoff, Workspace: o.Workspace, Contract: contract, WorkerV2: worker, Store: host.Store, FreshCapture: censusFreshCaptureDependencies(ctx, runtime, o.RequestTimeout), StopAfter: stopAfter})
	if os.Getenv("LSP_TRACE_PRIVATE_TEST_DIAGNOSTIC") == "1" {
		_, _ = fmt.Fprintf(stderr, "PRIVATE_CATALOG_OUTCOME status=%s checkpoint=%t error=%t stage=%s code=%s subcode=%s invocation_bytes=%d response_present=%t response_bytes=%d composite_bytes=%d store_cap=%d invocation_stage=%s invocation_status=%s invocation_code=%s\n", result.Status, result.CheckpointID != "", result.Err != nil, result.PrivateStage, result.PrivateCode, result.PrivateSubcode, result.InvocationCanonicalBytes, result.ResponsePresent, result.ResponseBytes, result.WorkerCompositeBytes, result.WorkerStoreCap, result.InvocationStage, result.InvocationStatus, result.InvocationCode)
	}
	if result.Err != nil && result.Err.Error() == "stop checkpoint mismatch" {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationStopCheckpointMismatch, censusresult.ContinuationDiagnosticContext{ExpectedStage: string(censuscontinuation.StageRequestsRendered), ExpectedStatus: string(censuscontinuation.StatusRunning), ObservedStage: result.InvocationStage, ObservedStatus: result.InvocationStatus})
	}
	if d, ok := result.CaptureFailureDiagnostic(); ok {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationCaptureFailed, censusresult.ContinuationDiagnosticContext{ObservedStage: string(d.Stage), ObservedStatus: string(censuscontinuation.StatusFailedCapture), ResourceCategory: d.Category, ResourceObserved: d.Observed, ResourceLimit: d.Limit, ResourceField: d.FailedField, CaptureInvariant: d.Invariant, CaptureCallerAction: d.CallerAction, CaptureRecovery: d.Recovery})
	}
	selector, publishErr := publishCatalogResult(context.Background(), host, result, deps)
	if publishErr != nil {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationPublicationFailed, censusresult.ContinuationDiagnosticContext{})
	}
	return writeCensusCatalogOutcome(stdout, stderr, o.Machine, census, result, selector)
}

func writePrivateHandoffPreflight(stderr io.Writer, handoff censuscontinuation.CommittedHandoff, maxObjectBytes int64) {
	raw, err := handoff.Bytes()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "PRIVATE_HANDOFF_PREFLIGHT canonical_bytes=ERROR max_object_bytes=", maxObjectBytes)
		return
	}
	_, _ = fmt.Fprintf(stderr, "PRIVATE_HANDOFF_PREFLIGHT canonical_bytes=%d max_object_bytes=%d\n", len(raw), maxObjectBytes)
}

func writePrivateCatalogStage(stderr io.Writer, stage string) {
	if os.Getenv("LSP_TRACE_PRIVATE_TEST_DIAGNOSTIC") == "1" {
		_, _ = fmt.Fprintf(stderr, "PRIVATE_CATALOG_STAGE %s\n", stage)
	}
}

func runCensusCatalogResume(o censusCLIOptions, stdout, stderr io.Writer, deps censusRunnerDependencies) int {
	host, err := deps.openCatalogHost(o.CatalogConfigPath, o.PublicationRoot)
	if err != nil {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationHostConstructionFailed, censusresult.ContinuationDiagnosticContext{})
	}
	descriptor, err := deps.resolveCatalogDescriptor(context.Background(), host, o.Resume)
	if err != nil {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationDescriptorUnavailable, censusresult.ContinuationDiagnosticContext{PublicSelector: o.Resume})
	}
	var worker censuscontinuation.WorkerV2
	stopAfter := censuscontinuation.StopAfter("")
	if o.StopAfter == "describe-requests" {
		stopAfter = censuscontinuation.StopAfterDescribeRequests
	} else {
		worker = deps.newCatalogWorker(o.CatalogConfigPath, o.PublicationRoot)
	}
	result := deps.resumeCatalog(context.Background(), censuscontinuation.ResumeRequest{Selector: descriptor.CheckpointSelector, Workspace: o.Workspace, Store: host.Store, WorkerV2: worker, StopAfter: stopAfter})
	if result.Err != nil && result.Err.Error() == "stop checkpoint mismatch" {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationStopCheckpointMismatch, censusresult.ContinuationDiagnosticContext{PublicSelector: o.Resume, ExpectedStage: string(censuscontinuation.StageRequestsRendered), ExpectedStatus: string(censuscontinuation.StatusRunning), ObservedStage: result.InvocationStage, ObservedStatus: result.InvocationStatus})
	}
	if d, ok := result.CaptureFailureDiagnostic(); ok {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationCaptureFailed, censusresult.ContinuationDiagnosticContext{PublicSelector: o.Resume, ObservedStage: string(d.Stage), ObservedStatus: string(censuscontinuation.StatusFailedCapture), ResourceCategory: d.Category, ResourceObserved: d.Observed, ResourceLimit: d.Limit, ResourceField: d.FailedField, CaptureInvariant: d.Invariant, CaptureCallerAction: d.CallerAction, CaptureRecovery: d.Recovery})
	}
	selector, publishErr := publishCatalogResult(context.Background(), host, result, deps)
	if publishErr != nil {
		return writeCensusContinuationDiagnostic(stdout, stderr, o.Machine, censusresult.ContinuationPublicationFailed, censusresult.ContinuationDiagnosticContext{PublicSelector: o.Resume})
	}
	return writeCensusCatalogOutcome(stdout, stderr, o.Machine, censusPublicationOutcome{}, result, selector)
}

func writeCensusContinuationDiagnostic(stdout, stderr io.Writer, machine bool, cause censusresult.ContinuationCause, context censusresult.ContinuationDiagnosticContext) int {
	diagnostic, err := censusresult.NewContinuationDiagnostic(cause, context)
	if err != nil {
		return writeCensusFailure(stderr, censusStageCommitted, machine, err)
	}
	if machine {
		raw, marshalErr := censusresult.MarshalContinuationDiagnostic(diagnostic)
		if marshalErr != nil {
			return writeCensusFailure(stderr, censusStageCommitted, true, marshalErr)
		}
		_, _ = stdout.Write(append(raw, '\n'))
		return 0
	}
	_, _ = fmt.Fprintf(stdout, "census commit preserved; continuation failed code=%s; %s\n", diagnostic.Code, diagnostic.Guidance)
	return 0
}

func publishCatalogResult(ctx context.Context, host *continuationhost.Bundle, r censuscontinuation.Result, deps censusRunnerDependencies) (string, error) {
	checkpoint := r.CheckpointID
	if checkpoint == "" {
		return "", errors.New("continuation checkpoint unavailable")
	}
	catalog := r.CatalogSelector
	if catalog == "" && r.Status == censuscontinuation.StatusPaused {
		catalog = checkpoint
	}
	if catalog == "" {
		return "", errors.New("continuation catalog unavailable")
	}
	composite := checkpoint
	if r.Err == nil {
		if raw, err := r.Composite.Bytes(); err == nil {
			if s, e := host.Store.Put(ctx, raw); e == nil {
				composite = s
			}
		}
	}
	d, err := deps.publishCatalogDescriptor(ctx, host, continuationhost.DescriptorInput{CatalogSelector: catalog, CheckpointSelector: checkpoint, CompositeSelector: composite})
	if err != nil {
		return "", errors.New("continuation publication failed")
	}
	return d, nil
}

type censusCatalogDescriptor struct {
	Kind               string `json:"kind"`
	CheckpointSelector string `json:"checkpoint_selector"`
	CompositeSelector  string `json:"composite_selector"`
	CatalogSelector    string `json:"catalog_selector"`
	Status             string `json:"status"`
	RequestCount       int    `json:"request_count,omitempty"`
	PreparationCount   int    `json:"preparation_count,omitempty"`
	ResumeGuidance     string `json:"resume_guidance,omitempty"`
	Authority          int    `json:"authority"`
	Accepted           bool   `json:"accepted"`
	Completeness       string `json:"completeness"`
}
type censusCatalogIdentity struct {
	Selector   string `json:"selector"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
}
type censusCatalogResult struct {
	SchemaVersion  string                  `json:"schema_version"`
	Census         *censusCLIResult        `json:"census,omitempty"`
	CensusIdentity *censusCatalogIdentity  `json:"census_identity,omitempty"`
	Catalog        censusCatalogDescriptor `json:"catalog"`
}

func writeCensusCatalogOutcome(stdout, stderr io.Writer, machine bool, census censusPublicationOutcome, r censuscontinuation.Result, selector string) int {
	status := "COMPLETE"
	if r.Status == censuscontinuation.StatusPaused && r.Err == nil && selector != "" {
		status = "PAUSED"
	} else if r.Err != nil || r.Status != censuscontinuation.StatusComplete || selector == "" {
		status = "DEGRADED"
		if selector == "" {
			selector = r.CheckpointID
		}
	}
	if selector == "" {
		selector = "sha256:" + strings.Repeat("0", 64) + ":0"
	}
	descriptor := censusCatalogDescriptor{Kind: "ADR_0007_FEATURE_CATALOG", CheckpointSelector: selector, CompositeSelector: selector, CatalogSelector: selector, Status: status, Authority: 0, Accepted: false, Completeness: "UNKNOWN"}
	if status == "PAUSED" {
		descriptor.RequestCount = r.RequestCount
		descriptor.PreparationCount = r.PreparationCount
		descriptor.ResumeGuidance = "Resume with this selector and omit stop_after to continue exactly the remaining work once."
	}
	out := censusCatalogResult{SchemaVersion: "lsp-trace.census-feature-catalog-result.v2", Census: census.Result, Catalog: descriptor}
	if census.Result == nil {
		out.CensusIdentity = &censusCatalogIdentity{Selector: selector, Digest: digestCatalog(selector), ByteLength: uint64(len(selector))}
	}
	if machine {
		raw, err := json.Marshal(out)
		if err == nil {
			err = mcpcontract.ValidateFutureCensusCompositeResultV2(raw)
		}
		if err != nil {
			writePrivateCatalogStage(stderr, "RESULT_VALIDATE_FAILED: "+err.Error())
			return writeCensusFailure(stderr, censusStageCommitted, true, err)
		}
		_, _ = stdout.Write(append(raw, '\n'))
		return 0
	}
	fmt.Fprintf(stdout, "census committed; catalog %s selector=%s\n", status, selector)
	return 0
}
