package censuschunking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"reflect"

	"lsp-trace/internal/strictjson"

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
const (
	ChunkExecutionVersion = "lsp-trace.census-chunk-execution.private.v1"
	chunkExecutionDomain  = "lsp-trace:census-chunk-execution:private:v1"
)

// ChunkExecution is local execution evidence, not a global census result.
type ChunkExecution struct {
	SchemaVersion       string          `json:"schema_version"`
	LogicalDigest       string          `json:"logical_digest"`
	ImmutableSelector   string          `json:"immutable_selector"`
	Descriptor          ChunkDescriptor `json:"descriptor"`
	PlanDigest          string          `json:"plan_digest"`
	IndexDigest         string          `json:"index_digest"`
	ChunkOrdinal        int             `json:"chunk_ordinal"`
	TargetStart         int             `json:"target_start"`
	TargetCount         int             `json:"target_count"`
	ProjectionIdentity  string          `json:"projection_identity"`
	ProjectionDigest    string          `json:"projection_digest"`
	SidecarIdentity     string          `json:"sidecar_identity"`
	SidecarDigest       string          `json:"sidecar_digest"`
	CaptureSetIdentity  string          `json:"capture_set_identity"`
	CaptureSetDigest    string          `json:"capture_set_digest"`
	ManifestDigest      string          `json:"manifest_digest"`
	ManifestSelector    string          `json:"manifest_selector"`
	ManifestBytesDigest string          `json:"manifest_bytes_digest"`
	ConstituentDigests  []string        `json:"constituent_digests"`
	Authority           int             `json:"authority"`
	Accepted            bool            `json:"accepted"`
	Completeness        string          `json:"completeness"`

	Projection    censusacquisition.Projection `json:"-"`
	Manifest      captureset.Manifest          `json:"-"`
	ManifestBytes []byte                       `json:"-"`
}

