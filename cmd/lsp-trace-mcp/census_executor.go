package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type censusFailureStage string
type censusFailureCode string

const (
	censusStageConfig      censusFailureStage = "config"
	censusStageAcquisition censusFailureStage = "acquisition"
	censusStageDiscovery   censusFailureStage = "discovery"
	censusStagePublication censusFailureStage = "publication"

	censusCodeInvalidConfig     censusFailureCode = "INVALID_CONFIG"
	censusCodeAcquisitionFailed censusFailureCode = "ACQUISITION_FAILED"
	censusCodeDiscoveryFailed   censusFailureCode = "DISCOVERY_FAILED"
	censusCodePublicationFailed censusFailureCode = "PUBLICATION_FAILED"
)

type censusFailureReason string

const (
	reasonNone                         censusFailureReason = ""
	reasonRequestUndecodable           censusFailureReason = "request-undecodable"
	reasonRuntimeUnprovisioned         censusFailureReason = "runtime-unprovisioned"
	reasonSessionSelectorInvalid       censusFailureReason = "session-selector-invalid"
	reasonNoReadySession               censusFailureReason = "no-ready-session"
	reasonCancelled                    censusFailureReason = "request-cancelled"
	reasonWorkspaceNotCanonical        censusFailureReason = "workspace-not-canonical"
	reasonSessionMetadataMissing       censusFailureReason = "session-metadata-unavailable"
	reasonCapabilitiesUnsupported      censusFailureReason = "language-server-capabilities-unsupported"
	reasonPublicationRootRequired      censusFailureReason = "publication-root-required"
	reasonDiscoveryFailed              censusFailureReason = "workspace-discovery-failed"
	reasonDiscoveryIncomplete          censusFailureReason = "discovery-accounting-incomplete"
	reasonAcquisitionFailed            censusFailureReason = "symbol-acquisition-failed"
	reasonPublicationNoReceipt         censusFailureReason = "capture-set-publication-failed"
	reasonPublicationRootOpen          censusFailureReason = "ROOT_OPEN"
	reasonPublicationPrivateValidation censusFailureReason = "PRIVATE_VALIDATION"
	reasonPublicationEncode            censusFailureReason = "ENCODE"
	reasonPublicationCanonicalize      censusFailureReason = "CANONICALIZE"
	reasonPublicationTempWrite         censusFailureReason = "TEMP_WRITE"
	reasonPublicationFsync             censusFailureReason = "FSYNC"
	reasonPublicationNoReplace         censusFailureReason = "NO_REPLACE"
	reasonPublicationVerify            censusFailureReason = "VERIFY"
	reasonPublicationReceipt           censusFailureReason = "RECEIPT"
	reasonPublicationCleanup           censusFailureReason = "CLEANUP"
	reasonPublicationInternal          censusFailureReason = "INTERNAL"
)

// censusFailureDetail maps a closed reason token to a static, path-free,
// request-data-free human explanation safe to surface in the public
// diagnostic. Returning a fixed catalog string (never interpolated request
// data) preserves the failure-serialization privacy invariant.
func censusFailureDetail(reason censusFailureReason) string {
	switch reason {
	case reasonRequestUndecodable:
		return "request could not be decoded against the census schema"
	case reasonRuntimeUnprovisioned:
		return "census runtime is not provisioned on this server"
	case reasonSessionSelectorInvalid:
		return "session_id must be a non-empty string and generation a positive integer"
	case reasonNoReadySession:
		return "no READY language-server session matches the requested session_id and generation"
	case reasonCancelled:
		return "request was cancelled before completion"
	case reasonWorkspaceNotCanonical:
		return "the matched session workspace is not a cleaned absolute path equal to its routing root"
	case reasonSessionMetadataMissing:
		return "language-server session metadata was unavailable"
	case reasonCapabilitiesUnsupported:
		return "the language server does not advertise the document-symbol and call-hierarchy capabilities census requires"
	case reasonPublicationRootRequired:
		return "census requires a publication root; start the server with --publication-root pointing at an owner-only (0700) directory"
	case reasonDiscoveryFailed:
		return "workspace symbol discovery failed"
	case reasonDiscoveryIncomplete:
		return "discovery accounting was incomplete; raise max_nodes or narrow sources/depth"
	case reasonAcquisitionFailed:
		return "symbol acquisition over the session failed"
	case reasonPublicationNoReceipt:
		return "the capture set could not be published (verify the publication root is an owner-only directory)"
	case reasonPublicationRootOpen, reasonPublicationPrivateValidation, reasonPublicationEncode, reasonPublicationCanonicalize, reasonPublicationTempWrite, reasonPublicationFsync, reasonPublicationNoReplace, reasonPublicationVerify, reasonPublicationReceipt, reasonPublicationCleanup, reasonPublicationInternal:
		return "capture-set publication failed at the reported publication stage"
	default:
		return ""
	}
}

