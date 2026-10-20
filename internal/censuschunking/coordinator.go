package censuschunking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"

	"lsp-trace/internal/censusacquisition"
)

const (
	CoordinatorRootVersion       = "lsp-trace.census-chunk-coordinator-root.private.v2"
	CoordinatorCheckpointVersion = "lsp-trace.census-chunk-coordinator-checkpoint.private.v2"
	CoordinatorHeadVersion       = "lsp-trace.census-chunk-coordinator-head.private.v1"
	TerminalIndexVersion         = "lsp-trace.census-chunk-terminal-index.private.v2"
	GlobalDispositionVersion     = "lsp-trace.census-chunk-global-disposition.private.v2"
)

type CoordinatorStage string

const (
	CoordinatorStageValidation    CoordinatorStage = "VALIDATION"
	CoordinatorStageHead          CoordinatorStage = "HEAD"
	CoordinatorStageConflict      CoordinatorStage = "CONFLICT"
	CoordinatorStageSession       CoordinatorStage = "SESSION"
	CoordinatorStageCore          CoordinatorStage = "CORE"
	CoordinatorStageIndeterminate CoordinatorStage = "INDETERMINATE_EXECUTION"
	CoordinatorStagePublication   CoordinatorStage = "PUBLICATION"
	CoordinatorStageReadback      CoordinatorStage = "READBACK"
	CoordinatorStageCheckpoint    CoordinatorStage = "CHECKPOINT"
	CoordinatorStageTerminal      CoordinatorStage = "TERMINAL"
	CoordinatorStagePanic         CoordinatorStage = "PANIC"
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

// CoordinatorStore is immutable content-addressed object storage. It is not authoritative head storage.
type CoordinatorStore interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
	MaxObjectBytes() int64
}

type HeadRevision struct {
	Generation uint64
	Selector   string
}

// ParseCoordinatorHeadRevision strictly validates one private canonical coordinator
// head and exposes only the revision needed by authoritative HeadStore adapters.
func ParseCoordinatorHeadRevision(raw []byte) (HeadRevision, error) {
	var h coordinatorHead
	if err := decodeCoordinator(raw, &h); err != nil {
		return HeadRevision{}, err
	}
	if h.SchemaVersion != CoordinatorHeadVersion || h.Generation < 1 || h.ImmutableSelector == "" {
		return HeadRevision{}, errors.New("invalid coordinator head revision")
	}
	x := h
	setHeadIdentity(&x)
	if x.LogicalDigest != h.LogicalDigest || x.ImmutableSelector != h.ImmutableSelector {
		return HeadRevision{}, errors.New("coordinator head identity mismatch")
	}
	return HeadRevision{Generation: h.Generation, Selector: h.ImmutableSelector}, nil
}

// HeadStore is the private authoritative mutable register. CompareAndSwap must atomically
// compare both expected generation and selector and install replacement bytes under key.
type HeadStore interface {
	Load(context.Context, string) ([]byte, error)
	CompareAndSwap(context.Context, string, HeadRevision, []byte) error
}

type ExpectedSession struct {
	SessionID    string
	Generation   uint64
	WorkspaceURI string
}

// SessionLease validates exact READY identity and holds custody while fn uses Core.
type SessionLease interface {
	WithReadySession(context.Context, ExpectedSession, func(censusacquisition.Core) error) error
}

type CoordinatorRequest struct {
	Input       Input
	Plan        Plan
	Index       Index
	Projections []Projection
	Sidecars    []Sidecar
	Store       CoordinatorStore
	Heads       HeadStore
	Sessions    SessionLease
}

type CoordinatorResult struct {
	RootSelector, FinalCheckpoint string
	CommitSelectors               []string
	TerminalSelector              string
}

type coordinatorRoot struct {
	SchemaVersion, LogicalDigest, ImmutableSelector string
	PlanSelector                                    string `json:"plan_selector"`
	IndexSelector                                   string `json:"index_selector"`
	SessionID                                       string `json:"session_id"`
	Generation                                      uint64 `json:"generation"`
	WorkspaceURI                                    string `json:"workspace_uri"`
	WorkspaceDigest                                 string `json:"workspace_digest"`
	RequestFingerprint                              string `json:"request_fingerprint"`
}

