package vcssidecar

import (
	"crypto/sha256"
	"fmt"
	"testing"

	tsr "lsp-trace/internal/transientstructuralresult"
)

type fakeHistory struct {
	gotPaths []string
	metrics  map[string]FileMetric
}

func (f *fakeHistory) Collect(_ string, paths []string, from, to string) (HistoryCollection, error) {
	f.gotPaths = append([]string(nil), paths...)
	if from != "base" || to != "head" {
		return HistoryCollection{}, fmt.Errorf("unexpected range")
	}
	return HistoryCollection{FromRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ToRevision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Metrics: f.metrics}, nil
}

func TestBuildBindsExactArtifactAndAccountsEveryNode(t *testing.T) {
	raw := []byte("exact-v2-bytes\n")
	input := tsr.LocatorResultV2{
		SchemaVersion: "lsp-trace.transient-structural-result.v2",
		Authority:     0, SourceGraphComplete: "UNKNOWN", PositionEncoding: "utf-16",
		TargetID: "n1", AnalyticsScope: "BOUNDED_LOCAL",
		Nodes: []tsr.LocatorNodeV2{
			{ID: "n1", Path: "a.go", Kind: 12, Name: "A"},
			{ID: "n2", Path: "a.go", Kind: 12, Name: "B"},
			{ID: "n3", Path: "b.go", Kind: 12, Name: "C"},
		},
	}
	h := &fakeHistory{metrics: map[string]FileMetric{
		"a.go": {CommitCount: 3, LinesAdded: 8, LinesDeleted: 2, LastRevision: "cccccccccccccccccccccccccccccccccccccccc"},
	}}
	got, err := Build(raw, input, Request{Workspace: "/repo", FromRevision: "base", ToRevision: "head"}, h)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	if got.GraphArtifactDigest != wantDigest || got.FromRevision != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || got.ToRevision != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.Attribution != "FILE_PATH_ONLY" {
		t.Fatalf("identity/ceiling mismatch: %+v", got)
	}
	if got.NodeCount != 3 || len(got.Nodes) != 3 || got.ChangedNodeCount != 2 || got.ZeroChurnNodeCount != 1 {
		t.Fatalf("closed accounting mismatch: %+v", got)
	}
	if fmt.Sprint(h.gotPaths) != "[a.go b.go]" {
		t.Fatalf("paths not sorted and deduplicated: %v", h.gotPaths)
	}
	if got.Nodes[0].NodeID != "n1" || got.Nodes[0].CommitCount != 3 || got.Nodes[1].CommitCount != 3 || got.Nodes[2].CommitCount != 0 {
		t.Fatalf("node join mismatch: %+v", got.Nodes)
	}
}

func TestBuildRejectsNonV2OrNonConservativeInput(t *testing.T) {
	for _, input := range []tsr.LocatorResultV2{
		{SchemaVersion: "wrong", Authority: 0, SourceGraphComplete: "UNKNOWN"},
		{SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 1, SourceGraphComplete: "UNKNOWN"},
		{SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 0, SourceGraphComplete: "COMPLETE"},
	} {
		if _, err := Build([]byte("x"), input, Request{Workspace: "/repo", FromRevision: "a", ToRevision: "b"}, &fakeHistory{}); err == nil {
			t.Fatalf("accepted invalid input: %+v", input)
		}
	}
}
