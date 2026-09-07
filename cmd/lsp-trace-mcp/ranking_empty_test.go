package main

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedranking"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestRankingEmptyExportPublic(t *testing.T) {
	cli, mcp := buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t)
	for _, complete := range []bool{true, false} {
		root := t.TempDir()
		file := filepath.Join(root, "empty.go")
		if err := os.WriteFile(file, []byte("package empty\n"), 0600); err != nil {
			t.Fatal(err)
		}
		uri := (&url.URL{Scheme: "file", Path: file}).String()
		r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: complete}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
		r.Invocation.Target = graph.Target{URI: uri}
		r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: uri + ":0:0", ResolvedURI: uri}}
		r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target}}
		r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: uri, DownDepth: 1, UpDepth: 1}
		g, _ := json.Marshal(r)
		provenance, err := graphprovenance.Capture(context.Background(), g, root, uri, "empty-ranking", 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := retainedcalls.Export(provenance)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(t.TempDir(), "retained.json")
		_ = os.WriteFile(input, raw, 0600)
		want := runCLIProcess(t, cli, "bounded-retained-ranking", "--algorithm", "PAGERANK", input)
		if _, err = boundedranking.ValidateFor(want, boundedranking.Family, "v1"); err != nil {
			t.Fatal(err)
		}
		var e boundedranking.Evidence
		_ = json.Unmarshal(want, &e)
		if e.Status != "COMPLETE" || e.Iterations != 0 || e.Work != 0 || len(e.Scores) != 0 || len(e.Ranks) != 0 || e.Residual == nil || *e.Residual != 0 || e.Mass == nil || *e.Mass != 0 || !bytes.Equal(e.InputBytes, raw) {
			t.Fatal("empty convention", e)
		}
		if _, err = boundedranking.Analyze(context.Background(), raw, boundedranking.Defaults("PPR")); err == nil {
			t.Fatal("empty PPR admitted")
		}
		calls := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": string(raw), "algorithm": "PAGERANK"}), callRequest(2, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": string(raw), "algorithm": "PPR", "seeds": []any{map[string]any{"node_id": "missing", "weight": 1}}})})
		if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), want) || decodeProcessCall(t, calls[1]).env["isError"] != true {
			t.Fatal("empty MCP")
		}
	}
}
