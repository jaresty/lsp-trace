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
	"lsp-trace/internal/seedformat"
)

const (
	BatchFailureInvalidInput = "CENSUS_BATCH_INVALID_INPUT"
	BatchFailureSessionDrift = "CENSUS_BATCH_SESSION_DRIFT"
	BatchFailureCancelled    = "CENSUS_BATCH_CANCELLED"
	BatchFailureAcquisition  = "CENSUS_BATCH_ACQUISITION_FAILED"
	BatchFailureAdmission    = "CENSUS_BATCH_ADMISSION_FAILED"
)

type BatchResult struct {
	Session                SessionIdentity
	CensusID               string
	BatchID                string
	Ordinal                int
	CanonicalSeedsV2SHA256 string
	Raw                    []byte
}

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

// batchSession is a package-private test seam. Production construction below
// always binds it to an orchestration-owned opaque capability.
type batchSession interface {
	identity() SessionIdentity
	workspace() (string, error)
	execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type BatchAdapter struct {
	session               batchSession
	limits                acquisitionops.Limits
	beforeNativeAdmission func()
}

func newBatchAdapter(session batchSession, limits acquisitionops.Limits) *BatchAdapter {
	return &BatchAdapter{session: session, limits: limits}
}

func NewRuntimeBatchAdapter(runtime acquisitionorchestration.Runtime, session SessionIdentity, limits acquisitionops.Limits) *BatchAdapter {
	capability := acquisitionorchestration.NewCensusBatchCapability(runtime)
	execute := func(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
		return acquisitionorchestration.ExecuteCensusBatch(ctx, capability, request, request.RetainedSeedSpec)
	}
	return newBatchAdapter(&runtimeBatchSession{runtime: runtime, executeFn: execute, requested: session}, limits)
}

type runtimeBatchSession struct {
	runtime   acquisitionorchestration.Runtime
	executeFn func(context.Context, operation.Request) (operation.Result, *operation.Failure)
	requested SessionIdentity
}

func (s *runtimeBatchSession) identity() SessionIdentity {
	if s == nil || s.runtime == nil {
		return SessionIdentity{}
	}
	found := SessionIdentity{}
	for _, r := range s.runtime.Records() {
		if r.SessionID == s.requested.SessionID {
			if found.Generation != 0 {
				return SessionIdentity{}
			}
			found = SessionIdentity{SessionID: r.SessionID, Generation: r.Generation}
		}
	}
	return found
}
func (s *runtimeBatchSession) workspace() (string, error) {
	if s == nil || s.runtime == nil {
		return "", errors.New("managed runtime required")
	}
	workspace := ""
	for _, r := range s.runtime.Records() {
		if r.SessionID == s.requested.SessionID && r.Generation == s.requested.Generation {
			if workspace != "" {
				return "", errors.New("ambiguous session workspace")
			}
			workspace = r.Profile.Workspace().String()
		}
	}
	if workspace == "" {
		return "", errors.New("session workspace unavailable")
	}
	return workspace, nil
}
func (s *runtimeBatchSession) execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	if s == nil || s.executeFn == nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("census capability unavailable")}
	}
	return s.executeFn(ctx, request)
}

