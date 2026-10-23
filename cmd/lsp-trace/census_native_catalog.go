package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
)

const censusNativeSourcePartitionSize = 16

var (
	errNativeCatalogAggregateUnavailable = errors.New("native census catalog handoff unavailable")
	errNativeCatalogBoundReached         = errors.New("native census catalog assembly bound reached")
	errNativeCatalogDeadlineReached      = errors.New("native census catalog assembly deadline reached")
	errNativeCatalogConflict             = errors.New("native census catalog assembly conflict")
	errNativeCatalogRetained             = errors.New("native census catalog assembly retained")
	errNativeCatalogIndeterminate        = errors.New("native census catalog assembly indeterminate execution")
	errNativeCatalogAssemblyFailed       = errors.New("native census catalog assembly failed")
)

type censusNativeCatalogDependencies struct {
	enumerate         func(censusCLIOptions) ([]censusacquisition.SourceFile, error)
	buildPlan         func([]censusacquisition.SourceFile, string, int) (censussourceplan.Plan, error)
	validatePlan      func(censussourceplan.Plan, int) error
	runSession        func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int
	openRoots         func(string) (*publication.Root, *publication.Root, func() error, error)
	discoverPartition func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, []censusacquisition.SourceFile) (censusacquisition.Discovery, error)
	runAssembly       func(context.Context, censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord)
}

func productionCensusNativeCatalogDependencies() censusNativeCatalogDependencies {
	return censusNativeCatalogDependencies{
		enumerate:         enumerateCensusPreflight,
		buildPlan:         censussourceplan.Build,
		validatePlan:      censussourceplan.Validate,
		runSession:        productionCensusCoreDependencies().runSession,
		openRoots:         openCensusNativeCatalogRoots,
		discoverPartition: discoverInitializedCensusPartition,
		runAssembly:       censussourceassembly.Run,
	}
}

// runCensusNativeCatalog is the fresh --catalog command-owned assembly seam.
// Resume and noncatalog routing do not call it.
func runCensusNativeCatalog(options censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer) int {
	return runCensusNativeCatalogWithDependencies(options, cfg, stdout, stderr, productionCensusNativeCatalogDependencies())
}

func runCensusNativeCatalogWithDependencies(options censusCLIOptions, cfg censusCoreConfig, _ io.Writer, stderr io.Writer, deps censusNativeCatalogDependencies) int {
	fail := func(stage censusFailureStage, err error) int {
		cfg.controlFlow.cause(censuscontrolflow.CoreFailed, censusControlFlowPhase(stage))
		return writeCensusFailure(stderr, stage, options.Machine, err)
	}
	if deps.enumerate == nil || deps.buildPlan == nil || deps.validatePlan == nil || deps.runSession == nil || deps.openRoots == nil || deps.discoverPartition == nil || deps.runAssembly == nil {
		return fail(censusStageConfig, errors.New("native census catalog unavailable"))
	}
	files, err := deps.enumerate(options)
	if err != nil {
		return fail(censusStageDiscovery, err)
	}
	plan, err := deps.buildPlan(files, options.Workspace, censusNativeSourcePartitionSize)
	if err != nil {
		return fail(censusStageDiscovery, err)
	}
	if err := deps.validatePlan(plan, censusNativeSourcePartitionSize); err != nil {
		return fail(censusStageDiscovery, err)
	}
	partitionCount := len(plan.Partitions)
	if partitionCount <= 0 || partitionCount > censusMaxDiscoveredEntries {
		return fail(censusStageConfig, errors.New("invalid native census catalog bound"))
	}
	workflowKey, err := censusNativeWorkflowKey(options, plan.Identity)
	if err != nil {
		return fail(censusStageConfig, err)
	}

	var assemblyResult censussourcedriver.Result
	callbackRan := false
	sessionCode := deps.runSession(cfg.runner, func(ctx context.Context, runtime *initializedAcquisitionRuntime) int {
		callbackRan = true
		if ctx == nil || runtime == nil || runtime.SessionID() == "" || runtime.Generation() == 0 {
			return 1
		}
		objectRoot, headRoot, closeRoots, openErr := deps.openRoots(options.PublicationRoot)
		if openErr != nil {
			assemblyResult = censussourcedriver.Result{Stop: censussourcedriver.StopFailed, Err: openErr}
			return 1
		}
		assemblyResult, _ = deps.runAssembly(ctx, censussourceassembly.Request{
			Plan: plan, PartitionSize: censusNativeSourcePartitionSize,
			WorkflowKey: workflowKey, HeadKey: workflowKey,
			ObjectRoot: objectRoot, HeadRoot: headRoot,
			StepBound: partitionCount,
			Identity:  censussourceassembly.Identity{SessionID: runtime.SessionID(), ManagerGeneration: runtime.Generation()},
			Discover: func(discoverCtx context.Context, ordinal int, members []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
				discovery, discoverErr := deps.discoverPartition(discoverCtx, options, runtime, members)
				if discoverErr != nil && cfg.diagnostic != nil {
					raw, _ := json.Marshal(members)
					record := censusdiagnostic.Record{Fingerprint: rawDigest(raw), Category: "ADMISSION", Ordinal: &ordinal}
					if recordErr := cfg.diagnostic.Record(record); recordErr != nil {
						cfg.controlFlow.batch(censuscontrolflow.BatchRejected)
					} else {
						cfg.controlFlow.batch(censuscontrolflow.BatchAccepted)
					}
				}
				return discovery, discoverErr
			},
		})
		if closeErr := closeRoots(); closeErr != nil && assemblyResult.Stop == censussourcedriver.StopComplete {
			assemblyResult = censussourcedriver.Result{Stop: censussourcedriver.StopFailed, Err: closeErr}
			return 1
		}
		return 0
	})
	if !callbackRan || assemblyResult.Stop == "" {
		return fail(censusStageAcquisition, errors.New("native census catalog execution failed"))
	}
	switch assemblyResult.Stop {
	case censussourcedriver.StopComplete:
		if sessionCode != 0 {
			return fail(censusStageAcquisition, errors.New("native census catalog execution failed"))
		}
		return fail(censusStageCommitted, censusNativeCatalogTerminalError(assemblyResult.Stop, assemblyResult.Err))
	case censussourcedriver.StopDeadline:
		return fail(censusStageAcquisition, censusNativeCatalogTerminalError(assemblyResult.Stop, assemblyResult.Err))
	case censussourcedriver.StopBoundReached, censussourcedriver.StopConflict, censussourcedriver.StopRetained, censussourcedriver.StopIndeterminateExecution, censussourcedriver.StopFailed:
		return fail(censusStageAssembly, censusNativeCatalogTerminalError(assemblyResult.Stop, assemblyResult.Err))
	default:
		return fail(censusStageAssembly, errors.New("native census catalog execution failed"))
	}
}

