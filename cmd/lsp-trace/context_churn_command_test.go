package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphkernel"
	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssidecar"
)

func TestContextChurnReportsFileMetricsWithoutChangingGraph(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(bytes.TrimSpace(out))
	}
	runGit("init", "-q")
	runGit("config", "user.name", "test")
	runGit("config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.go")
	runGit("commit", "-qm", "base")
	base := runGit("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc A() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit("commit", "-qam", "change")
	head := runGit("rev-parse", "HEAD")

	id := "tn_0123456789abcdef0123456789abcdef"
	artifact := tsr.LocatorResultV2{SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 0, SourceGraphComplete: "UNKNOWN", PositionEncoding: "utf-16", TargetID: id, Nodes: []tsr.LocatorNodeV2{{ID: id, Name: "A", Kind: 12, Path: "a.go", DeclarationRange: graph.Range{}}}, AnalyticsScope: "BOUNDED_LOCAL", Coupling: []tsr.CouplingV2{{NodeID: id}}, StrongComponents: []tsr.StrongComponentV2{{Nodes: []string{id}}}, WeakProjection: graphkernel.WeakProjectionPolicy, PageRankDamping: graphkernel.PageRankDamping, AnalyticsTolerance: graphkernel.AnalyticsTolerance, PageRank: []tsr.NodeScoreV2{{NodeID: id, Score: 1}}, HITS: []tsr.HubAuthorityV2{{NodeID: id}}}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(repo, "context.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runContextChurn([]string{"--input", input, "--workspace", repo, "--from", base, "--to", head, "--machine"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var got vcssidecar.Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.NodeCount != 1 || got.ChangedNodeCount != 1 || len(got.Nodes) != 1 || got.Nodes[0].CommitCount != 1 || got.Nodes[0].LinesAdded != 2 {
		t.Fatalf("unexpected sidecar: %+v", got)
	}
	if got.FromRevision != base || got.ToRevision != head {
		t.Fatalf("revision range was not resolved exactly: %+v", got)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.Attribution != "FILE_PATH_ONLY" {
		t.Fatalf("claim ceiling changed: %+v", got)
	}
}
