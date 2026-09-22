package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/targetpacket"
)

type Worker interface {
	Run(context.Context, describerequest.Record, string) (describeworker.RunResult, error)
}

type WorkerV2 interface {
	RunV2(context.Context, describerequest.Record, targetpacket.Packet, string) (describeworker.RunResultV2, error)
}

type boundedStore interface {
	MaxObjectBytes() int64
}
type privateResourceLimit interface {
	ResourceComponent() string
	ResourceField() string
	ResourceLimit() int64
	ResourceObserved() int64
	ResourceCategory() string
}
type RunnerAdapter struct{ Runner *describeworker.Runner }

func (a RunnerAdapter) Run(ctx context.Context, r describerequest.Record, id string) (describeworker.RunResult, error) {
	if a.Runner == nil {
		return describeworker.RunResult{}, errors.New("worker unavailable")
	}
	return a.Runner.Run(ctx, r, id)
}
func (a RunnerAdapter) RunV2(ctx context.Context, r describerequest.Record, packet targetpacket.Packet, id string) (describeworker.RunResultV2, error) {
	if a.Runner == nil {
		return describeworker.RunResultV2{}, errors.New("worker unavailable")
	}
	return a.Runner.RunV2(ctx, r, packet, id)
}

type StopAfter string

const StopAfterDescribeRequests StopAfter = "DESCRIBE_REQUESTS"
const StatusPaused ContinuationStatus = "PAUSED"

func (s StopAfter) valid() bool { return s == "" || s == StopAfterDescribeRequests }

type Request struct {
	Handoff      CommittedHandoff
	Workspace    string
	Contract     ContinuationContract
	Worker       Worker
	WorkerV2     WorkerV2
	Store        Store
	FreshCapture *FreshCaptureDependencies
	StopAfter    StopAfter
}
type ResumeRequest struct {
	Selector              string
	Workspace             string
	UseCommittedWorkspace bool
	Store                 Store
	Worker                Worker
	WorkerV2              WorkerV2
	FreshCapture          *FreshCaptureDependencies
	StopAfter             StopAfter
}

type Result struct {
	Status                   ContinuationStatus
	CensusID                 string
	HandoffID                string
	CheckpointID             string
	CatalogSelector          string
	Composite                Composite
	PrivateStage             string
	PrivateCode              string
	PrivateSubcode           string
	PrivateResourceComponent string
	PrivateResourceCategory  string
	PrivateResourceLimit     int64
	PrivateResourceObserved  int64
	HandoffBytes             int64
	MaxObjectBytes           int64
	InvocationCanonicalBytes int64
	ResponsePresent          bool
	RequestCount             int
	PreparationCount         int
	ResponseBytes            int64
	WorkerCompositeBytes     int64
	WorkerStoreCap           int64
	InvocationStage          string
	InvocationStatus         string
	InvocationCode           string
	Err                      error
	checkpoint               Checkpoint
}

func FailedCaptureResult(checkpoint Checkpoint) (Result, error) {
	if _, ok := checkpoint.CaptureFailureDiagnostic(); !ok {
		return Result{}, errors.New("typed failed capture checkpoint required")
	}
	return Result{Status: StatusFailedCapture, CensusID: checkpoint.CensusID(), HandoffID: checkpoint.HandoffID(), CheckpointID: checkpoint.ID(), PrivateStage: "CAPTURE", PrivateCode: "CAPTURE", Err: errors.New("FAILED_CAPTURE:CAPTURE"), checkpoint: checkpoint}, nil
}

func (r Result) CaptureFailureDiagnostic() (Diagnostic, bool) {
	return r.checkpoint.CaptureFailureDiagnostic()
}

func (r Result) CaptureFailureCoordinates() (stage, status, category string, observed, limit int64, ok bool) {
	d, found := r.CaptureFailureDiagnostic()
	if !found {
		return "", "", "", 0, 0, false
	}
	return string(d.Stage), string(StatusFailedCapture), d.Category, d.Observed, d.Limit, true
}

type compositeWire struct {
	SchemaVersion      string `json:"schema_version"`
	CompositeID        string `json:"composite_id"`
	CensusID           string `json:"census_id"`
	HandoffID          string `json:"handoff_id"`
	CatalogID          string `json:"catalog_id"`
	CatalogStatus      string `json:"catalog_status"`
	CheckpointSelector string `json:"checkpoint_selector"`
	Authority          int    `json:"authority"`
	Accepted           bool   `json:"accepted"`
	Completeness       string `json:"completeness"`
}
type Composite struct{ wire compositeWire }

func (c Composite) ID() string                 { return c.wire.CompositeID }
func (c Composite) CatalogID() string          { return c.wire.CatalogID }
func (c Composite) CheckpointSelector() string { return c.wire.CheckpointSelector }
func (c Composite) Authority() int             { return c.wire.Authority }
func (c Composite) Accepted() bool             { return c.wire.Accepted }
func (c Composite) Completeness() string       { return c.wire.Completeness }
func (c Composite) Bytes() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c.wire)
}
func ParseComposite(raw []byte) (Composite, error) {
	var w compositeWire
	if err := strictCanonical(raw, &w); err != nil {
		return Composite{}, err
	}
	c := Composite{w}
	return c, c.Validate()
}
func (c Composite) Validate() error {
	w := c.wire
	if w.SchemaVersion != CompositeSchema || !validDigest(w.CompositeID) || w.CensusID == "" || !validDigest(w.HandoffID) || !validDigest(w.CatalogID) || !validDigest(w.CheckpointSelector) || w.CatalogStatus == "" || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness {
		return errors.New("composite integrity mismatch")
	}
	if identityJSON("lsp-trace:census-continuation-composite:v1", w, func(v *compositeWire) { v.CompositeID = "" }) != w.CompositeID {
		return errors.New("composite identity mismatch")
	}
	return nil
}

type stageHookPoint uint8

const (
	stageHookBefore stageHookPoint = iota
	stageHookAfter
)

type stageHook func(context.Context, stageHookPoint, Stage) error

type stageHookError struct{ err error }

func (e stageHookError) Error() string { return "stage hook failed" }
func (e stageHookError) Unwrap() error { return e.err }

type pipelineState struct {
	hook                           stageHook
	handoff                        CommittedHandoff
	contract                       ContinuationContract
	program                        censusprogramc.Result
	programWitness                 ProgramCWitness
	capture                        CaptureResult
	packets                        targetpacket.Result
	requests                       []describerequest.Record
	invocations                    []describeworker.InvocationRecord
	responses                      []describeworker.ResponseRecord
	catalog                        provisionalfeaturecatalog.Catalog
	invocationsV2                  []describeworker.InvocationRecordV2
	responsesV2                    []describeworker.ResponseRecordV2
	catalogV2                      provisionalfeaturecatalog.CatalogV2
	composite                      Composite
	artifacts                      []ArtifactRef
	checkpoint                     Checkpoint
	checkpointSelector             string
	contractID, profileID          string
	historyVersion, catalogVersion string
}

func Run(ctx context.Context, req Request) Result { return runWithHooks(ctx, req, nil) }