type coordinatorCheckpoint struct {
	SchemaVersion, LogicalDigest, ImmutableSelector string
	RootSelector                                    string `json:"root_selector"`
	PriorSelector                                   string `json:"prior_selector"`
	ChunkOrdinal                                    int    `json:"chunk_ordinal"`
	CommitSelector                                  string `json:"commit_selector"`
	CommitDigest                                    string `json:"commit_digest"`
	ManifestSelector                                string `json:"manifest_selector"`
	ManifestDigest                                  string `json:"manifest_digest"`
	TargetStart                                     int    `json:"target_start"`
	TargetCount                                     int    `json:"target_count"`
	CommittedTargetCount                            int    `json:"committed_target_count"`
	ClaimGeneration                                 uint64 `json:"claim_generation"`
	ClaimSelector                                   string `json:"claim_selector"`
	IdempotencyKey                                  string `json:"idempotency_key"`
}

type headState string

const (
	headCommitted headState = "COMMITTED"
	headClaimed   headState = "CLAIMED"
	headTerminal  headState = "TERMINAL"
)

type coordinatorHead struct {
	SchemaVersion      string    `json:"schema_version"`
	LogicalDigest      string    `json:"logical_digest"`
	ImmutableSelector  string    `json:"immutable_selector"`
	WorkflowKey        string    `json:"workflow_key"`
	RootSelector       string    `json:"root_selector"`
	PlanDigest         string    `json:"plan_digest"`
	IndexDigest        string    `json:"index_digest"`
	SessionID          string    `json:"session_id"`
	SessionGeneration  uint64    `json:"session_generation"`
	WorkspaceDigest    string    `json:"workspace_digest"`
	Generation         uint64    `json:"generation"`
	State              headState `json:"state"`
	CommittedPrefix    int       `json:"committed_prefix"`
	CheckpointSelector string    `json:"checkpoint_selector"`
	ClaimOrdinal       int       `json:"claim_ordinal"`
	IdempotencyKey     string    `json:"idempotency_key"`
	ReceiptSelector    string    `json:"receipt_selector"`
	ManifestSelector   string    `json:"manifest_selector"`
	TerminalSelector   string    `json:"terminal_selector"`
}

const claimReceiptVersion = "lsp-trace.census-chunk-claim-receipt.private.v1"

