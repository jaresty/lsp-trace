package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/operation"
	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssidecar"
)

func TestContextChurnExecutorBindsExactBytesAndResolvedHistory(t *testing.T) {
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
	input, _ := json.Marshal(contextChurnInput{Input: string(raw), Workspace: repo, FromRevision: "HEAD~1", ToRevision: "HEAD", TimeoutMS: 30000})
	result, failure := (contextChurnExecutor{}).Execute(context.Background(), operation.Request{Name: operation.Name("context_churn"), Input: input})
	if failure != nil {
		t.Fatal(failure)
	}
	var got vcssidecar.Result
	if err := json.Unmarshal(result.Artifact, &got); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if got.GraphArtifactDigest != fmt.Sprintf("sha256:%x", digest) || got.FromRevision != base || got.ToRevision != head {
		t.Fatalf("ASSERT_CONTEXT_CHURN_EXACT_BINDING: %+v", got)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.Attribution != "FILE_PATH_ONLY" || got.NodeCount != 1 || got.ChangedNodeCount != 1 || got.ZeroChurnNodeCount != 0 {
		t.Fatalf("ASSERT_CONTEXT_CHURN_CONSERVATIVE_ACCOUNTING: %+v", got)
	}
	var object map[string]any
	_ = json.Unmarshal(result.Artifact, &object)
	for _, forbidden := range []string{"calls", "edges", "analytics"} {
		if _, ok := object[forbidden]; ok {
			t.Fatalf("ASSERT_CONTEXT_CHURN_NO_GRAPH_MUTATION: forbidden field %s", forbidden)
		}
	}
}
