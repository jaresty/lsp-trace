package censusacquisition

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/operation"
)

const (
	BatchFailureInvalidInput = "CENSUS_BATCH_INVALID_INPUT"
	BatchFailureSessionDrift = "CENSUS_BATCH_SESSION_DRIFT"
	BatchFailureCancelled    = "CENSUS_BATCH_CANCELLED"
	BatchFailureAcquisition  = "CENSUS_BATCH_ACQUISITION_FAILED"
	BatchFailureAdmission    = "CENSUS_BATCH_ADMISSION_FAILED"
)

// BatchResult is an immutable-by-copy exact result for one census batch.
type BatchResult struct {
	Session                SessionIdentity
	CensusID               string
	BatchID                string
	Ordinal                int
	CanonicalSeedsV2SHA256 string
	Raw                    []byte
}

// BatchFailure is the typed, fail-closed census acquisition failure.
type BatchFailure struct {
	Code string
	Err  error
}

func (f *BatchFailure) Error() string {
	if f == nil {
		return ""
	}
	if f.Err != nil {
		return fmt.Sprintf("%s: %v", f.Code, f.Err)
	}
	return f.Code
}
func (f *BatchFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

// BatchSession is the least-authority view of one already initialized session.
// Production implementations must route Execute through trusted acquisition
// orchestration; the adapter never starts, stops, restarts, or publishes.
type BatchSession interface {
	SessionID() string
	Generation() uint64
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type BatchAdapter struct {
	session BatchSession
	limits  acquisitionops.Limits
}

func NewBatchAdapter(session BatchSession, limits acquisitionops.Limits) *BatchAdapter {
	return &BatchAdapter{session: session, limits: limits}
}

// NewRuntimeBatchAdapter binds the adapter to the trusted census orchestration
// route over one runtime that has already initialized the supplied session.
func NewRuntimeBatchAdapter(runtime acquisitionorchestration.Runtime, session SessionIdentity, limits acquisitionops.Limits) *BatchAdapter {
	return NewBatchAdapter(&runtimeBatchSession{runtime: runtime, identity: session}, limits)
}

type runtimeBatchSession struct {
	runtime  acquisitionorchestration.Runtime
	identity SessionIdentity
}

func (s *runtimeBatchSession) SessionID() string { return s.identity.SessionID }
func (s *runtimeBatchSession) Generation() uint64 {
	if s == nil || s.runtime == nil {
		return 0
	}
	found := uint64(0)
	for _, record := range s.runtime.Records() {
		if record.SessionID == s.identity.SessionID {
			if found != 0 {
				return 0
			}
			found = record.Generation
		}
	}
	return found
}
func (s *runtimeBatchSession) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	return acquisitionorchestration.ExecuteCensusBatch(ctx, s.runtime, request, request.RetainedSeedSpec)
}

func batchFail(code string, err error) (BatchResult, *BatchFailure) {
	return BatchResult{}, &BatchFailure{Code: code, Err: err}
}

func (a *BatchAdapter) AcquireV5(ctx context.Context, request BatchRequest) (AcquiredV5, error) {
	result, failure := a.AcquireBatch(ctx, request)
	if failure != nil {
		return AcquiredV5{}, failure
	}
	return AcquiredV5{Session: result.Session, Raw: bytes.Clone(result.Raw)}, nil
}

func (a *BatchAdapter) AcquireBatch(ctx context.Context, request BatchRequest) (BatchResult, *BatchFailure) {
	request = cloneBatchRequest(request)
	if err := ctx.Err(); err != nil {
		return batchFail(BatchFailureCancelled, err)
	}
	if a == nil || a.session == nil {
		return batchFail(BatchFailureInvalidInput, errors.New("initialized session required"))
	}
	if err := request.Session.Validate(); err != nil {
		return batchFail(BatchFailureInvalidInput, err)
	}
	if request.CensusID == "" || request.BatchID == "" || request.Ordinal < 0 || len(request.Targets) < 1 || len(request.Targets) > 63 || len(request.CanonicalSeedsV2) == 0 {
		return batchFail(BatchFailureInvalidInput, errors.New("complete bounded batch identity required"))
	}
	if a.session.SessionID() != request.Session.SessionID || a.session.Generation() != request.Session.Generation {
		return batchFail(BatchFailureSessionDrift, errors.New("generation drift before batch"))
	}
	manifest := request.AcquisitionManifest(a.limits)
	input, err := json.Marshal(acquisitionops.Input{SessionID: request.Session.SessionID, Generation: request.Session.Generation, SeedManifest: manifest, OutputVersion: graphprovenance.VersionV5})
	if err != nil {
		return batchFail(BatchFailureInvalidInput, err)
	}
	result, failed := a.session.Execute(ctx, operation.Request{Name: acquisitionops.SliceV3, RequestID: fmt.Sprintf("census:%s:%06d:%s", request.CensusID, request.Ordinal, request.BatchID), Input: input, RetainedSeedSpec: bytes.Clone(request.CanonicalSeedsV2)})
	if failed != nil {
		return batchFail(BatchFailureAcquisition, operation.NormalizeFailure(failed))
	}
	if err := ctx.Err(); err != nil {
		return batchFail(BatchFailureCancelled, err)
	}
	if a.session.SessionID() != request.Session.SessionID || a.session.Generation() != request.Session.Generation {
		return batchFail(BatchFailureSessionDrift, errors.New("generation drift after batch"))
	}
	if err := admitCompleteBatch(result.Artifact, request); err != nil {
		return batchFail(BatchFailureAdmission, err)
	}
	return BatchResult{Session: request.Session, CensusID: request.CensusID, BatchID: request.BatchID, Ordinal: request.Ordinal, CanonicalSeedsV2SHA256: rawDigest(request.CanonicalSeedsV2), Raw: bytes.Clone(result.Artifact)}, nil
}

func cloneBatchRequest(in BatchRequest) BatchRequest {
	in.CanonicalSeedsV2 = bytes.Clone(in.CanonicalSeedsV2)
	in.Targets = append([]PreparedTarget(nil), in.Targets...)
	for i := range in.Targets {
		in.Targets[i] = cloneTarget(in.Targets[i])
	}
	return in
}

func admitCompleteBatch(raw []byte, request BatchRequest) error {
	if _, err := admit(raw, request); err != nil {
		return err
	}
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		return err
	}
	result, err := graph.DecodeNativeV3(native)
	if err != nil {
		return err
	}
	if !result.Summary.Complete || result.Summary.Truncated {
		return errors.New("incomplete or truncated traversal")
	}
	if len(result.Seeds) != len(request.Targets) {
		return errors.New("seed result cardinality mismatch")
	}
	for i, seed := range result.Seeds {
		want := request.AcquisitionManifest(acquisitionops.Limits{}).Root.ID
		if i > 0 {
			want = request.AcquisitionManifest(acquisitionops.Limits{}).RequiredTargets[i-1].ID
		}
		if seed.Label != want || seed.Failure != nil || len(seed.ReachedNodeIDs) == 0 {
			return fmt.Errorf("seed %d traversal incomplete", i)
		}
	}
	if len(result.SiblingCandidates) != 0 || len(result.DispatchRelationships) != 0 {
		return errors.New("non-CALLS inference present")
	}
	return nil
}
