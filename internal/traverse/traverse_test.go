package traverse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

type fakeClient struct {
	targets         []lsp.CallHierarchyItem
	prepareErr      error
	calls           map[string][]lsp.CallHierarchyIncomingCall
	callErrs        map[string]error
	seenData        map[string]json.RawMessage
	expansions      map[string]int
	documentSymbols []lsp.DocumentSymbol
	symbolSupported bool
	symbolRequests  int
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
func (f *fakeClient) SupportsDocumentSymbols() bool { return f.symbolSupported }
func (f *fakeClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	f.symbolRequests++
	return f.documentSymbols, nil
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

func TestIncomingRejectsMalformedPreparedItem(t *testing.T) {
	malformed := item("", 8)
	r := Incoming(context.Background(), &fakeClient{targets: []lsp.CallHierarchyItem{malformed}}, lsp.PrepareCallHierarchyParams{}, Options{})
	if r.Summary.Complete || len(r.Nodes) != 0 || len(r.Terminals) != 1 || r.Terminals[0].Reason != graph.InvalidServerResponse {
		t.Fatalf("ASSERT_INVALID_PREPARED_ITEM_REJECTED: %#v", r)
	}
}

func TestIncomingCancellationAccountsEveryQueuedNode(t *testing.T) {
	leaf, a, b := item("leaf", 8), item("a", 5), item("b", 6)
	f := &cancellingClient{fakeClient: fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf": {{From: a}, {From: b}},
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	r := Incoming(ctx, f, lsp.PrepareCallHierarchyParams{}, Options{})
	frontier := map[string]graph.Reason{}
	for _, boundary := range r.Frontier {
		frontier[boundary.NodeID] = boundary.Reason
	}
	if r.Summary.Complete || len(frontier) != 2 {
		t.Fatalf("ASSERT_CANCELLED_FRONTIER_COMPLETE: frontier=%#v result=%#v", frontier, r)
	}
	for id, reason := range frontier {
		if id == "" || reason != graph.Cancelled {
			t.Fatalf("ASSERT_CANCELLED_FRONTIER_REASON: %#v", frontier)
		}
	}
}

type cancellingClient struct {
	fakeClient
	cancel context.CancelFunc
}

func (f *cancellingClient) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	calls, wasNull, err := f.fakeClient.IncomingCalls(ctx, item)
	if item.Name == "leaf" {
		f.cancel()
	}
	return calls, wasNull, err
}

func TestIncomingDeadlineAccountsEveryQueuedNodeAsGlobalTimeout(t *testing.T) {
	leaf, a, b := item("leaf", 8), item("a", 5), item("b", 6)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	f := &deadlineClient{fakeClient: fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf": {{From: a}, {From: b}},
	}}}
	r := Incoming(ctx, f, lsp.PrepareCallHierarchyParams{}, Options{})
	if len(r.Frontier) != 2 {
		t.Fatalf("ASSERT_GLOBAL_TIMEOUT_FRONTIER_COMPLETE: %#v", r)
	}
	for _, boundary := range r.Frontier {
		if boundary.Reason != graph.GlobalTimeout {
			t.Fatalf("ASSERT_GLOBAL_TIMEOUT_FRONTIER_REASON: %#v", r.Frontier)
		}
	}
}

type deadlineClient struct{ fakeClient }

func (f *deadlineClient) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	calls, wasNull, err := f.fakeClient.IncomingCalls(ctx, item)
	if item.Name == "leaf" {
		<-ctx.Done()
	}
	return calls, wasNull, err
}

func TestIncomingInjectedNodeIDCollisionIsVisibleAndNotMerged(t *testing.T) {
	leaf, a, b := item("leaf", 8), item("a", 5), item("b", 6)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: a}, {From: b}}, "a": {}}}
	factory := func(i graph.Item) graph.Node {
		n := graph.NewNode(i)
		if i.Name == "a" || i.Name == "b" {
			n.ID = "collision"
		}
		return n
	}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{NodeFactory: factory})
	collisions := 0
	for _, boundary := range r.Terminals {
		if boundary.Reason == graph.NodeIDCollision {
			collisions++
		}
	}
	if r.Summary.Complete || collisions != 1 || r.Summary.NodeCount != 2 {
		t.Fatalf("ASSERT_INJECTED_COLLISION_REJECTED: %#v", r)
	}
}

