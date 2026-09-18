package main

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"sync"
	"time"
)

const runtimeTracePathEnv = "LSP_TRACE_RUNTIME_TRACE_PATH"

var runtimeTraceState struct {
	sync.Mutex
	recorder *trace.FlightRecorder
	path     string
}

func startRuntimeFlightRecorder() (func(), error) {
	path := os.Getenv(runtimeTracePathEnv)
	if path == "" {
		return func() {}, nil
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
	return func() {
		runtimeTraceState.Lock()
		defer runtimeTraceState.Unlock()
		recorder.Stop()
		runtimeTraceState.recorder = nil
		runtimeTraceState.path = ""
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