type censusAdmissionFailure struct {
	stage censusFailureStage
	code  censusFailureCode
	err   error
}

type censusAdmittedSession struct {
	sessionID        string
	generation       uint64
	workspace        string
	positionEncoding string
	options          censusRuntimeConfig
	requestReceipt   censusrequest.Receipt
}

type censusExecutor struct {
	runtime *hostSelectorRuntime
}

func newCensusExecutor(runtime *hostSelectorRuntime) *censusExecutor {
	return &censusExecutor{runtime: runtime}
}

func (e *censusExecutor) execute(ctx context.Context, raw []byte) (censusAdmittedSession, *censusAdmissionFailure, censusFailureReason) {
	batchTargets := censusBatchTargetsFromContext(ctx)
	decodeRaw := raw
	if batchTargets != 0 {
		var internal map[string]json.RawMessage
		if json.Unmarshal(raw, &internal) != nil {
			return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
		}
		delete(internal, "batch_targets")
		var err error
		decodeRaw, err = json.Marshal(internal)
		if err != nil {
			return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
		}
	}
	request, err := mcpcontract.DecodeFutureCensusRequestV1(decodeRaw)
	if err != nil {
		return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
	}
	receipt, err := censusrequest.Decode(decodeRaw)
	if err != nil {
		return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
	}
	options := censusRuntimeConfigFromDecoded(request)
	if batchTargets != 0 {
		if batchTargets < 1 || batchTargets > 63 {
			return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
		}
		semantic := receipt.Semantic
		semantic.BatchTargets = uint64(batchTargets)
		receipt, err = censusrequest.New(receipt.Refresh, semantic)
		if err != nil {
			return censusAdmittedSession{}, censusConfigFailure(), reasonRequestUndecodable
		}
		options.maxBatchTargets = uint64(batchTargets)
	}
	if ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure(), reasonCancelled
	}
	if e == nil || e.runtime == nil || e.runtime.Manager == nil {
		return censusAdmittedSession{}, censusConfigFailure(), reasonRuntimeUnprovisioned
	}

	requestedID, idOK := request["session_id"].(string)
	generationNumber, generationOK := request["generation"].(json.Number)
	generation, generationErr := generationNumber.Int64()
	if !idOK || !generationOK || generationErr != nil || generation <= 0 {
		return censusAdmittedSession{}, censusConfigFailure(), reasonSessionSelectorInvalid
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
			return censusAdmittedSession{}, censusAcquisitionFailure(), reasonNoReadySession
		}
		copy := record
		match = &copy
	}
	if match == nil {
		return censusAdmittedSession{}, censusAcquisitionFailure(), reasonNoReadySession
	}
	if ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure(), reasonCancelled
	}
	workspace := match.Profile.Workspace().String()
	if strings.TrimSpace(workspace) == "" || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace || workspace != match.Routing.WorkspaceRoot {
		return censusAdmittedSession{}, censusConfigFailure(), reasonWorkspaceNotCanonical
	}

	metadata, metadataFailure := e.runtime.Metadata(match.SessionID, match.Generation)
	if metadataFailure != "" {
		return censusAdmittedSession{}, censusAcquisitionFailure(), reasonSessionMetadataMissing
	}
	if !metadata.DocumentSymbolSupport || !metadata.CallHierarchySupport || !supportedCensusEncoding(metadata.PositionEncoding) {
		return censusAdmittedSession{}, censusConfigFailure(), reasonCapabilitiesUnsupported
	}
	if ctx.Err() != nil {
		return censusAdmittedSession{}, censusAcquisitionFailure(), reasonCancelled
	}
	return censusAdmittedSession{
		sessionID:        match.SessionID,
		generation:       match.Generation,
		workspace:        workspace,
		positionEncoding: metadata.PositionEncoding,
		options:          cloneCensusRuntimeConfig(options),
		requestReceipt:   receipt,
	}, nil, reasonNone
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

func censusDiscoveryFailure() *censusAdmissionFailure {
	return &censusAdmissionFailure{stage: censusStageDiscovery, code: censusCodeDiscoveryFailed}
}

func censusPublicationFailure() *censusAdmissionFailure {
	return &censusAdmissionFailure{stage: censusStagePublication, code: censusCodePublicationFailed}
}
