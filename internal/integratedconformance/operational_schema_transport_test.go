package integratedconformance

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/schema"
)

func TestOperationalSchemaTransport(t *testing.T) {
	h := newOperationalHarness(t)
	want, err := schema.BytesFor(schema.FamilyOperationalCustody, "v1")
	if err != nil {
		t.Fatal(err)
	}
	cli, err := exec.Command(h.cli, "schema", "get", "--family", schema.FamilyOperationalCustody, "--version", "v1").Output()
	if err != nil || !bytes.Equal(cli, want) {
		t.Fatal("ASSERT_OPERATIONAL_SCHEMA_PARITY: CLI schema differs")
	}
	params := map[string]any{"schema": map[string]any{"family": schema.FamilyOperationalCustody, "version": "v1"}}
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_schema_get", "arguments": params}})
	cmd := exec.Command(h.mcp)
	cmd.Stdin = bytes.NewReader(append(line, '\n'))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result struct {
			IsError    bool `json:"isError"`
			Structured struct {
				Content string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.IsError || !bytes.Equal([]byte(envelope.Result.Structured.Content), want) {
		t.Fatalf("ASSERT_OPERATIONAL_SCHEMA_PARITY: MCP schema differs %s", out)
	}
}

func TestOperationalValidatedMCPPublication(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	a := operationalSuccess(t, h.run("direct", request, "", nil))
	raw, _ := json.Marshal(a.Operational)
	root := t.TempDir()
	args := map[string]any{"schema": map[string]any{"family": schema.FamilyOperationalCustody, "version": "v1"}, "input": string(raw), "output_selector": "validated.json"}
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_validate", "arguments": args}})
	cmd := exec.Command(h.mcp, "--publication-root", root)
	cmd.Stdin = bytes.NewReader(append(line, '\n'))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result struct {
			IsError    bool `json:"isError"`
			Structured struct {
				Status string `json:"operation_status"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.IsError || envelope.Result.Structured.Status != "SUCCEEDED" {
		t.Fatalf("ASSERT_OPERATIONAL_MCP_PUBLICATION: %s", out)
	}
	published, err := os.ReadFile(filepath.Join(root, "validated.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, published) {
		t.Fatal("ASSERT_OPERATIONAL_MCP_PUBLICATION: validated artifact bytes changed")
	}
}