type claimReceipt struct {
	SchemaVersion          string          `json:"schema_version"`
	WorkflowKey            string          `json:"workflow_key"`
	RootSelector           string          `json:"root_selector"`
	IdempotencyKey         string          `json:"idempotency_key"`
	ClaimedHeadGeneration  uint64          `json:"claimed_head_generation"`
	ClaimedHeadSelector    string          `json:"claimed_head_selector"`
	PredecessorSelector    string          `json:"predecessor_selector"`
	ChunkOrdinal           int             `json:"chunk_ordinal"`
	ChunkExecutionDigest   string          `json:"chunk_execution_digest"`
	ChunkExecutionSelector string          `json:"chunk_execution_selector"`
	ChunkExecution         json.RawMessage `json:"chunk_execution"`
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
	GlobalDispositionDigest   string   `json:"global_disposition_digest"`
	GlobalTargetCount         int      `json:"global_target_count"`
	WorkflowKey               string   `json:"workflow_key"`
	HeadGeneration            uint64   `json:"head_generation"`
	HeadSelector              string   `json:"head_selector"`
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

func Coordinate(ctx context.Context, r CoordinatorRequest) (out CoordinatorResult, err error) {
	stage := CoordinatorStageValidation
	coreMayHaveBegun := false
	defer func() {
		if p := recover(); p != nil {
			if coreMayHaveBegun {
				stage = CoordinatorStageIndeterminate
			}
			err = coordinatorError(CoordinatorStagePanic, fmt.Errorf("%s panic: %v", stage, p))
			out = CoordinatorResult{}
		}
	}()
	if r.Store == nil || r.Heads == nil || r.Sessions == nil || r.Store.MaxObjectBytes() < 1 {
		return out, coordinatorError(stage, errors.New("invalid coordinator dependencies"))
	}
	if err = Reconcile(r.Input, r.Plan, r.Index, r.Projections, r.Sidecars); err != nil {
		return out, coordinatorError(stage, err)
	}
	planRaw, _ := EncodePlan(r.Plan)
	indexRaw, _ := EncodeIndex(r.Index)
	stage = CoordinatorStagePublication
	planSelector, e := putVerified(ctx, r.Store, planRaw)
	if e != nil {
		return out, coordinatorError(stage, e)
	}
	indexSelector, e := putVerified(ctx, r.Store, indexRaw)
	if e != nil {
		return out, coordinatorError(stage, e)
	}
	for i := range r.Projections {
		b, _ := EncodeProjection(r.Projections[i])
		if _, e = putVerified(ctx, r.Store, b); e != nil {
			return out, coordinatorError(stage, e)
		}
		b, _ = EncodeSidecar(r.Sidecars[i])
		if _, e = putVerified(ctx, r.Store, b); e != nil {
			return out, coordinatorError(stage, e)
		}
	}
	root := coordinatorRoot{SchemaVersion: CoordinatorRootVersion, PlanSelector: planSelector, IndexSelector: indexSelector, SessionID: r.Plan.SessionID, Generation: r.Plan.Generation, WorkspaceURI: r.Input.WorkspaceURI, WorkspaceDigest: r.Plan.WorkspaceDigest, RequestFingerprint: r.Plan.RequestFingerprint}
	setCoordinatorIdentity("lsp-trace:census-chunk-coordinator-root:private:v2", "census-chunk-roots/v2/", &root, func(v *coordinatorRoot, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
	rootRaw, _ := encodeCoordinator(root)
	rootSelector, e := putVerified(ctx, r.Store, rootRaw)
	if e != nil {
		return out, coordinatorError(stage, e)
	}
	out.RootSelector = rootSelector
	key := workflowKey(r, rootSelector)
	head, e := loadOrCreateHead(ctx, r, key, rootSelector)
	if e != nil {
		return out, e
	}
	for {
		if e = validateHead(head, r, key, rootSelector); e != nil {
			return out, coordinatorError(CoordinatorStageHead, e)
		}
		if head.State == headTerminal {
			if e = validateTerminalChain(ctx, r, head); e != nil {
				return out, e
			}
			return resultFromHead(ctx, r, head)
		}
		if head.State == headClaimed {
			if head.ReceiptSelector == "" {
				return out, coordinatorError(CoordinatorStageIndeterminate, errors.New("claimed execution has no verified receipt; automatic retry forbidden"))
			}
			stage = CoordinatorStageReadback
			receipt, manifest, envelope, e := readVerifiedReceipt(ctx, r, head, head.ReceiptSelector, head.ManifestSelector)
			if e != nil {
				return out, coordinatorError(stage, e)
			}
			head, e = commitClaim(ctx, r, key, head, receipt, manifest, envelope)
			if e != nil {
				return out, e
			}
			continue
		}
		if head.CommittedPrefix == len(r.Plan.Chunks) {
			return publishTerminal(ctx, r, key, head)
		}
		ordinal := head.CommittedPrefix
		claim := head
		claim.Generation++
		claim.State = headClaimed
		claim.ClaimOrdinal = ordinal
		claim.IdempotencyKey = claimKey(key, ordinal)
		claim.ReceiptSelector = ""
		claim.ManifestSelector = ""
		setHeadIdentity(&claim)
		if e = casHead(ctx, r, key, head, claim); e != nil {
			return out, e
		}
		head = claim
		var receipt ChunkExecution
		var callbackCalls atomic.Uint32
		var duplicateCallback atomic.Bool
		stage = CoordinatorStageSession
		e = r.Sessions.WithReadySession(ctx, ExpectedSession{r.Plan.SessionID, r.Plan.Generation, r.Input.WorkspaceURI}, func(core censusacquisition.Core) error {
			if !callbackCalls.CompareAndSwap(0, 1) {
				duplicateCallback.Store(true)
				return coordinatorError(CoordinatorStageConflict, errors.New("session lease callback invoked more than once"))
			}
			coreMayHaveBegun = true
			stage = CoordinatorStageCore
			var runErr error
			receipt, runErr = ExecuteChunk(ctx, ExecutionRequest{Input: r.Input, Plan: r.Plan, Index: r.Index, Projections: r.Projections, Sidecars: r.Sidecars, ChunkOrdinal: ordinal, Core: core})
			return runErr
		})
		calls := callbackCalls.Load()
		if duplicateCallback.Load() {
			return out, coordinatorError(CoordinatorStageConflict, errors.New("session lease callback invoked more than once"))
		}
		if calls == 0 {
			if e == nil {
				e = errors.New("session lease did not invoke callback")
			}
			return out, coordinatorError(CoordinatorStageSession, e)
		}
		if e != nil {
			return out, coordinatorError(CoordinatorStageIndeterminate, e)
		}
		stage = CoordinatorStagePublication
		manifestSelector, e := putVerified(ctx, r.Store, receipt.ManifestBytes)
		if e != nil {
			return out, coordinatorError(stage, e)
		}
		receiptRaw, _ := EncodeChunkExecution(receipt)
		envelopeRaw, e := encodeClaimReceipt(head, receiptRaw)
		if e != nil {
			return out, coordinatorError(stage, e)
		}
		receiptSelector, e := putVerified(ctx, r.Store, envelopeRaw)
		if e != nil {
			return out, coordinatorError(stage, e)
		}
		linked := head
		linked.Generation++
		linked.ReceiptSelector = receiptSelector
		linked.ManifestSelector = manifestSelector
		setHeadIdentity(&linked)
		if e = casHead(ctx, r, key, head, linked); e != nil {
			return out, e
		}
		head = linked
		head, e = commitClaim(ctx, r, key, head, receipt, receipt.ManifestBytes, envelopeRaw)
		if e != nil {
			return out, e
		}
		coreMayHaveBegun = false
	}
}

func workflowKey(r CoordinatorRequest, root string) string {
	return "census-chunk-workflows/v1/" + strings.TrimPrefix(domainDigest("lsp-trace:census-chunk-workflow-key:private:v1", []byte(root+"\x00"+r.Plan.LogicalDigest+"\x00"+r.Index.LogicalDigest+"\x00"+r.Plan.SessionID+fmt.Sprint("\x00", r.Plan.Generation, "\x00")+r.Plan.WorkspaceDigest)), "sha256:")
}
func claimKey(key string, ordinal int) string {
	return domainDigest("lsp-trace:census-chunk-claim:private:v1", []byte(fmt.Sprintf("%s\x00%d", key, ordinal)))
}

func loadOrCreateHead(ctx context.Context, r CoordinatorRequest, key, root string) (coordinatorHead, error) {
	raw, e := safeHeadLoad(ctx, r.Heads, key)
	if e == nil {
		var h coordinatorHead
		if e = decodeCoordinator(raw, &h); e != nil {
			return h, coordinatorError(CoordinatorStageHead, e)
		}
		return h, nil
	}
	h := coordinatorHead{SchemaVersion: CoordinatorHeadVersion, WorkflowKey: key, RootSelector: root, PlanDigest: r.Plan.LogicalDigest, IndexDigest: r.Index.LogicalDigest, SessionID: r.Plan.SessionID, SessionGeneration: r.Plan.Generation, WorkspaceDigest: r.Plan.WorkspaceDigest, Generation: 1, State: headCommitted, ClaimOrdinal: -1, CheckpointSelector: root}
	setHeadIdentity(&h)
	raw, _ = encodeCoordinator(h)
	if e = safeHeadCAS(ctx, r.Heads, key, HeadRevision{}, raw); e != nil {
		raw, e = safeHeadLoad(ctx, r.Heads, key)
		if e != nil {
			return h, coordinatorError(CoordinatorStageConflict, e)
		}
		if e = decodeCoordinator(raw, &h); e != nil {
			return h, coordinatorError(CoordinatorStageHead, e)
		}
	}
	return h, nil
}
func validateHead(h coordinatorHead, r CoordinatorRequest, key, root string) error {
	if h.SchemaVersion != CoordinatorHeadVersion || h.WorkflowKey != key || h.RootSelector != root || h.PlanDigest != r.Plan.LogicalDigest || h.IndexDigest != r.Index.LogicalDigest || h.SessionID != r.Plan.SessionID || h.SessionGeneration != r.Plan.Generation || h.WorkspaceDigest != r.Plan.WorkspaceDigest || h.Generation < 1 || h.CommittedPrefix < 0 || h.CommittedPrefix > len(r.Plan.Chunks) {
		return errors.New("head domain binding mismatch")
	}
	x := h
	x.LogicalDigest = ""
	x.ImmutableSelector = ""
	b, _ := json.Marshal(x)
	want := domainDigest("lsp-trace:census-chunk-coordinator-head:private:v1", b)
	if h.LogicalDigest != want || h.ImmutableSelector != "census-chunk-heads/v1/"+want[7:] {
		return errors.New("head identity mismatch")
	}
	if h.State == headClaimed && (h.ClaimOrdinal != h.CommittedPrefix || h.IdempotencyKey != claimKey(key, h.ClaimOrdinal)) {
		return errors.New("claim transition mismatch")
	}
	return nil
}
func setHeadIdentity(h *coordinatorHead) {
	setCoordinatorIdentity("lsp-trace:census-chunk-coordinator-head:private:v1", "census-chunk-heads/v1/", h, func(v *coordinatorHead, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
}
func casHead(ctx context.Context, r CoordinatorRequest, key string, old, next coordinatorHead) error {
	if e := validateTransition(old, next, len(r.Plan.Chunks)); e != nil {
		return coordinatorError(CoordinatorStageCheckpoint, e)
	}
	setHeadIdentity(&next)
	raw, _ := encodeCoordinator(next)
	if e := safeHeadCAS(ctx, r.Heads, key, HeadRevision{old.Generation, old.ImmutableSelector}, raw); e != nil {
		return coordinatorError(CoordinatorStageConflict, e)
	}
	read, e := safeHeadLoad(ctx, r.Heads, key)
	if e != nil || !bytes.Equal(read, raw) {
		return coordinatorError(CoordinatorStageReadback, errors.New("head CAS readback mismatch"))
	}
	return nil
}
func validateTransition(a, b coordinatorHead, chunks int) error {
	if b.Generation != a.Generation+1 || b.WorkflowKey != a.WorkflowKey || b.RootSelector != a.RootSelector || b.PlanDigest != a.PlanDigest || b.IndexDigest != a.IndexDigest || b.SessionID != a.SessionID || b.SessionGeneration != a.SessionGeneration || b.WorkspaceDigest != a.WorkspaceDigest || b.CommittedPrefix < a.CommittedPrefix || b.CommittedPrefix > chunks {
		return errors.New("invalid head transition")
	}
	switch {
	case a.State == headCommitted && b.State == headClaimed:
		return nil
	case a.State == headClaimed && b.State == headClaimed && a.ReceiptSelector == "" && b.ReceiptSelector != "":
		return nil
	case a.State == headClaimed && b.State == headCommitted && a.ReceiptSelector != "" && b.CommittedPrefix == a.CommittedPrefix+1:
		return nil
	case a.State == headCommitted && b.State == headTerminal && a.CommittedPrefix == chunks:
		return nil
	default:
		return errors.New("invalid head state transition")
	}
}

func commitClaim(ctx context.Context, r CoordinatorRequest, key string, h coordinatorHead, receipt ChunkExecution, manifest, envelope []byte) (coordinatorHead, error) {
	claim := claimedHeadIdentity(h)
	if _, e := validateClaimReceipt(claim, envelope); e != nil || digestBytes(envelope) != h.ReceiptSelector {
		return h, coordinatorError(CoordinatorStageReadback, errors.New("claim receipt binding mismatch"))
	}
	d := r.Plan.Chunks[h.ClaimOrdinal]
	prior := h.CheckpointSelector
	cp := coordinatorCheckpoint{SchemaVersion: CoordinatorCheckpointVersion, RootSelector: h.RootSelector, PriorSelector: prior, ChunkOrdinal: h.ClaimOrdinal, CommitSelector: h.ReceiptSelector, CommitDigest: digestBytes(envelope), ManifestSelector: h.ManifestSelector, ManifestDigest: digestBytes(manifest), TargetStart: d.TargetStart, TargetCount: d.TargetCount, CommittedTargetCount: d.TargetStart + d.TargetCount, ClaimGeneration: claim.Generation, ClaimSelector: claim.ImmutableSelector, IdempotencyKey: claim.IdempotencyKey}
	setCoordinatorIdentity("lsp-trace:census-chunk-coordinator-checkpoint:private:v2", "census-chunk-checkpoints/v2/", &cp, func(v *coordinatorCheckpoint, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
	raw, _ := encodeCoordinator(cp)
	selector, e := putVerified(ctx, r.Store, raw)
	if e != nil {
		return h, coordinatorError(CoordinatorStageCheckpoint, e)
	}
	next := h
	next.Generation++
	next.State = headCommitted
	next.CommittedPrefix++
	next.CheckpointSelector = selector
	next.ClaimOrdinal = -1
	next.IdempotencyKey = ""
	next.ReceiptSelector = ""
	next.ManifestSelector = ""
	setHeadIdentity(&next)
	if e = casHead(ctx, r, key, h, next); e != nil {
		return h, e
	}
	return next, nil
}
func mustEncodeReceipt(v ChunkExecution) []byte { b, _ := EncodeChunkExecution(v); return b }

func claimedHeadIdentity(h coordinatorHead) coordinatorHead {
	if h.ReceiptSelector != "" {
		h.Generation--
		h.ReceiptSelector = ""
		h.ManifestSelector = ""
		setHeadIdentity(&h)
	}
	return h
}

func encodeClaimReceipt(h coordinatorHead, execution []byte) ([]byte, error) {
	v, e := ParseChunkExecution(execution)
	if e != nil {
		return nil, e
	}
	envelope := claimReceipt{SchemaVersion: claimReceiptVersion, WorkflowKey: h.WorkflowKey, RootSelector: h.RootSelector, IdempotencyKey: h.IdempotencyKey, ClaimedHeadGeneration: h.Generation, ClaimedHeadSelector: h.ImmutableSelector, PredecessorSelector: h.CheckpointSelector, ChunkOrdinal: h.ClaimOrdinal, ChunkExecutionDigest: v.LogicalDigest, ChunkExecutionSelector: v.ImmutableSelector, ChunkExecution: append(json.RawMessage(nil), execution...)}
	return encodeCoordinator(envelope)
}

func validateClaimReceipt(h coordinatorHead, raw []byte) (ChunkExecution, error) {
	var envelope claimReceipt
	if e := decodeCoordinator(raw, &envelope); e != nil {
		return ChunkExecution{}, e
	}
	v, e := ParseChunkExecution(envelope.ChunkExecution)
	if e != nil || envelope.SchemaVersion != claimReceiptVersion || envelope.WorkflowKey != h.WorkflowKey || envelope.RootSelector != h.RootSelector || envelope.IdempotencyKey != h.IdempotencyKey || envelope.ClaimedHeadGeneration != h.Generation || envelope.ClaimedHeadSelector != h.ImmutableSelector || envelope.PredecessorSelector != h.CheckpointSelector || envelope.ChunkOrdinal != h.ClaimOrdinal || envelope.ChunkExecutionDigest != v.LogicalDigest || envelope.ChunkExecutionSelector != v.ImmutableSelector || v.ChunkOrdinal != h.ClaimOrdinal {
		return ChunkExecution{}, errors.New("claim receipt identity mismatch")
	}
	return v, nil
}

func readVerifiedReceipt(ctx context.Context, r CoordinatorRequest, h coordinatorHead, rs, ms string) (ChunkExecution, []byte, []byte, error) {
	raw, e := safeGet(ctx, r.Store, rs)
	if e != nil {
		return ChunkExecution{}, nil, nil, e
	}
	if digestBytes(raw) != rs {
		return ChunkExecution{}, nil, nil, errors.New("receipt selector mismatch")
	}
	v, e := validateClaimReceipt(claimedHeadIdentity(h), raw)
	if e != nil || v.PlanDigest != r.Plan.LogicalDigest || v.IndexDigest != r.Index.LogicalDigest {
		return ChunkExecution{}, nil, nil, errors.New("receipt binding mismatch")
	}
	m, e := safeGet(ctx, r.Store, ms)
	if e != nil || digestBytes(m) != ms || v.ManifestBytesDigest != digestBytes(m) {
		return ChunkExecution{}, nil, nil, errors.New("manifest binding mismatch")
	}
	return v, m, raw, nil
}

func validateTerminalChain(ctx context.Context, r CoordinatorRequest, h coordinatorHead) error {
	_, _, _, e := readChain(ctx, r, h)
	return e
}
func readChain(ctx context.Context, r CoordinatorRequest, h coordinatorHead) ([]string, string, int, error) {
	seen := map[string]bool{}
	id := h.CheckpointSelector
	rev := []coordinatorCheckpoint{}
	for id != h.RootSelector {
		if id == "" || seen[id] {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint cycle or hole"))
		}
		seen[id] = true
		raw, e := safeGet(ctx, r.Store, id)
		if e != nil || digestBytes(raw) != id {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, errors.New("checkpoint readback mismatch"))
		}
		var c coordinatorCheckpoint
		if e = decodeCoordinator(raw, &c); e != nil {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, e)
		}
		rev = append(rev, c)
		id = c.PriorSelector
	}
	if len(rev) != h.CommittedPrefix {
		return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("checkpoint prefix mismatch"))
	}
	commits := make([]string, len(rev))
	prior := h.RootSelector
	count := 0
	for i := range rev {
		c := rev[len(rev)-1-i]
		d := r.Plan.Chunks[i]
		if c.RootSelector != h.RootSelector || c.PriorSelector != prior || c.ChunkOrdinal != i || c.TargetStart != d.TargetStart || c.TargetCount != d.TargetCount || c.CommittedTargetCount != count+d.TargetCount {
			return nil, "", 0, coordinatorError(CoordinatorStageCheckpoint, errors.New("ordered checkpoint range mismatch"))
		}
		claim := coordinatorHead{WorkflowKey: h.WorkflowKey, RootSelector: h.RootSelector, IdempotencyKey: c.IdempotencyKey, Generation: c.ClaimGeneration, ImmutableSelector: c.ClaimSelector, CheckpointSelector: c.PriorSelector, ClaimOrdinal: i}
		_, m, envelope, e := readVerifiedReceipt(ctx, r, claim, c.CommitSelector, c.ManifestSelector)
		if e != nil || digestBytes(m) != c.ManifestDigest || digestBytes(envelope) != c.CommitDigest {
			return nil, "", 0, coordinatorError(CoordinatorStageReadback, errors.New("commit relational mismatch"))
		}
		commits[i] = c.CommitSelector
		count = c.CommittedTargetCount
		raw, _ := encodeCoordinator(c)
		prior = digestBytes(raw)
	}
	return commits, prior, count, nil
}

func publishTerminal(ctx context.Context, r CoordinatorRequest, key string, h coordinatorHead) (CoordinatorResult, error) {
	commits, final, count, e := readChain(ctx, r, h)
	if e != nil {
		return CoordinatorResult{}, e
	}
	disp := globalDisposition{GlobalDispositionVersion, r.Plan.LogicalDigest, count, "CHUNK_COMMITS_ONLY", 0, false}
	dr, _ := encodeCoordinator(disp)
	ds, e := putVerified(ctx, r.Store, dr)
	if e != nil {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, e)
	}
	read, e := safeGet(ctx, r.Store, ds)
	var check globalDisposition
	if e != nil || decodeCoordinator(read, &check) != nil || check != disp || digestBytes(read) != ds {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, errors.New("global disposition substitution"))
	}
	term := TerminalIndex{SchemaVersion: TerminalIndexVersion, PlanDigest: r.Plan.LogicalDigest, IndexDigest: r.Index.LogicalDigest, CommitSelectors: commits, FinalCheckpoint: final, GlobalDispositionSelector: ds, GlobalDispositionDigest: digestBytes(dr), GlobalTargetCount: count, WorkflowKey: h.WorkflowKey, HeadGeneration: h.Generation, HeadSelector: h.ImmutableSelector, SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", AggregateCapture: "NOT_MATERIALIZED"}
	setCoordinatorIdentity("lsp-trace:census-chunk-terminal-index:private:v2", "census-chunk-terminal-indexes/v2/", &term, func(v *TerminalIndex, d, s string) { v.LogicalDigest = d; v.ImmutableSelector = s })
	tr, _ := encodeCoordinator(term)
	ts, e := putVerified(ctx, r.Store, tr)
	if e != nil {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, e)
	}
	next := h
	next.Generation++
	next.State = headTerminal
	next.TerminalSelector = ts
	setHeadIdentity(&next)
	if e = casHead(ctx, r, key, h, next); e != nil {
		return CoordinatorResult{}, e
	}
	return CoordinatorResult{h.RootSelector, final, commits, ts}, nil
}
func resultFromHead(ctx context.Context, r CoordinatorRequest, h coordinatorHead) (CoordinatorResult, error) {
	commits, final, count, e := readChain(ctx, r, h)
	if e != nil {
		return CoordinatorResult{}, e
	}
	raw, e := safeGet(ctx, r.Store, h.TerminalSelector)
	if e != nil || digestBytes(raw) != h.TerminalSelector {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, errors.New("terminal readback mismatch"))
	}
	term, e := ParseTerminalIndex(raw)
	preTerminal := h
	preTerminal.Generation--
	preTerminal.State = headCommitted
	preTerminal.TerminalSelector = ""
	setHeadIdentity(&preTerminal)
	if e != nil || term.PlanDigest != r.Plan.LogicalDigest || term.IndexDigest != r.Index.LogicalDigest || !reflect.DeepEqual(term.CommitSelectors, commits) || term.GlobalTargetCount != count || term.FinalCheckpoint != final || term.WorkflowKey != h.WorkflowKey || term.HeadGeneration != preTerminal.Generation || term.HeadSelector != preTerminal.ImmutableSelector {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, errors.New("terminal chain mismatch"))
	}
	dr, e := safeGet(ctx, r.Store, term.GlobalDispositionSelector)
	var d globalDisposition
	if e != nil || decodeCoordinator(dr, &d) != nil || digestBytes(dr) != term.GlobalDispositionDigest || d.SchemaVersion != GlobalDispositionVersion || d.PlanDigest != term.PlanDigest || d.TargetCount != term.GlobalTargetCount || d.Disposition != "CHUNK_COMMITS_ONLY" || d.Authority != term.Authority || d.Accepted != term.Accepted || term.GlobalDispositionSelector != digestBytes(dr) {
		return CoordinatorResult{}, coordinatorError(CoordinatorStageTerminal, errors.New("terminal disposition mismatch"))
	}
	return CoordinatorResult{h.RootSelector, final, commits, h.TerminalSelector}, nil
}

