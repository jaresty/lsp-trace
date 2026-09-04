package integratedconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/verification"
)

const custodyPerturbation = "LSP_TRACE_CUSTODY_PERTURB"

type fixedCustodyLoader struct{ material operation.CustodyMaterial }

func (l fixedCustodyLoader) Load(context.Context, json.RawMessage) (operation.CustodyMaterial, *operation.Failure) {
	return l.material, nil
}

func TestIntegratedCustodyConformance(t *testing.T) {
	const (
		assertDirectCLI       = "ASSERT_CUSTODY_DIRECT_CLI_PARITY"
		assertCLIMCP          = "ASSERT_CUSTODY_CLI_MCP_PARITY"
		assertOffline         = "ASSERT_CUSTODY_OFFLINE_PUBLICATION_VERIFY"
		assertReplay          = "ASSERT_CUSTODY_DETERMINISTIC_REPLAY"
		assertAcquisition     = "ASSERT_CUSTODY_ACQUISITION_STAGE_ATTRIBUTION"
		assertValidation      = "ASSERT_CUSTODY_VALIDATION_STAGE_ATTRIBUTION"
		assertHistoricalBase  = "ASSERT_CUSTODY_FROZEN_V2_V3_BASELINE"
		assertHistoricalAfter = "ASSERT_CUSTODY_FROZEN_V2_V3_AFTER"
	)

	root := repositoryRoot(t)
	cli := buildCommand(t, root, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildCommand(t, root, "lsp-trace-mcp", "./cmd/lsp-trace-mcp")
	artifact, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV3})
	if err != nil {
		t.Fatal(err)
	}
	artifact = append(artifact, '\n')
	receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
	if err != nil {
		t.Fatal(err)
	}

	before := frozenHistoricalBytes(t, root)
	if os.Getenv(custodyPerturbation) == assertHistoricalBase {
		before[0] = append(before[0], '\n')
	}
	assertFrozenHistorical(t, assertHistoricalBase, root, before)
	t.Log("PASS " + assertHistoricalBase)

	selector := publishCustody(t, artifact, receipt)
	directResult, directFailure := operation.NewVerifyHandler(fixedCustodyLoader{operation.CustodyMaterial{Artifact: artifact, Receipt: receipt}})(context.Background(), operation.Request{Name: operation.Verify, Input: json.RawMessage(`{"input":"custody"}`)})
	if directFailure != nil {
		t.Fatalf("%s: direct failure=%v", assertDirectCLI, directFailure)
	}

	cliOut, cliErr, cliCode := runCommand(cli, "verify", selector)
	if cliCode != 0 {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q", assertDirectCLI, cliCode, cliOut, cliErr)
	}
	if os.Getenv(custodyPerturbation) == assertDirectCLI {
		directResult.Artifact = append(directResult.Artifact, 'x')
	}
	if !bytes.Equal(directResult.Artifact, artifact) || cliOut != "verified integrity and custody\n" {
		t.Fatalf("%s: direct=%q cli=%q", assertDirectCLI, directResult.Artifact, cliOut)
	}
	t.Log("PASS " + assertDirectCLI)

	mcpArtifact, mcpEnvelope := runMCPVerify(t, mcp, selector)
	if os.Getenv(custodyPerturbation) == assertCLIMCP {
		mcpArtifact = append(mcpArtifact, 'x')
	}
	if !bytes.Equal(mcpArtifact, artifact) || mcpEnvelope["operation_status"] != "SUCCEEDED" {
		t.Fatalf("%s: artifact=%q envelope=%v", assertCLIMCP, mcpArtifact, mcpEnvelope)
	}
	t.Log("PASS " + assertCLIMCP)

	if os.Getenv(custodyPerturbation) == assertOffline {
		_ = os.Remove(filepath.Join(filepath.Dir(selector), "generation", "receipt.json"))
	}
	offlineOut, offlineErr, offlineCode := runCommand(cli, "verify", selector)
	if offlineCode != 0 || offlineOut != cliOut {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q", assertOffline, offlineCode, offlineOut, offlineErr)
	}
	t.Log("PASS " + assertOffline)

	secondSelector := publishCustody(t, artifact, receipt)
	secondCLIOut, _, secondCode := runCommand(cli, "verify", secondSelector)
	secondMCPArtifact, _ := runMCPVerify(t, mcp, secondSelector)
	if os.Getenv(custodyPerturbation) == assertReplay {
		secondCLIOut += "perturbed"
	}
	if secondCode != 0 || secondCLIOut != cliOut || !bytes.Equal(secondMCPArtifact, mcpArtifact) {
		t.Fatalf("%s: cli=(%d,%q)/(%d,%q) mcp=%q/%q", assertReplay, cliCode, cliOut, secondCode, secondCLIOut, mcpArtifact, secondMCPArtifact)
	}
	t.Log("PASS " + assertReplay)

	assertFailureStages(t, cli, artifact, receipt, assertAcquisition, assertValidation)
	t.Log("PASS " + assertAcquisition)
	t.Log("PASS " + assertValidation)

	if os.Getenv(custodyPerturbation) == assertHistoricalAfter {
		if err := os.WriteFile(filepath.Join(root, "internal", "graph", "testdata", "historical_identity_v2.json"), append(before[0], '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.WriteFile(filepath.Join(root, "internal", "graph", "testdata", "historical_identity_v2.json"), before[0], 0600)
		})
	}
	assertFrozenHistorical(t, assertHistoricalAfter, root, before)
	t.Log("PASS " + assertHistoricalAfter)
}

