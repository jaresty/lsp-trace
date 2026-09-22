package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"sync"
	"time"

	"lsp-trace/internal/publication"
)

const runtimeTracePathEnv = "LSP_TRACE_RUNTIME_TRACE_PATH"

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
var continuationTraceClass = map[string]bool{"NONE": true, "TYPED": true, "UNTYPED": true}

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
		MinAge:   time.Minute,
		MaxBytes: 16 << 20,
	})
	if err := recorder.Start(); err != nil {
		return nil, fmt.Errorf("start runtime flight recorder: %w", err)
	}
	runtimeTraceState.Lock()
	runtimeTraceState.recorder = recorder
	runtimeTraceState.path = path
	runtimeTraceState.Unlock()
	continuationBoundaryTrace(context.Background(), "STARTUP", "ENTER", "OK", "NONE")
	return func() {
		recorder.Stop()
		runtimeTraceState.Lock()
		if runtimeTraceState.recorder == recorder {
			runtimeTraceState.recorder = nil
			runtimeTraceState.path = ""
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
