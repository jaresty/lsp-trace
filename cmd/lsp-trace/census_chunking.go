package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const privateCensusChunkTerminalVersion = "lsp-trace.census-chunk-terminal.cli.private.v1"

type privateCensusChunkTerminal struct {
	SchemaVersion       string `json:"schema_version"`
	TerminalSelector    string `json:"terminal_selector"`
	Authority           int    `json:"authority"`
	Accepted            bool   `json:"accepted"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CrossChunkCalls     string `json:"cross_chunk_calls"`
	AggregateCapture    string `json:"aggregate_capture"`
}

type privateCensusChunkDependencies struct {
	coordinate func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error)
}

func productionPrivateCensusChunkDependencies() privateCensusChunkDependencies {
	return privateCensusChunkDependencies{coordinate: censuschunking.Coordinate}
}

func marshalPrivateCensusChunkTerminal(v privateCensusChunkTerminal) ([]byte, error) {
	if v.SchemaVersion != privateCensusChunkTerminalVersion || v.TerminalSelector == "" || v.Authority != 0 || v.Accepted || v.SourceGraphComplete != "UNKNOWN" || v.CrossChunkCalls != "NOT_COMPUTED" || v.AggregateCapture != "NOT_MATERIALIZED" {
		return nil, errors.New("invalid private census chunk terminal")
	}
	return json.Marshal(v)
}

// runPrivateCensusChunking is the sole private CLI routing seam. Compatibility
// returns handled=false without acquiring a lease or opening publication state.
func runPrivateCensusChunking(ctx context.Context, options censusCLIOptions, cfg censusCoreConfig, runtime *initializedAcquisitionRuntime, discovery censusacquisition.Discovery, observer *censusControlFlowObserver, dependencies privateCensusChunkDependencies) (handled bool, terminal *privateCensusChunkTerminal, err error) {
	if !options.PrivateEnumerationProjection {
		return false, nil, nil
	}
	compatibility, err := censuschunking.CompatibilityForTargetCount(len(discovery.Targets))
	if err != nil {
		return true, nil, err
	}
	if compatibility {
		return false, nil, nil
	}
	request, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation}, censusrequest.SemanticFields{Sources: options.Sources, Includes: options.Includes, Excludes: options.Excludes, DownDepth: uint64(options.DownDepth), UpDepth: uint64(options.UpDepth), MaxNodes: uint64(options.MaxNodes), BatchTargets: uint64(options.MaxBatchTargets), TimeoutMS: uint64(options.Timeout.Milliseconds()), RequestTimeoutMS: uint64(options.RequestTimeout.Milliseconds()), StopAfter: options.StopAfter})
	if err != nil {
		return true, nil, err
	}
	input, projection, err := privateCensusChunkInput(discovery, request)
	if err != nil {
		return true, nil, err
	}
	projectionBytes, err := censusenumerationprojection.EncodeCanonical(projection)
	if err != nil {
		return true, nil, err
	}
	built, err := censuschunking.Build(input, projectionBytes)
	if err != nil {
		return true, nil, err
	}
	if built.Compatibility {
		return true, nil, errors.New("private census chunk compatibility changed after translation")
	}
	if runtime == nil || runtime.manager == nil || dependencies.coordinate == nil {
		return true, nil, errors.New("private census chunk runtime unavailable")
	}
	workspace, ok := runtime.WorkspaceRoot(discovery.Session.SessionID, discovery.Session.Generation)
	if !ok || workspace != discovery.Workspace {
		return true, nil, errors.New("private census chunk workspace changed before lease")
	}
	lease := sessionruntime.SessionLeaseRequest{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, Workspace: workspace}
	var result censuschunking.CoordinatorResult
	err = runtime.manager.WithReadySessionLease(ctx, lease, func(leaseCtx context.Context) error {
		root, openErr := publication.OpenRoot(options.PublicationRoot)
		if openErr != nil {
			return openErr
		}
		defer root.Close()
		projectionReceipt, publishErr := censusenumerationprojection.Publish(root, projection)
		if publishErr != nil {
			return publishErr
		}
		input.GlobalProjection = censuschunking.ArtifactRef{Selector: projectionReceipt.Selector, SHA256: projectionReceipt.Digest, ByteLength: projectionReceipt.ByteLength}
		built, buildErr := censuschunking.Build(input, projectionBytes)
		if buildErr != nil || built.Compatibility {
			if buildErr != nil {
				return buildErr
			}
			return errors.New("private census chunk plan changed under lease")
		}
		store, storeErr := newPrivateCensusCoordinatorStore(root)
		if storeErr != nil {
			return storeErr
		}
		heads, headsErr := newPrivateCensusHeadStore(root)
		if headsErr != nil {
			return headsErr
		}
		acquirer := newInitializedCensusBatchAcquirer(runtime, cfg.limits, cfg.diagnostic, observer)
		core := censusacquisition.Core{Discoverer: fixedCensusDiscovery{discovery: discovery}, Acquirer: acquirer, Planning: &censusacquisition.PlanningConfig{DownDepth: options.DownDepth, UpDepth: options.UpDepth, MaxBatchTargets: options.MaxBatchTargets}}
		core.OnErrorPhase = func(phase censusacquisition.Phase) {
			observer.cause("CORE_FAILED", censusControlFlowPhase(censusStageAcquisition))
		}
		held, heldErr := newPrivateHeldCensusSession(censuschunking.ExpectedSession{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, WorkspaceURI: input.WorkspaceURI}, core)
		if heldErr != nil {
			return heldErr
		}
		projections := make([]censuschunking.Projection, len(built.Chunks))
		sidecars := make([]censuschunking.Sidecar, len(built.Chunks))
		for i := range built.Chunks {
			projections[i], sidecars[i] = built.Chunks[i].Projection, built.Chunks[i].Sidecar
		}
		result, err = dependencies.coordinate(leaseCtx, censuschunking.CoordinatorRequest{Input: input, Plan: built.Plan, Index: built.Index, Projections: projections, Sidecars: sidecars, Store: store, Heads: heads, Sessions: held})
		return err
	})
	if err != nil {
		return true, nil, err
	}
	root, err := publication.OpenRoot(options.PublicationRoot)
	if err != nil {
		return true, nil, err
	}
	raw, err := root.ReadSelector(result.TerminalSelector, censuschunking.MaxCanonicalBytes)
	closeErr := root.Close()
	if err != nil || closeErr != nil {
		return true, nil, errors.Join(err, closeErr)
	}
	index, err := censuschunking.ParseTerminalIndex(raw)
	if err != nil {
		return true, nil, err
	}
	return true, &privateCensusChunkTerminal{SchemaVersion: privateCensusChunkTerminalVersion, TerminalSelector: result.TerminalSelector, Authority: index.Authority, Accepted: index.Accepted, SourceGraphComplete: index.SourceGraphComplete, CrossChunkCalls: index.CrossChunkCalls, AggregateCapture: index.AggregateCapture}, nil
}

func privateCensusChunkInput(discovery censusacquisition.Discovery, request censusrequest.Receipt) (censuschunking.Input, censusenumerationprojection.Projection, error) {
	workspaceURI := (&url.URL{Scheme: "file", Path: discovery.Workspace}).String()
	files := make([]censusenumerationprojection.File, len(discovery.FileLedger.Entries))
	globalFiles := make([]censuschunking.GlobalFile, len(files))
	for i, entry := range discovery.FileLedger.Entries {
		files[i] = censusenumerationprojection.File{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
		globalFiles[i] = censuschunking.GlobalFile{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
	}
	if len(discovery.EnumerationSymbols) != len(discovery.Targets) {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk target enumeration mismatch")
	}
	symbols := make([]censusenumerationprojection.Symbol, len(discovery.EnumerationSymbols))
	targets := make([]censuschunking.GlobalTarget, len(symbols))
	for i, symbol := range discovery.EnumerationSymbols {
		if symbol.Ordinal != i || symbol.FileOrdinal < 0 || symbol.FileOrdinal >= len(files) {
			return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk ordinal mismatch")
		}
		target := discovery.Targets[i]
		position := censusenumerationprojection.Position{Line: symbol.SelectionStart.Line, Character: symbol.SelectionStart.Character}
		identity, _, digestErr := censusenumerationprojection.SymbolIdentityDigest(files[symbol.FileOrdinal].Path, position, symbol.Kind, symbol.Name, symbol.Ordinal)
		if digestErr != nil || target.CensusOrdinal != i || target.Name != symbol.Name || target.Kind != symbol.Kind || target.Position() != symbol.SelectionStart {
			return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk target identity mismatch")
		}
		symbols[i] = censusenumerationprojection.Symbol{Ordinal: symbol.Ordinal, FileOrdinal: symbol.FileOrdinal, SelectionStart: position, Kind: symbol.Kind, Disposition: string(symbol.Disposition), IdentityDigest: identity}
		targets[i] = censuschunking.GlobalTarget{GlobalOrdinal: i, GlobalFileOrdinal: symbol.FileOrdinal, GlobalIdentity: identity, CanonicalSeedV2: string(target.CanonicalSeedV2), CanonicalSeedV2SHA256: privateCensusObjectSelector(target.CanonicalSeedV2), Coordinate: censuschunking.SourceCoordinate{Path: files[symbol.FileOrdinal].Path, Line: position.Line, Character: position.Character}, Kind: symbol.Kind, Name: symbol.Name, URI: target.URI, LanguageID: target.LanguageID, Disposition: "prepared"}
	}
	projection, err := censusenumerationprojection.Build(censusenumerationprojection.Input{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, RequestFingerprint: request.Fingerprint, WorkspaceURI: workspaceURI, Files: files, Symbols: symbols, Observations: []censusenumerationprojection.Observation{}, ProviderFailures: []censusenumerationprojection.ProviderFailure{}, IncompleteCauses: []censusenumerationprojection.IncompleteCause{}, RequestDownDepth: request.Semantic.DownDepth, RequestUpDepth: request.Semantic.UpDepth, RequestMaxNodes: request.Semantic.MaxNodes, RequestBatchTargets: request.Semantic.BatchTargets, RequestTimeoutMS: request.Semantic.RequestTimeoutMS, TimeoutMS: request.Semantic.TimeoutMS})
	if err != nil {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, err
	}
	workspaceDigest, err := censusenumerationprojection.WorkspaceDigest(workspaceURI)
	if err != nil {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, err
	}
	input := censuschunking.Input{GlobalProjection: censuschunking.ArtifactRef{Selector: "pending", SHA256: privateCensusObjectSelector([]byte("pending")), ByteLength: 1}, RequestFingerprint: request.Fingerprint, SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, WorkspaceURI: workspaceURI, WorkspaceDigest: workspaceDigest, CanonicalOrderPolicy: "captureset-order-targets-v1", DuplicatePolicy: "reject-global-ordinal-identity-seed-bytes-digest-v1", BatchTargets: int(request.Semantic.BatchTargets), DownDepth: int(request.Semantic.DownDepth), UpDepth: int(request.Semantic.UpDepth), Files: globalFiles, Targets: targets, GlobalProviderFailures: []string{}}
	return input, projection, nil
}
