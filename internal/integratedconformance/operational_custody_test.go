package integratedconformance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/operation"
)

type operationalObservation struct {
	Artifact []byte
	Failed   bool
	Output   string
}

type operationalHarness struct {
	t        *testing.T
	cli, mcp string
}

func newOperationalHarness(t *testing.T) operationalHarness {
	t.Helper()
	root := repositoryRoot(t)
	bin := t.TempDir()
	return operationalHarness{t: t, cli: buildProductionBinary(t, root, filepath.Join(bin, "cli"), "./cmd/lsp-trace"), mcp: buildProductionBinary(t, root, filepath.Join(bin, "mcp"), "./cmd/lsp-trace-mcp")}
}
func (h operationalHarness) run(mode string, input any, config string, executor operation.Executor) operationalObservation {
	h.t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		h.t.Fatal(err)
	}
	if mode == "direct" {
		if executor == nil {
			store, err := executionruntime.LoadHostTrustStore(config)
			if err != nil {
				return operationalObservation{Failed: true, Output: err.Error()}
			}
			executor = executionruntime.NewProductionExecutorWithTrust(store)
		}
		result, failure := executor.Execute(context.Background(), operation.Request{Name: operation.CustodyExecute, RequestID: "offline-1", Input: raw})
		if failure != nil {
			b, _ := json.Marshal(failure.Diagnostics)
			return operationalObservation{Failed: true, Output: string(b)}
		}
		return operationalObservation{Artifact: result.Artifact}
	}
	var cmd *exec.Cmd
	if mode == "cli" {
		args := []string{"execute", "--request-id", "offline-1", "--input", "-"}
		if config != "" {
			args = append(args, "--custody-trust-config", config)
		}
		cmd = exec.Command(h.cli, args...)
		cmd.Stdin = bytes.NewReader(raw)
	} else {
		args := []string{}
		if config != "" {
			args = append(args, "--custody-trust-config", config)
		}
		cmd = exec.Command(h.mcp, args...)
		line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": input}}})
		cmd.Stdin = bytes.NewReader(append(line, '\n'))
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if len(out) == 0 {
		return operationalObservation{Failed: true, Output: stderr.String()}
	}
	if mode == "cli" {
		var e struct {
			State    string `json:"state"`
			Artifact string `json:"artifact"`
		}
		if err := json.Unmarshal(out, &e); err != nil {
			h.t.Fatalf("CLI envelope: %v %s", err, out)
		}
		artifact, err := base64.StdEncoding.DecodeString(e.Artifact)
		if err != nil {
			h.t.Fatal(err)
		}
		return operationalObservation{Artifact: artifact, Failed: e.State != "ok" || runErr != nil, Output: string(out)}
	}
	var e struct {
		Error  any `json:"error"`
		Result struct {
			IsError    bool `json:"isError"`
			Structured struct {
				Status  string `json:"operation_status"`
				Content string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &e); err != nil {
		h.t.Fatalf("MCP envelope: %v %s", err, out)
	}
	return operationalObservation{Artifact: []byte(e.Result.Structured.Content), Failed: e.Error != nil || e.Result.IsError || e.Result.Structured.Status != "SUCCEEDED" || runErr != nil, Output: string(out)}
}

func operationalRequest(t *testing.T) map[string]any {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"input.go", "config.json", "types.d.ts", "mapping.json"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("actual bytes: "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]any{"root": t.TempDir(), "operational": map[string]any{"source_root": root, "inputs": []any{
		map[string]any{"path": "input.go", "class": "SOURCE"}, map[string]any{"path": "config.json", "class": "CONFIGURATION"}, map[string]any{"path": "types.d.ts", "class": "DECLARATION"}, map[string]any{"path": "mapping.json", "class": "GENERATED_MAPPING"},
	}, "require_authenticated": false}}
}

func TestOperationalActualCommands(t *testing.T) {
	h := newOperationalHarness(t)
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			// Real legacy invocation proves this executable/transport works before testing
			// the absent operational behavior; command setup failures are not RED witnesses.
			legacy := h.run(mode, map[string]any{"root": t.TempDir(), "source": "package legacy\n"}, "", nil)
			if legacy.Failed {
				t.Fatalf("legacy control failed: %s", legacy.Output)
			}
			input := operationalRequest(t)
			got := h.run(mode, input, "", nil)
			if got.Failed {
				t.Fatalf("ASSERT_OPERATIONAL_ACTUAL_READ_%s: %s", mode, got.Output)
			}
			if !bytes.Contains(got.Artifact, []byte(`"operational":`)) || !bytes.Contains(got.Artifact, []byte(`"INCOMPLETE"`)) {
				t.Fatalf("ASSERT_OPERATIONAL_ACTUAL_READ_%s: missing retained operational evidence", mode)
			}
			published, err := os.ReadFile(filepath.Join(input["root"].(string), "artifact.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(published, []byte(`"input_evidence":`)) {
				t.Fatalf("ASSERT_OPERATIONAL_ACTUAL_READ_%s: published artifact lacks original receipts/content", mode)
			}
		})
	}
}