func censusNativeCatalogTerminalError(stop censussourcedriver.Stop, err error) error {
	if stop == censussourcedriver.StopComplete {
		return errNativeCatalogAggregateUnavailable
	}
	if err != nil {
		return err
	}
	switch stop {
	case censussourcedriver.StopBoundReached:
		return errNativeCatalogBoundReached
	case censussourcedriver.StopDeadline:
		return errNativeCatalogDeadlineReached
	case censussourcedriver.StopConflict:
		return errNativeCatalogConflict
	case censussourcedriver.StopRetained:
		return errNativeCatalogRetained
	case censussourcedriver.StopIndeterminateExecution:
		return errNativeCatalogIndeterminate
	case censussourcedriver.StopFailed:
		return errNativeCatalogAssemblyFailed
	default:
		return errors.New("native census catalog execution failed")
	}
}

func discoverInitializedCensusPartition(ctx context.Context, options censusCLIOptions, runtime *initializedAcquisitionRuntime, members []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
	if ctx == nil || runtime == nil || runtime.privateAcquisitionRuntime == nil || runtime.SessionID() == "" || runtime.Generation() == 0 {
		return censusacquisition.Discovery{}, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = runtime.requestTimeout
	}
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}
	client := censusProductionDiscoveryClient{productionDiscoveryClient: productionDiscoveryClient{runtime: runtime.privateAcquisitionRuntime, sessionID: runtime.SessionID(), generation: runtime.Generation(), timeout: requestTimeout}, callHierarchy: true}
	return (censusacquisition.DiscoveryAdapter{
		Workspace: options.Workspace,
		Files:     censusacquisition.StaticFiles(append([]censusacquisition.SourceFile(nil), members...)),
		Supplier:  censusRuntimeDocumentSupplier{runtime: runtime},
		Client:    client,
	}).Discover(ctx, censusacquisition.SessionIdentity{SessionID: runtime.SessionID(), Generation: runtime.Generation()})
}

func openCensusNativeCatalogRoots(publicationRoot string) (*publication.Root, *publication.Root, func() error, error) {
	paths := []string{
		filepath.Join(publicationRoot, "census-source-objects-v1"),
		filepath.Join(publicationRoot, "census-source-heads-v1"),
	}
	roots := make([]*publication.Root, 0, len(paths))
	closeRoots := func() error {
		return closeCensusNativeCatalogRoots(roots, func(root *publication.Root) error { return root.Close() })
	}
	for _, path := range paths {
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			_ = closeRoots()
			return nil, nil, func() error { return nil }, err
		}
		root, err := publication.OpenRoot(path)
		if err != nil {
			_ = closeRoots()
			return nil, nil, func() error { return nil }, err
		}
		roots = append(roots, root)
	}
	return roots[0], roots[1], closeRoots, nil
}

func closeCensusNativeCatalogRoots(roots []*publication.Root, closeRoot func(*publication.Root) error) error {
	var closeErrors []error
	for _, root := range roots {
		if err := closeRoot(root); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func censusNativeWorkflowKey(options censusCLIOptions, planIdentity string) (string, error) {
	sorted := func(values []string) []string {
		copy := append([]string(nil), values...)
		sort.Strings(copy)
		return copy
	}
	canonical, err := json.Marshal(struct {
		Version           string   `json:"version"`
		PlanIdentity      string   `json:"plan_identity"`
		Workspace         string   `json:"workspace"`
		Sources           []string `json:"sources"`
		Includes          []string `json:"includes,omitempty"`
		Excludes          []string `json:"excludes,omitempty"`
		PrunedDirectories []string `json:"pruned_directories,omitempty"`
		ServerArgs        []string `json:"server_args,omitempty"`
	}{
		Version: "lsp-trace:cli-census-native-catalog:v1", PlanIdentity: planIdentity,
		Workspace: options.Workspace, Sources: sorted(options.Sources),
		Includes: sorted(options.Includes), Excludes: sorted(options.Excludes),
		PrunedDirectories: sorted(options.PrunedDirectories), ServerArgs: append([]string(nil), options.ServerArgs...),
	})
	if err != nil || planIdentity == "" {
		return "", errors.New("native census catalog workflow identity unavailable")
	}
	sum := sha256.Sum256(canonical)
	return "cli-census-native-v1:" + hex.EncodeToString(sum[:]), nil
}