func publishCustody(t *testing.T, artifact, receipt []byte) string {
	t.Helper()
	dir := t.TempDir()
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := publication.NewPublisher()
	for _, item := range []struct {
		selector string
		data     []byte
	}{
		{"generation/artifact.json", artifact},
		{"generation/receipt.json", receipt},
		{"selector.json", []byte("{\"generation\":\"generation\"}\n")},
	} {
		result := publisher.Publish(publication.Request{Root: root, Selector: item.selector, Bytes: item.data})
		if result.Err() != nil || result.Receipt.ByteLength != uint64(len(item.data)) {
			t.Fatalf("publish %s: result=%+v", item.selector, result)
		}
	}
	return filepath.Join(dir, "selector.json")
}

func assertFailureStages(t *testing.T, cli string, artifact, receipt []byte, acquisitionAssertion, validationAssertion string) {
	t.Helper()
	cases := []struct {
		name, assertion, want string
		setup                 func(string)
	}{
		{"selector", acquisitionAssertion, "verify: malformed generation selector", func(dir string) { _ = os.WriteFile(filepath.Join(dir, "selector.json"), []byte("not-json"), 0600) }},
		{"artifact", acquisitionAssertion, "verify: incomplete selected generation", func(dir string) {
			writeSelectorAndGeneration(t, dir)
			_ = os.WriteFile(filepath.Join(dir, "generation", "receipt.json"), receipt, 0600)
		}},
		{"receipt", validationAssertion, "verify receipt: incomplete selected generation", func(dir string) {
			writeSelectorAndGeneration(t, dir)
			_ = os.WriteFile(filepath.Join(dir, "generation", "artifact.json"), artifact, 0600)
		}},
		{"semantic", validationAssertion, "verify semantic receipt:", func(dir string) {
			bad := []byte("{\"schema_version\":\"lsp-trace.graph.v2\"}\n")
			badReceipt, _ := verification.ReceiptBytes(bad, verification.DirectoryDurabilityChecked)
			writeSelectorAndGeneration(t, dir)
			_ = os.WriteFile(filepath.Join(dir, "generation", "artifact.json"), bad, 0600)
			_ = os.WriteFile(filepath.Join(dir, "generation", "receipt.json"), badReceipt, 0600)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(dir)
			_, stderr, code := runCommand(cli, "verify", filepath.Join(dir, "selector.json"))
			want := tc.want
			if os.Getenv(custodyPerturbation) == tc.assertion && ((tc.assertion == acquisitionAssertion && tc.name == "artifact") || (tc.assertion == validationAssertion && tc.name == "semantic")) {
				want = "deliberately absent stage"
			}
			if code == 0 || !strings.Contains(stderr, want) {
				t.Fatalf("%s: case=%s code=%d stderr=%q want=%q", tc.assertion, tc.name, code, stderr, want)
			}
		})
	}
}

func writeSelectorAndGeneration(t *testing.T, dir string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(dir, "generation"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "selector.json"), []byte("{\"generation\":\"generation\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func runMCPVerify(t *testing.T, binary, selector string) ([]byte, map[string]any) {
	t.Helper()
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_verify", "arguments": map[string]any{"input": selector}}})
	cmd := exec.Command(binary)
	cmd.Stdin = bytes.NewReader(append(request, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("MCP verify: %v stderr=%s", err, stderr.String())
	}
	var response struct {
		Result struct {
			Structured map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
		t.Fatalf("MCP response: %v: %s", err, stdout.String())
	}
	content, _ := response.Result.Structured["content"].(string)
	return []byte(content), response.Result.Structured
}

func runCommand(binary string, args ...string) (string, string, int) {
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exit.ExitCode()
	}
	return stdout.String(), stderr.String(), -1
}

func buildCommand(t *testing.T, root, name, pkg string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", path, pkg)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", pkg, err, output)
	}
	return path
}

func frozenHistoricalBytes(t *testing.T, root string) [][]byte {
	t.Helper()
	out := make([][]byte, 2)
	for i, version := range []string{"v2", "v3"} {
		data, err := os.ReadFile(filepath.Join(root, "internal", "graph", "testdata", "historical_identity_"+version+".json"))
		if err != nil {
			t.Fatal(err)
		}
		out[i] = data
	}
	return out
}

func assertFrozenHistorical(t *testing.T, assertion, root string, want [][]byte) {
	t.Helper()
	got := frozenHistoricalBytes(t, root)
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("%s: historical v%d bytes changed", assertion, i+2)
		}
	}
}
