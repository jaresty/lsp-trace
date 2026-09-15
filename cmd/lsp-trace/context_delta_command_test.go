package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/transientstructuraldelta"
	tsr "lsp-trace/internal/transientstructuralresult"
)

func TestContextDeltaRejectsSymlinkInput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "link.json")
	if err := os.WriteFile(target, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := readContextDeltaFile(link); err == nil {
		t.Fatal("ASSERT_CONTEXT_DELTA_NO_SYMLINK_INPUT")
	}
}

func TestContextDeltaMachineByteEquivalentToKernel(t *testing.T) {
	id := "tn_0123456789abcdef0123456789abcdef"
	v := tsr.LocatorResultV2{SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 0, SourceGraphComplete: "UNKNOWN", PositionEncoding: "utf-16", TargetID: id, Nodes: []tsr.LocatorNodeV2{{ID: id, Name: "A", Kind: 12, Path: "src/a.go", DeclarationRange: graph.Range{}}}, AnalyticsScope: "BOUNDED_LOCAL", Coupling: []tsr.CouplingV2{{NodeID: id}}, StrongComponents: []tsr.StrongComponentV2{{Nodes: []string{id}}}, WeakProjection: graphkernel.WeakProjectionPolicy, PageRankDamping: graphkernel.PageRankDamping, AnalyticsTolerance: graphkernel.AnalyticsTolerance, PageRank: []tsr.NodeScoreV2{{NodeID: id, Score: 1}}, HITS: []tsr.HubAuthorityV2{{NodeID: id}}}
	raw, _ := json.Marshal(v)
	dir := t.TempDir()
	b := filepath.Join(dir, "before.json")
	a := filepath.Join(dir, "after.json")
	if os.WriteFile(b, raw, 0600) != nil || os.WriteFile(a, raw, 0600) != nil {
		t.Fatal("fixture")
	}
	var out, errout bytes.Buffer
	if code := runContextDelta([]string{"--before", b, "--after", a, "--machine"}, &out, &errout); code != 0 {
		t.Fatalf("ASSERT_CONTEXT_DELTA_CLI: code=%d err=%s", code, errout.String())
	}
	want, _ := transientstructuraldelta.Execute(mustDeltaJSON(t, transientstructuraldelta.Input{Before: v, After: v}))
	wantRaw, _ := json.Marshal(want)
	wantRaw = append(wantRaw, '\n')
	if !bytes.Equal(out.Bytes(), wantRaw) {
		t.Fatalf("ASSERT_CONTEXT_DELTA_BYTE_EQUIVALENCE\n got %s\nwant %s", out.Bytes(), wantRaw)
	}
}
func mustDeltaJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
