package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// CustodyStage identifies one transport-neutral stage in the custody lifecycle.
type CustodyStage string

const (
	StageDiscovery   CustodyStage = "discovery"
	StageReceipt     CustodyStage = "receipt_canonicalization"
	StageManifest    CustodyStage = "manifest_assembly"
	StageSnapshot    CustodyStage = "snapshot_binding"
	StageAdmission   CustodyStage = "trust_admission"
	StagePublication CustodyStage = "publication"
)

// Availability reports whether a stage implementation is present.
type Availability string

const (
	AvailabilityAvailable   Availability = "available"
	AvailabilityUnavailable Availability = "unavailable"
)

// LifecycleState reports the terminal execution state of a custody stage.
type LifecycleState string

const (
	LifecyclePending   LifecycleState = "pending"
	LifecycleSucceeded LifecycleState = "succeeded"
	LifecycleFailed    LifecycleState = "failed"
	LifecycleSkipped   LifecycleState = "skipped"
)

// CustodyRequest is the canonical request accepted by the shared operation.
type CustodyRequest struct {
	OperationID string          `json:"operation_id"`
	Input       json.RawMessage `json:"input"`
}

// StageResult is the transport-neutral output of one custody stage.
type StageResult struct {
	Artifact json.RawMessage `json:"artifact,omitempty"`
	Admitted *bool           `json:"admitted,omitempty"`
}

// StageRecord makes each stage's availability and terminal state explicit.
type StageRecord struct {
	Stage        CustodyStage   `json:"stage"`
	Availability Availability   `json:"availability"`
	State        LifecycleState `json:"state"`
}

// CustodyResponse is the canonical response returned by the shared operation.
type CustodyResponse struct {
	OperationID   string                       `json:"operation_id"`
	LogicalDigest string                       `json:"logical_digest,omitempty"`
	Lifecycle     []StageRecord                `json:"lifecycle"`
	Results       map[CustodyStage]StageResult `json:"results,omitempty"`
}

// CustodyStageHandler implements one sibling-owned custody stage.
type CustodyStageHandler func(context.Context, CustodyRequest, CustodyResponse) (StageResult, error)

const (
	CustodyFailureInvalidRequest    = "CUSTODY_INVALID_REQUEST"
	CustodyFailureStageUnavailable  = "CUSTODY_STAGE_UNAVAILABLE"
	CustodyFailureStageFailed       = "CUSTODY_STAGE_FAILED"
	CustodyFailureAdmissionRejected = "CUSTODY_ADMISSION_REJECTED"
	CustodyFailureDigestFailed      = "CUSTODY_DIGEST_FAILED"
	CustodyFailurePublicationFailed = "CUSTODY_PUBLICATION_FAILED"
)

// CustodyFailure is a stable stage-aware operation error.
type CustodyFailure struct {
	Code  string
	Stage CustodyStage
	Err   error
}

func (f *CustodyFailure) Error() string { return f.Code }
func (f *CustodyFailure) Unwrap() error { return f.Err }

// CustodyOperation orchestrates injected sibling-owned stages.
type CustodyOperation struct {
	Handlers map[CustodyStage]CustodyStageHandler
}

var orderedCustodyStages = [...]CustodyStage{
	StageDiscovery,
	StageReceipt,
	StageManifest,
	StageSnapshot,
	StageAdmission,
	StagePublication,
}

// ExecuteCustody runs one shared custody operation.
func (o *CustodyOperation) ExecuteCustody(ctx context.Context, request CustodyRequest) (CustodyResponse, *CustodyFailure) {
	response := CustodyResponse{
		OperationID: request.OperationID,
		Lifecycle:   make([]StageRecord, len(orderedCustodyStages)),
		Results:     make(map[CustodyStage]StageResult, len(orderedCustodyStages)),
	}
	for i, stage := range orderedCustodyStages {
		availability := AvailabilityUnavailable
		if o != nil && o.Handlers[stage] != nil {
			availability = AvailabilityAvailable
		}
		response.Lifecycle[i] = StageRecord{Stage: stage, Availability: availability, State: LifecyclePending}
	}

	if request.OperationID == "" || len(request.Input) == 0 || !json.Valid(request.Input) {
		return failCustody(response, 0, CustodyFailureInvalidRequest, errors.New("operation_id and valid input are required"))
	}
	request.Input = append(json.RawMessage(nil), request.Input...)

	for i, stage := range orderedCustodyStages {
		handler := CustodyStageHandler(nil)
		if o != nil {
			handler = o.Handlers[stage]
		}
		if handler == nil {
			return failCustody(response, i, CustodyFailureStageUnavailable, errors.New("stage handler unavailable"))
		}

		stageRequest := request
		stageRequest.Input = append(json.RawMessage(nil), request.Input...)
		result, err := handler(ctx, stageRequest, response)
		if err != nil {
			code := CustodyFailureStageFailed
			if stage == StagePublication {
				code = CustodyFailurePublicationFailed
			}
			return failCustody(response, i, code, err)
		}
		if len(result.Artifact) > 0 && !json.Valid(result.Artifact) {
			return failCustody(response, i, CustodyFailureDigestFailed, errors.New("stage artifact is not valid JSON"))
		}
		result.Artifact = append(json.RawMessage(nil), result.Artifact...)
		response.Results[stage] = result
		if stage == StageAdmission && (result.Admitted == nil || !*result.Admitted) {
			return failCustody(response, i, CustodyFailureAdmissionRejected, errors.New("trust admission rejected"))
		}
		response.Lifecycle[i].State = LifecycleSucceeded
	}

	digest, err := custodyLogicalDigest(request.Input, response)
	if err != nil {
		return failCustody(response, len(orderedCustodyStages)-1, CustodyFailureDigestFailed, err)
	}
	response.LogicalDigest = digest
	return response, nil
}

func failCustody(response CustodyResponse, failedIndex int, code string, cause error) (CustodyResponse, *CustodyFailure) {
	response.Lifecycle[failedIndex].State = LifecycleFailed
	for i := failedIndex + 1; i < len(response.Lifecycle); i++ {
		response.Lifecycle[i].State = LifecycleSkipped
	}
	return response, &CustodyFailure{Code: code, Stage: response.Lifecycle[failedIndex].Stage, Err: cause}
}

func custodyLogicalDigest(input json.RawMessage, response CustodyResponse) (string, error) {
	var logicalInput any
	if err := json.Unmarshal(input, &logicalInput); err != nil {
		return "", fmt.Errorf("canonicalize request input: %w", err)
	}
	type logicalResponse struct {
		Input     any                          `json:"input"`
		Lifecycle []StageRecord                `json:"lifecycle"`
		Results   map[CustodyStage]StageResult `json:"results"`
	}
	canonical, err := json.Marshal(logicalResponse{Input: logicalInput, Lifecycle: response.Lifecycle, Results: response.Results})
	if err != nil {
		return "", fmt.Errorf("canonicalize logical response: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
