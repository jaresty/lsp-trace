package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
)

const runtimeTracePathEnv = "LSP_TRACE_GO_RUNTIME_TRACE_PATH"
const censusMappingDiagnosticEnv = "LSP_TRACE_CENSUS_MAPPING_DIAGNOSTICS"

var censusAcquisitionBranchTag = map[string]bool{
	"BEFORE_ACQUIRE": true, "UNMAPPED_OPERATION_CODE": true, "RECORDER_ABSENT": true,
	"RECORDER_REJECTED": true, "RECORD_ACCEPTED": true,
	"CORE_ERROR_INPUT": true, "CORE_ERROR_DISCOVERY": true, "CORE_ERROR_PLANNING": true,
	"CORE_ERROR_BATCH_ACQUIRE": true, "CORE_ERROR_BATCH_ADMISSION": true, "CORE_ERROR_MANIFEST": true,
	"BATCH_RESULT_INCOMPLETE": true, "BATCH_RESULT_IDENTITY_DRIFT": true,
}

func validCensusAcquisitionBranchTag(tag string) bool {
	return censusAcquisitionBranchTag[tag]
}

func censusAcquisitionBranchTrace(_ context.Context, tag string) {
	if os.Getenv(runtimeTracePathEnv) == "" || !validCensusAcquisitionBranchTag(tag) {
		return
	}
	line := "CENSUS_ACQUISITION_BRANCH tag=" + tag
	runtimeTraceState.Lock()
	if runtimeTraceState.recorder != nil && runtimeTraceState.path != "" {
		runtimeTraceState.markers = append(runtimeTraceState.markers, line)
	}
	runtimeTraceState.Unlock()
}

var censusHandoffBranch = map[programccompose.BranchCode]bool{
	programccompose.BranchUnknown: true, programccompose.BranchInputCount: true, programccompose.BranchInputBytes: true,
	programccompose.BranchInputIdentity: true, programccompose.BranchInputMetadata: true, programccompose.BranchInputIdentityConflict: true,
	programccompose.BranchInputProvenance: true, programccompose.BranchInputEnvelopeDecode: true, programccompose.BranchInputGraphBase64: true,
	programccompose.BranchInputGraphDecode: true, programccompose.BranchSourceRecords: true, programccompose.BranchCompatibility: true,
	programccompose.BranchSummaryDecode: true, programccompose.BranchNodeConflict: true, programccompose.BranchOccurrenceLimit: true,
	programccompose.BranchEdgeConflict: true, programccompose.BranchResourceLimit: true,
}

type diagnosticSinkUnavailable struct {
	Sink  string
	cause error
}

func (e *diagnosticSinkUnavailable) Error() string { return "diagnostic sink unavailable: " + e.Sink }
func (e *diagnosticSinkUnavailable) Unwrap() error { return e.cause }

var runtimeTraceState struct {
	sync.Mutex
	recorder *trace.FlightRecorder
	path     string
	markers  []string
}

var descriptorTraceOutput io.Writer = os.Stderr

var descriptorTraceStage = map[string]bool{
	"ENTER": true, "DESCRIPTOR_CANONICAL": true, "CHECKPOINT_OBJECT": true,
	"DESCRIPTOR_VALIDATION": true, "BYTE_CEILING": true, "SELECTOR_CONSTRUCTION": true,
	"ROOT_NAMESPACE": true, "TARGET": true, "TEMP_CREATE": true, "TEMP_SYNC_CLOSE": true,
	"RENAME": true, "NO_REPLACE": true, "REREAD_EQUAL": true, "TARGET_EQUAL": true, "EXIT": true,
}

