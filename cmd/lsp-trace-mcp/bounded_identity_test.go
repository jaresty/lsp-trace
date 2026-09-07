package main

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"testing"
)

func TestBoundedMCPObjectInputExactNumbers(t *testing.T) {
	raw, _ := boundedFixture(t)
	var retained retainedcalls.Evidence
	_ = json.Unmarshal(raw, &retained)
	var provenance graphprovenance.Evidence
	_ = json.Unmarshal(retained.InputBytes, &provenance)
	provenance.Generation = 9007199254740993
	input, _ := json.Marshal(provenance)
	raw, err := retainedcalls.Export(input)
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	raw = compact.Bytes()
	want, err := boundedanalysis.Analyze(context.Background(), raw, boundedanalysis.Parameters{Operation: "PROJECT"})
	if err != nil {
		t.Fatal(err)
	}
	calls := runMCPProcess(t, buildMCPBinary(t), nil, []map[string]any{
		callRequest(1, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": json.RawMessage(raw), "operation": "PROJECT"}),
		callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": json.RawMessage(bytes.TrimSpace(want)), "schema": map[string]any{"family": boundedanalysis.Family, "version": "v1"}}),
	})
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), want) {
		t.Fatal("ASSERT_MCP_OBJECT_EXACT_NUMBER_AND_BYTES")
	}
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[1]).env), bytes.TrimSpace(want)) {
		t.Fatal("ASSERT_MCP_VALIDATE_OBJECT_EXACT_BYTES")
	}
}
