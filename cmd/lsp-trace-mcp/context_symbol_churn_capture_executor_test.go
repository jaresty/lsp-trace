package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/operation"
)

func TestContextSymbolChurnCapturePreservesStructuralFailure(t *testing.T) {
	want := &operation.Failure{Code: "REQUEST_TIMEOUT", Err: errors.New("structural request timed out")}
	_, got := preserveStructuralFailure(want)
	if got != want || got.Code != "REQUEST_TIMEOUT" {
		t.Fatalf("ASSERT_SYMBOL_CHURN_CAPTURE_STRUCTURAL_FAILURE_PRESERVED: got=%+v want=%+v", got, want)
	}
}

type captureExecutorFunc func(context.Context, operation.Request) (operation.Result, *operation.Failure)

func (f captureExecutorFunc) Execute(ctx context.Context, req operation.Request) (operation.Result, *operation.Failure) {
	return f(ctx, req)
}

func TestContextSymbolChurnCaptureExactByteDigest(t *testing.T) {
	structuralV2 := []byte("{\n  \"schema_version\": \"lsp-trace.transient-structural-result.v2\",\n  \"authority\": 0\n}\n")
	want := fmt.Sprintf("sha256:%x", sha256.Sum256(structuralV2))
	var got string
	var received []byte
	executor := &contextSymbolChurnCaptureExecutor{
		structural: captureExecutorFunc(func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
			return operation.Result{Artifact: structuralV2}, nil
		}),
		churn: captureExecutorFunc(func(_ context.Context, req operation.Request) (operation.Result, *operation.Failure) {
			var input contextSymbolChurnInput
			if err := json.Unmarshal(req.Input, &input); err != nil {
				t.Fatal(err)
			}
			received = []byte(input.Input)
			got = fmt.Sprintf("sha256:%x", sha256.Sum256(received))
			return operation.Result{Artifact: []byte(`{"ok":true}`)}, nil
		}),
		resolveWorkspace: func(string, uint64) (string, *operation.Failure) { return "/workspace", nil },
	}
	input := []byte(`{"session_id":"project","generation":1,"uri":"file:///workspace/a.go","symbol":"A","down_depth":1,"up_depth":0,"max_nodes":10,"timeout_ms":1000,"request_timeout_ms":500,"analysis":{"kind":"NEIGHBORHOOD"},"from_revision":"HEAD~1","to_revision":"HEAD","profile":"go","language_id":"go"}`)
	if _, failure := executor.Execute(context.Background(), operation.Request{Name: contextSymbolChurnCaptureOperation, Input: input}); failure != nil {
		t.Fatal(failure.Err)
	}
	if got != want {
		t.Fatalf("ASSERT_SYMBOL_CHURN_CAPTURE_EXACT_BYTE_DIGEST: got=%s want=%s", got, want)
	}
	if !bytes.Equal(received, structuralV2) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_CAPTURE_NO_CALL_MANUFACTURE: structural bytes changed: got=%q want=%q", received, structuralV2)
	}
}