func runWithHooks(ctx context.Context, req Request, hook stageHook) Result {
	if err := req.Handoff.Validate(); err != nil {
		return preconditionFailure(req, "HANDOFF_VALIDATE")
	}
	if req.Store == nil {
		return preconditionFailure(req, "STORE_REQUIRED")
	}
	if err := req.Contract.Validate(); err != nil {
		return preconditionFailure(req, "CONTRACT_VALIDATE")
	}
	if !req.StopAfter.valid() {
		return preconditionFailure(req, "STOP_AFTER_INVALID")
	}
	if req.StopAfter == "" {
		if req.Contract.ResponseVersion() == describeworker.ResponseVersionV2 {
			if req.WorkerV2 == nil {
				return preconditionFailure(req, "WORKER_V2_REQUIRED")
			}
		} else if req.Worker == nil {
			return preconditionFailure(req, "WORKER_REQUIRED")
		}
	}
	s := pipelineState{hook: hook, handoff: req.Handoff, contract: req.Contract, contractID: req.Contract.ID(), profileID: req.Contract.ProfileID()}
	if err := callStageHook(ctx, hook, stageHookBefore, StageCensusCommitted); err != nil {
		return preconditionFailure(req, "CENSUS_COMMITTED_HOOK")
	}
	contractBytes, _ := req.Contract.Bytes()
	contractRef, err := putArtifact(ctx, req.Store, "continuation_contract", contractBytes)
	if err != nil {
		return preconditionFailure(req, "CONTRACT_PERSIST")
	}
	hb, err := req.Handoff.Bytes()
	if err != nil {
		return preconditionFailure(req, "HANDOFF_SERIALIZE")
	}
	handoffBytes := int64(len(hb))
	var maxObjectBytes int64
	if bounded, ok := req.Store.(boundedStore); ok {
		maxObjectBytes = bounded.MaxObjectBytes()
	}
	ref, err := putArtifact(ctx, req.Store, "handoff", hb)
	if err != nil {
		result := preconditionFailure(req, "HANDOFF_PERSIST")
		result.HandoffBytes = handoffBytes
		result.MaxObjectBytes = maxObjectBytes
		return result
	}
	s.artifacts = []ArtifactRef{contractRef, ref}
	if err = persistCheckpoint(ctx, req.Store, &s, StageCensusCommitted, StatusRunning, nil); err != nil {
		return preconditionFailure(req, "INITIAL_CHECKPOINT_PERSIST")
	}
	if err = ctx.Err(); err != nil {
		return fail(ctx, req.Store, &s, StatusCancelledAfterCommit, "CANCELLED")
	}
	s.program, err = ReconstructProgramC(req.Handoff, req.Contract.ProgramCSeed())
	if err != nil {
		return fail(ctx, req.Store, &s, StatusFailedCapture, "PROGRAM_C")
	}
	if err = saveProgramCStage(ctx, req.Store, &s); err != nil {
		return fail(ctx, req.Store, &s, StatusFailedCapture, "PROGRAM_C_STORE")
	}
	if req.Workspace == "" {
		return fail(ctx, req.Store, &s, StatusFailedCapture, "WORKSPACE_REQUIRED")
	}
	s.capture, err = captureFresh(req.Handoff, s.program, req.Workspace, req.Handoff.PositionEncoding(), req.Contract.CaptureLimits(), req.FreshCapture)
	if err != nil {
		return failCapture(ctx, req.Store, &s, err)
	}
	if err = saveJSONStage(ctx, req.Store, &s, StageSnapshotsCaptured, "snapshots", s.capture); err != nil {
		result := fail(ctx, req.Store, &s, StatusFailedCapture, "CAPTURE_STORE")
		applyPrivateResourceLimit(&result, err)
		return result
	}
	snaps, lookup, err := ReplaySnapshots(s.capture)
	if err != nil {
		return fail(ctx, req.Store, &s, StatusFailedPacket, "REPLAY")
	}
	s.packets, err = targetpacket.Build(targetpacket.Request{Census: s.program, Snapshots: snaps, Lookup: lookup, Policy: req.Contract.PacketSourcePolicy(), ResolveLimits: req.Contract.PacketResolveLimits(), MaxResponseBytes: req.Contract.PacketMaxResponseBytes()})
	if err != nil {
		return fail(ctx, req.Store, &s, StatusFailedPacket, "PACKET")
	}
	if err = saveJSONStage(ctx, req.Store, &s, StagePacketsPrepared, "packets", s.packets); err != nil {
		return fail(ctx, req.Store, &s, StatusFailedPacket, "PACKET_STORE")
	}
	s.requests, err = buildDescribeRequests(s.packets, req.Contract)
	if err != nil {
		return fail(ctx, req.Store, &s, StatusFailedRender, "RENDER")
	}
	if err = saveRequestStage(ctx, req.Store, &s); err != nil {
		return fail(ctx, req.Store, &s, StatusFailedRender, "RENDER_STORE")
	}
	if req.StopAfter == StopAfterDescribeRequests {
		return pausedResult(s)
	}
	if req.Contract.ResponseVersion() == describeworker.ResponseVersionV2 {
		if r := runWorkersV2(ctx, req.Store, req.WorkerV2, &s); r.Err != nil {
			return r
		}
		return assembleAndCommitV2(ctx, req.Store, &s)
	}
	if r := runWorkers(ctx, req.Store, req.Worker, &s); r.Err != nil {
		return r
	}
	return assembleAndCommit(ctx, req.Store, &s)
}

type TerminalCheckpointPersistError struct{ cause error }

func (e *TerminalCheckpointPersistError) Error() string {
	return "capture terminal checkpoint persistence failed"
}
func (e *TerminalCheckpointPersistError) Unwrap() error { return e.cause }

func failCapture(ctx context.Context, store Store, s *pipelineState, captureErr error) Result {
	diagnostic := classifyCaptureFailure(captureErr)
	if err := persistCheckpointWithoutHooks(context.WithoutCancel(ctx), store, s, s.checkpoint.Stage(), StatusFailedCapture, []Diagnostic{diagnostic}); err != nil {
		return Result{Status: StatusFailedCapture, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, PrivateStage: "CAPTURE_CHECKPOINT", PrivateCode: "TERMINAL_CHECKPOINT_PERSIST", Err: &TerminalCheckpointPersistError{cause: err}, checkpoint: s.checkpoint}
	}
	result := Result{Status: StatusFailedCapture, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, PrivateStage: "CAPTURE", PrivateCode: "CAPTURE", Err: errors.New("FAILED_CAPTURE:CAPTURE"), checkpoint: s.checkpoint}
	applyPrivateResourceLimit(&result, captureErr)
	return result
}

func classifyCaptureFailure(captureErr error) Diagnostic {
	d := Diagnostic{Code: "CAPTURE", Stage: StageProgramCComputed, Category: "internal", FailedField: "capture", Invariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CallerAction: "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION", Recovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}
	var managedPreparation *ManagedPreparationError
	if errors.As(captureErr, &managedPreparation) {
		switch managedPreparation.Kind {
		case ManagedPreparationAvailability:
			d.Category, d.FailedField, d.CallerAction = "availability", "source_preparation", "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME"
			return d
		}
	}
	var limit privateResourceLimit
	if errors.As(captureErr, &limit) {
		d.Category, d.Observed, d.Limit, d.FailedField = limit.ResourceCategory(), limit.ResourceObserved(), limit.ResourceLimit(), limit.ResourceField()
		d.Invariant, d.CallerAction = "OBSERVED_MUST_NOT_EXCEED_LIMIT", "INCREASE_BOUNDED_CAPTURE_LIMIT"
		return d
	}
	message := captureErr.Error()
	switch {
	case strings.Contains(message, "invalid fresh capture dependencies"):
		d.FailedField, d.CallerAction = "runtime_injection", "FIX_CAPTURE_RUNTIME_INJECTION"
	case strings.Contains(message, "resolver"):
		d.FailedField = "resolver"
	case strings.Contains(message, "marshal") || strings.Contains(message, "serialization") || strings.Contains(message, "replay"):
		d.FailedField, d.CallerAction = "serialization", "REPORT_CAPTURE_IMPLEMENTATION"
	}
	return d
}

func applyPrivateResourceLimit(result *Result, err error) {
	var limit privateResourceLimit
	if result == nil || !errors.As(err, &limit) {
		return
	}
	result.PrivateResourceComponent = limit.ResourceComponent()
	result.PrivateResourceCategory = limit.ResourceCategory()
	result.PrivateResourceLimit = limit.ResourceLimit()
	result.PrivateResourceObserved = limit.ResourceObserved()
}

func preconditionFailure(req Request, code string) Result {
	return Result{
		Status:       StatusFailedCatalog,
		CensusID:     req.Handoff.CensusID(),
		HandoffID:    req.Handoff.HandoffID(),
		PrivateStage: "RUN_PRECONDITION",
		PrivateCode:  code,
		Err:          errors.New("continuation precondition failed"),
	}
}

