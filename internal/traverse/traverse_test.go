package traverse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type fakeClient struct {
	targets    []lsp.CallHierarchyItem
	prepareErr error
	calls      map[string][]lsp.CallHierarchyIncomingCall
	callErrs   map[string]error
	seenData   map[string]json.RawMessage
	expansions map[string]int
}

func (f *fakeClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return f.targets, f.prepareErr
}
func (f *fakeClient) IncomingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	if f.seenData != nil {
		f.seenData[item.Name] = append([]byte(nil), item.Data...)
	}
	if f.expansions != nil {
		f.expansions[item.Name]++
	}
	return f.calls[item.Name], false, f.callErrs[item.Name]
}
func item(name string, line uint32) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: "file:///w/a.go", Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 4}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 4}}, Data: json.RawMessage(`{"name":"` + name + `"}`)}
}

func TestIncomingBranchingDiamondAndOpaqueData(t *testing.T) {
	leaf, a, b, root := item("leaf", 8), item("a", 5), item("b", 6), item("root", 1)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, seenData: map[string]json.RawMessage{}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf": {{From: b, FromRanges: []lsp.Range{b.Range}}, {From: a, FromRanges: []lsp.Range{a.Range}}},
		"a":    {{From: root, FromRanges: []lsp.Range{root.Range}}}, "b": {{From: root, FromRanges: []lsp.Range{root.Range}}}, "root": {}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if !r.Summary.Complete || r.Summary.NodeCount != 4 || r.Summary.EdgeCount != 4 || r.Summary.TerminalCount != 1 {
		t.Fatalf("ASSERT_TRAVERSAL_COMPLETE_ACCOUNTED: %#v", r.Summary)
	}
	if string(f.seenData["leaf"]) != string(leaf.Data) {
		t.Fatalf("ASSERT_OPAQUE_DATA_REPLAYED: got %s want %s", f.seenData["leaf"], leaf.Data)
	}
	if r.Nodes[0].Name != "root" {
		t.Fatalf("ASSERT_CANONICAL_NODE_ORDER: first=%s", r.Nodes[0].Name)
	}
}

func TestIncomingDepthLimitCreatesFrontier(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: caller}}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{MaxDepth: 1})
	if r.Summary.Complete || !r.Summary.Truncated || len(r.Frontier) != 1 || r.Frontier[0].Reason != graph.MaxDepth {
		t.Fatalf("ASSERT_DEPTH_FRONTIER_VISIBLE: %#v %#v", r.Summary, r.Frontier)
	}
}

func TestIncomingCycleExpandsEachNodeOnceAndCountsSCC(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	f := &fakeClient{
		targets: []lsp.CallHierarchyItem{leaf},
		calls: map[string][]lsp.CallHierarchyIncomingCall{
			"leaf":   {{From: caller}},
			"caller": {{From: leaf}},
		},
		expansions: map[string]int{},
	}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if !r.Summary.Complete || r.Summary.CycleCount != 1 || f.expansions["leaf"] != 1 || f.expansions["caller"] != 1 {
		t.Fatalf("ASSERT_CYCLE_SAFE_ACCOUNTED: summary=%#v expansions=%#v", r.Summary, f.expansions)
	}
}

func TestIncomingMaxNodesPreservesObservedGraphAndFrontier(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: caller}}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{MaxNodes: 1})
	if r.Summary.Complete || !r.Summary.Truncated || r.Summary.NodeCount != 1 || r.Summary.EdgeCount != 0 || len(r.Frontier) != 1 || r.Frontier[0].Reason != graph.MaxNodes {
		t.Fatalf("ASSERT_MAX_NODES_ACCOUNTED: summary=%#v frontier=%#v", r.Summary, r.Frontier)
	}
}

func TestIncomingClassifiesPrepareAndBranchContextErrors(t *testing.T) {
	leaf := item("leaf", 8)
	for _, tc := range []struct {
		name string
		err  error
		want graph.Reason
	}{
		{name: "cancelled", err: context.Canceled, want: graph.Cancelled},
		{name: "deadline", err: context.DeadlineExceeded, want: graph.RequestTimeout},
	} {
		t.Run("prepare_"+tc.name, func(t *testing.T) {
			r := Incoming(context.Background(), &fakeClient{prepareErr: tc.err}, lsp.PrepareCallHierarchyParams{}, Options{})
			if r.Summary.Complete || len(r.Terminals) != 1 || r.Terminals[0].Reason != tc.want {
				t.Fatalf("ASSERT_PREPARE_CONTEXT_CLASSIFIED: want=%s result=%#v", tc.want, r)
			}
		})
		t.Run("branch_"+tc.name, func(t *testing.T) {
			r := Incoming(context.Background(), &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{}, callErrs: map[string]error{"leaf": tc.err}}, lsp.PrepareCallHierarchyParams{}, Options{})
			if r.Summary.Complete || len(r.Terminals) != 1 || r.Terminals[0].Reason != tc.want {
				t.Fatalf("ASSERT_BRANCH_CONTEXT_CLASSIFIED: want=%s result=%#v", tc.want, r)
			}
		})
	}
}

func TestIncomingShuffledResponsesProduceIdenticalCanonicalJSON(t *testing.T) {
	leaf, a, b := item("leaf", 8), item("a", 5), item("b", 6)
	run := func(calls []lsp.CallHierarchyIncomingCall) []byte {
		f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": calls, "a": {}, "b": {}}}
		r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	forward := run([]lsp.CallHierarchyIncomingCall{{From: a}, {From: b}})
	reverse := run([]lsp.CallHierarchyIncomingCall{{From: b}, {From: a}})
	if !bytes.Equal(forward, reverse) {
		t.Fatalf("ASSERT_SHUFFLED_RESPONSES_STABLE: %s != %s", forward, reverse)
	}
}

func TestIncomingPrepareGenericErrorRemainsDiagnostic(t *testing.T) {
	r := Incoming(context.Background(), &fakeClient{prepareErr: errors.New("boom")}, lsp.PrepareCallHierarchyParams{}, Options{})
	if r.Summary.Complete || len(r.Diagnostics) != 1 || r.Diagnostics[0].Message != "boom" {
		t.Fatalf("ASSERT_PREPARE_ERROR_DIAGNOSTIC: %#v", r)
	}
}
