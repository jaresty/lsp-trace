package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

const (
	assertCustodyDelegates   = "P_CLI_CUSTODY_SHARED_OPERATION"
	assertCustodyCanonical   = "P_CLI_CUSTODY_CANONICAL_COMPLETE_JSON"
	assertCustodyReference   = "P_CLI_CUSTODY_IMMUTABLE_REFERENCE"
	assertCustodyUnavailable = "P_CLI_CUSTODY_EXPLICIT_UNAVAILABLE"
	assertCustodyExit        = "P_CLI_CUSTODY_EXIT_MAPPING"
)

func TestCustodyTransportDelegatesAndEmitsCanonicalReference(t *testing.T) {
	artifact := json.RawMessage(`{"schema_version":"lsp-trace.graph.v3","opaque":"bytes stay retained"}`)
	calls := 0
	handler := func(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
		calls++
		if request.Name != operation.Verify || string(request.Input) != `{"input":"/retained/selector.json"}` {
			t.Fatalf("%s: request=%#v", assertCustodyDelegates, request)
		}
		return operation.Result{Artifact: artifact}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runCustodyWithHandler([]string{"/retained/selector.json"}, &stdout, &stderr, handler, func(string) (generationSelector, error) {
		return generationSelector{Generation: "generation-7"}, nil
	})
	want := "{\"schema_version\":\"lsp-trace.custody.v1\",\"state\":\"available\",\"selector\":\"/retained/selector.json\",\"generation\":\"/retained/generation-7\",\"artifact\":\"/retained/generation-7/artifact.json\"}\n"
	if calls != 1 || code != 0 || stderr.Len() != 0 {
		t.Fatalf("%s: calls=%d code=%d stderr=%q", assertCustodyDelegates, calls, code, stderr.String())
	}
	if !json.Valid(bytes.TrimSpace(stdout.Bytes())) || !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("%s: got=%q", assertCustodyCanonical, stdout.String())
	}
	if stdout.String() != want || bytes.Contains(stdout.Bytes(), artifact) {
		t.Fatalf("%s: got=%q want=%q artifact-copied=%t", assertCustodyReference, stdout.String(), want, bytes.Contains(stdout.Bytes(), artifact))
	}
}

func TestCustodyTransportUnavailableIsStructuredAndUntruncated(t *testing.T) {
	large := strings.Repeat("custody moved ", 10000)
	handler := func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{}, &operation.Failure{Code: "CUSTODY_UNAVAILABLE", Diagnostics: []string{large}, Err: errors.New("unstable detail must not leak")}
	}
	var stdout, stderr bytes.Buffer
	code := runCustodyWithHandler([]string{"selector.json"}, &stdout, &stderr, handler, func(string) (generationSelector, error) {
		return generationSelector{}, errors.New("must not be consulted after shared failure")
	})
	var got struct {
		SchemaVersion string   `json:"schema_version"`
		State         string   `json:"state"`
		Code          string   `json:"code"`
		Diagnostics   []string `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("%s: %v output=%q", assertCustodyCanonical, err, stdout.String())
	}
	if code != 2 || stderr.Len() != 0 || got.SchemaVersion != "lsp-trace.custody.v1" || got.State != "unavailable" || got.Code != "CUSTODY_UNAVAILABLE" {
		t.Fatalf("%s: code=%d stderr=%q envelope=%#v", assertCustodyUnavailable, code, stderr.String(), got)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0] != large || !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("%s: diagnostic bytes=%d want=%d", assertCustodyCanonical, len(got.Diagnostics[0]), len(large))
	}
}

func TestCustodyTransportRejectsInvalidArgumentsCanonically(t *testing.T) {
	want := "{\"schema_version\":\"lsp-trace.custody.v1\",\"state\":\"error\",\"code\":\"INVALID_INPUT\",\"diagnostics\":[\"usage: lsp-trace custody SELECTOR\"]}\n"
	for _, args := range [][]string{nil, {"one", "two"}} {
		var stdout, stderr bytes.Buffer
		code := runCustody(args, &stdout, &stderr)
		if code != 1 || stderr.Len() != 0 || stdout.String() != want {
			t.Fatalf("%s: args=%v code=%d stdout=%q stderr=%q", assertCustodyExit, args, code, stdout.String(), stderr.String())
		}
	}
	stdout, stderr, code := captureRun(t, []string{"custody"})
	if code != 1 || stderr != "" || stdout != want {
		t.Fatalf("%s: top-level code=%d stdout=%q stderr=%q", assertCustodyDelegates, code, stdout, stderr)
	}
}