func TestIncomingRejectsMalformedCallSiteRange(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	bad := lsp.Range{Start: lsp.Position{Line: 5}, End: lsp.Position{Line: 4}}
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: caller, FromRanges: []lsp.Range{bad}}}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if r.Summary.Complete || r.Summary.EdgeCount != 0 || len(r.Diagnostics) != 1 || len(r.Terminals) != 1 || r.Terminals[0].Reason != graph.InvalidServerResponse {
		t.Fatalf("ASSERT_MALFORMED_CALL_SITE_REJECTED: %#v", r)
	}
}

func TestIncomingRetainsCallSiteOutsideCallerItemRange(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	outside := lsp.Range{Start: lsp.Position{Line: 5, Character: 2}, End: lsp.Position{Line: 5, Character: 6}}
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf":   {{From: caller, FromRanges: []lsp.Range{outside}}},
		"caller": {},
	}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if !r.Summary.Complete || r.Summary.EdgeCount != 1 || len(r.Edges[0].CallSites) != 1 {
		t.Fatalf("ASSERT_OUTSIDE_CALL_SITE_EDGE_RETAINED: %#v", r)
	}
	if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE") {
		t.Fatalf("ASSERT_OUTSIDE_CALL_SITE_WARNING: %#v", r.Diagnostics)
	}
}

func TestIncomingSchemaV2CompletenessTerminalProvenanceAndQuality(t *testing.T) {
	leaf, caller := item("leaf", 8), item("caller", 4)
	caller.URI = "file:///w/other.go"
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {{From: caller}}, "caller": {}}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != "lsp-trace.graph.v2" {
		t.Fatalf("ASSERT_V2_SCHEMA_VERSION: %s", encoded)
	}
	summary := got["summary"].(map[string]any)
	if _, found := summary["complete"]; found {
		t.Fatalf("ASSERT_V2_REMOVES_UNQUALIFIED_COMPLETE: %s", encoded)
	}
	if summary["traversal_complete"] != true || summary["source_graph_complete"] != "UNKNOWN" || summary["completeness_scope"] != "SERVER_REPORTED_CALL_HIERARCHY" {
		t.Fatalf("ASSERT_V2_SCOPED_COMPLETENESS: %s", encoded)
	}
	quality := got["capability_quality"].(map[string]any)
	if quality["prepare_succeeded"] != true || quality["incoming_request_successes"] != float64(2) || quality["incoming_edges"] != float64(1) || quality["cross_file_edges"] != float64(1) || quality["cross_module_edges"] != "UNKNOWN" {
		t.Fatalf("ASSERT_V2_CAPABILITY_QUALITY: %s", encoded)
	}
	terminals := got["terminals"].([]any)
	terminal := terminals[0].(map[string]any)
	if terminal["reason"] != "SERVER_REPORTED_NO_INCOMING_CALLS" || terminal["provenance"] != "SERVER_REPORTED" {
		t.Fatalf("ASSERT_V2_TERMINAL_PROVENANCE: %s", encoded)
	}
}

func TestIncomingSchemaV1CompatibilityProjection(t *testing.T) {
	leaf := item("leaf", 8)
	r := Incoming(context.Background(), &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {}}}, lsp.PrepareCallHierarchyParams{}, Options{SchemaVersion: "lsp-trace.graph.v1"})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"complete":true`)) || !bytes.Contains(encoded, []byte(`"reason":"NO_INCOMING_CALLS"`)) || bytes.Contains(encoded, []byte(`capability_quality`)) {
		t.Fatalf("ASSERT_V1_COMPATIBILITY: %s", encoded)
	}
}

func TestIncomingCanonicalGoldenByteIdentical(t *testing.T) {
	leaf := item("leaf", 8)
	r := Incoming(context.Background(), &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {}}}, lsp.PrepareCallHierarchyParams{}, Options{})
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema_version":"lsp-trace.graph.v2","invocation":{"workspace_uri":"","target":{"uri":"","line":0,"column":0},"server":{"command":"","arguments":null},"limits":{"max_depth":0,"max_nodes":0,"timeout_ms":0}},"capabilities":{"call_hierarchy_provider":false},"capability_quality":{"advertised":false,"prepare_succeeded":true,"incoming_request_successes":1,"incoming_edges":0,"cross_file_edges":0,"cross_module_edges":"UNKNOWN"},"targets":["f170a06be92aae4db099707bbfd2b3773f84f9f159e5e49d1cb0a9f1bce158af"],"nodes":[{"id":"f170a06be92aae4db099707bbfd2b3773f84f9f159e5e49d1cb0a9f1bce158af","name":"leaf","kind":12,"uri":"file:///w/a.go","range":{"start":{"line":8,"character":0},"end":{"line":8,"character":4}},"selection_range":{"start":{"line":8,"character":0},"end":{"line":8,"character":4}},"data":{"name":"leaf"}}],"edges":null,"terminals":[{"node_id":"f170a06be92aae4db099707bbfd2b3773f84f9f159e5e49d1cb0a9f1bce158af","reason":"SERVER_REPORTED_NO_INCOMING_CALLS","provenance":"SERVER_REPORTED"}],"frontier":null,"diagnostics":null,"summary":{"node_count":1,"edge_count":0,"terminal_count":1,"cycle_count":0,"traversal_complete":true,"source_graph_complete":"UNKNOWN","completeness_scope":"SERVER_REPORTED_CALL_HIERARCHY","truncated":false}}`
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("ASSERT_CANONICAL_GOLDEN_BYTES: got %s", got)
	}
}