// Resume does not provide an at-most-once worker-execution guarantee across
// concurrent callers or independent processes. Completed checkpoint stages are
// never rerun; callers racing on the same incomplete stage may execute that
// stage more than once, and correctness relies on deterministic identities and
// the Store's immutable no-replace convergence.
func Resume(ctx context.Context, req ResumeRequest) Result { return resumeWithHooks(ctx, req, nil) }

func resumeWithHooks(ctx context.Context, req ResumeRequest, hook stageHook) Result {
	if req.Store == nil || !validDigest(req.Selector) || !req.StopAfter.valid() {
		return Result{Err: errors.New("invalid resume request")}
	}
	verification := newVerificationStore(req.Store)
	chain, err := verifyChainOperation(ctx, verification, req.Selector)
	if err != nil {
		return Result{Err: safePipelineError(err)}
	}
	if len(chain) == 0 {
		return Result{Err: errors.New("empty checkpoint chain")}
	}
	last := chain[len(chain)-1]
	s := pipelineState{hook: hook, checkpoint: last, checkpointSelector: req.Selector, artifacts: last.Artifacts(), contractID: last.ContractID(), profileID: last.ProfileID()}
	if err = loadState(ctx, verification, &s); err != nil {
		return Result{CensusID: last.CensusID(), HandoffID: last.HandoffID(), CheckpointID: last.ID(), Err: safePipelineError(err)}
	}
	if _, ok := last.CaptureFailureDiagnostic(); ok {
		result, _ := FailedCaptureResult(last)
		return result
	}
	if req.StopAfter == StopAfterDescribeRequests {
		if last.Stage() != StageRequestsRendered || last.Status() != StatusRunning {
			return Result{Err: errors.New("stop checkpoint mismatch"), InvocationStage: string(last.Stage()), InvocationStatus: string(last.Status())}
		}
		return pausedResult(s)
	}
	if last.Stage() == StageCatalogCommitted {
		return finalResult(s)
	}
	if stageIndex(last.Stage()) < stageIndex(StageSnapshotsCaptured) && req.Workspace == "" && !req.UseCommittedWorkspace {
		return Result{CensusID: last.CensusID(), HandoffID: last.HandoffID(), CheckpointID: s.checkpointSelector, Err: errors.New("fresh workspace request required before snapshots captured")}
	}
	if stageIndex(last.Stage()) < stageIndex(StageProgramCComputed) {
		s.program, err = ReconstructProgramC(s.handoff, s.contract.ProgramCSeed())
		if err != nil {
			return fail(ctx, req.Store, &s, StatusFailedCapture, "PROGRAM_C")
		}
		if err = saveProgramCStage(ctx, req.Store, &s); err != nil {
			return fail(ctx, req.Store, &s, StatusFailedCapture, "PROGRAM_C_STORE")
		}
	}
	if stageIndex(last.Stage()) < stageIndex(StageSnapshotsCaptured) {
		workspace := req.Workspace
		if workspace == "" && req.UseCommittedWorkspace {
			workspaceURI, parseErr := url.Parse(s.handoff.WorkspaceIdentity().URI)
			if parseErr != nil || workspaceURI.Scheme != "file" {
				return fail(ctx, req.Store, &s, StatusFailedCapture, "WORKSPACE_REQUIRED")
			}
			workspace = workspaceURI.Path
		}
		s.capture, err = captureFresh(s.handoff, s.program, workspace, s.handoff.PositionEncoding(), s.contract.CaptureLimits(), req.FreshCapture)
		if err != nil {
			return failCapture(ctx, req.Store, &s, err)
		}
		if err = saveJSONStage(ctx, req.Store, &s, StageSnapshotsCaptured, "snapshots", s.capture); err != nil {
			result := fail(ctx, req.Store, &s, StatusFailedCapture, "CAPTURE_STORE")
			applyPrivateResourceLimit(&result, err)
			return result
		}
	}
	if stageIndex(last.Stage()) < stageIndex(StagePacketsPrepared) {
		snaps, lookup, e := ReplaySnapshots(s.capture)
		if e != nil {
			return fail(ctx, req.Store, &s, StatusFailedPacket, "REPLAY")
		}
		s.packets, e = targetpacket.Build(targetpacket.Request{Census: s.program, Snapshots: snaps, Lookup: lookup, Policy: s.contract.PacketSourcePolicy(), ResolveLimits: s.contract.PacketResolveLimits(), MaxResponseBytes: s.contract.PacketMaxResponseBytes()})
		if e != nil {
			return fail(ctx, req.Store, &s, StatusFailedPacket, "PACKET")
		}
		if e = saveJSONStage(ctx, req.Store, &s, StagePacketsPrepared, "packets", s.packets); e != nil {
			return fail(ctx, req.Store, &s, StatusFailedPacket, "PACKET_STORE")
		}
	}
	if stageIndex(last.Stage()) < stageIndex(StageRequestsRendered) {
		s.requests, err = buildDescribeRequests(s.packets, s.contract)
		if err != nil {
			return fail(ctx, req.Store, &s, StatusFailedRender, "RENDER")
		}
		if err = saveRequestStage(ctx, req.Store, &s); err != nil {
			return fail(ctx, req.Store, &s, StatusFailedRender, "RENDER_STORE")
		}
	}
	if len(s.requests) != 0 {
		if s.contract.ResponseVersion() == describeworker.ResponseVersionV2 && req.WorkerV2 == nil {
			return Result{Err: errors.New("invalid resume worker version")}
		}
		if s.contract.ResponseVersion() != describeworker.ResponseVersionV2 && req.Worker == nil {
			return Result{Err: errors.New("invalid resume worker version")}
		}
	}
	if s.contract.ResponseVersion() == describeworker.ResponseVersionV2 {
		if stageIndex(last.Stage()) < stageIndex(StageDescribeComplete) {
			if r := runWorkersV2(ctx, req.Store, req.WorkerV2, &s); r.Err != nil {
				return r
			}
		}
		return assembleAndCommitV2(ctx, req.Store, &s)
	}
	if stageIndex(last.Stage()) < stageIndex(StageDescribeComplete) {
		if r := runWorkers(ctx, req.Store, req.Worker, &s); r.Err != nil {
			return r
		}
	}
	return assembleAndCommit(ctx, req.Store, &s)
}

func pausedResult(s pipelineState) Result {
	return Result{Status: StatusPaused, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, RequestCount: len(s.requests), PreparationCount: len(s.packets.Packets)}
}

func saveProgramCStage(ctx context.Context, store Store, s *pipelineState) error {
	witness, err := NewProgramCWitness(s.program)
	if err != nil {
		return err
	}
	b, err := witness.Bytes()
	if err != nil {
		return err
	}
	ref, err := putArtifact(ctx, store, "program_c", b)
	if err != nil {
		return err
	}
	s.programWitness = witness
	s.artifacts = appendRef(s.artifacts, ref)
	return persistCheckpoint(ctx, store, s, StageProgramCComputed, StatusRunning, nil)
}

func saveJSONStage(ctx context.Context, store Store, s *pipelineState, stage Stage, kind string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ref, err := putArtifact(ctx, store, kind, b)
	if err != nil {
		return err
	}
	s.artifacts = appendRef(s.artifacts, ref)
	return persistCheckpoint(ctx, store, s, stage, StatusRunning, nil)
}
func saveRequestStage(ctx context.Context, store Store, s *pipelineState) error {
	b, err := describerequest.Bytes(s.requests)
	if err != nil {
		return err
	}
	ref, err := putArtifact(ctx, store, "requests", b)
	if err != nil {
		return err
	}
	s.artifacts = appendRef(s.artifacts, ref)
	return persistCheckpoint(ctx, store, s, StageRequestsRendered, StatusRunning, nil)
}
func persistCheckpoint(ctx context.Context, store Store, s *pipelineState, stage Stage, status ContinuationStatus, d []Diagnostic) error {
	if stage != StageCensusCommitted {
		if err := callStageHook(ctx, s.hook, stageHookBefore, stage); err != nil {
			return err
		}
	}
	sort.Slice(s.artifacts, func(i, j int) bool { return s.artifacts[i].Kind < s.artifacts[j].Kind })
	prior := s.checkpointSelector
	cp, err := NewCheckpoint(CheckpointInput{PriorCheckpointID: prior, Stage: stage, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), ContractID: s.contractID, ProfileID: s.profileID, Artifacts: s.artifacts, Status: status, Diagnostics: d})
	if err != nil {
		return err
	}
	selector, err := putCheckpoint(ctx, store, cp)
	if err != nil {
		return err
	}
	s.checkpoint = cp
	s.checkpointSelector = selector
	if err := callStageHook(ctx, s.hook, stageHookAfter, stage); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return stageHookError{err: err}
	}
	return nil
}

