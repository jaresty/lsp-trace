package main

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type censusFailureStage string
type censusFailureCode string

const (
	censusStageConfig      censusFailureStage = "config"
	censusStageAcquisition censusFailureStage = "acquisition"

	censusCodeInvalidConfig     censusFailureCode = "INVALID_CONFIG"
	censusCodeAcquisitionFailed censusFailureCode = "ACQUISITION_FAILED"
)

type censusAdmissionFailure struct {
	stage censusFailureStage
	code  censusFailureCode
}

type censusAdmittedSession struct {
	sessionID        string
	generation       uint64
	workspace        string
	positionEncoding string
}

type censusExecutor struct {
	runtime *hostSelectorRuntime
}

func newCensusExecutor(runtime *hostSelectorRuntime) *censusExecutor {
	return &censusExecutor{runtime: runtime}
}

func (e *censusExecutor) execute(ctx context.Context, raw []byte) (censusAdmittedSession, *censusAdmissionFailure) {
	request, err := mcpcontract.DecodeFutureCensusRequestV1(raw)
	if err != nil {
		return censusAdmittedSession{}, censusConfigFailure()
	}
	if ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure()
	}
	if e == nil || e.runtime == nil || e.runtime.Manager == nil {
		return censusAdmittedSession{}, censusConfigFailure()
	}

	requestedID, idOK := request["session_id"].(string)
	generationNumber, generationOK := request["generation"].(json.Number)
	generation, generationErr := generationNumber.Int64()
	if !idOK || !generationOK || generationErr != nil || generation <= 0 {
		return censusAdmittedSession{}, censusConfigFailure()
	}

	sessionID := requestedID
	if canonical, ok := e.runtime.aliases[requestedID]; ok {
		sessionID = canonical
	}
	var match *sessionruntime.Record
	for _, record := range e.runtime.Records() {
		if record.SessionID != sessionID || record.Generation != uint64(generation) || record.State != session.Ready {
			continue
		}
		if match != nil {
			return censusAdmittedSession{}, censusAcquisitionFailure()
		}
		copy := record
		match = &copy
	}
	if match == nil || ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure()
	}
	workspace := match.Profile.Workspace().String()
	if workspace == "" || workspace != match.Routing.WorkspaceRoot {
		return censusAdmittedSession{}, censusConfigFailure()
	}

	metadata, metadataFailure := e.runtime.Metadata(match.SessionID, match.Generation)
	if metadataFailure != "" {
		return censusAdmittedSession{}, censusAcquisitionFailure()
	}
	if !metadata.DocumentSymbolSupport || !metadata.CallHierarchySupport || !supportedCensusEncoding(metadata.PositionEncoding) {
		return censusAdmittedSession{}, censusConfigFailure()
	}
	if ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure()
	}
	return censusAdmittedSession{
		sessionID:        match.SessionID,
		generation:       match.Generation,
		workspace:        workspace,
		positionEncoding: metadata.PositionEncoding,
	}, nil
}

func supportedCensusEncoding(encoding string) bool {
	switch encoding {
	case "utf-8", "utf-16", "utf-32":
		return true
	default:
		return false
	}
}

func censusConfigFailure() *censusAdmissionFailure {
	return &censusAdmissionFailure{stage: censusStageConfig, code: censusCodeInvalidConfig}
}

func censusAcquisitionFailure() *censusAdmissionFailure {
	return &censusAdmissionFailure{stage: censusStageAcquisition, code: censusCodeAcquisitionFailed}
}