func batchFail(code string, err error) (BatchResult, *BatchFailure) {
	return BatchResult{}, &BatchFailure{Code: code, Err: err}
}
func cancelled(ctx context.Context) *BatchFailure {
	if err := ctx.Err(); err != nil {
		return &BatchFailure{Code: BatchFailureCancelled, Err: err}
	}
	return nil
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
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
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
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
	}
	if a.session.identity() != request.Session {
		return batchFail(BatchFailureSessionDrift, errors.New("generation drift before batch"))
	}
	workspace, err := a.session.workspace()
	if err != nil {
		return batchFail(BatchFailureSessionDrift, err)
	}
	if err = reconcileBatchSeeds(request, workspace); err != nil {
		return batchFail(BatchFailureInvalidInput, err)
	}
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
	}
	manifest := request.AcquisitionManifest(a.limits)
	input, err := json.Marshal(acquisitionops.Input{SessionID: request.Session.SessionID, Generation: request.Session.Generation, SeedManifest: manifest, OutputVersion: graphprovenance.VersionV5})
	if err != nil {
		return batchFail(BatchFailureInvalidInput, err)
	}
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
	}
	result, failed := a.session.execute(ctx, operation.Request{Name: acquisitionops.SliceV3, RequestID: fmt.Sprintf("census:%s:%06d:%s", request.CensusID, request.Ordinal, request.BatchID), Input: input, RetainedSeedSpec: bytes.Clone(request.CanonicalSeedsV2)})
	if failed != nil {
		return batchFail(BatchFailureAcquisition, operation.NormalizeFailure(failed))
	}
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
	}
	if err := admitCompleteBatch(ctx, result.Artifact, request, a.beforeNativeAdmission); err != nil {
		if f := cancelled(ctx); f != nil {
			return BatchResult{}, f
		}
		return batchFail(BatchFailureAdmission, err)
	}
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
	}
	if a.session.identity() != request.Session {
		return batchFail(BatchFailureSessionDrift, errors.New("generation drift after admission"))
	}
	if f := cancelled(ctx); f != nil {
		return BatchResult{}, f
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

func reconcileBatchSeeds(request BatchRequest, workspace string) error {
	seenOrdinal, seenIdentity, seenCoordinate, seenCanonical := map[int]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, target := range request.Targets {
		if target.CensusOrdinal < 0 || seenOrdinal[target.CensusOrdinal] || target.SymbolIdentity == "" || seenIdentity[target.SymbolIdentity] {
			return fmt.Errorf("target %d duplicate ordinal or identity", i)
		}
		coordinate := fmt.Sprintf("%s\x00%d\x00%d", target.URI, target.Position().Line, target.Position().Character)
		if seenCoordinate[coordinate] || seenCanonical[string(target.CanonicalSeedV2)] {
			return fmt.Errorf("target %d duplicate coordinate or canonical bytes", i)
		}
		if err := reconcileSeed(target, workspace); err != nil {
			return fmt.Errorf("target %d: %w", i, err)
		}
		seenOrdinal[target.CensusOrdinal], seenIdentity[target.SymbolIdentity], seenCoordinate[coordinate], seenCanonical[string(target.CanonicalSeedV2)] = true, true, true, true
	}
	provided, err := seedformat.Decode(request.CanonicalSeedsV2, workspace)
	if err != nil {
		return fmt.Errorf("invalid provided canonical Seeds V2: %w", err)
	}
	canonical, err := seedformat.EncodeCanonical(provided, workspace)
	if err != nil || !bytes.Equal(canonical, request.CanonicalSeedsV2) {
		return errors.New("provided Seeds V2 is not canonical")
	}
	recomputed, err := combineSeeds(request.Targets, workspace)
	if err != nil || !bytes.Equal(recomputed, request.CanonicalSeedsV2) {
		return errors.New("provided Seeds V2 does not exactly match ordered targets")
	}
	return nil
}

func admitCompleteBatch(ctx context.Context, raw []byte, request BatchRequest, beforeNative func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := admit(raw, request); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var envelope graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(envelope.GraphV5) > base64.StdEncoding.EncodedLen(graph.MaxNativeV3Bytes) {
		return errors.New("native graph encoded byte limit")
	}
	native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		return err
	}
	if err := graph.PreflightNativeV3(native); err != nil {
		return err
	}
	if beforeNative != nil {
		beforeNative()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := graph.DecodeNativeV3(native)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !result.Summary.Complete || result.Summary.Truncated {
		return errors.New("incomplete or truncated traversal")
	}
	if len(result.Seeds) != len(request.Targets) {
		return errors.New("seed result cardinality mismatch")
	}
	manifest := request.AcquisitionManifest(acquisitionops.Limits{})
	for i, seed := range result.Seeds {
		want := manifest.Root.ID
		if i > 0 {
			want = manifest.RequiredTargets[i-1].ID
		}
		if seed.Label != want || seed.Failure != nil || len(seed.ReachedNodeIDs) == 0 {
			return fmt.Errorf("seed %d traversal incomplete", i)
		}
	}
	if len(result.SiblingCandidates) != 0 || len(result.DispatchRelationships) != 0 {
		return errors.New("non-CALLS inference present")
	}
	return ctx.Err()
}
