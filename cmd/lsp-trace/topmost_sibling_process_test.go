package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
)

func TestBuiltCLIEmitsV5SiblingFromHierarchicalDocumentSymbol(t *testing.T) {
	cli := buildFR23Binary(t, "lsp-trace", "./cmd/lsp-trace")
	fake := buildFR23Binary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(sourcePath, []byte("leaf\n\npeer\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")
	cmd := exec.Command(cli, "incoming", "--workspace", workspace, "--server", fake, "--at", "main.go:1:1", "--language-id", "go", "--expand-topmost-siblings", "--trace-lsp", tracePath, "--provenance-invocation-id", "process-session", "--provenance-source-revision", "process-revision", "--provenance-server-version", "fake@1")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=hierarchical")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ASSERT_V5_PROCESS_RUN: %v stderr=%s", err, stderr.String())
	}
	if err := graph.ValidateSemanticBundle(stdout.Bytes()); err != nil {
		t.Fatalf("ASSERT_V5_PROCESS_SEMANTIC_VALIDATION: %v artifact=%s", err, stdout.String())
	}
	var got struct {
		SchemaVersion string                   `json:"schema_version"`
		Edges         []graph.Edge             `json:"edges"`
		Siblings      []graph.SiblingCandidate `json:"sibling_candidates"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: sourcePath}).String()
	if got.SchemaVersion != graph.SchemaVersionV5 || len(got.Edges) != 1 || len(got.Siblings) != 1 || got.Siblings[0].Origin.Name != "leaf" || got.Siblings[0].Candidate.Name != "peer" || got.Siblings[0].SeedURI != uri {
		t.Fatalf("ASSERT_V5_PROCESS_EXACT_SIBLING_NO_CALLS: %+v", got)
	}
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range [][]byte{[]byte(`"method":"textDocument/prepareCallHierarchy"`), []byte(`"method":"textDocument/documentSymbol"`)} {
		if !bytes.Contains(trace, method) {
			t.Fatalf("ASSERT_V5_PROCESS_EXACT_METHOD_%s: trace=%s", method, trace)
		}
	}
}
