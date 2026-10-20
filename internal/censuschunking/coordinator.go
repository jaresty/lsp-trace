package censuschunking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/censusacquisition"
)

const (
	CoordinatorRootVersion       = "lsp-trace.census-chunk-coordinator-root.private.v1"
	CoordinatorCheckpointVersion = "lsp-trace.census-chunk-coordinator-checkpoint.private.v1"
	TerminalIndexVersion         = "lsp-trace.census-chunk-terminal-index.private.v1"
	GlobalDispositionVersion     = "lsp-trace.census-chunk-global-disposition.private.v1"
)

type CoordinatorStage string

const (
	CoordinatorStageValidation  CoordinatorStage = "VALIDATION"
	CoordinatorStageSession     CoordinatorStage = "SESSION"
	CoordinatorStageCore        CoordinatorStage = "CORE"
	CoordinatorStagePublication CoordinatorStage = "PUBLICATION"
	CoordinatorStageReadback    CoordinatorStage = "READBACK"
	CoordinatorStageCheckpoint  CoordinatorStage = "CHECKPOINT"
	CoordinatorStageTerminal    CoordinatorStage = "TERMINAL"
)

type CoordinatorError struct {
	Stage CoordinatorStage
	Err   error
}

func (e *CoordinatorError) Error() string {
	return fmt.Sprintf("chunk coordinator %s: %v", e.Stage, e.Err)
}
func (e *CoordinatorError) Unwrap() error { return e.Err }
func IsCoordinatorStage(err error, stage CoordinatorStage) bool {
	var e *CoordinatorError
	return errors.As(err, &e) && e.Stage == stage
}
func coordinatorError(stage CoordinatorStage, err error) error {
	return &CoordinatorError{Stage: stage, Err: err}
}

type CoordinatorStore interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
	MaxObjectBytes() int64
}
type SessionBinding struct {
	SessionID    string
	Generation   uint64
	WorkspaceURI string
	Ready        bool
}
type SessionValidator interface {
	Validate(context.Context, SessionBinding) error
}
type CoreFactory func(chunkOrdinal int) censusacquisition.Core

type CoordinatorRequest struct {
	Input       Input
	Plan        Plan
	Index       Index
	Projections []Projection
	Sidecars    []Sidecar
	Validator   SessionValidator
	CoreFactory CoreFactory
	Store       CoordinatorStore
	ResumeHead  string
}
type CoordinatorResult struct {
	RootSelector     string
	FinalCheckpoint  string
	CommitSelectors  []string
	TerminalSelector string
}

type coordinatorRoot struct {
	SchemaVersion      string `json:"schema_version"`
	LogicalDigest      string `json:"logical_digest"`
	ImmutableSelector  string `json:"immutable_selector"`
	PlanSelector       string `json:"plan_selector"`
	IndexSelector      string `json:"index_selector"`
	SessionID          string `json:"session_id"`
	Generation         uint64 `json:"generation"`
	WorkspaceURI       string `json:"workspace_uri"`
	WorkspaceDigest    string `json:"workspace_digest"`
	RequestFingerprint string `json:"request_fingerprint"`
	CommittedPrefix    int    `json:"committed_prefix"`
}
type coordinatorCheckpoint struct {
	SchemaVersion        string `json:"schema_version"`
	LogicalDigest        string `json:"logical_digest"`
	ImmutableSelector    string `json:"immutable_selector"`
	RootSelector         string `json:"root_selector"`
	PriorSelector        string `json:"prior_selector"`
	ChunkOrdinal         int    `json:"chunk_ordinal"`
	CommitSelector       string `json:"commit_selector"`
	CommitDigest         string `json:"commit_digest"`
	ManifestSelector     string `json:"manifest_selector"`
	ManifestDigest       string `json:"manifest_digest"`
	TargetStart          int    `json:"target_start"`
	TargetCount          int    `json:"target_count"`
	CommittedTargetCount int    `json:"committed_target_count"`
}
type globalDisposition struct {
	SchemaVersion string `json:"schema_version"`
	PlanDigest    string `json:"plan_digest"`
	TargetCount   int    `json:"target_count"`
	Disposition   string `json:"disposition"`
	Authority     int    `json:"authority"`
	Accepted      bool   `json:"accepted"`
}
type TerminalIndex struct {
	SchemaVersion             string   `json:"schema_version"`
	LogicalDigest             string   `json:"logical_digest"`
	ImmutableSelector         string   `json:"immutable_selector"`
	PlanDigest                string   `json:"plan_digest"`
	IndexDigest               string   `json:"index_digest"`
	CommitSelectors           []string `json:"commit_selectors"`
	FinalCheckpoint           string   `json:"final_checkpoint"`
	GlobalDispositionSelector string   `json:"global_disposition_selector"`
	GlobalTargetCount         int      `json:"global_target_count"`
	Authority                 int      `json:"authority"`
	Accepted                  bool     `json:"accepted"`
	SourceGraphComplete       string   `json:"source_graph_complete"`
	CrossChunkCalls           string   `json:"cross_chunk_calls"`
	AggregateCapture          string   `json:"aggregate_capture"`
}