func callStageHook(ctx context.Context, hook stageHook, point stageHookPoint, stage Stage) error {
	if hook == nil {
		return nil
	}
	if err := hook(ctx, point, stage); err != nil {
		return stageHookError{err: err}
	}
	return nil
}
func runWorkers(ctx context.Context, store Store, w Worker, s *pipelineState) Result {
	selected, attempts, err := selectCatalogAttempts(s, false)
	if err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_HISTORY")
	}
	if len(s.requests) == 0 {
		if err = persistWorkerRecords(ctx, store, s, StageDescribeAttempts); err != nil {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_STORE")
		}
	}
	for _, request := range s.requests {
		prior := selected[request.RecordID]
		if prior.ID() != "" && prior.Status() == describeworker.StatusSucceeded {
			continue
		}
		ordinal := attempts[request.RecordID] + 1
		if ordinal > s.contract.MaxAttempts() {
			continue
		}
		if ctx.Err() != nil {
			return fail(context.Background(), store, s, StatusCancelledAfterCommit, "CANCELLED")
		}
		attempt := attemptID(s.contract.ID(), request.RecordID, ordinal)
		rr, runErr := w.Run(ctx, request, attempt)
		diagnostic := newWorkerPrivateDiagnostic(store, rr, runErr)
		if rr.Invocation.ID() == "" {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_NO_TERMINAL")
		}
		s.invocations = append(s.invocations, rr.Invocation)
		if rr.Response.ID() != "" {
			s.responses = append(s.responses, rr.Response)
		}
		if err = persistWorkerRecords(ctx, store, s, StageDescribeAttempts); err != nil {
			result := fail(ctx, store, s, StatusFailedWorker, "WORKER_STORE")
			diagnostic.apply(&result)
			return result
		}
		if runErr != nil || rr.Invocation.Status() != describeworker.StatusSucceeded {
			result := fail(ctx, store, s, StatusFailedWorker, "WORKER_TERMINAL_FAILURE")
			diagnostic.apply(&result)
			return result
		}
		selected[request.RecordID] = rr.Invocation
		attempts[request.RecordID] = ordinal
	}
	if _, _, err = selectCatalogAttempts(s, true); err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_HISTORY")
	}
	if err = persistWorkerRecords(ctx, store, s, StageDescribeComplete); err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_CHECKPOINT")
	}
	return Result{CheckpointID: s.checkpointSelector}
}

func runWorkersV2(ctx context.Context, store Store, w WorkerV2, s *pipelineState) Result {
	selected, attempts, err := selectCatalogAttemptsV2(s, false)
	if err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_HISTORY")
	}
	packetByID := make(map[string]targetpacket.Packet, len(s.packets.Packets))
	for _, packet := range s.packets.Packets {
		if packet.PacketID == "" || packetByID[packet.PacketID].PacketID != "" {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_PACKET_BINDING")
		}
		packetByID[packet.PacketID] = packet
	}
	if len(s.requests) == 0 {
		if err = persistWorkerRecordsV2(ctx, store, s, StageDescribeAttempts); err != nil {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_STORE")
		}
	}
	for _, request := range s.requests {
		prior := selected[request.RecordID]
		if prior.ID() != "" && prior.Status() == describeworker.StatusSucceeded {
			continue
		}
		ordinal := attempts[request.RecordID] + 1
		if ordinal > s.contract.MaxAttempts() {
			continue
		}
		packet, ok := packetByID[request.Lineage.PacketID]
		if !ok {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_PACKET_BINDING")
		}
		if ctx.Err() != nil {
			return fail(context.Background(), store, s, StatusCancelledAfterCommit, "CANCELLED")
		}
		attempt := attemptID(s.contract.ID(), request.RecordID, ordinal)
		rr, runErr := w.RunV2(ctx, request, packet, attempt)
		if rr.Invocation.ID() == "" {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_NO_TERMINAL")
		}
		s.invocationsV2 = append(s.invocationsV2, rr.Invocation)
		if rr.Response.ID() != "" {
			s.responsesV2 = append(s.responsesV2, rr.Response)
		}
		if err = persistWorkerRecordsV2(ctx, store, s, StageDescribeAttempts); err != nil {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_STORE")
		}
		if runErr != nil || rr.Invocation.Status() != describeworker.StatusSucceeded {
			return fail(ctx, store, s, StatusFailedWorker, "WORKER_TERMINAL_FAILURE")
		}
		selected[request.RecordID] = rr.Invocation
		attempts[request.RecordID] = ordinal
	}
	if _, _, err = selectCatalogAttemptsV2(s, true); err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_HISTORY")
	}
	if err = persistWorkerRecordsV2(ctx, store, s, StageDescribeComplete); err != nil {
		return fail(ctx, store, s, StatusFailedWorker, "WORKER_CHECKPOINT")
	}
	return Result{CheckpointID: s.checkpointSelector}
}

type workerPrivateDiagnostic struct {
	invocationBytes, responseBytes, compositeBytes, storeCap int64
	responsePresent                                          bool
	stage, status, code, subcode                             string
}

func newWorkerPrivateDiagnostic(store Store, rr describeworker.RunResult, runErr error) workerPrivateDiagnostic {
	d := workerPrivateDiagnostic{responsePresent: rr.Response.ID() != "", status: string(rr.Invocation.Status())}
	if bounded, ok := store.(boundedStore); ok {
		d.storeCap = bounded.MaxObjectBytes()
	}
	if raw, err := rr.Invocation.Bytes(); err == nil {
		d.invocationBytes = int64(len(raw))
	}
	if d.responsePresent {
		if raw, err := rr.Response.Bytes(); err == nil {
			d.responseBytes = int64(len(raw))
		}
	}
	if raw, err := encodeWorkerRecords([]describeworker.InvocationRecord{rr.Invocation}, func() []describeworker.ResponseRecord {
		if d.responsePresent {
			return []describeworker.ResponseRecord{rr.Response}
		}
		return nil
	}()); err == nil {
		d.compositeBytes = int64(len(raw))
	}
	var failure *describeworker.Failure
	if describeworker.AsFailure(runErr, &failure) {
		d.stage, d.code, d.subcode = string(failure.Stage()), string(failure.Code()), failure.Subcode()
	}
	return d
}
func (d workerPrivateDiagnostic) apply(result *Result) {
	result.PrivateStage = d.stage
	result.PrivateCode = d.code
	result.PrivateSubcode = d.subcode
	result.InvocationCanonicalBytes = d.invocationBytes
	result.ResponsePresent = d.responsePresent
	result.ResponseBytes = d.responseBytes
	result.WorkerCompositeBytes = d.compositeBytes
	result.WorkerStoreCap = d.storeCap
	result.InvocationStage = d.stage
	result.InvocationStatus = d.status
	result.InvocationCode = d.code
}