func EncodeChunkExecution(v ChunkExecution) ([]byte, error) {
	if v.LogicalDigest == "" && v.ImmutableSelector == "" {
		setChunkExecutionIdentity(&v)
	}
	if err := validateChunkExecution(v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func ParseChunkExecution(raw []byte) (ChunkExecution, error) {
	var zero ChunkExecution
	if len(raw) == 0 || len(raw) > MaxCanonicalBytes || strictjson.RejectDuplicates(raw) != nil {
		return zero, errors.New("invalid chunk execution JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var v ChunkExecution
	if decoder.Decode(&v) != nil {
		return zero, errors.New("invalid chunk execution JSON")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return zero, errors.New("trailing chunk execution JSON")
	}
	if err := validateChunkExecution(v); err != nil {
		return zero, err
	}
	canonical, _ := json.Marshal(v)
	if !bytes.Equal(raw, canonical) {
		return zero, errors.New("noncanonical chunk execution JSON")
	}
	return v, nil
}

func ValidateChunkExecution(v ChunkExecution, plan Plan, index Index, descriptor ChunkDescriptor, local Projection, sidecar Sidecar, projection censusacquisition.Projection) error {
	if err := validateChunkExecution(v); err != nil {
		return err
	}
	constituentDigests := make([]string, len(projection.Manifest.Constituents))
	for i, constituent := range projection.Manifest.Constituents {
		constituentDigests[i] = constituent.SHA256
	}
	if v.PlanDigest != plan.LogicalDigest || v.IndexDigest != index.LogicalDigest || v.Descriptor != descriptor || descriptor.Ordinal != v.ChunkOrdinal || descriptor.TargetStart != v.TargetStart || descriptor.TargetCount != v.TargetCount || v.ProjectionIdentity != local.LogicalDigest || v.SidecarIdentity != sidecar.LogicalDigest || v.CaptureSetIdentity != projection.CensusID || v.CaptureSetDigest != digestBytes(projection.ManifestBytes) || v.ManifestDigest != projection.Manifest.LogicalDigest || v.ManifestSelector != projection.Manifest.ImmutableSelector || v.ManifestBytesDigest != digestBytes(projection.ManifestBytes) || !reflect.DeepEqual(v.ConstituentDigests, constituentDigests) {
		return errors.New("chunk execution aggregate binding mismatch")
	}
	return nil
}

func validateChunkExecution(v ChunkExecution) error {
	if v.SchemaVersion != ChunkExecutionVersion || v.Accepted || v.Authority != 0 || v.Completeness != "UNKNOWN" || v.ChunkOrdinal < 0 || v.TargetStart < 0 || v.TargetCount < 1 || v.TargetCount > ChunkSize || v.Descriptor.Ordinal != v.ChunkOrdinal || v.Descriptor.TargetStart != v.TargetStart || v.Descriptor.TargetCount != v.TargetCount || !validRef(v.Descriptor.Projection) || !validRef(v.Descriptor.Sidecar) || !validDigest(v.PlanDigest) || !validDigest(v.IndexDigest) || !validDigest(v.ProjectionIdentity) || v.ProjectionDigest != v.Descriptor.Projection.SHA256 || !validDigest(v.SidecarIdentity) || v.SidecarDigest != v.Descriptor.Sidecar.SHA256 || v.CaptureSetIdentity == "" || !validDigest(v.CaptureSetDigest) || !validDigest(v.ManifestDigest) || v.ManifestSelector == "" || !validDigest(v.ManifestBytesDigest) || v.ConstituentDigests == nil {
		return errors.New("invalid chunk execution receipt")
	}
	for _, digest := range v.ConstituentDigests {
		if !validDigest(digest) {
			return errors.New("invalid chunk execution constituent digest")
		}
	}
	return validateIdentity(v.LogicalDigest, v.ImmutableSelector, "census-chunk-executions/v1/", chunkExecutionDomain, v, func(x *ChunkExecution) { x.LogicalDigest = ""; x.ImmutableSelector = "" })
}

func setChunkExecutionIdentity(v *ChunkExecution) {
	setIdentity(chunkExecutionDomain, "census-chunk-executions/v1/", v, func(x *ChunkExecution, digest, selector string) {
		x.LogicalDigest, x.ImmutableSelector = digest, selector
	})
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
	frozen, err := freezeExecutionRequest(request)
	if err != nil {
		return ChunkExecution{}, executionError(StagePrecondition, err)
	}
	request = frozen
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

func freezeExecutionRequest(request ExecutionRequest) (ExecutionRequest, error) {
	type frozenState struct {
		Input       Input
		Plan        Plan
		Index       Index
		Projections []Projection
		Sidecars    []Sidecar
	}
	raw, err := json.Marshal(frozenState{request.Input, request.Plan, request.Index, request.Projections, request.Sidecars})
	if err != nil {
		return ExecutionRequest{}, err
	}
	var state frozenState
	if err := json.Unmarshal(raw, &state); err != nil {
		return ExecutionRequest{}, err
	}
	return ExecutionRequest{Input: state.Input, Plan: state.Plan, Index: state.Index, Projections: state.Projections, Sidecars: state.Sidecars, ChunkOrdinal: request.ChunkOrdinal, Core: request.Core}, nil
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
	descriptor := request.Plan.Chunks[request.ChunkOrdinal]
	constituentDigests := make([]string, len(manifest.Constituents))
	for i, constituent := range manifest.Constituents {
		constituentDigests[i] = constituent.SHA256
	}
	receipt := ChunkExecution{
		SchemaVersion:       ChunkExecutionVersion,
		Descriptor:          descriptor,
		PlanDigest:          request.Plan.LogicalDigest,
		IndexDigest:         request.Index.LogicalDigest,
		ChunkOrdinal:        descriptor.Ordinal,
		TargetStart:         descriptor.TargetStart,
		TargetCount:         descriptor.TargetCount,
		ProjectionIdentity:  local.LogicalDigest,
		ProjectionDigest:    descriptor.Projection.SHA256,
		SidecarIdentity:     sidecar.LogicalDigest,
		SidecarDigest:       descriptor.Sidecar.SHA256,
		CaptureSetIdentity:  validated.CensusID,
		CaptureSetDigest:    digestBytes(validated.ManifestBytes),
		ManifestDigest:      manifest.LogicalDigest,
		ManifestSelector:    manifest.ImmutableSelector,
		ManifestBytesDigest: digestBytes(validated.ManifestBytes),
		ConstituentDigests:  constituentDigests,
		Projection:          validated,
		Manifest:            manifest,
		ManifestBytes:       append([]byte(nil), validated.ManifestBytes...),
		Authority:           0,
		Accepted:            false,
		Completeness:        "UNKNOWN",
	}
	setChunkExecutionIdentity(&receipt)
	if err := ValidateChunkExecution(receipt, request.Plan, request.Index, descriptor, local, sidecar, validated); err != nil {
		return ChunkExecution{}, executionError(StageMapping, err)
	}
	return receipt, nil
}