func ParseTerminalIndex(raw []byte) (TerminalIndex, error) {
	var v TerminalIndex
	if err := decodeCoordinator(raw, &v); err != nil {
		return v, err
	}
	if err := validateTerminal(v); err != nil {
		return TerminalIndex{}, err
	}
	return v, nil
}
func Coordinate(ctx context.Context, r CoordinatorRequest) (CoordinatorResult, error) {
	var out CoordinatorResult
	if r.Store == nil || r.Validator == nil || r.CoreFactory == nil || r.Store.MaxObjectBytes() < 1 {
		return out, coordinatorError(CoordinatorStageValidation, errors.New("invalid coordinator dependencies"))
	}
	if err := Reconcile(r.Input, r.Plan, r.Index, r.Projections, r.Sidecars); err != nil {
		return out, coordinatorError(CoordinatorStageValidation, err)
	}
	planRaw, _ := EncodePlan(r.Plan)
	indexRaw, _ := EncodeIndex(r.Index)
	planSelector, err := putVerified(ctx, r.Store, planRaw)
	if err != nil {
		return out, coordinatorError(CoordinatorStagePublication, err)
	}
	indexSelector, err := putVerified(ctx, r.Store, indexRaw)
	if err != nil {
		return out, coordinatorError(CoordinatorStagePublication, err)
	}
	for i := range r.Projections {
		pb, _ := EncodeProjection(r.Projections[i])
		if _, err = putVerified(ctx, r.Store, pb); err != nil {
			return out, coordinatorError(CoordinatorStagePublication, err)
		}
		sb, _ := EncodeSidecar(r.Sidecars[i])
		if _, err = putVerified(ctx, r.Store, sb); err != nil {
			return out, coordinatorError(CoordinatorStagePublication, err)
		}
	}
	root := coordinatorRoot{SchemaVersion: CoordinatorRootVersion, PlanSelector: planSelector, IndexSelector: indexSelector, SessionID: r.Plan.SessionID, Generation: r.Plan.Generation, WorkspaceURI: r.Input.WorkspaceURI, WorkspaceDigest: r.Plan.WorkspaceDigest, RequestFingerprint: r.Plan.RequestFingerprint, CommittedPrefix: 0}
	setCoordinatorIdentity("lsp-trace:census-chunk-coordinator-root:private:v1", "census-chunk-roots/v1/", &root, func(v *coordinatorRoot, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
	rootRaw, _ := encodeCoordinator(root)
	rootSelector, err := putVerified(ctx, r.Store, rootRaw)
	if err != nil {
		return out, coordinatorError(CoordinatorStagePublication, err)
	}
	out.RootSelector = rootSelector
	commits := []string{}
	prior := rootSelector
	committedTargets := 0
	if r.ResumeHead != "" {
		commits, prior, committedTargets, err = validateCoordinatorChain(ctx, r, rootSelector)
		if err != nil {
			return out, err
		}
	}
	out.CommitSelectors = append([]string(nil), commits...)
	out.FinalCheckpoint = prior
	for ordinal := len(commits); ordinal < len(r.Plan.Chunks); ordinal++ {
		binding := SessionBinding{SessionID: r.Plan.SessionID, Generation: r.Plan.Generation, WorkspaceURI: r.Input.WorkspaceURI, Ready: true}
		if err = r.Validator.Validate(ctx, binding); err != nil {
			return out, coordinatorError(CoordinatorStageSession, err)
		}
		receipt, runErr := ExecuteChunk(ctx, ExecutionRequest{Input: r.Input, Plan: r.Plan, Index: r.Index, Projections: r.Projections, Sidecars: r.Sidecars, ChunkOrdinal: ordinal, Core: r.CoreFactory(ordinal)})
		if runErr != nil {
			return out, coordinatorError(CoordinatorStageCore, runErr)
		}
		manifestSelector, putErr := putVerified(ctx, r.Store, receipt.ManifestBytes)
		if putErr != nil {
			return out, coordinatorError(CoordinatorStagePublication, putErr)
		}
		receiptRaw, _ := EncodeChunkExecution(receipt)
		commitSelector, putErr := putVerified(ctx, r.Store, receiptRaw)
		if putErr != nil {
			return out, coordinatorError(CoordinatorStagePublication, putErr)
		}
		readReceipt, readErr := r.Store.Get(ctx, commitSelector)
		if readErr != nil {
			return out, coordinatorError(CoordinatorStageReadback, readErr)
		}
		parsed, parseErr := ParseChunkExecution(readReceipt)
		if parseErr != nil {
			return out, coordinatorError(CoordinatorStageReadback, parseErr)
		}
		if parsed.ChunkOrdinal != ordinal || parsed.ManifestBytesDigest != digestBytes(receipt.ManifestBytes) || manifestSelector != digestBytes(receipt.ManifestBytes) {
			return out, coordinatorError(CoordinatorStageReadback, errors.New("chunk receipt readback mismatch"))
		}
		d := r.Plan.Chunks[ordinal]
		committedTargets += d.TargetCount
		checkpoint := coordinatorCheckpoint{SchemaVersion: CoordinatorCheckpointVersion, RootSelector: rootSelector, PriorSelector: prior, ChunkOrdinal: ordinal, CommitSelector: commitSelector, CommitDigest: digestBytes(receiptRaw), ManifestSelector: manifestSelector, ManifestDigest: digestBytes(receipt.ManifestBytes), TargetStart: d.TargetStart, TargetCount: d.TargetCount, CommittedTargetCount: committedTargets}
		setCoordinatorIdentity("lsp-trace:census-chunk-coordinator-checkpoint:private:v1", "census-chunk-checkpoints/v1/", &checkpoint, func(v *coordinatorCheckpoint, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
		checkpointRaw, _ := encodeCoordinator(checkpoint)
		checkpointSelector, putErr := putVerified(ctx, r.Store, checkpointRaw)
		if putErr != nil {
			return out, coordinatorError(CoordinatorStageCheckpoint, putErr)
		}
		prior = checkpointSelector
		commits = append(commits, commitSelector)
		out.FinalCheckpoint = prior
		out.CommitSelectors = append([]string(nil), commits...)
	}
	disposition := globalDisposition{SchemaVersion: GlobalDispositionVersion, PlanDigest: r.Plan.LogicalDigest, TargetCount: committedTargets, Disposition: "CHUNK_COMMITS_ONLY", Authority: 0, Accepted: false}
	dispositionRaw, _ := encodeCoordinator(disposition)
	dispositionSelector, err := putVerified(ctx, r.Store, dispositionRaw)
	if err != nil {
		return out, coordinatorError(CoordinatorStageTerminal, err)
	}
	terminal := TerminalIndex{SchemaVersion: TerminalIndexVersion, PlanDigest: r.Plan.LogicalDigest, IndexDigest: r.Index.LogicalDigest, CommitSelectors: append([]string(nil), commits...), FinalCheckpoint: prior, GlobalDispositionSelector: dispositionSelector, GlobalTargetCount: committedTargets, SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", AggregateCapture: "NOT_MATERIALIZED"}
	setCoordinatorIdentity("lsp-trace:census-chunk-terminal-index:private:v1", "census-chunk-terminal-indexes/v1/", &terminal, func(v *TerminalIndex, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
	terminalRaw, _ := encodeCoordinator(terminal)
	terminalSelector, err := putVerified(ctx, r.Store, terminalRaw)
	if err != nil {
		return out, coordinatorError(CoordinatorStageTerminal, err)
	}
	if _, err = ParseTerminalIndex(terminalRaw); err != nil {
		return out, coordinatorError(CoordinatorStageTerminal, err)
	}
	out.TerminalSelector = terminalSelector
	return out, nil
}

func putVerified(ctx context.Context, s CoordinatorStore, raw []byte) (string, error) {
	if int64(len(raw)) > s.MaxObjectBytes() {
		return "", errors.New("store object limit exceeded")
	}
	selector, err := s.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	read, err := s.Get(ctx, selector)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(read, raw) || selector != digestBytes(raw) {
		return "", errors.New("publication readback mismatch")
	}
	return selector, nil
}
func setCoordinatorIdentity[T any](domain, prefix string, v *T, set func(*T, string, string)) {
	set(v, "", "")
	b, _ := json.Marshal(v)
	d := domainDigest(domain, b)
	set(v, d, prefix+d[7:])
}
func validateCoordinatorChain(ctx context.Context, r CoordinatorRequest, rootSelector string) ([]string, string, int, error) {
	seen := map[string]bool{}
	var reverse []coordinatorCheckpoint
	id := r.ResumeHead
	for id != rootSelector {
		if id == "" || seen[id] {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint cycle or hole"))
		}
		seen[id] = true
		raw, err := r.Store.Get(ctx, id)
		if err != nil {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, err)
		}
		if digestBytes(raw) != id {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, errors.New("checkpoint selector mismatch"))
		}
		var c coordinatorCheckpoint
		if err = decodeCoordinator(raw, &c); err != nil {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, err)
		}
		copyC := c
		copyC.LogicalDigest = ""
		copyC.ImmutableSelector = ""
		b, _ := json.Marshal(copyC)
		want := domainDigest("lsp-trace:census-chunk-coordinator-checkpoint:private:v1", b)
		if c.SchemaVersion != CoordinatorCheckpointVersion || c.LogicalDigest != want || c.ImmutableSelector != "census-chunk-checkpoints/v1/"+want[7:] || c.RootSelector != rootSelector {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint identity mismatch"))
		}
		reverse = append(reverse, c)
		id = c.PriorSelector
	}
	commits := make([]string, len(reverse))
	targets := 0
	prior := rootSelector
	for i := range reverse {
		c := reverse[len(reverse)-1-i]
		d := r.Plan.Chunks[i]
		if c.PriorSelector != prior || c.ChunkOrdinal != i || c.TargetStart != d.TargetStart || c.TargetCount != d.TargetCount || c.CommittedTargetCount != targets+d.TargetCount {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint prefix mismatch"))
		}
		receiptRaw, err := r.Store.Get(ctx, c.CommitSelector)
		if err != nil {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, err)
		}
		receipt, err := ParseChunkExecution(receiptRaw)
		if err != nil || receipt.ChunkOrdinal != i || digestBytes(receiptRaw) != c.CommitDigest || receipt.PlanDigest != r.Plan.LogicalDigest || receipt.IndexDigest != r.Index.LogicalDigest {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint receipt mismatch"))
		}
		manifestRaw, err := r.Store.Get(ctx, c.ManifestSelector)
		if err != nil || digestBytes(manifestRaw) != c.ManifestDigest || receipt.ManifestBytesDigest != c.ManifestDigest {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, errors.New("checkpoint manifest mismatch"))
		}
		commits[i] = c.CommitSelector
		targets = c.CommittedTargetCount
		raw, _ := encodeCoordinator(c)
		prior = digestBytes(raw)
	}
	if prior != r.ResumeHead {
		return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("resume head mismatch"))
	}
	return commits, prior, targets, nil
}

func encodeCoordinator(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxCanonicalBytes {
		return nil, errors.New("canonical byte limit exceeded")
	}
	return b, nil
}
func decodeCoordinator(raw []byte, out any) error {
	if len(raw) == 0 || len(raw) > MaxCanonicalBytes {
		return errors.New("invalid canonical object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var x any
	if d.Decode(&x) == nil {
		return errors.New("trailing canonical object")
	}
	b, err := json.Marshal(out)
	if err != nil || !bytes.Equal(b, raw) {
		return errors.New("noncanonical object")
	}
	return nil
}
func validateTerminal(v TerminalIndex) error {
	if v.SchemaVersion != TerminalIndexVersion || v.Authority != 0 || v.Accepted || v.SourceGraphComplete != "UNKNOWN" || v.CrossChunkCalls != "NOT_COMPUTED" || v.AggregateCapture != "NOT_MATERIALIZED" || !validDigest(v.PlanDigest) || !validDigest(v.IndexDigest) || len(v.CommitSelectors) == 0 || v.FinalCheckpoint == "" || v.GlobalDispositionSelector == "" {
		return errors.New("invalid terminal index")
	}
	x := v
	x.LogicalDigest = ""
	x.ImmutableSelector = ""
	b, _ := json.Marshal(x)
	want := domainDigest("lsp-trace:census-chunk-terminal-index:private:v1", b)
	if v.LogicalDigest != want || v.ImmutableSelector != "census-chunk-terminal-indexes/v1/"+want[7:] {
		return errors.New("terminal identity mismatch")
	}
	return nil
}
