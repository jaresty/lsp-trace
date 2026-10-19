package censuschunking

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
)

type ExecutionStage string

const (
	StagePrecondition ExecutionStage = "PRECONDITION"
	StageCore         ExecutionStage = "CORE"
	StageProjection   ExecutionStage = "PROJECTION"
	StageMapping      ExecutionStage = "MAPPING"
)

type ExecutionError struct {
	Stage ExecutionStage
	Err   error
}

func (e *ExecutionError) Error() string { return fmt.Sprintf("chunk execution %s: %v", e.Stage, e.Err) }
func (e *ExecutionError) Unwrap() error { return e.Err }

func IsExecutionStage(err error, stage ExecutionStage) bool {
	var executionErr *ExecutionError
	return errors.As(err, &executionErr) && executionErr.Stage == stage
}

// ExecutionRequest is a private, transport-neutral request to execute exactly
// one member of an already materialized chunk aggregate. It is not accepted
// evidence and is not used by production adapters.
type ExecutionRequest struct {
	Input        Input
	Plan         Plan
	Index        Index
	Projections  []Projection
	Sidecars     []Sidecar
	ChunkOrdinal int
	Core         censusacquisition.Core
}

// ChunkExecution is a private reconciliation receipt. Projection and manifest
// are the unchanged censusacquisition outputs; this does not represent a global
// census result or grant completion, acceptance, or publication authority.
type ChunkExecution struct {
	Descriptor    ChunkDescriptor
	PlanDigest    string
	IndexDigest   string
	Projection    censusacquisition.Projection
	Manifest      captureset.Manifest
	ManifestBytes []byte
	Authority     int
	Accepted      bool
	Completeness  string
}

func executionError(stage ExecutionStage, err error) error {
	return &ExecutionError{Stage: stage, Err: err}
}

// ExecuteChunk validates all supplied aggregate bindings before delegating one
// selected local projection to the unchanged censusacquisition Core.Run.
func ExecuteChunk(ctx context.Context, request ExecutionRequest) (ChunkExecution, error) {
	if err := Reconcile(request.Input, request.Plan, request.Index, request.Projections, request.Sidecars); err != nil {
		return ChunkExecution{}, executionError(StagePrecondition, err)
	}
	if request.ChunkOrdinal < 0 || request.ChunkOrdinal >= len(request.Plan.Chunks) {
		return ChunkExecution{}, executionError(StagePrecondition, errors.New("chunk ordinal outside plan"))
	}
	projection := request.Projections[request.ChunkOrdinal]
	sidecar := request.Sidecars[request.ChunkOrdinal]
	descriptor := request.Plan.Chunks[request.ChunkOrdinal]
	if descriptor.TargetCount < 1 || descriptor.TargetCount > captureset.MaxTargets || len(projection.Targets) != descriptor.TargetCount || len(sidecar.Mappings) != descriptor.TargetCount {
		return ChunkExecution{}, executionError(StagePrecondition, errors.New("chunk target cardinality outside execution bound"))
	}
	if request.Plan.BatchTargets < 1 || request.Plan.BatchTargets > captureset.MaxBatchTargets {
		return ChunkExecution{}, executionError(StagePrecondition, errors.New("chunk batch target bound exceeded"))
	}
	discovery, err := chunkDiscovery(request, projection, sidecar)
	if err != nil {
		return ChunkExecution{}, executionError(StagePrecondition, err)
	}
	core := request.Core
	core.Discoverer = staticChunkDiscoverer{discovery: discovery}
	core.Planning = &censusacquisition.PlanningConfig{DownDepth: request.Plan.DownDepth, UpDepth: request.Plan.UpDepth, MaxBatchTargets: request.Plan.BatchTargets}
	result, err := core.Run(ctx, censusacquisition.SessionIdentity{SessionID: request.Plan.SessionID, Generation: request.Plan.Generation})
	if err != nil {
		return ChunkExecution{}, executionError(StageCore, err)
	}
	return reconcileExecutionProjection(request, result)
}

type staticChunkDiscoverer struct{ discovery censusacquisition.Discovery }

func (d staticChunkDiscoverer) Discover(_ context.Context, session censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	if d.discovery.Session != session {
		return censusacquisition.Discovery{}, errors.New("chunk discovery session binding mismatch")
	}
	return d.discovery, nil
}

