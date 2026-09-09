package main

import (
	"bytes"
	"fmt"
	"testing"

	"lsp-trace/internal/schema"
)

func TestPublicAnalyticsV2SchemaGetProcessExactPermissions(t *testing.T) {
	mcpBinary := buildMCPBinary(t)
	for i, tc := range []struct {
		family string
	}{
		{schema.FamilyBoundedGraphV2},
		{schema.FamilyBoundedAnalysisV2},
		{schema.FamilyBoundedMetricsV2},
		{schema.FamilyBoundedRankingV2},
	} {
		t.Run(tc.family, func(t *testing.T) {
			want, err := schema.BytesFor(tc.family, "v2")
			if err != nil {
				t.Fatal(err)
			}
			response := runMCPProcess(t, mcpBinary, nil, []map[string]any{
				callRequest(i+1, "lsp_trace_v1_schema_get", map[string]any{
					"schema": map[string]any{"family": tc.family, "version": "v2"},
				}),
			})[0]
			call := decodeProcessCall(t, response)
			if call.env["outcome"] != "COMPLETE" || call.env["operation_status"] != "SUCCEEDED" {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SCHEMA_GET_PERMISSION_%s: outcome=%v status=%v code=%v diagnostics=%v", tc.family, call.env["outcome"], call.env["operation_status"], call.env["code"], call.env["diagnostics"])
			}
			got := inlineArtifactBytes(t, call.env)
			if !bytes.Equal(got, want) {
				t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SCHEMA_GET_EXACT_BYTES_%s: got=%d want=%d", tc.family, len(got), len(want))
			}
		})
	}

	response := runMCPProcess(t, mcpBinary, nil, []map[string]any{
		callRequest(99, "lsp_trace_v1_schema_get", map[string]any{
			"schema": map[string]any{"family": "bounded-retained-unknown-v2", "version": "v2"},
		}),
	})[0]
	call := decodeProcessCall(t, response)
	if call.env["outcome"] != "DOMAIN_ERROR" || call.env["operation_status"] != "FAILED" || call.env["code"] != "INPUT_FAMILY_MISMATCH" {
		t.Fatalf("ASSERT_PUBLIC_ANALYTICS_V2_SCHEMA_GET_UNKNOWN_REJECTED: %s", fmt.Sprint(call.env))
	}
}