func TestIncomingTopmostSiblingCandidatesAreOptInSeparateAndCanonical(t *testing.T) {
	leaf := item("leaf", 8)
	rootB := lsp.DocumentSymbol{Name: "B", Kind: 12, Range: item("B", 20).Range, SelectionRange: item("B", 20).SelectionRange}
	rootA := lsp.DocumentSymbol{Name: "A", Kind: 12, Range: item("A", 1).Range, SelectionRange: item("A", 1).SelectionRange,
		Children: []lsp.DocumentSymbol{{Name: "nested", Kind: 12, Range: item("nested", 2).Range, SelectionRange: item("nested", 2).SelectionRange}}}
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {}}, symbolSupported: true,
		documentSymbols: []lsp.DocumentSymbol{rootB, rootA, rootA}}

	params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: leaf.URI}}
	baseline := Incoming(context.Background(), f, params, Options{})
	if len(baseline.SiblingCandidates) != 0 || f.symbolRequests != 0 || baseline.Summary.EdgeCount != 0 {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_DEFAULT_OFF_AND_CALLS_UNCHANGED: candidates=%#v requests=%d summary=%#v", baseline.SiblingCandidates, f.symbolRequests, baseline.Summary)
	}

	got := Incoming(context.Background(), f, params, Options{IncludeTopmostSiblings: true})
	if f.symbolRequests != 1 || len(got.SiblingCandidates) != 2 {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_OPT_IN_ROOTS_ONLY: requests=%d candidates=%#v", f.symbolRequests, got.SiblingCandidates)
	}
	if got.SiblingCandidates[0].Candidate.Name != "A" || got.SiblingCandidates[1].Candidate.Name != "B" {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_CANONICAL_UNIQUE: %#v", got.SiblingCandidates)
	}
	if got.Summary.EdgeCount != 0 || len(got.Edges) != 0 {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_NOT_CALL_EDGES: %#v", got.Edges)
	}
}

func TestIncomingTopmostSiblingCandidatesSkipUnsupportedServer(t *testing.T) {
	leaf := item("leaf", 8)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{"leaf": {}}, documentSymbols: []lsp.DocumentSymbol{{Name: "A"}}}
	got := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{IncludeTopmostSiblings: true})
	if f.symbolRequests != 0 || len(got.SiblingCandidates) != 0 {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_UNSUPPORTED_NO_REQUEST: requests=%d candidates=%#v", f.symbolRequests, got.SiblingCandidates)
	}
}

func TestIncomingRejectsMalformedCallerAndContinues(t *testing.T) {
	leaf, valid, malformed := item("leaf", 8), item("valid", 4), item("", 2)
	f := &fakeClient{targets: []lsp.CallHierarchyItem{leaf}, calls: map[string][]lsp.CallHierarchyIncomingCall{
		"leaf":  {{From: malformed}, {From: valid}},
		"valid": {},
	}}
	r := Incoming(context.Background(), f, lsp.PrepareCallHierarchyParams{}, Options{})
	if r.Summary.Complete || r.Summary.NodeCount != 2 || r.Summary.EdgeCount != 1 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Method != "callHierarchy/incomingCalls" {
		t.Fatalf("ASSERT_INVALID_CALLER_REJECTED_BRANCH_CONTINUES: %#v", r)
	}
	found := false
	for _, terminal := range r.Terminals {
		if terminal.Reason == graph.InvalidServerResponse {
			found = true
		}
	}
	if !found {
		t.Fatalf("ASSERT_INVALID_CALLER_BOUNDARY_VISIBLE: %#v", r.Terminals)
	}
}
