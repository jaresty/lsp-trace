package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/operation"
)

const (
	censusBatchFailureInvalidInput = "CENSUS_BATCH_INVALID_INPUT"
	censusBatchFailureSessionDrift = "CENSUS_BATCH_SESSION_DRIFT"
	censusBatchFailureCancelled    = "CENSUS_BATCH_CANCELLED"
	censusBatchFailureAcquisition  = "CENSUS_BATCH_ACQUISITION_FAILED"
	censusBatchFailureAdmission    = "CENSUS_BATCH_ADMISSION_FAILED"
)

type censusBatchResult struct {
	Session                censusacquisition.SessionIdentity
	CensusID               string
	BatchID                string
	Ordinal                int
	CanonicalSeedsV2SHA256 string
	Raw                    []byte
}

type censusBatchFailure struct {
	Code string
	Err  error
}

func (f *censusBatchFailure) Error() string {
	if f == nil {
		return ""
	}
	if f.Err != nil {
		return fmt.Sprintf("%s: %v", f.Code, f.Err)
	}
	return f.Code
}
func (f *censusBatchFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

// censusBatchSession is a package-private test seam. Production construction
// binds directly to the concrete initialized runtime owned by package main.
type censusBatchSession interface {
	identity() censusacquisition.SessionIdentity
	workspace() (string, error)
	execute(context.Context, acquisitionorchestration.PlannedBatchRequest) (acquisitionorchestration.PlannedBatchResult, *operation.Failure)
}

type censusBatchAcquirer struct {
	session               censusBatchSession
	limits                acquisitionops.Limits
	beforeNativeAdmission func()
}

func newCensusBatchAcquirer(session censusBatchSession, limits acquisitionops.Limits) *censusBatchAcquirer {
	return &censusBatchAcquirer{session: session, limits: limits}
}

func newInitializedCensusBatchAcquirer(runtime *initializedAcquisitionRuntime, limits acquisitionops.Limits) *censusBatchAcquirer {
	return newCensusBatchAcquirer(&initializedCensusBatchSession{runtime: runtime}, limits)
}

type initializedCensusBatchSession struct {
	runtime *initializedAcquisitionRuntime
}

func (s *initializedCensusBatchSession) identity() censusacquisition.SessionIdentity {
	if s == nil || s.runtime == nil {
		return censusacquisition.SessionIdentity{}
	}
	found := censusacquisition.SessionIdentity{}
	for _, r := range s.runtime.Records() {
		if r.SessionID == s.runtime.SessionID() && r.Generation == s.runtime.Generation() {
			if found.Generation != 0 {
				return censusacquisition.SessionIdentity{}
			}
			found = censusacquisition.SessionIdentity{SessionID: r.SessionID, Generation: r.Generation}
		}
	}
	return found
}
func (s *initializedCensusBatchSession) workspace() (string, error) {
	if s == nil || s.runtime == nil {
		return "", errors.New("managed runtime required")
	}
	workspace := ""
	for _, r := range s.runtime.Records() {
		if r.SessionID == s.runtime.SessionID() && r.Generation == s.runtime.Generation() {
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
func (s *initializedCensusBatchSession) execute(ctx context.Context, request acquisitionorchestration.PlannedBatchRequest) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
	if s == nil || s.runtime == nil || s.runtime.privateAcquisitionRuntime == nil {
		return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("initialized census runtime unavailable")}
	}
	return acquisitionorchestration.ExecutePlannedBatch(ctx, s.runtime.privateAcquisitionRuntime, request)
}

func batchFail(code string, err error) (censusBatchResult, *censusBatchFailure) {
	return censusBatchResult{}, &censusBatchFailure{Code: code, Err: err}
}
func cancelled(ctx context.Context) *censusBatchFailure {
	if err := ctx.Err(); err != nil {
		return &censusBatchFailure{Code: censusBatchFailureCancelled, Err: err}
	}
	return nil
}

func (a *censusBatchAcquirer) AcquireV5(ctx context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	result, failure := a.acquireBatch(ctx, request)
	if failure != nil {
		return censusacquisition.AcquiredV5{}, failure
	}
	return censusacquisition.AcquiredV5{Session: result.Session, Raw: bytes.Clone(result.Raw)}, nil
}

func (a *censusBatchAcquirer) acquireBatch(ctx context.Context, request censusacquisition.BatchRequest) (censusBatchResult, *censusBatchFailure) {
	request = cloneBatchRequest(request)
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	if a == nil || a.session == nil {
		return batchFail(censusBatchFailureInvalidInput, errors.New("initialized session required"))
	}
	if err := request.Session.Validate(); err != nil {
		return batchFail(censusBatchFailureInvalidInput, err)
	}
	if request.CensusID == "" || request.BatchID == "" || request.Ordinal < 0 || len(request.Targets) < 1 || len(request.Targets) > 63 || len(request.CanonicalSeedsV2) == 0 {
		return batchFail(censusBatchFailureInvalidInput, errors.New("complete bounded batch identity required"))
	}
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	if a.session.identity() != request.Session {
		return batchFail(censusBatchFailureSessionDrift, errors.New("generation drift before batch"))
	}
	workspace, err := a.session.workspace()
	if err != nil {
		return batchFail(censusBatchFailureSessionDrift, err)
	}
	if err = censusacquisition.ValidateBatchSeeds(request, workspace); err != nil {
		return batchFail(censusBatchFailureInvalidInput, err)
	}
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	manifest := request.AcquisitionManifest(a.limits)
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	result, failed := a.session.execute(ctx, acquisitionorchestration.PlannedBatchRequest{SessionID: request.Session.SessionID, Generation: request.Session.Generation, RequestID: fmt.Sprintf("census:%s:%06d:%s", request.CensusID, request.Ordinal, request.BatchID), Manifest: manifest, CanonicalSeedsV2: bytes.Clone(request.CanonicalSeedsV2)})
	if failed != nil {
		return batchFail(censusBatchFailureAcquisition, operation.NormalizeFailure(failed))
	}
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	if err := admitCompleteBatch(ctx, result.RawV5, request, a.beforeNativeAdmission); err != nil {
		if f := cancelled(ctx); f != nil {
			return censusBatchResult{}, f
		}
		return batchFail(censusBatchFailureAdmission, err)
	}
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	if a.session.identity() != request.Session {
		return batchFail(censusBatchFailureSessionDrift, errors.New("generation drift after admission"))
	}
	if f := cancelled(ctx); f != nil {
		return censusBatchResult{}, f
	}
	return censusBatchResult{Session: request.Session, CensusID: request.CensusID, BatchID: request.BatchID, Ordinal: request.Ordinal, CanonicalSeedsV2SHA256: rawDigest(request.CanonicalSeedsV2), Raw: bytes.Clone(result.RawV5)}, nil
}

func rawDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func cloneTarget(in censusacquisition.PreparedTarget) censusacquisition.PreparedTarget {
	in.CanonicalSeedV2 = bytes.Clone(in.CanonicalSeedV2)
	return in
}

func cloneBatchRequest(in censusacquisition.BatchRequest) censusacquisition.BatchRequest {
	in.CanonicalSeedsV2 = bytes.Clone(in.CanonicalSeedsV2)
	in.Targets = append([]censusacquisition.PreparedTarget(nil), in.Targets...)
	for i := range in.Targets {
		in.Targets[i] = cloneTarget(in.Targets[i])
	}
	return in
}

func admitCompleteBatch(ctx context.Context, raw []byte, request censusacquisition.BatchRequest, beforeNative func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := censusacquisition.ValidateBatchArtifact(raw, request); err != nil {
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