func persistWorkerRecords(ctx context.Context, store Store, s *pipelineState, stage Stage) error {
	b, err := encodeWorkerRecords(s.invocations, s.responses)
	if err != nil {
		return err
	}
	ref, err := putArtifact(ctx, store, "describe_records", b)
	if err != nil {
		return err
	}
	s.artifacts = appendRef(s.artifacts, ref)
	return persistCheckpoint(ctx, store, s, stage, StatusRunning, nil)
}
func persistWorkerRecordsV2(ctx context.Context, store Store, s *pipelineState, stage Stage) error {
	b, err := encodeWorkerHistoryV2(s.invocationsV2, s.responsesV2)
	if err != nil {
		return err
	}
	ref, err := putArtifact(ctx, store, "describe_records", b)
	if err != nil {
		return err
	}
	s.artifacts = appendRef(s.artifacts, ref)
	return persistCheckpoint(ctx, store, s, stage, StatusRunning, nil)
}
func assembleAndCommit(ctx context.Context, store Store, s *pipelineState) Result {
	selected, _, err := selectCatalogAttempts(s, true)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_HISTORY")
	}
	invocations := make([]describeworker.InvocationRecord, 0, len(s.requests))
	responses := make([]describeworker.ResponseRecord, 0, len(s.requests))
	for _, request := range s.requests {
		invocation := selected[request.RecordID]
		invocations = append(invocations, invocation)
		if response := invocation.Response(); response.ID() != "" {
			responses = append(responses, response)
		}
	}
	s.catalog, err = provisionalfeaturecatalog.BuildWithPreparationFailures(len(s.program.Representatives.Nominations), catalogPreparationFailures(s.packets.PreparationFailures), s.packets, s.requests, invocations, responses)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG")
	}
	b, _ := s.catalog.Bytes()
	ref, err := putArtifact(ctx, store, "catalog", b)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_STORE")
	}
	s.artifacts = appendRef(s.artifacts, ref)
	if err = persistCheckpoint(ctx, store, s, StageCatalogAssembled, StatusRunning, nil); err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_CHECKPOINT")
	}
	cw := compositeWire{SchemaVersion: CompositeSchema, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CatalogID: s.catalog.ID(), CatalogStatus: string(s.catalog.Outcome()), CheckpointSelector: s.checkpointSelector, Completeness: ContinuationCompleteness}
	cw.CompositeID = identityJSON("lsp-trace:census-continuation-composite:v1", cw, func(v *compositeWire) { v.CompositeID = "" })
	s.composite = Composite{cw}
	cb, _ := s.composite.Bytes()
	cref, err := putArtifact(ctx, store, "composite", cb)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "COMPOSITE_STORE")
	}
	s.artifacts = appendRef(s.artifacts, cref)
	if err = persistCheckpoint(ctx, store, s, StageCatalogCommitted, StatusComplete, nil); err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "COMMIT")
	}
	return Result{Status: StatusComplete, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, CatalogSelector: ref.ID, Composite: s.composite}
}
func assembleAndCommitV2(ctx context.Context, store Store, s *pipelineState) Result {
	selected, _, err := selectCatalogAttemptsV2(s, true)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_HISTORY")
	}
	responses := make([]describeworker.ResponseRecordV2, 0, len(s.requests))
	for _, request := range s.requests {
		if response := selected[request.RecordID].Response(); response.ID() != "" {
			responses = append(responses, response)
		}
	}
	s.catalogV2, err = provisionalfeaturecatalog.BuildV2WithPreparationFailures(len(s.program.Representatives.Nominations), catalogPreparationFailures(s.packets.PreparationFailures), responses)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG")
	}
	b, _ := s.catalogV2.Bytes()
	ref, err := putArtifact(ctx, store, "catalog", b)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_STORE")
	}
	s.artifacts = appendRef(s.artifacts, ref)
	if err = persistCheckpoint(ctx, store, s, StageCatalogAssembled, StatusRunning, nil); err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "CATALOG_CHECKPOINT")
	}
	cw := compositeWire{SchemaVersion: CompositeSchema, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CatalogID: s.catalogV2.ID(), CatalogStatus: string(s.catalogV2.Outcome()), CheckpointSelector: s.checkpointSelector, Completeness: ContinuationCompleteness}
	cw.CompositeID = identityJSON("lsp-trace:census-continuation-composite:v1", cw, func(v *compositeWire) { v.CompositeID = "" })
	s.composite = Composite{cw}
	cb, _ := s.composite.Bytes()
	cref, err := putArtifact(ctx, store, "composite", cb)
	if err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "COMPOSITE_STORE")
	}
	s.artifacts = appendRef(s.artifacts, cref)
	if err = persistCheckpoint(ctx, store, s, StageCatalogCommitted, StatusComplete, nil); err != nil {
		return fail(ctx, store, s, StatusFailedCatalog, "COMMIT")
	}
	return Result{Status: StatusComplete, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, CatalogSelector: ref.ID, Composite: s.composite}
}
func fail(ctx context.Context, store Store, s *pipelineState, status ContinuationStatus, code string) Result {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		status, code = StatusCancelledAfterCommit, "CANCELED"
	}
	_ = persistCheckpointWithoutHooks(context.WithoutCancel(ctx), store, s, s.checkpoint.Stage(), status, []Diagnostic{{Code: code, Stage: s.checkpoint.Stage()}})
	return Result{Status: status, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, Err: errors.New(string(status) + ":" + code)}
}
func persistCheckpointWithoutHooks(ctx context.Context, store Store, s *pipelineState, stage Stage, status ContinuationStatus, diagnostics []Diagnostic) error {
	hook := s.hook
	s.hook = nil
	err := persistCheckpoint(ctx, store, s, stage, status, diagnostics)
	s.hook = hook
	return err
}

func finalResult(s pipelineState) Result {
	return Result{Status: StatusComplete, CensusID: s.handoff.CensusID(), HandoffID: s.handoff.HandoffID(), CheckpointID: s.checkpointSelector, CatalogSelector: artifactSelector(s.checkpoint, "catalog"), Composite: s.composite}
}
func attemptID(contract, request string, ordinal int) string {
	return identityJSON("lsp-trace:census-continuation-attempt:v1", struct {
		Contract string `json:"contract"`
		Request  string `json:"request"`
		Ordinal  int    `json:"ordinal"`
	}{contract, request, ordinal}, func(*struct {
		Contract string `json:"contract"`
		Request  string `json:"request"`
		Ordinal  int    `json:"ordinal"`
	}) {
	})
}