func putVerified(ctx context.Context, s CoordinatorStore, raw []byte) (string, error) {
	if int64(len(raw)) > s.MaxObjectBytes() {
		return "", errors.New("store object limit exceeded")
	}
	selector, e := safePut(ctx, s, raw)
	if e != nil {
		return "", e
	}
	read, e := safeGet(ctx, s, selector)
	if e != nil {
		return "", e
	}
	if !bytes.Equal(read, raw) || selector != digestBytes(raw) {
		return "", errors.New("publication readback mismatch")
	}
	return selector, nil
}
func safePut(ctx context.Context, s CoordinatorStore, b []byte) (v string, e error) {
	defer func() {
		if p := recover(); p != nil {
			e = fmt.Errorf("store put panic: %v", p)
		}
	}()
	return s.Put(ctx, b)
}
func safeGet(ctx context.Context, s CoordinatorStore, id string) (v []byte, e error) {
	defer func() {
		if p := recover(); p != nil {
			e = fmt.Errorf("store get panic: %v", p)
		}
	}()
	return s.Get(ctx, id)
}
func safeHeadLoad(ctx context.Context, s HeadStore, k string) (v []byte, e error) {
	defer func() {
		if p := recover(); p != nil {
			e = fmt.Errorf("head load panic: %v", p)
		}
	}()
	return s.Load(ctx, k)
}
func safeHeadCAS(ctx context.Context, s HeadStore, k string, x HeadRevision, b []byte) (e error) {
	defer func() {
		if p := recover(); p != nil {
			e = fmt.Errorf("head CAS panic: %v", p)
		}
	}()
	return s.CompareAndSwap(ctx, k, x, b)
}
func setCoordinatorIdentity[T any](domain, prefix string, v *T, set func(*T, string, string)) {
	set(v, "", "")
	b, _ := json.Marshal(v)
	d := domainDigest(domain, b)
	set(v, d, prefix+d[7:])
}
func encodeCoordinator(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
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
	if e := d.Decode(out); e != nil {
		return e
	}
	var x any
	if d.Decode(&x) == nil {
		return errors.New("trailing canonical object")
	}
	b, e := json.Marshal(out)
	if e != nil || !bytes.Equal(b, raw) {
		return errors.New("noncanonical object")
	}
	return nil
}
func validateTerminal(v TerminalIndex) error {
	if v.SchemaVersion != TerminalIndexVersion || v.Authority != 0 || v.Accepted || v.SourceGraphComplete != "UNKNOWN" || v.CrossChunkCalls != "NOT_COMPUTED" || v.AggregateCapture != "NOT_MATERIALIZED" || !validDigest(v.PlanDigest) || !validDigest(v.IndexDigest) || len(v.CommitSelectors) == 0 || v.FinalCheckpoint == "" || v.GlobalDispositionSelector == "" || !validDigest(v.GlobalDispositionDigest) || v.WorkflowKey == "" || v.HeadGeneration < 1 || v.HeadSelector == "" {
		return errors.New("invalid terminal index")
	}
	x := v
	x.LogicalDigest = ""
	x.ImmutableSelector = ""
	b, _ := json.Marshal(x)
	want := domainDigest("lsp-trace:census-chunk-terminal-index:private:v2", b)
	if v.LogicalDigest != want || v.ImmutableSelector != "census-chunk-terminal-indexes/v2/"+want[7:] {
		return errors.New("terminal identity mismatch")
	}
	return nil
}
