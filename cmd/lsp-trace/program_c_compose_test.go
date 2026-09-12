package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/programctestfixture"
)

func composeCLIInput(t testing.TB) []byte {
	t.Helper()
	raw := programctestfixture.ValidV5(t)
	sum := sha256.Sum256(raw)
	capture := func(id string) map[string]any {
		return map[string]any{
			"input": string(raw), "identity": id, "sha256": fmt.Sprintf("sha256:%x", sum), "byte_length": len(raw),
			"exact_metadata": map[string]any{"workspace_identity": "workspace-1", "revision_custody": "CALLER_ASSERTED", "position_encoding": "utf-16", "acquisition_semantics": "managed-lsp-v1", "privacy_policy": "bundle-custodian-v1"},
		}
	}
	encoded, err := json.Marshal(map[string]any{"captures": []any{capture("capture-z"), capture("capture-a")}})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestProgramCComposeCLIUsesSharedHandlerExactBytes(t *testing.T) {
	input := composeCLIInput(t)
	var stdout, stderr strings.Builder
	if code := runProgramCCompose([]string{"-"}, strings.NewReader(string(input)), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_CLI_SUCCESS: code=%d stderr=%q", code, stderr.String())
	}
	result, failure := operation.ProgramCComposeHandler(context.Background(), operation.Request{Name: operation.ProgramCCompose, Input: input})
	if failure != nil || stdout.String() != string(result.Artifact) {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_CLI_SHARED_EXACT_BYTES: failure=%v", failure)
	}
}

func TestProgramCComposeCLIRejectsMalformedRequest(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := runProgramCCompose([]string{"-"}, strings.NewReader(`{"captures":[]}`), &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("ASSERT_PROGRAM_C_COMPOSE_CLI_SCHEMA_REJECTION: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
