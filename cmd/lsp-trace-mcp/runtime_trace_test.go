package main

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func TestBasePublicationTraceSilentWhenDisabled(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, "")
	var output bytes.Buffer
	previous := descriptorTraceOutput
	descriptorTraceOutput = &output
	t.Cleanup(func() { descriptorTraceOutput = previous })

	basePublicationTrace(context.Background(), publication.BoundFileTraceEvent{Stage: "ROOT_SOURCE", Result: "CLI_ARGUMENT", OK: true})
	if output.Len() != 0 {
		t.Fatalf("ASSERT_BASE_PUBLICATION_TRACE_DEFAULT_OFF_SILENCE: %q", output.String())
	}
}

func TestBasePublicationTraceEnabledClosedAndPrivate(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, t.TempDir()+"/runtime.trace")
	var output bytes.Buffer
	previous := descriptorTraceOutput
	descriptorTraceOutput = &output
	t.Cleanup(func() { descriptorTraceOutput = previous })

	basePublicationTrace(context.Background(), publication.BoundFileTraceEvent{Stage: "ROOT_SOURCE", Result: "CLI_ARGUMENT", OK: true})
	basePublicationTrace(context.Background(), publication.BoundFileTraceEvent{Stage: "/private/root", Result: "raw error", OK: false})
	got := strings.TrimSpace(output.String())
	want := "BASE_PUBLICATION_TRACE stage=ROOT_SOURCE result=CLI_ARGUMENT ok=TRUE"
	if got != want {
		t.Fatalf("ASSERT_BASE_PUBLICATION_TRACE_CLOSED_VALUES: got=%q want=%q", got, want)
	}
	for _, forbidden := range []string{"/private", "root=", "raw", "error"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Fatalf("ASSERT_BASE_PUBLICATION_TRACE_PRIVACY_SAFE: forbidden=%q trace=%q", forbidden, got)
		}
	}
}

func TestBasePublicationTraceDoesNotBlockOnBackpressuredOutput(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, t.TempDir()+"/runtime.trace")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	chunk := bytes.Repeat([]byte{'x'}, 4096)
	for {
		if err := writer.SetWriteDeadline(time.Now().Add(time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(chunk); err != nil {
			break
		}
	}
	if err := writer.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}

	previous := descriptorTraceOutput
	descriptorTraceOutput = writer
	t.Cleanup(func() { descriptorTraceOutput = previous })

	done := make(chan struct{})
	go func() {
		basePublicationTrace(context.Background(), publication.BoundFileTraceEvent{Stage: "ROOT_SOURCE", Result: "CLI_ARGUMENT", OK: true})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ASSERT_BASE_PUBLICATION_TRACE_BACKPRESSURE_NONBLOCKING")
	}
}

func TestDescriptorPublicationTraceSilentWhenDisabled(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, "")
	var output bytes.Buffer
	previous := descriptorTraceOutput
	descriptorTraceOutput = &output
	t.Cleanup(func() { descriptorTraceOutput = previous })

	descriptorPublicationTrace(withDescriptorTraceCorrelation(context.Background(), 2), publication.TraceEvent{Stage: "ENTER", Reason: "START", OK: true})
	if output.Len() != 0 {
		t.Fatalf("ASSERT_DESCRIPTOR_TRACE_DEFAULT_OFF_SILENCE: %q", output.String())
	}
}

func TestContinuationBoundaryTraceStartupAndSyntheticPath(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(runtimeTracePathEnv, parent+"/runtime.trace")
	var output bytes.Buffer
	previous := descriptorTraceOutput
	descriptorTraceOutput = &output
	t.Cleanup(func() { descriptorTraceOutput = previous })

	stop, err := startRuntimeFlightRecorder()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)

	ctx := withDescriptorTraceCorrelation(context.Background(), 2)
	continuationBoundaryTrace(ctx, "BINDING", "ENTER", "NONE", "NONE")
	continuationBoundaryTrace(ctx, "HOST_FRESH", "RETURN", "ERROR", "TYPED")
	continuationBoundaryTrace(ctx, "private/path", "RETURN", "ERROR", "raw bytes")

	got := strings.Split(strings.TrimSpace(output.String()), "\n")
	want := []string{
		"CONTINUATION_TRACE operation=CENSUS generation=OTHER boundary=STARTUP stage=ENTER result=OK class=NONE",
		"CONTINUATION_TRACE operation=CENSUS generation=G2 boundary=BINDING stage=ENTER result=NONE class=NONE",
		"CONTINUATION_TRACE operation=CENSUS generation=G2 boundary=HOST_FRESH stage=RETURN result=ERROR class=TYPED",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ASSERT_CONTINUATION_TRACE_STARTUP_SYNTHETIC_PATH: got=%q want=%q", got, want)
	}
}

func TestRuntimeTraceSinkIsCreatedAndPrivateAtStartup(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := parent + "/runtime.trace"
	t.Setenv(runtimeTracePathEnv, path)
	stop, err := startRuntimeFlightRecorder()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_RUNTIME_TRACE_SINK_READY: info=%v err=%v", info, err)
	}
}

func TestRuntimeTraceSinkUnavailableFailsBeforeServing(t *testing.T) {
	path := t.TempDir() + "/missing/runtime.trace"
	t.Setenv(runtimeTracePathEnv, path)
	if stop, err := startRuntimeFlightRecorder(); err == nil {
		stop()
		t.Fatal("ASSERT_RUNTIME_TRACE_SINK_UNAVAILABLE")
	}
}

func TestDescriptorPublicationTraceEnabledClosedAndPrivate(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, t.TempDir()+"/runtime.trace")
	var output bytes.Buffer
	previous := descriptorTraceOutput
	descriptorTraceOutput = &output
	t.Cleanup(func() { descriptorTraceOutput = previous })
	ctx := withDescriptorTraceCorrelation(context.Background(), 2)

	descriptorPublicationTrace(ctx, publication.TraceEvent{Stage: "ENTER", Reason: "START", OK: true})
	descriptorPublicationTrace(ctx, publication.TraceEvent{Stage: "UNBOUNDED/private/path", Reason: "raw descriptor pin source", OK: false})

	got := strings.TrimSpace(output.String())
	want := "DESCRIPTOR_PUBLICATION_TRACE operation=CENSUS generation=G2 stage=ENTER reason=START ok=TRUE"
	if got != want {
		t.Fatalf("ASSERT_DESCRIPTOR_TRACE_CLOSED_VALUES: got=%q want=%q", got, want)
	}
	for _, forbidden := range []string{"/private", "source", "raw", "pin", "descriptor="} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Fatalf("ASSERT_DESCRIPTOR_TRACE_PRIVACY_SAFE: forbidden=%q trace=%q", forbidden, got)
		}
	}
}
