package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

const (
	assertExecutionRejectsBeforeDelegate = "ASSERT_EXECUTION_REJECTS_BEFORE_DELEGATE"
	assertExecutionDelegatesExactlyOnce  = "ASSERT_EXECUTION_DELEGATES_EXACTLY_ONCE"
	assertExecutionPreservesRequest      = "ASSERT_EXECUTION_PRESERVES_REQUEST"
	assertExecutionCanonicalJSON         = "ASSERT_EXECUTION_CANONICAL_JSON"
	assertExecutionStableFailure         = "ASSERT_EXECUTION_STABLE_FAILURE"
	assertExecutionStableExits           = "ASSERT_EXECUTION_STABLE_EXITS"
	assertExecutionImmutableResult       = "ASSERT_EXECUTION_IMMUTABLE_RESULT"
)

type recordingExecutor struct {
	calls    int
	requests []operation.Request
	result   operation.Result
	failure  *operation.Failure
}

func (e *recordingExecutor) Execute(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	e.calls++
	request.Input = append(json.RawMessage(nil), request.Input...)
	e.requests = append(e.requests, request)
	return e.result, e.failure
}

func TestExecutionTransportRejectsBeforeDelegation(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		input string
	}{
		{"missing request id", []string{"--input", "-"}, `{}`},
		{"missing input", []string{"--request-id", "req-1"}, `{}`},
		{"trailing argument", []string{"--request-id", "req-1", "--input", "-", "extra"}, `{}`},
		{"invalid json", []string{"--request-id", "req-1", "--input", "-"}, `{`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &recordingExecutor{}
			var stdout, stderr bytes.Buffer
			code := runExecutionWithExecutor(context.Background(), tt.args, strings.NewReader(tt.input), &stdout, &stderr, executor)
			if code != executionExitInvalidInput || executor.calls != 0 {
				t.Fatalf("%s: code=%d calls=%d stdout=%q stderr=%q", assertExecutionRejectsBeforeDelegate, code, executor.calls, stdout.String(), stderr.String())
			}
			assertSingleCanonicalJSON(t, stdout.Bytes(), assertExecutionCanonicalJSON)
		})
	}
}

func TestExecutionTransportDelegatesOnceAndPreservesRequest(t *testing.T) {
	executor := &recordingExecutor{result: operation.Result{
		Value:         map[string]any{"reference": "immutable://generation/1"},
		Artifact:      []byte{0, 1, 2, 255},
		LogicalDigest: "sha256:abc",
	}}
	var stdout, stderr bytes.Buffer
	code := runExecutionWithExecutor(context.Background(), []string{"--request-id", "req-1", "--input", "-"}, strings.NewReader(`{"z":2,"a":1}`), &stdout, &stderr, executor)
	if code != executionExitSuccess || executor.calls != 1 {
		t.Fatalf("%s: code=%d calls=%d stderr=%q", assertExecutionDelegatesExactlyOnce, code, executor.calls, stderr.String())
	}
	if got := executor.requests[0]; got.Name != operation.Name("execute") || got.RequestID != "req-1" || string(got.Input) != `{"z":2,"a":1}` {
		t.Fatalf("%s: request=%#v", assertExecutionPreservesRequest, got)
	}
	want := `{"schema_version":"lsp-trace.execution.v1","state":"ok","operation":"execute","request_id":"req-1","result":{"reference":"immutable://generation/1"},"artifact":"` + base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255}) + `","logical_digest":"sha256:abc"}` + "\n"
	if stdout.String() != want {
		t.Fatalf("%s: got=%q want=%q", assertExecutionCanonicalJSON, stdout.String(), want)
	}
}

func TestExecutionTransportPreservesImmutableResultData(t *testing.T) {
	executor := &recordingExecutor{result: operation.Result{
		Value:         map[string]any{"reference": "immutable://generation/1"},
		Artifact:      []byte{0, 1, 2, 255},
		LogicalDigest: "sha256:abc",
	}}
	var stdout, stderr bytes.Buffer
	code := runExecutionWithExecutor(context.Background(), []string{"--request-id", "req-1", "--input", "-"}, strings.NewReader(`{}`), &stdout, &stderr, executor)
	var envelope struct {
		Result        map[string]any `json:"result"`
		Artifact      []byte         `json:"artifact"`
		LogicalDigest string         `json:"logical_digest"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); code != executionExitSuccess || err != nil || !bytes.Equal(envelope.Artifact, []byte{0, 1, 2, 255}) || envelope.LogicalDigest != "sha256:abc" || envelope.Result["reference"] != "immutable://generation/1" {
		t.Fatalf("%s: code=%d envelope=%#v err=%v stderr=%q", assertExecutionImmutableResult, code, envelope, err, stderr.String())
	}
}

func TestExecutionTransportFailureIsStableAndClassed(t *testing.T) {
	executor := &recordingExecutor{failure: &operation.Failure{Code: "PUBLICATION_DENIED", Diagnostics: []string{"destination denied"}, Err: errors.New("/private/unstable/path")}}
	var stdout, stderr bytes.Buffer
	code := runExecutionWithExecutor(context.Background(), []string{"--request-id", "req-2", "--input", "-"}, strings.NewReader(`{}`), &stdout, &stderr, executor)
	if code != executionExitOperationFailure {
		t.Fatalf("%s: code=%d", assertExecutionStableExits, code)
	}
	want := "{\"schema_version\":\"lsp-trace.execution.v1\",\"state\":\"error\",\"operation\":\"execute\",\"request_id\":\"req-2\",\"code\":\"PUBLICATION_DENIED\",\"diagnostics\":[\"destination denied\"]}\n"
	if stdout.String() != want || strings.Contains(stdout.String(), "unstable") || stderr.Len() != 0 {
		t.Fatalf("%s: stdout=%q stderr=%q", assertExecutionStableFailure, stdout.String(), stderr.String())
	}
}

func TestExecutionTransportCancellationExitIsDistinct(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &recordingExecutor{failure: &operation.Failure{Code: operation.FailureInternal, Err: context.Canceled}}
	var stdout, stderr bytes.Buffer
	code := runExecutionWithExecutor(ctx, []string{"--request-id", "req-3", "--input", "-"}, strings.NewReader(`{}`), &stdout, &stderr, executor)
	if code != executionExitCancelled {
		t.Fatalf("%s: code=%d", assertExecutionStableExits, code)
	}
}

func assertSingleCanonicalJSON(t *testing.T, encoded []byte, assertion string) {
	t.Helper()
	if bytes.Count(encoded, []byte("\n")) != 1 || len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
		t.Fatalf("%s: expected one newline-terminated document: %q", assertion, encoded)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatalf("%s: invalid JSON: %v", assertion, err)
	}
}
