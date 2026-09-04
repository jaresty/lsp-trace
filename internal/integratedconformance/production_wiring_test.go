package integratedconformance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/operation"
)

const productionParityPerturbation = "LSP_TRACE_PRODUCTION_PARITY_PERTURB"

func TestProductionExecutionTransportParity(t *testing.T) {
	const assertion = "ASSERT_PRODUCTION_TRANSPORT_SEMANTIC_PARITY"
	t.Log("ASSERTION: " + assertion)

	repository := repositoryRoot(t)
	binDir := t.TempDir()
	cli := buildProductionBinary(t, repository, filepath.Join(binDir, "lsp-trace"), "./cmd/lsp-trace")
	mcp := buildProductionBinary(t, repository, filepath.Join(binDir, "lsp-trace-mcp"), "./cmd/lsp-trace-mcp")
	publicationRoot := filepath.Join(t.TempDir(), "publication")
	input := executionruntime.ProductionInput{Root: publicationRoot, Source: "package fixture\nfunc ProductionParity() {}\n"}
	rawInput, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	resetPublication := func() {
		t.Helper()
		if err := os.RemoveAll(publicationRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(publicationRoot, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	resetPublication()
	directResult, directFailure := executionruntime.NewProductionExecutor().Execute(context.Background(), operation.Request{
		Name: operation.CustodyExecute, RequestID: "offline-1", Input: rawInput,
	})
	if directFailure != nil {
		t.Fatalf("%s: direct failure=%+v", assertion, directFailure)
	}
	direct := productionParityObservation{Digest: directResult.LogicalDigest, Artifact: append([]byte(nil), directResult.Artifact...)}

	resetPublication()
	cliCmd := exec.Command(cli, "execute", "--request-id", "offline-1", "--input", "-")
	cliCmd.Stdin = bytes.NewReader(rawInput)
	cliOutput, err := cliCmd.Output()
	if err != nil {
		t.Fatalf("%s: cli: %v", assertion, err)
	}
	var cliEnvelope struct {
		State         string `json:"state"`
		LogicalDigest string `json:"logical_digest"`
		Artifact      string `json:"artifact"`
	}
	if err := json.Unmarshal(cliOutput, &cliEnvelope); err != nil {
		t.Fatalf("%s: decode cli envelope: %v", assertion, err)
	}
	cliArtifact, err := base64.StdEncoding.Strict().DecodeString(cliEnvelope.Artifact)
	if err != nil {
		t.Fatalf("%s: decode cli artifact: %v", assertion, err)
	}
	if cliEnvelope.State != "ok" {
		t.Fatalf("%s: cli state=%q", assertion, cliEnvelope.State)
	}
	cliObservation := productionParityObservation{Digest: cliEnvelope.LogicalDigest, Artifact: cliArtifact}

	resetPublication()
	params, _ := json.Marshal(map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": input}})
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	mcpCmd := exec.Command(mcp)
	mcpCmd.Stdin = bytes.NewReader(append(line, '\n'))
	var mcpStdout, mcpStderr bytes.Buffer
	mcpCmd.Stdout, mcpCmd.Stderr = &mcpStdout, &mcpStderr
	if err := mcpCmd.Run(); err != nil {
		t.Fatalf("%s: mcp: %v stderr=%s", assertion, err, mcpStderr.String())
	}
	var mcpEnvelope struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				OperationStatus string `json:"operation_status"`
				LogicalDigest   string `json:"logical_digest"`
				Content         string `json:"content"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(mcpStdout.Bytes(), &mcpEnvelope); err != nil {
		t.Fatalf("%s: decode mcp envelope: %v", assertion, err)
	}
	if mcpEnvelope.Result.IsError || mcpEnvelope.Result.StructuredContent.OperationStatus != "SUCCEEDED" {
		t.Fatalf("%s: mcp result=%+v", assertion, mcpEnvelope.Result)
	}
	mcpObservation := productionParityObservation{
		Digest:   mcpEnvelope.Result.StructuredContent.LogicalDigest,
		Artifact: []byte(mcpEnvelope.Result.StructuredContent.Content),
	}
	switch os.Getenv(productionParityPerturbation) {
	case "mcp-artifact":
		mcpObservation.Artifact = append(mcpObservation.Artifact, '\n')
	case "mcp-digest":
		mcpObservation.Digest = "sha256:perturbed"
	}

	for name, got := range map[string]productionParityObservation{"cli": cliObservation, "mcp": mcpObservation} {
		if got.Digest != direct.Digest || !bytes.Equal(got.Artifact, direct.Artifact) {
			t.Fatalf("%s: path=%s digest_equal=%t artifact_bytes_equal=%t", assertion, name, got.Digest == direct.Digest, bytes.Equal(got.Artifact, direct.Artifact))
		}
	}
	t.Log("PASS " + assertion)
}

type productionParityObservation struct {
	Digest   string
	Artifact []byte
}

func buildProductionBinary(t *testing.T, repository, output, packagePath string) string {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", output, packagePath)
	cmd.Dir = repository
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v output=%s", packagePath, err, combined)
	}
	return output
}

func TestProductionExecutionWiring(t *testing.T) {
	root := repositoryRoot(t)
	request := func() map[string]any {
		return map[string]any{"root": t.TempDir(), "source": "package fixture\nfunc Production() {}\n"}
	}

	t.Run("ASSERT_PRODUCTION_CLI_CANONICAL_CUSTODY", func(t *testing.T) {
		input, err := json.Marshal(request())
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "run", "./cmd/lsp-trace", "execute", "--request-id", "production-cli", "--input", "-")
		cmd.Dir, cmd.Stdin = root, bytes.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte(`"state":"ok"`)) || !bytes.Contains(out, []byte(`"operation":"execute"`)) || !bytes.Contains(out, []byte(`"artifact":`)) {
			t.Fatalf("ASSERT_PRODUCTION_CLI_CANONICAL_CUSTODY: err=%v output=%s", err, out)
		}
	})

	t.Run("ASSERT_PRODUCTION_MCP_CANONICAL_CUSTODY", func(t *testing.T) {
		params, _ := json.Marshal(map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": request()}})
		line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
		cmd := exec.Command("go", "run", "./cmd/lsp-trace-mcp")
		cmd.Dir, cmd.Stdin = root, bytes.NewReader(append(line, '\n'))
		out, err := cmd.CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte(`"operation_status":"SUCCEEDED"`)) || !bytes.Contains(out, []byte(`"content":`)) || bytes.Contains(out, []byte("unknown offline operation")) {
			t.Fatalf("ASSERT_PRODUCTION_MCP_CANONICAL_CUSTODY: err=%v output=%s", err, out)
		}
	})

	t.Run("ASSERT_PRODUCTION_FAILS_CLOSED", func(t *testing.T) {
		publication := filepath.Join(t.TempDir(), "publication")
		cmd := exec.Command("go", "run", "./cmd/lsp-trace", "execute", "--request-id", "invalid", "--input", "-")
		cmd.Dir, cmd.Stdin = root, strings.NewReader(`{"root":"","source":"x"}`)
		out, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte(`"state":"error"`)) {
			t.Fatalf("ASSERT_PRODUCTION_FAILS_CLOSED: err=%v output=%s", err, out)
		}
		if _, statErr := os.Stat(publication); !os.IsNotExist(statErr) {
			t.Fatalf("ASSERT_PRODUCTION_FAILS_CLOSED: publication exists: %v", statErr)
		}
	})
}
