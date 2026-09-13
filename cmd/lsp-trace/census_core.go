package main

import (
	"context"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
)

// censusCoreDependencies is a package-main-only test seam. Production callers
// use productionCensusCoreDependencies; none of these functions confer census
// assembly or publication authority outside package main.
type censusCoreDependencies struct {
	runSession func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int
	discover   func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error)
	acquire    func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits) (censusAssembly, error)
	capability func(censusAssembly) (censusPublicationCapability, error)
	publish    func(context.Context, censusPublicationCapability, string) censusPublicationOutcome
}

type censusCoreConfig struct {
	runner        initializedAcquisitionRunnerConfig
	limits        acquisitionops.Limits
	callHierarchy bool
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
		acquire:  runInitializedCensusAcquisition,
		capability: func(assembly censusAssembly) (censusPublicationCapability, error) {
			return assembly.publicationCapability()
		},
		publish: publishCensusCaptureSet,
	}
}

// runCensusCore coordinates one already-preflighted census through the existing
// initialized-session lifecycle. It performs no parsing and writes no streams.
func runCensusCore(options censusCLIOptions, cfg censusCoreConfig, deps censusCoreDependencies) censusPublicationOutcome {
	outcome := censusPublicationOutcome{}
	callbackRan := false
	code := deps.runSession(cfg.runner, func(ctx context.Context, runtime *initializedAcquisitionRuntime) int {
		callbackRan = true
		fail := func(stage censusFailureStage, batch *int) int {
			diagnostic, _ := buildCensusCLIDiagnostic(stage, batch)
			outcome = censusPublicationOutcome{Diagnostic: &diagnostic}
			return 1
		}
		if err := ctx.Err(); err != nil || runtime == nil || runtime.SessionID() == "" || runtime.Generation() == 0 {
			return fail(censusStageAcquisition, nil)
		}
		discovery, err := deps.discover(ctx, options, runtime, cfg.callHierarchy)
		if err != nil || !discovery.Complete || len(discovery.Targets) == 0 || discovery.Session.SessionID != runtime.SessionID() || discovery.Session.Generation != runtime.Generation() {
			return fail(censusStageDiscovery, nil)
		}
		assembly, err := deps.acquire(ctx, runtime, fixedCensusDiscovery{discovery: discovery}, cfg.limits)
		if err != nil {
			return fail(censusStageAcquisition, censusBatchOrdinal(err))
		}
		if err := ctx.Err(); err != nil || runtime.SessionID() != discovery.Session.SessionID || runtime.Generation() != discovery.Session.Generation {
			return fail(censusStageAcquisition, nil)
		}
		capability, err := deps.capability(assembly)
		if err != nil {
			return fail(censusStageAssembly, nil)
		}
		outcome = deps.publish(ctx, capability, options.PublicationRoot)
		if outcome.Result == nil {
			if outcome.Diagnostic == nil {
				return fail(censusStagePublication, nil)
			}
			return 1
		}
		if err := validateCensusCLIResult(*outcome.Result); err != nil {
			return fail(censusStageCommitted, nil)
		}
		return 0
	})
	if !callbackRan || code != 0 && outcome.Diagnostic == nil {
		diagnostic, _ := buildCensusCLIDiagnostic(censusStageAcquisition, nil)
		outcome = censusPublicationOutcome{Diagnostic: &diagnostic}
	}
	return outcome
}

// censusBatchOrdinal intentionally recognizes only the private typed batch
// failure boundary; raw error text is never parsed or projected.
func censusBatchOrdinal(error) *int { return nil }
