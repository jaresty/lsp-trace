package main

import (
	"context"
	"errors"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/programccompose"
)

// censusCoreDependencies is a package-main-only test seam. Production callers
// use productionCensusCoreDependencies; none of these functions confer census
// assembly or publication authority outside package main.
type censusCoreDependencies struct {
	runSession          func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int
	discover            func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error)
	acquire             func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error)
	acquireDiagnostic   func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int, *censusdiagnostic.Recorder, *censusControlFlowObserver) (censusAssembly, error)
	capability          func(censusAssembly) (censusPublicationCapability, error)
	publish             func(context.Context, censusPublicationCapability, string) censusPublicationOutcome
	publishContinuation func(context.Context, censusPublicationCapability, string, programccompose.ExactMetadata) censusPublicationOutcome
	publishEnumeration  func(context.Context, censusacquisition.Discovery, censusrequest.Receipt, string) censusenumerationprojection.Receipt
}

type censusCoreConfig struct {
	runner        initializedAcquisitionRunnerConfig
	limits        acquisitionops.Limits
	callHierarchy bool
	diagnostic    *censusdiagnostic.Recorder
	afterCommit   func(context.Context, *initializedAcquisitionRuntime, censusPublicationOutcome) int
}

type fixedCensusDiscovery struct{ discovery censusacquisition.Discovery }

func (d fixedCensusDiscovery) Discover(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return d.discovery, nil
}

func productionCensusCoreDependencies() censusCoreDependencies {
	return censusCoreDependencies{
		runSession: func(cfg initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			return runInitializedAcquisitionSession(cfg, func(ctx context.Context, session initializedAcquisitionSession) int {
				runtime, ok := session.(*initializedAcquisitionRuntime)
				if !ok {
					return 1
				}
				return callback(ctx, runtime)
			})
		},
		discover: runInitializedCensusDiscovery,
		acquire: func(ctx context.Context, runtime *initializedAcquisitionRuntime, discoverer censusacquisition.Discoverer, limits acquisitionops.Limits, downDepth, upDepth, maxBatchTargets int) (censusAssembly, error) {
			return runInitializedCensusAcquisition(ctx, runtime, discoverer, limits, downDepth, upDepth, maxBatchTargets)
		},
		acquireDiagnostic: func(ctx context.Context, runtime *initializedAcquisitionRuntime, discoverer censusacquisition.Discoverer, limits acquisitionops.Limits, downDepth, upDepth, maxBatchTargets int, diagnostic *censusdiagnostic.Recorder, observer *censusControlFlowObserver) (censusAssembly, error) {
			return runInitializedCensusAcquisition(ctx, runtime, discoverer, limits, downDepth, upDepth, maxBatchTargets, diagnostic, observer)
		},
		capability: func(assembly censusAssembly) (censusPublicationCapability, error) {
			return assembly.publicationCapability()
		},
		publish:             publishCensusCaptureSet,
		publishContinuation: publishCensusCaptureSetWithContinuation,
		publishEnumeration:  publishCensusEnumerationProjection,
	}
}

func censusOutcomeTyped(outcome censusPublicationOutcome) bool {
	return outcome.Diagnostic != nil || outcome.Result != nil || outcome.DiscoveryDiagnostic != nil && outcome.RequestReceipt != nil
}