var descriptorTraceReason = map[string]bool{
	"START": true, "VALID": true, "INVALID": true, "ABSENT_OR_INVALID": true,
	"EXISTS_INVALID": true, "EXISTS_VALID": true, "ENCODING_FAILED": true,
	"EXCEEDED": true, "WITHIN": true, "PRIVATE_DIRECTORY": true, "UNSAFE": true,
	"EXISTS": true, "ABSENT": true, "STAT_FAILED": true, "FAILED": true,
	"CREATED": true, "SYNCED_CLOSED": true, "NOT_USED": true, "TARGET_EXISTS": true,
	"HARD_LINK_FAILED": true, "HARD_LINK_INSTALLED": true, "REREAD_FAILED": true,
	"NOT_EQUAL": true, "EQUAL": true, "PUBLISHED": true, "SELECTOR_REJECTED": true,
	"INSTALL_FAILED": true, "STAGE_FAILED": true, "REREAD_MISMATCH": true,
	"CONTEXT_DONE": true, "CANONICALIZATION_FAILED": true, "BYTE_CEILING": true,
	"SELECTOR_CONSTRUCTION_FAILED": true, "UNREADABLE": true, "EXISTING_UNREADABLE": true,
	"IMMUTABLE_COLLISION": true, "IDEMPOTENT": true, "RECEIPT_INVALID": true,
}

var basePublicationTraceStage = map[string]bool{
	"ROOT_SOURCE": true, "ROOT_OPEN": true, "OPEN_VALIDATE": true, "CANDIDATE": true,
	"TARGET": true, "TEMP": true, "WRITE_FSYNC": true, "HARDLINK": true,
	"TARGET_EQUAL": true, "RECEIPT": true, "CLEANUP": true,
}

var basePublicationTraceResult = map[string]bool{
	"CLI_ARGUMENT": true, "ABSENT": true, "OPENED": true, "OPEN_FAILED": true,
	"INVALID_REQUEST": true, "PRIVATE_INVALID": true, "PRIVATE_VALID": true,
	"CANONICALIZATION_FAILED": true, "CANONICAL": true, "EXISTS": true,
	"STAT_FAILED": true, "CREATED": true, "FAILED": true, "COMPLETE": true,
	"UNSUPPORTED": true, "INSTALLED": true, "NOT_EQUAL": true, "EQUAL": true,
}

var continuationTraceBoundary = map[string]bool{
	"STARTUP": true, "BINDING": true, "HOST_FRESH": true, "HOST_RESUME": true,
	"CHECKPOINT": true, "DESCRIPTOR": true, "MAPPING": true,
}
var continuationTraceStage = map[string]bool{"ENTER": true, "RETURN": true}
var continuationTraceResult = map[string]bool{"NONE": true, "OK": true, "ERROR": true}
var continuationTraceClass = map[string]bool{"NONE": true, "TYPED": true, "UNTYPED": true, "MAPPING_ERROR": true}

func validCompletionSubcause(s completionSubcause) bool {
	switch s {
	case completionResultMissing, completionDiagnosticPresent, custodyMissing, custodyCloneProjection, custodyCloneManifest, handoffProjection, handoffWorkspace, handoffPositionEncoding, handoffResult, handoffIdentityReconciliation, handoffPublicationReconciliation, handoffManifest, handoffCompose, handoffIdentity, completionUnknown:
		return true
	default:
		return false
	}
}

var continuationPreconditionCause = map[string]bool{
	"HANDOFF_BUILD": true, "STORE_OPEN": true, "CONTRACT_OBJECT": true,
	"HANDOFF_OBJECT": true, "CENSUS_COMMIT_OBJECT": true, "CHECKPOINT_PERSIST": true,
	"INPUT_VALIDATION": true, "OBJECT_IDENTITY": true, "OBJECT_SIZE": true,
	"UNKNOWN_PRE_PUBLICATION": true,
}

type descriptorTraceCorrelationKey struct{}

type descriptorTraceCorrelation struct {
	Operation  string
	Generation string
}

