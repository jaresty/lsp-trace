package traverse

import (
	"context"
	"testing"

	"lsp-trace/internal/lsp"
)

func TestTopmostSiblingClosedNearestContainerSemantics(t *testing.T) {
	sym := func(name string, kind int, line uint32, children ...lsp.DocumentSymbol) lsp.DocumentSymbol {
		i := item(name, line)
		return lsp.DocumentSymbol{Name: name, Kind: kind, Range: i.Range, SelectionRange: i.SelectionRange, Children: children}
	}
	cases := []struct {
		name    string
		seed    lsp.CallHierarchyItem
		symbols []lsp.DocumentSymbol
		want    []string
	}{
		{"top level document scope", func() lsp.CallHierarchyItem { s := item("origin", 1); s.Kind = 12; return s }(), []lsp.DocumentSymbol{sym("origin", 12, 1), sym("peer", 12, 2), sym("Type", 5, 3, sym("nested", 6, 4))}, []string{"peer"}},
		{"nested callable scope", func() lsp.CallHierarchyItem { s := item("local", 2); s.Kind = 12; return s }(), []lsp.DocumentSymbol{sym("outer", 12, 1, sym("local", 12, 2), sym("localPeer", 12, 3)), sym("documentPeer", 12, 4)}, []string{"localPeer"}},
		{"nearest type mixed callable scope", func() lsp.CallHierarchyItem { s := item("method", 2); s.Kind = 6; return s }(), []lsp.DocumentSymbol{sym("Type", 5, 1, sym("method", 6, 2), sym("ctor", 9, 3), sym("function", 12, 4), sym("field", 8, 5))}, []string{"ctor", "function"}},
		{"constructor origin", func() lsp.CallHierarchyItem { s := item("ctor", 2); s.Kind = 9; return s }(), []lsp.DocumentSymbol{sym("Type", 5, 1, sym("ctor", 9, 2), sym("method", 6, 3))}, []string{"method"}},
		{"non callable origin", func() lsp.CallHierarchyItem { s := item("field", 2); s.Kind = 8; return s }(), []lsp.DocumentSymbol{sym("Type", 5, 1, sym("field", 8, 2), sym("method", 6, 3))}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeClient{targets: []lsp.CallHierarchyItem{tc.seed}, calls: map[string][]lsp.CallHierarchyIncomingCall{tc.seed.Name: {}}, symbolSupported: true, documentSymbols: tc.symbols}
			got := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{IncludeTopmostSiblings: true})
			if len(got.SiblingCandidates) != len(tc.want) {
				t.Fatalf("ASSERT_CLOSED_NEAREST_CONTAINER_%s: got=%#v", tc.name, got.SiblingCandidates)
			}
			seen := map[string]bool{}
			for _, c := range got.SiblingCandidates {
				seen[c.Candidate.Name] = true
			}
			for _, want := range tc.want {
				if !seen[want] {
					t.Fatalf("ASSERT_CLOSED_NEAREST_CONTAINER_%s: missing=%s got=%#v", tc.name, want, got.SiblingCandidates)
				}
			}
		})
	}
}
