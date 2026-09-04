package traverse

import (
	"context"
	"fmt"
	"testing"

	"lsp-trace/internal/lsp"
)

func BenchmarkIncomingEdgeGrowth(b *testing.B) {
	const edges = 1024
	items := make([]lsp.CallHierarchyItem, edges+1)
	calls := make(map[string][]lsp.CallHierarchyIncomingCall, edges+1)
	for i := range items {
		items[i] = item(fmt.Sprintf("node-%04d", i), uint32(i))
	}
	for i := 0; i < edges; i++ {
		calls[items[i].Name] = []lsp.CallHierarchyIncomingCall{{From: items[i+1], FromRanges: []lsp.Range{items[i+1].Range}}}
	}
	calls[items[edges].Name] = nil
	params := lsp.PrepareCallHierarchyParams{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := &fakeClient{targets: items[:1], calls: calls}
		got := Incoming(context.Background(), client, params, Options{MaxNodes: edges + 1})
		if len(got.Edges) != edges {
			b.Fatalf("edge count = %d, want %d; nodes=%d terminals=%+v diagnostics=%+v", len(got.Edges), edges, len(got.Nodes), got.Terminals, got.Diagnostics)
		}
	}
}