func withDescriptorTraceCorrelation(ctx context.Context, generation any) context.Context {
	label := "OTHER"
	switch fmt.Sprint(generation) {
	case "1":
		label = "G1"
	case "2":
		label = "G2"
	}
	return context.WithValue(ctx, descriptorTraceCorrelationKey{}, descriptorTraceCorrelation{Operation: "CENSUS", Generation: label})
}

func censusHandoffBranchTrace(ctx context.Context, branch programccompose.BranchCode) {
	if os.Getenv(runtimeTracePathEnv) == "" || !censusHandoffBranch[branch] {
		return
	}
	line := fmt.Sprintf("CENSUS_HANDOFF_BRANCH branch=%s", branch)
	trace.Log(ctx, "census_continuation", line)
	writeDescriptorTrace(line)
}

func censusHandoffStageTrace(ctx context.Context, stage censusprogramc.Stage) {
	if os.Getenv(runtimeTracePathEnv) == "" {
		return
	}
	switch stage {
	case censusprogramc.StagePublication, censusprogramc.StageReconciliation, censusprogramc.StageComposition,
		censusprogramc.StageAdmission, censusprogramc.StageComputation, censusprogramc.StageRepresentative:
	default:
		stage = "UNKNOWN"
	}
	line := fmt.Sprintf("CENSUS_HANDOFF_STAGE stage=%s", stage)
	trace.Log(ctx, "census_continuation", line)
	writeDescriptorTrace(line)
}

func continuationPreconditionCauseTrace(ctx context.Context, cause censuscontinuation.PreconditionCause, subcause ...completionSubcause) {
	if os.Getenv(runtimeTracePathEnv) == "" || !continuationPreconditionCause[string(cause)] {
		return
	}
	line := fmt.Sprintf("CONTINUATION_PRECONDITION_CAUSE cause=%s", cause)
	if cause == censuscontinuation.PreconditionCauseHandoffBuild && len(subcause) > 0 && validCompletionSubcause(subcause[0]) {
		line = fmt.Sprintf("CONTINUATION_PRECONDITION_CAUSE subcause=%s", subcause[0])
	}
	trace.Log(ctx, "census_continuation", line)
	writeDescriptorTrace(line)
}

func continuationMappingReturnTrace(ctx context.Context, mapErr error) {
	if mapErr == nil {
		continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "OK", "NONE")
		return
	}
	continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "ERROR", "MAPPING_ERROR")
}

func censusMappingDiagnosticTrace(ctx context.Context, predicate string) {
	if os.Getenv(runtimeTracePathEnv) == "" || os.Getenv(censusMappingDiagnosticEnv) == "" {
		return
	}
	if !validCensusMappingDiagnostic(predicate) {
		return
	}
	continuationBoundaryTrace(ctx, "MAPPING", "RETURN", "ERROR", "MAPPING_ERROR")
	trace.Log(ctx, "census_continuation", "CENSUS_MAPPING_DIAGNOSTIC predicate="+predicate)
	writeDescriptorTrace("CENSUS_MAPPING_DIAGNOSTIC predicate=" + predicate)
}