func chunkDiscovery(request ExecutionRequest, projection Projection, sidecar Sidecar) (censusacquisition.Discovery, error) {
	workspaceURI, err := url.Parse(request.Input.WorkspaceURI)
	if err != nil || workspaceURI.Scheme != "file" || workspaceURI.Host != "" {
		return censusacquisition.Discovery{}, errors.New("chunk workspace URI binding invalid")
	}
	workspace := filepath.FromSlash(workspaceURI.Path)
	globalByOrdinal := make(map[int]GlobalTarget, len(request.Input.Targets))
	for _, target := range request.Input.Targets {
		globalByOrdinal[target.GlobalOrdinal] = target
	}
	d := censusacquisition.Discovery{
		Session:          censusacquisition.SessionIdentity{SessionID: request.Plan.SessionID, Generation: request.Plan.Generation},
		Workspace:        workspace,
		Complete:         true,
		ProviderFailures: []censusacquisition.ProviderFailure{},
		IncompleteCauses: []censusacquisition.IncompleteCause{},
		Observations:     []censusacquisition.DiscoveryObservation{},
	}
	d.Accounting.FileDenominator = len(projection.Files)
	d.FileLedger.Denominator = len(projection.Files)
	d.Accounting.Files = make([]census.FileEntry, len(projection.Files))
	d.FileLedger.Entries = make([]captureset.LedgerEntry, len(projection.Files))
	for i, file := range projection.Files {
		if file.Ordinal != i {
			return censusacquisition.Discovery{}, errors.New("local file ledger ordinal mismatch")
		}
		d.Accounting.Files[i] = census.FileEntry{Ordinal: i, Disposition: census.FileSelected}
		d.FileLedger.Entries[i] = captureset.LedgerEntry{Ordinal: i, Identity: file.Path, Disposition: file.Disposition}
	}
	d.Accounting.SymbolDenominator = len(projection.Symbols)
	d.SymbolLedger.Denominator = len(projection.Symbols)
	d.Accounting.Symbols = make([]census.SymbolEntry, len(projection.Symbols))
	d.SymbolLedger.Entries = make([]captureset.LedgerEntry, len(projection.Symbols))
	d.EnumerationSymbols = make([]censusacquisition.EnumerationSymbol, len(projection.Symbols))
	d.Targets = make([]censusacquisition.PreparedTarget, len(projection.Targets))
	for i, symbol := range projection.Symbols {
		if symbol.Ordinal != i || projection.Targets[i].CensusOrdinal != i || sidecar.Mappings[i].LocalOrdinal != i {
			return censusacquisition.Discovery{}, errors.New("local symbol target ledger mismatch")
		}
		global, ok := globalByOrdinal[sidecar.Mappings[i].GlobalOrdinal]
		if !ok || global.GlobalIdentity != sidecar.Mappings[i].GlobalIdentity || global.CanonicalSeedV2SHA256 != sidecar.Mappings[i].GlobalCanonicalSeedDigest || global.GlobalFileOrdinal != sidecar.Mappings[i].GlobalFileOrdinal || global.Coordinate != sidecar.Mappings[i].Coordinate {
			return censusacquisition.Discovery{}, errors.New("sidecar global target binding mismatch")
		}
		position := lsp.Position{Line: symbol.Coordinate.Line, Character: symbol.Coordinate.Character}
		coreIdentity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", symbol.Coordinate.Path, symbol.Coordinate.Line, symbol.Coordinate.Character, symbol.Kind, global.Name, i)
		d.Accounting.Symbols[i] = census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected}
		d.SymbolLedger.Entries[i] = captureset.LedgerEntry{Ordinal: i, Identity: coreIdentity, Disposition: censusacquisition.SymbolPrepared}
		d.EnumerationSymbols[i] = censusacquisition.EnumerationSymbol{Ordinal: i, FileOrdinal: symbol.FileOrdinal, SelectionStart: position, Kind: symbol.Kind, Name: global.Name, Disposition: census.SymbolSelected}
		d.Targets[i] = censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: []byte(projection.Targets[i].CanonicalSeedV2), URI: global.URI, LanguageID: global.LanguageID, SelectionRange: lsp.Range{Start: position, End: position}, Name: global.Name, Kind: symbol.Kind, Range: lsp.Range{Start: position, End: position}, SymbolIdentity: coreIdentity}
	}
	return d, nil
}