// runCensusCore coordinates one already-preflighted census through the existing
// initialized-session lifecycle. It performs no parsing and writes no streams.
func runCensusCore(options censusCLIOptions, cfg censusCoreConfig, deps censusCoreDependencies) censusPublicationOutcome {
	outcome := censusPublicationOutcome{}
	observer := newCensusControlFlowObserver()
	callbackRan := false
	code := deps.runSession(cfg.runner, func(ctx context.Context, runtime *initializedAcquisitionRuntime) int {
		callbackRan = true
		fail := func(stage censusFailureStage, batch *int) int {
			observer.cause(censuscontrolflow.CoreFailed, censusControlFlowPhase(stage))
			diagnostic, _ := buildCensusCLIDiagnostic(stage, batch)
			outcome = censusPublicationOutcome{Diagnostic: &diagnostic}
			return 1
		}
		if err := ctx.Err(); err != nil || runtime == nil || runtime.SessionID() == "" || runtime.Generation() == 0 {
			observer.cause(censuscontrolflow.CallbackRuntimeInvalid, censuscontrolflow.PhaseInput)
			return fail(censusStageAcquisition, nil)
		}
		discovery, err := deps.discover(ctx, options, runtime, cfg.callHierarchy)
		if err != nil || discovery.Session.SessionID != runtime.SessionID() || discovery.Session.Generation != runtime.Generation() {
			return fail(censusStageDiscovery, nil)
		}
		if !discovery.Complete {
			receipt, receiptErr := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: runtime.SessionID(), Generation: runtime.Generation()}, censusrequest.SemanticFields{Sources: options.Sources, Includes: options.Includes, Excludes: options.Excludes, DownDepth: uint64(options.DownDepth), UpDepth: uint64(options.UpDepth), MaxNodes: uint64(options.MaxNodes), BatchTargets: uint64(options.MaxBatchTargets), TimeoutMS: uint64(options.Timeout.Milliseconds()), RequestTimeoutMS: uint64(options.RequestTimeout.Milliseconds()), StopAfter: options.StopAfter})
			if receiptErr != nil {
				return fail(censusStageDiscovery, nil)
			}
			diagnostic, diagnosticErr := censusresult.NewDiscoveryDiagnosticFromAcquisition(discovery.Observations, receipt.Semantic)
			if diagnosticErr != nil {
				return fail(censusStageDiscovery, nil)
			}
			outcome = censusPublicationOutcome{DiscoveryDiagnostic: &diagnostic, RequestReceipt: &receipt}
			if options.PrivateEnumerationProjection {
				privateReceipt := censusenumerationprojection.Unavailable("CANCELED_PRE_RECONCILIATION")
				if ctx.Err() == nil {
					publisher := deps.publishEnumeration
					if publisher == nil {
						publisher = publishCensusEnumerationProjection
					}
					privateReceipt = publisher(ctx, discovery, receipt, options.PublicationRoot)
				}
				outcome.EnumerationReceipt = &privateReceipt
			}
			return 1
		}
		if len(discovery.Targets) == 0 {
			return fail(censusStageDiscovery, nil)
		}
		var assembly censusAssembly
		if deps.acquireDiagnostic != nil {
			assembly, err = deps.acquireDiagnostic(ctx, runtime, fixedCensusDiscovery{discovery: discovery}, cfg.limits, options.DownDepth, options.UpDepth, options.MaxBatchTargets, cfg.diagnostic, observer)
		} else {
			assembly, err = deps.acquire(ctx, runtime, fixedCensusDiscovery{discovery: discovery}, cfg.limits, options.DownDepth, options.UpDepth, options.MaxBatchTargets)
		}
		if err != nil {
			return fail(censusStageAcquisition, censusBatchOrdinal(err))
		}
		if err := ctx.Err(); err != nil || runtime.SessionID() != discovery.Session.SessionID || runtime.Generation() != discovery.Session.Generation {
			observer.cause(censuscontrolflow.PostCoreRuntimeInvalid, censuscontrolflow.PhaseCompleted)
			return fail(censusStageAcquisition, nil)
		}
		capability, err := deps.capability(assembly)
		if err != nil {
			return fail(censusStageAssembly, nil)
		}
		if err := ctx.Err(); err != nil {
			observer.cause(censuscontrolflow.PostCoreRuntimeInvalid, censuscontrolflow.PhaseCompleted)
			return fail(censusStageAcquisition, nil)
		}
		if deps.publishContinuation != nil {
			metadata, metadataFailure := runtime.Metadata(runtime.SessionID(), runtime.Generation())
			if metadataFailure != "" || metadata.PositionEncoding == "" {
				return fail(censusStageCommitted, nil)
			}
			outcome = deps.publishContinuation(ctx, capability, options.PublicationRoot, programccompose.ExactMetadata{
				RevisionCustody: "CALLER_ASSERTED", PositionEncoding: metadata.PositionEncoding,
				AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1",
			})
		} else {
			outcome = deps.publish(ctx, capability, options.PublicationRoot)
		}
		if outcome.Result == nil {
			if outcome.Diagnostic == nil {
				return fail(censusStagePublication, nil)
			}
			return 1
		}
		if err := validateCensusCLIResult(*outcome.Result); err != nil {
			return fail(censusStageCommitted, nil)
		}
		if cfg.afterCommit != nil {
			return cfg.afterCommit(ctx, runtime, outcome)
		}
		return 0
	})
	if !callbackRan {
		observer.cause(censuscontrolflow.CallbackNotEntered, censuscontrolflow.PhaseNotEntered)
	} else if code != 0 && !censusOutcomeTyped(outcome) {
		observer.cause(censuscontrolflow.FinalNonzeroFallback, censuscontrolflow.PhaseCompleted)
	}
	if !callbackRan || code != 0 && !censusOutcomeTyped(outcome) {
		diagnostic, _ := buildCensusCLIDiagnostic(censusStageAcquisition, nil)
		outcome = censusPublicationOutcome{Diagnostic: &diagnostic}
	}
	if outcome.Diagnostic != nil && outcome.Diagnostic.Stage == censusStageAcquisition {
		if record, ok := observer.record(); ok {
			outcome.controlFlow = &record
		}
	}
	return outcome
}

type censusBatchAcquisitionFailure struct {
	ordinal int
	Err     error
}

func (f *censusBatchAcquisitionFailure) Error() string {
	if f == nil || f.Err == nil {
		return "census batch acquisition failed"
	}
	return f.Err.Error()
}

func (f *censusBatchAcquisitionFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

func (f *censusBatchAcquisitionFailure) BatchOrdinal() int {
	if f == nil {
		return 0
	}
	return f.ordinal
}

// censusBatchOrdinal intentionally recognizes only the private typed batch
// failure boundary; raw error text is never parsed or projected.
func censusBatchOrdinal(err error) *int {
	var failure *censusBatchAcquisitionFailure
	if !errors.As(err, &failure) || failure.BatchOrdinal() < 1 {
		return nil
	}
	ordinal := failure.BatchOrdinal()
	return &ordinal
}
