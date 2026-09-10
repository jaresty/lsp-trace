package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSliceSelectorSchemaRendersThroughInstalledPiAdapter(t *testing.T) {
	const adapterShape = "/Users/schwa/.pi/agent/npm/node_modules/pi-mcp-adapter/ts-shape.ts"
	adapterSource, err := os.ReadFile(adapterShape)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("installed pi-mcp-adapter conformance requires developer integration substrate")
		}
		t.Fatal(err)
	}
	adapterCopy := filepath.Join(t.TempDir(), "ts-shape.ts")
	if err := os.WriteFile(adapterCopy, adapterSource, 0o600); err != nil {
		t.Fatal(err)
	}
	response := runMCPProcess(t, buildMCPBinary(t), []string{"--tool-profile", "compact"}, []map[string]any{{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}})[0]
	result, _ := response["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 10 {
		t.Fatalf("ASSERT_SLICE_SELECTOR_COMPACT_TEN: tools=%d", len(tools))
	}
	var inputSchema any
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if tool["name"] == "lsp_trace_v1_slice" {
			inputSchema = tool["inputSchema"]
		}
	}
	if inputSchema == nil {
		t.Fatal("ASSERT_SLICE_SELECTOR_ADVERTISED")
	}
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	schemaBytes, _ := json.Marshal(inputSchema)
	if err := os.WriteFile(schemaPath, schemaBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	script := `import fs from "node:fs"; import {renderTsShape} from ` + fmt.Sprintf("%q", adapterCopy) + `; console.log(renderTsShape(JSON.parse(fs.readFileSync(process.argv[1],"utf8"))));`
	output, err := exec.Command("node", "--experimental-strip-types", "--input-type=module", "-e", script, schemaPath).CombinedOutput()
	shape := string(output)
	if err != nil || strings.Contains(shape, "unknown | unknown") || !strings.Contains(shape, "line:") || !strings.Contains(shape, "character:") || !strings.Contains(shape, "symbol:") || !strings.Contains(shape, "} | {") {
		t.Fatalf("ASSERT_SLICE_SELECTOR_PI_ADAPTER_BOTH_FORMS: err=%v shape=%s", err, shape)
	}
}

func TestSliceSelectorDirectProcessValidation(t *testing.T) {
	binary := buildMCPBinary(t)
	base := map[string]any{"session_id": "missing", "generation": 1, "start_mode": "at", "uri": "file:///workspace/main.go"}
	requests := []map[string]any{}
	for i, selector := range []map[string]any{{"symbol": "Target"}, {"line": 4, "character": 7}} {
		arguments := cloneMap(base)
		for key, value := range selector {
			arguments[key] = value
		}
		requests = append(requests, callRequest(i+1, "lsp_trace_v1_slice", arguments))
	}
	invalid := []struct {
		selector   map[string]any
		diagnostic string
	}{
		{map[string]any{"line": 4}, "character is required when line is provided"},
		{map[string]any{"character": 7}, "line is required when character is provided"},
		{map[string]any{"symbol": "Target", "line": 4, "character": 7}, "symbol is mutually exclusive with line and character"},
		{map[string]any{}, "one target selector is required: symbol or line and character"},
	}
	for i, tc := range invalid {
		arguments := cloneMap(base)
		for key, value := range tc.selector {
			arguments[key] = value
		}
		requests = append(requests, callRequest(i+3, "lsp_trace_v1_slice", arguments))
	}
	responses := runMCPProcess(t, binary, nil, requests)
	for i := 0; i < 2; i++ {
		call := decodeProcessCall(t, responses[i])
		if call.env["code"] != "SESSION_NOT_FOUND" {
			t.Fatalf("ASSERT_SLICE_SELECTOR_VALID_REACHES_EXECUTOR_%d: %v", i, call.env)
		}
	}
	for i, tc := range invalid {
		rpcError, _ := responses[i+2]["error"].(map[string]any)
		if rpcError["code"] != float64(-32602) || !strings.Contains(fmt.Sprint(rpcError["message"]), tc.diagnostic) {
			t.Fatalf("ASSERT_SLICE_SELECTOR_FIELD_DIAGNOSTIC_%d: %v want %s", i, rpcError, tc.diagnostic)
		}
	}
}