func selectCatalogAttempts(s *pipelineState, requireComplete bool) (map[string]describeworker.InvocationRecord, map[string]int, error) {
	requests := make(map[string]describerequest.Record, len(s.requests))
	for _, request := range s.requests {
		if _, duplicate := requests[request.RecordID]; duplicate {
			return nil, nil, errors.New("retry history invalid")
		}
		requests[request.RecordID] = request
	}
	type attemptRecord struct {
		ordinal    int
		invocation describeworker.InvocationRecord
	}
	byRequest := make(map[string][]attemptRecord, len(requests))
	for _, invocation := range s.invocations {
		if err := invocation.Validate(); err != nil {
			return nil, nil, errors.New("retry history invalid")
		}
		binding := invocation.Binding()
		request, ok := requests[binding.RequestRecordID]
		if !ok || binding.MessageID != request.Envelope.MessageID {
			return nil, nil, errors.New("retry history invalid")
		}
		ordinal := 0
		for candidate := 1; candidate <= s.contract.MaxAttempts(); candidate++ {
			if binding.AttemptID == attemptID(s.contract.ID(), request.RecordID, candidate) {
				ordinal = candidate
				break
			}
		}
		if ordinal == 0 {
			return nil, nil, errors.New("retry history invalid")
		}
		byRequest[request.RecordID] = append(byRequest[request.RecordID], attemptRecord{ordinal: ordinal, invocation: invocation})
	}
	responseByID := make(map[string]describeworker.ResponseRecord, len(s.responses))
	for _, response := range s.responses {
		if err := response.Validate(); err != nil || response.ID() == "" {
			return nil, nil, errors.New("retry history invalid")
		}
		if _, duplicate := responseByID[response.ID()]; duplicate {
			return nil, nil, errors.New("retry history invalid")
		}
		responseByID[response.ID()] = response
	}
	selected := make(map[string]describeworker.InvocationRecord, len(requests))
	counts := make(map[string]int, len(requests))
	usedResponses := make(map[string]bool, len(responseByID))
	for requestID := range requests {
		history := byRequest[requestID]
		sort.Slice(history, func(i, j int) bool { return history[i].ordinal < history[j].ordinal })
		var baseline describeworker.IdentityBinding
		for i, item := range history {
			if item.ordinal != i+1 {
				return nil, nil, errors.New("retry history invalid")
			}
			binding := item.invocation.Binding()
			if i == 0 {
				baseline = binding
			} else {
				candidate := binding
				baseline.AttemptID, candidate.AttemptID = "", ""
				if !equalJSON(baseline, candidate) {
					return nil, nil, errors.New("retry history invalid")
				}
			}
			response := item.invocation.Response()
			if item.invocation.Status() == describeworker.StatusSucceeded {
				if i != len(history)-1 || response.ID() == "" {
					return nil, nil, errors.New("retry history invalid")
				}
				stored, ok := responseByID[response.ID()]
				if !ok || !equalJSON(stored.Binding(), response.Binding()) {
					return nil, nil, errors.New("retry history invalid")
				}
				usedResponses[response.ID()] = true
			} else if response.ID() != "" {
				return nil, nil, errors.New("retry history invalid")
			}
		}
		if len(history) == 0 {
			if requireComplete {
				return nil, nil, errors.New("retry history incomplete")
			}
			continue
		}
		selected[requestID] = history[len(history)-1].invocation
		counts[requestID] = len(history)
	}
	if len(usedResponses) != len(responseByID) {
		return nil, nil, errors.New("retry history invalid")
	}
	return selected, counts, nil
}
func selectCatalogAttemptsV2(s *pipelineState, requireComplete bool) (map[string]describeworker.InvocationRecordV2, map[string]int, error) {
	requests := make(map[string]describerequest.Record, len(s.requests))
	for _, request := range s.requests {
		if _, duplicate := requests[request.RecordID]; duplicate {
			return nil, nil, errors.New("retry history invalid")
		}
		requests[request.RecordID] = request
	}
	type record struct {
		ordinal int
		value   describeworker.InvocationRecordV2
	}
	byRequest := make(map[string][]record, len(requests))
	for _, invocation := range s.invocationsV2 {
		if err := invocation.Validate(); err != nil {
			return nil, nil, errors.New("retry history invalid")
		}
		binding := invocation.Binding()
		request, ok := requests[binding.RequestRecordID]
		if !ok || binding.MessageID != request.Envelope.MessageID {
			return nil, nil, errors.New("retry history invalid")
		}
		ordinal := 0
		for candidate := 1; candidate <= s.contract.MaxAttempts(); candidate++ {
			if binding.AttemptID == attemptID(s.contract.ID(), request.RecordID, candidate) {
				ordinal = candidate
				break
			}
		}
		if ordinal == 0 {
			return nil, nil, errors.New("retry history invalid")
		}
		byRequest[request.RecordID] = append(byRequest[request.RecordID], record{ordinal, invocation})
	}
	responseByID := make(map[string]describeworker.ResponseRecordV2, len(s.responsesV2))
	for _, response := range s.responsesV2 {
		if err := response.Validate(); err != nil || response.ID() == "" || responseByID[response.ID()].ID() != "" {
			return nil, nil, errors.New("retry history invalid")
		}
		responseByID[response.ID()] = response
	}
	selected := make(map[string]describeworker.InvocationRecordV2, len(requests))
	counts := make(map[string]int, len(requests))
	usedResponses := make(map[string]bool, len(responseByID))
	for requestID := range requests {
		history := byRequest[requestID]
		sort.Slice(history, func(i, j int) bool { return history[i].ordinal < history[j].ordinal })
		var baseline describeworker.IdentityBinding
		for i, item := range history {
			if item.ordinal != i+1 {
				return nil, nil, errors.New("retry history invalid")
			}
			binding := item.value.Binding()
			if i == 0 {
				baseline = binding
			} else {
				candidate := binding
				baseline.AttemptID, candidate.AttemptID = "", ""
				if !equalJSON(baseline, candidate) {
					return nil, nil, errors.New("retry history invalid")
				}
			}
			response := item.value.Response()
			if item.value.Status() == describeworker.StatusSucceeded {
				if i != len(history)-1 || response.ID() == "" {
					return nil, nil, errors.New("retry history invalid")
				}
				stored, ok := responseByID[response.ID()]
				if !ok || !equalJSON(stored.Host(), response.Host()) || usedResponses[response.ID()] {
					return nil, nil, errors.New("retry history invalid")
				}
				usedResponses[response.ID()] = true
			} else if response.ID() != "" {
				return nil, nil, errors.New("retry history invalid")
			}
		}
		if len(history) == 0 {
			if requireComplete {
				return nil, nil, errors.New("retry history incomplete")
			}
			continue
		}
		selected[requestID] = history[len(history)-1].value
		counts[requestID] = len(history)
	}
	if len(usedResponses) != len(responseByID) {
		return nil, nil, errors.New("retry history invalid")
	}
	return selected, counts, nil
}

func buildDescribeRequests(packets targetpacket.Result, contract ContinuationContract) ([]describerequest.Record, error) {
	if contract.ResponseVersion() == describeworker.ResponseVersionV2 {
		return describerequest.BuildV2(packets, contract.DescribeDeadlineMS())
	}
	return describerequest.Build(packets, contract.DescribeDeadlineMS())
}

func catalogPreparationFailures(in []targetpacket.MemberFailure) []provisionalfeaturecatalog.PreparationFailureInput {
	out := make([]provisionalfeaturecatalog.PreparationFailureInput, len(in))
	for i, failure := range in {
		out[i] = provisionalfeaturecatalog.PreparationFailureInput{FailureID: failure.EvidenceID, NominationID: failure.NominationID, PacketIntentID: failure.PacketIntentID, Role: failure.Role, Code: failure.Code, EvidenceIDs: []string{failure.EvidenceID}}
	}
	return out
}

func appendRef(in []ArtifactRef, r ArtifactRef) []ArtifactRef {
	for i, v := range in {
		if v.Kind == r.Kind {
			in[i] = r
			return in
		}
	}
	return append(in, r)
}
func encodeWorkerRecords(i []describeworker.InvocationRecord, r []describeworker.ResponseRecord) ([]byte, error) {
	type wire struct {
		Invocations []json.RawMessage `json:"invocations"`
		Responses   []json.RawMessage `json:"responses"`
	}
	w := wire{Invocations: []json.RawMessage{}, Responses: []json.RawMessage{}}
	for _, v := range i {
		b, e := v.Bytes()
		if e != nil {
			return nil, e
		}
		w.Invocations = append(w.Invocations, b)
	}
	for _, v := range r {
		b, e := v.Bytes()
		if e != nil {
			return nil, e
		}
		w.Responses = append(w.Responses, b)
	}
	return json.Marshal(w)
}
func decodeWorkerRecords(b []byte) ([]describeworker.InvocationRecord, []describeworker.ResponseRecord, error) {
	var w struct {
		Invocations []json.RawMessage `json:"invocations"`
		Responses   []json.RawMessage `json:"responses"`
	}
	if err := strictCanonical(b, &w); err != nil {
		return nil, nil, err
	}
	var is []describeworker.InvocationRecord
	var rs []describeworker.ResponseRecord
	for _, x := range w.Invocations {
		v, e := describeworker.ParseInvocationRecord(x)
		if e != nil {
			return nil, nil, e
		}
		is = append(is, v)
	}
	for _, x := range w.Responses {
		v, e := describeworker.ParseResponseRecord(x)
		if e != nil {
			return nil, nil, e
		}
		rs = append(rs, v)
	}
	return is, rs, nil
}