func validCensusMappingDiagnostic(predicate string) bool {
	switch predicate {
	case "PRECONDITION", "RESULT_V2_VALIDATION", "UNKNOWN_SCHEMA", "MARSHAL", "ENVELOPE_V2_VALIDATION",
		"RESULT_V2_CENSUS_SCHEMA_VERSION", "RESULT_V2_CENSUS_STATUS", "RESULT_V2_CENSUS_SESSION_ID", "RESULT_V2_CENSUS_GENERATION", "RESULT_V2_CENSUS_TARGET_COUNT", "RESULT_V2_CENSUS_BATCH_COUNT", "RESULT_V2_CENSUS_ACCOUNTING_RANGE", "RESULT_V2_CENSUS_ACCOUNTING", "RESULT_V2_CENSUS_AUTHORITY", "RESULT_V2_CENSUS_PUBLICATION_SELECTOR", "RESULT_V2_CENSUS_PUBLICATION_DIGEST", "RESULT_V2_CENSUS_PUBLICATION_BYTE_LENGTH", "RESULT_V2_CENSUS_PUBLICATION_STATUS", "RESULT_V2_CATALOG_SELECTOR", "RESULT_V2_CATALOG_STATUS", "RESULT_V2_CATALOG_GUIDANCE",
		"RESULT_V2_ROOT_DECODE", "RESULT_V2_CATALOG_TYPE", "RESULT_V2_CATALOG_SELECTOR_EQUALITY", "RESULT_V2_CENSUS_IDENTITY_SELECTOR_EQUALITY", "RESULT_V2_CATALOG_PAUSED_FIELDS_FORBIDDEN", "RESULT_V2_CATALOG_PAUSED_FIELDS_REQUIRED", "RESULT_V2_CATALOG_GUIDANCE_CONST", "RESULT_V2_CATALOG_REQUEST_COUNT_MINIMUM", "RESULT_V2_CATALOG_PREPARATION_COUNT_MINIMUM", "RESULT_V2_CATALOG_PREPARATION_COUNT_LTE_REQUEST_COUNT":
		return true
	}
	parts := strings.Split(predicate, "_")
	if len(parts) < 6 || parts[0] != "RESULT" || parts[1] != "V2" || parts[3] != "DEPTH" {
		return false
	}
	if parts[2] != "ROOT" && parts[2] != "CENSUS" && parts[2] != "CATALOG" {
		return false
	}
	if parts[4] != "0" && parts[4] != "1" && parts[4] != "2" && parts[4] != "3" {
		if len(parts) < 7 || parts[4] != "3" || parts[5] != "PLUS" {
			return false
		}
	}
	for _, allowed := range []string{"PATTERN", "MAX_LENGTH", "REQUIRED", "TYPE", "MINIMUM", "MAXIMUM", "ENUM", "CONST", "MIN_ITEMS", "MAX_ITEMS", "ADDITIONALPROPERTIES", "ONEOF", "ALLOF", "NOT", "$REF", "OTHER"} {
		if strings.HasSuffix(predicate, "_"+allowed) {
			return true
		}
	}
	return false
}

func continuationBoundaryTrace(ctx context.Context, boundary, stage, result, class string) {
	if os.Getenv(runtimeTracePathEnv) == "" || !continuationTraceBoundary[boundary] || !continuationTraceStage[stage] || !continuationTraceResult[result] || !continuationTraceClass[class] {
		return
	}
	correlation, _ := ctx.Value(descriptorTraceCorrelationKey{}).(descriptorTraceCorrelation)
	if correlation.Operation != "CENSUS" {
		correlation.Operation = "CENSUS"
	}
	if correlation.Generation != "G1" && correlation.Generation != "G2" {
		correlation.Generation = "OTHER"
	}
	line := fmt.Sprintf("CONTINUATION_TRACE operation=%s generation=%s boundary=%s stage=%s result=%s class=%s", correlation.Operation, correlation.Generation, boundary, stage, result, class)
	trace.Log(ctx, "census_continuation", line)
	writeDescriptorTrace(line)
}

func basePublicationTrace(ctx context.Context, event publication.BoundFileTraceEvent) {
	if os.Getenv(runtimeTracePathEnv) == "" || !basePublicationTraceStage[event.Stage] || !basePublicationTraceResult[event.Result] {
		return
	}
	ok := "FALSE"
	if event.OK {
		ok = "TRUE"
	}
	line := fmt.Sprintf("BASE_PUBLICATION_TRACE stage=%s result=%s ok=%s", event.Stage, event.Result, ok)
	trace.Log(ctx, "base_publication", line)
	writeDescriptorTrace(line)
}

