package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/schema"
)

func TestPublicAnalyticsV2ToolsListRendersThroughInstalledPiAdapter(t *testing.T) {
	const adapterShape = "/Users/schwa/.pi/agent/npm/node_modules/pi-mcp-adapter/ts-shape.ts"
	if _, err := os.Stat(adapterShape); err != nil {
		if os.IsNotExist(err) {
			t.Skip("installed pi-mcp-adapter conformance requires developer integration substrate")
		}
		t.Fatalf("ASSERT_PI_MCP_ADAPTER_2_32_1_PRESENT: %v", err)
	}
	adapterSource, err := os.ReadFile(adapterShape)
	if err != nil {
		t.Fatal(err)
	}
	adapterCopy := filepath.Join(t.TempDir(), "ts-shape.ts")
	if err := os.WriteFile(adapterCopy, adapterSource, 0o600); err != nil {
		t.Fatal(err)
	}
	mcpBinary := buildMCPBinary(t)
	response := runMCPProcess(t, mcpBinary, nil, []map[string]any{{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	}})[0]
	result, _ := response["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	operations := map[string]string{
		"lsp_trace_v2_bounded_retained_analysis": "ANALYSIS",
		"lsp_trace_v2_bounded_retained_metrics":  "METRICS",
		"lsp_trace_v2_bounded_retained_ranking":  "RANKING",
	}
	found := 0
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		operation, target := operations[fmt.Sprint(tool["name"])]
		if !target {
			continue
		}
		found++
		schemaPath := filepath.Join(t.TempDir(), "schema.json")
		schemaBytes, _ := json.Marshal(tool["inputSchema"])
		if err := os.WriteFile(schemaPath, schemaBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		script := `import fs from "node:fs"; import {renderTsShape} from ` + fmt.Sprintf("%q", adapterCopy) + `; const shape=renderTsShape(JSON.parse(fs.readFileSync(process.argv[1],"utf8"))); console.log(shape);`
		cmd := exec.Command("node", "--experimental-strip-types", "--input-type=module", "-e", script, schemaPath)
		output, err := cmd.CombinedOutput()
		shape := strings.TrimSpace(string(output))
		if err != nil || strings.Contains(shape, "unknown | unknown") || !strings.Contains(shape, `operation: "`+operation+`"`) || !strings.Contains(shape, "input:") || !strings.Contains(shape, "publication_selector:") {
			t.Fatalf("ASSERT_PI_MCP_ADAPTER_RENDERABLE_COMPLETE_OBJECT_%s: err=%v shape=%s", operation, err, shape)
		}
	}
	if found != 3 {
		t.Fatalf("ASSERT_PI_MCP_ADAPTER_ALL_ANALYTICS_TOOLS: found=%d", found)
	}
}

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