type parsedWorkerRecords struct {
	version       string
	invocations   []describeworker.InvocationRecord
	responses     []describeworker.ResponseRecord
	invocationsV2 []describeworker.InvocationRecordV2
	responsesV2   []describeworker.ResponseRecordV2
}
type parsedCatalog struct {
	version string
	v1      provisionalfeaturecatalog.Catalog
	v2      provisionalfeaturecatalog.CatalogV2
}

func (store *verificationStore) parseArtifact(ctx context.Context, ref ArtifactRef) (any, error) {
	key, err := store.key(ref)
	if err != nil {
		return nil, err
	}
	if parsed, ok := store.parsed[key]; ok {
		return parsed, nil
	}
	raw, err := store.Get(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	var parsed any
	switch ref.Kind {
	case "continuation_contract":
		parsed, err = ParseContinuationContract(raw)
	case "handoff":
		parsed, err = Parse(raw)
	case "program_c":
		parsed, err = ParseProgramCWitness(raw)
	case "snapshots":
		var value CaptureResult
		err = strictCanonical(raw, &value)
		parsed = value
	case "packets":
		var value targetpacket.Result
		err = strictCanonical(raw, &value)
		parsed = value
	case "requests":
		parsed, err = describerequest.Parse(raw)
	case "describe_records":
		var header struct {
			SchemaVersion string `json:"schema_version"`
		}
		_ = json.Unmarshal(raw, &header)
		var value parsedWorkerRecords
		if header.SchemaVersion == workerRecordsV2Schema {
			value.version = describeworker.ResponseVersionV2
			value.invocationsV2, value.responsesV2, err = decodeWorkerHistoryV2(raw)
		} else {
			value.version = describeworker.ResponseVersionV1
			value.invocations, value.responses, err = decodeWorkerRecords(raw)
		}
		parsed = value
	case "catalog":
		var header struct {
			SchemaVersion string `json:"schema_version"`
		}
		_ = json.Unmarshal(raw, &header)
		value := parsedCatalog{}
		if header.SchemaVersion == provisionalfeaturecatalog.SchemaVersionV2 {
			value.version = describeworker.ResponseVersionV2
			value.v2, err = provisionalfeaturecatalog.ParseV2(raw)
		} else {
			value.version = describeworker.ResponseVersionV1
			value.v1, err = provisionalfeaturecatalog.Parse(raw)
		}
		parsed = value
	case "composite":
		parsed, err = ParseComposite(raw)
	default:
		err = errors.New("unknown checkpoint artifact kind")
	}
	if err != nil {
		return nil, err
	}
	store.parsed[key] = parsed
	store.parseCounts[key]++
	return parsed, nil
}

func loadStateFromVerificationStore(ctx context.Context, store *verificationStore, s *pipelineState) error {
	var programWitness ProgramCWitness
	for _, artifact := range s.artifacts {
		parsed, err := store.parseArtifact(ctx, artifact)
		if err != nil {
			return err
		}
		switch value := parsed.(type) {
		case ContinuationContract:
			s.contract = value
		case CommittedHandoff:
			s.handoff = value
		case ProgramCWitness:
			programWitness = value
		case CaptureResult:
			s.capture = value
		case targetpacket.Result:
			s.packets = value
		case []describerequest.Record:
			s.requests = value
		case parsedWorkerRecords:
			s.historyVersion = value.version
			s.invocations, s.responses = value.invocations, value.responses
			s.invocationsV2, s.responsesV2 = value.invocationsV2, value.responsesV2
		case parsedCatalog:
			s.catalogVersion = value.version
			s.catalog, s.catalogV2 = value.v1, value.v2
		case Composite:
			s.composite = value
		}
	}
	if s.handoff.HandoffID() == "" || s.contract.ID() != s.contractID || s.contract.ProfileID() != s.profileID {
		return errors.New("handoff or continuation contract artifact missing")
	}
	version := normalizedResponseVersion(s.contract.ResponseVersion())
	if (s.historyVersion != "" && s.historyVersion != version) || (s.catalogVersion != "" && s.catalogVersion != version) {
		return errors.New("continuation artifact version mismatch")
	}
	if programWitness.WitnessID != "" {
		derivedKey := s.handoff.HandoffID() + "\x00" + s.contract.ID()
		if cached, ok := store.derived[derivedKey]; ok {
			s.program = cached.(censusprogramc.Result)
		} else {
			program, err := ReconstructProgramC(s.handoff, s.contract.ProgramCSeed())
			if err != nil {
				return err
			}
			recomputed, err := NewProgramCWitness(program)
			if err != nil || !equalJSON(programWitness, recomputed) {
				return errors.New("program C witness recomputation mismatch")
			}
			s.program = program
			store.derived[derivedKey] = program
		}
		s.programWitness = programWitness
	}
	return nil
}

func loadState(ctx context.Context, store Store, s *pipelineState) error {
	if verification, ok := store.(*verificationStore); ok {
		return loadStateFromVerificationStore(ctx, verification, s)
	}
	for _, a := range s.artifacts {
		b, e := store.Get(ctx, a.ID)
		if e != nil {
			return e
		}
		switch a.Kind {
		case "continuation_contract":
			s.contract, e = ParseContinuationContract(b)
		case "handoff":
			s.handoff, e = Parse(b)
		case "program_c":
			s.program, e = ReconstructProgramC(s.handoff, s.contract.ProgramCSeed())
			if e == nil {
				s.programWitness, e = verifyProgramCWitness(s.program, b)
			}
		case "snapshots":
			e = strictCanonical(b, &s.capture)
		case "packets":
			e = strictCanonical(b, &s.packets)
		case "requests":
			s.requests, e = describerequest.Parse(b)
		case "describe_records":
			var header struct {
				SchemaVersion string `json:"schema_version"`
			}
			_ = json.Unmarshal(b, &header)
			if header.SchemaVersion == workerRecordsV2Schema {
				s.historyVersion = describeworker.ResponseVersionV2
				s.invocationsV2, s.responsesV2, e = decodeWorkerHistoryV2(b)
			} else {
				s.historyVersion = describeworker.ResponseVersionV1
				s.invocations, s.responses, e = decodeWorkerRecords(b)
			}
		case "catalog":
			var header struct {
				SchemaVersion string `json:"schema_version"`
			}
			_ = json.Unmarshal(b, &header)
			if header.SchemaVersion == provisionalfeaturecatalog.SchemaVersionV2 {
				s.catalogVersion = describeworker.ResponseVersionV2
				s.catalogV2, e = provisionalfeaturecatalog.ParseV2(b)
			} else {
				s.catalogVersion = describeworker.ResponseVersionV1
				s.catalog, e = provisionalfeaturecatalog.Parse(b)
			}
		case "composite":
			s.composite, e = ParseComposite(b)
		}
		if e != nil {
			return e
		}
	}
	if s.handoff.HandoffID() == "" || s.contract.ID() != s.contractID || s.contract.ProfileID() != s.profileID {
		return errors.New("handoff or continuation contract artifact missing")
	}
	version := normalizedResponseVersion(s.contract.ResponseVersion())
	if (s.historyVersion != "" && s.historyVersion != version) || (s.catalogVersion != "" && s.catalogVersion != version) {
		return errors.New("continuation artifact version mismatch")
	}
	return nil
}

func normalizedResponseVersion(version string) string {
	if version == describeworker.ResponseVersionV2 {
		return version
	}
	return describeworker.ResponseVersionV1
}

func artifactSelector(checkpoint Checkpoint, kind string) string {
	for _, artifact := range checkpoint.Artifacts() {
		if artifact.Kind == kind {
			return artifact.ID
		}
	}
	return ""
}

func verifyCheckpointStageArtifacts(ctx context.Context, store Store, checkpoint Checkpoint) error {
	s := pipelineState{checkpoint: checkpoint, artifacts: checkpoint.Artifacts(), contractID: checkpoint.ContractID(), profileID: checkpoint.ProfileID()}
	if err := loadState(ctx, store, &s); err != nil {
		return errors.New("checkpoint artifact semantic mismatch")
	}
	if s.handoff.HandoffID() != checkpoint.HandoffID() || s.handoff.CensusID() != checkpoint.CensusID() {
		return errors.New("checkpoint handoff identity mismatch")
	}
	if stageIndex(checkpoint.Stage()) >= stageIndex(StageSnapshotsCaptured) {
		if s.capture.HandoffID != checkpoint.HandoffID() {
			return errors.New("checkpoint snapshot custody mismatch")
		}
		packetKey := "packets\x00" + artifactSelector(checkpoint, "snapshots") + "\x00" + artifactSelector(checkpoint, "packets") + "\x00" + artifactSelector(checkpoint, "program_c") + "\x00" + checkpoint.ContractID()
		verification, cachedStore := store.(*verificationStore)
		_, alreadyValidated := verificationResult(verification, packetKey)
		if !alreadyValidated {
			snapshots, lookup, err := ReplaySnapshots(s.capture)
			if err != nil {
				return errors.New("checkpoint snapshot semantic mismatch")
			}
			if stageIndex(checkpoint.Stage()) >= stageIndex(StagePacketsPrepared) {
				packets, err := targetpacket.Build(targetpacket.Request{Census: s.program, Snapshots: snapshots, Lookup: lookup, Policy: s.contract.PacketSourcePolicy(), ResolveLimits: s.contract.PacketResolveLimits(), MaxResponseBytes: s.contract.PacketMaxResponseBytes()})
				if err != nil || !equalJSON(packets, s.packets) {
					return errors.New("checkpoint packet semantic mismatch")
				}
				if cachedStore {
					verification.derived[packetKey] = true
				}
			}
		}
	}
	if stageIndex(checkpoint.Stage()) >= stageIndex(StageRequestsRendered) {
		requestKey := "requests\x00" + artifactSelector(checkpoint, "packets") + "\x00" + artifactSelector(checkpoint, "requests") + "\x00" + checkpoint.ContractID()
		verification, cachedStore := store.(*verificationStore)
		_, alreadyValidated := verificationResult(verification, requestKey)
		if !alreadyValidated {
			requests, err := buildDescribeRequests(s.packets, s.contract)
			if err != nil || !equalJSON(requests, s.requests) {
				return errors.New("checkpoint request semantic mismatch")
			}
			if cachedStore {
				verification.derived[requestKey] = true
			}
		}
	}
	if stageIndex(checkpoint.Stage()) >= stageIndex(StageDescribeAttempts) {
		var err error
		if normalizedResponseVersion(s.contract.ResponseVersion()) == describeworker.ResponseVersionV2 {
			_, _, err = selectCatalogAttemptsV2(&s, checkpoint.Stage() != StageDescribeAttempts)
		} else {
			_, _, err = selectCatalogAttempts(&s, checkpoint.Stage() != StageDescribeAttempts)
		}
		if err != nil {
			return errors.New("checkpoint retry history semantic mismatch")
		}
	}
	if stageIndex(checkpoint.Stage()) >= stageIndex(StageCatalogAssembled) && normalizedResponseVersion(s.contract.ResponseVersion()) == describeworker.ResponseVersionV1 {
		selected, _, err := selectCatalogAttempts(&s, true)
		if err != nil {
			return errors.New("checkpoint catalog history mismatch")
		}
		invocations := make([]describeworker.InvocationRecord, 0, len(s.requests))
		responses := make([]describeworker.ResponseRecord, 0, len(s.requests))
		for _, request := range s.requests {
			invocation := selected[request.RecordID]
			invocations = append(invocations, invocation)
			if response := invocation.Response(); response.ID() != "" {
				responses = append(responses, response)
			}
		}
		catalog, err := provisionalfeaturecatalog.BuildWithPreparationFailures(len(s.program.Representatives.Nominations), catalogPreparationFailures(s.packets.PreparationFailures), s.packets, s.requests, invocations, responses)
		if err != nil || catalog.ID() != s.catalog.ID() {
			return errors.New("checkpoint catalog semantic mismatch")
		}
	}
	if stageIndex(checkpoint.Stage()) >= stageIndex(StageCatalogAssembled) && normalizedResponseVersion(s.contract.ResponseVersion()) == describeworker.ResponseVersionV2 {
		selected, _, err := selectCatalogAttemptsV2(&s, true)
		if err != nil {
			return errors.New("checkpoint catalog history mismatch")
		}
		responses := make([]describeworker.ResponseRecordV2, 0, len(s.requests))
		for _, request := range s.requests {
			if response := selected[request.RecordID].Response(); response.ID() != "" {
				responses = append(responses, response)
			}
		}
		catalog, err := provisionalfeaturecatalog.BuildV2WithPreparationFailures(len(s.program.Representatives.Nominations), catalogPreparationFailures(s.packets.PreparationFailures), responses)
		if err != nil || catalog.ID() != s.catalogV2.ID() {
			return errors.New("checkpoint catalog semantic mismatch")
		}
	}
	if checkpoint.Stage() == StageCatalogCommitted {
		for _, artifact := range checkpoint.Artifacts() {
			if artifact.Kind != "composite" {
				continue
			}
			var composite Composite
			var err error
			if verification, ok := store.(*verificationStore); ok {
				var parsed any
				parsed, err = verification.parseArtifact(ctx, artifact)
				if err == nil {
					composite = parsed.(Composite)
				}
			} else {
				var raw []byte
				raw, err = store.Get(ctx, artifact.ID)
				if err == nil {
					composite, err = ParseComposite(raw)
				}
			}
			expectedSelector, selectorErr := compositeCheckpointSelector(ctx, store, checkpoint)
			if err != nil || selectorErr != nil || composite.CatalogID() != func() string {
				if normalizedResponseVersion(s.contract.ResponseVersion()) == describeworker.ResponseVersionV2 {
					return s.catalogV2.ID()
				}
				return s.catalog.ID()
			}() || composite.CheckpointSelector() != expectedSelector || composite.wire.CensusID != checkpoint.CensusID() || composite.wire.HandoffID != checkpoint.HandoffID() {
				return fmt.Errorf("checkpoint composite semantic mismatch: parse=%t selector_resolution=%t selector_error=%v catalog=%t selector=%t census=%t handoff=%t", err == nil, selectorErr == nil, selectorErr, composite.CatalogID() == func() string {
					if normalizedResponseVersion(s.contract.ResponseVersion()) == describeworker.ResponseVersionV2 {
						return s.catalogV2.ID()
					}
					return s.catalog.ID()
				}(), composite.CheckpointSelector() == expectedSelector, composite.wire.CensusID == checkpoint.CensusID(), composite.wire.HandoffID == checkpoint.HandoffID())
			}
			return nil
		}
		return errors.New("checkpoint composite missing")
	}
	return nil
}

func compositeCheckpointSelector(ctx context.Context, store Store, checkpoint Checkpoint) (string, error) {
	if checkpoint.Status() == StatusComplete {
		return checkpoint.PriorID(), nil
	}
	if checkpoint.PriorID() == "" {
		return "", errors.New("checkpoint composite predecessor missing")
	}
	raw, err := store.Get(ctx, checkpoint.PriorID())
	if err != nil {
		return "", err
	}
	prior, err := ParseCheckpoint(raw)
	if err != nil {
		return "", err
	}
	if prior.Stage() != StageCatalogCommitted || prior.Status() != StatusComplete || prior.CensusID() != checkpoint.CensusID() || prior.HandoffID() != checkpoint.HandoffID() {
		return "", fmt.Errorf("checkpoint composite predecessor mismatch: stage=%t status=%t census=%t handoff=%t", prior.Stage() == StageCatalogCommitted, prior.Status() == StatusComplete, prior.CensusID() == checkpoint.CensusID(), prior.HandoffID() == checkpoint.HandoffID())
	}
	return prior.PriorID(), nil
}

func verificationResult(store *verificationStore, key string) (any, bool) {
	if store == nil {
		return nil, false
	}
	value, ok := store.derived[key]
	return value, ok
}

func safePipelineError(error) error { return errors.New("continuation validation failed") }

var _ = bytes.Equal