func descriptorPublicationTrace(ctx context.Context, event publication.TraceEvent) {
	if os.Getenv(runtimeTracePathEnv) == "" || !descriptorTraceStage[event.Stage] || !descriptorTraceReason[event.Reason] {
		return
	}
	correlation, _ := ctx.Value(descriptorTraceCorrelationKey{}).(descriptorTraceCorrelation)
	if correlation.Operation != "CENSUS" {
		correlation.Operation = "CENSUS"
	}
	if correlation.Generation != "G1" && correlation.Generation != "G2" {
		correlation.Generation = "OTHER"
	}
	ok := "FALSE"
	if event.OK {
		ok = "TRUE"
	}
	line := fmt.Sprintf("DESCRIPTOR_PUBLICATION_TRACE operation=%s generation=%s stage=%s reason=%s ok=%s", correlation.Operation, correlation.Generation, event.Stage, event.Reason, ok)
	trace.Log(ctx, "descriptor_publication", line)
	writeDescriptorTrace(line)
}

type writeDeadliner interface {
	SetWriteDeadline(time.Time) error
}

func writeDescriptorTrace(line string) {
	runtimeTraceState.Lock()
	output := descriptorTraceOutput
	runtimeTraceState.Unlock()
	if output == nil {
		return
	}
	if deadlineOutput, ok := output.(writeDeadliner); ok {
		if err := deadlineOutput.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err == nil {
			defer deadlineOutput.SetWriteDeadline(time.Time{})
		}
	}
	_, _ = fmt.Fprintln(output, line)
	runtimeTraceState.Lock()
	if runtimeTraceState.recorder != nil && runtimeTraceState.path != "" {
		runtimeTraceState.markers = append(runtimeTraceState.markers, line)
	}
	runtimeTraceState.Unlock()
}

func startRuntimeFlightRecorder() (func(), error) {
	path := os.Getenv(runtimeTracePathEnv)
	if path == "" {
		return func() {}, nil
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: errors.New("path invalid")}
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode().Perm() != 0o700 {
		return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: errors.New("parent unavailable")}
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: errors.New("sink custody invalid")}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: err}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: err}
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return nil, &diagnosticSinkUnavailable{Sink: "RUNTIME_TRACE", cause: errors.Join(statErr, closeErr, errors.New("sink custody invalid"))}
	}
	recorder := trace.NewFlightRecorder(trace.FlightRecorderConfig{
		MinAge:   0,
		MaxBytes: 16 << 20,
	})
	if err := recorder.Start(); err != nil {
		return nil, fmt.Errorf("start runtime flight recorder: %w", err)
	}
	runtimeTraceState.Lock()
	runtimeTraceState.recorder = recorder
	runtimeTraceState.path = path
	runtimeTraceState.markers = nil
	runtimeTraceState.Unlock()
	continuationBoundaryTrace(context.Background(), "STARTUP", "ENTER", "OK", "NONE")
	return func() {
		recorder.Stop()
		_ = dumpRuntimeTrace(context.Background(), "stop")
		runtimeTraceState.Lock()
		markers := append([]string(nil), runtimeTraceState.markers...)
		path := runtimeTraceState.path
		runtimeTraceState.Unlock()
		if file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			for _, marker := range markers {
				_, _ = fmt.Fprintln(file, marker)
			}
			_ = file.Close()
		}
		runtimeTraceState.Lock()
		if runtimeTraceState.recorder == recorder {
			runtimeTraceState.recorder = nil
			runtimeTraceState.path = ""
			runtimeTraceState.markers = nil
		}
		runtimeTraceState.Unlock()
	}, nil
}

func dumpRuntimeTrace(ctx context.Context, stage string) error {
	trace.Log(ctx, "projection_failure", stage)
	runtimeTraceState.Lock()
	defer runtimeTraceState.Unlock()
	if runtimeTraceState.recorder == nil || runtimeTraceState.path == "" {
		return nil
	}
	temporary := runtimeTraceState.path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := runtimeTraceState.recorder.WriteTo(file)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(temporary)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return closeErr
	}
	return os.Rename(temporary, runtimeTraceState.path)
}