func reconcileExecutionProjection(request ExecutionRequest, projection censusacquisition.Projection) (ChunkExecution, error) {
	validated, err := censusacquisition.CloneProjection(projection)
	if err != nil {
		return ChunkExecution{}, executionError(StageProjection, err)
	}
	manifest, err := captureset.Decode(validated.ManifestBytes)
	if err != nil || !reflect.DeepEqual(manifest, validated.Manifest) {
		if err == nil {
			err = errors.New("capture-set manifest object and bytes differ")
		}
		return ChunkExecution{}, executionError(StageProjection, err)
	}
	local := request.Projections[request.ChunkOrdinal]
	sidecar := request.Sidecars[request.ChunkOrdinal]
	flattened := make([]censusacquisition.PreparedTarget, 0, len(local.Targets))
	for _, batch := range validated.Batches {
		flattened = append(flattened, batch.Targets...)
	}
	if len(flattened) != len(local.Targets) || len(manifest.Targets) != len(local.Targets) || len(sidecar.Mappings) != len(local.Targets) {
		return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result target cardinality mismatch"))
	}
	seenGlobal := make(map[int]struct{}, len(local.Targets))
	for i, target := range flattened {
		localTarget := local.Targets[i]
		mapping := sidecar.Mappings[i]
		manifestTarget := manifest.Targets[i]
		localSymbol := local.Symbols[i]
		globalName := ""
		for _, global := range request.Input.Targets {
			if global.GlobalOrdinal == mapping.GlobalOrdinal {
				globalName = global.Name
				break
			}
		}
		coreIdentity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", localSymbol.Coordinate.Path, localSymbol.Coordinate.Line, localSymbol.Coordinate.Character, localSymbol.Kind, globalName, i)
		if target.CensusOrdinal != i || target.SymbolIdentity != coreIdentity || string(target.CanonicalSeedV2) != localTarget.CanonicalSeedV2 || manifestTarget.CensusOrdinal != i || manifestTarget.CanonicalSeedV2 != localTarget.CanonicalSeedV2 || manifestTarget.CanonicalSeedV2SHA256 != localTarget.CanonicalSeedV2SHA256 || mapping.LocalOrdinal != i {
			return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result local order or identity mismatch"))
		}
		if _, duplicate := seenGlobal[mapping.GlobalOrdinal]; duplicate {
			return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result duplicate global target"))
		}
		seenGlobal[mapping.GlobalOrdinal] = struct{}{}
	}
	if manifest.FileLedger.Denominator != len(local.Files) || len(manifest.FileLedger.Entries) != len(local.Files) || manifest.SymbolLedger.Denominator != len(local.Symbols) || len(manifest.SymbolLedger.Entries) != len(local.Symbols) {
		return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result ledger closure mismatch"))
	}
	for i, file := range local.Files {
		entry := manifest.FileLedger.Entries[i]
		if entry.Ordinal != i || entry.Identity != file.Path || entry.Disposition != file.Disposition {
			return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result file ledger mismatch"))
		}
	}
	for i, symbol := range local.Symbols {
		entry := manifest.SymbolLedger.Entries[i]
		mapping := sidecar.Mappings[i]
		globalName := ""
		for _, global := range request.Input.Targets {
			if global.GlobalOrdinal == mapping.GlobalOrdinal {
				globalName = global.Name
				break
			}
		}
		coreIdentity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", symbol.Coordinate.Path, symbol.Coordinate.Line, symbol.Coordinate.Character, symbol.Kind, globalName, i)
		if entry.Ordinal != i || entry.Identity != coreIdentity || entry.Disposition != censusacquisition.SymbolPrepared {
			return ChunkExecution{}, executionError(StageMapping, errors.New("chunk result symbol ledger mismatch"))
		}
	}
	return ChunkExecution{
		Descriptor:    request.Plan.Chunks[request.ChunkOrdinal],
		PlanDigest:    request.Plan.LogicalDigest,
		IndexDigest:   request.Index.LogicalDigest,
		Projection:    validated,
		Manifest:      manifest,
		ManifestBytes: append([]byte(nil), validated.ManifestBytes...),
		Authority:     0,
		Accepted:      false,
		Completeness:  "UNKNOWN",
	}, nil
}
