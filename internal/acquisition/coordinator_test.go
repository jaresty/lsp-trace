package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"reflect"
	"testing"
	"time"
)

type fakeClient struct {
	items    map[string][]lsp.CallHierarchyItem
	symbols  map[string][]lsp.DocumentSymbol
	outgoing map[string][]lsp.CallHierarchyOutgoingCall
	incoming map[string][]lsp.CallHierarchyIncomingCall
	errors   map[string]error
	calls    []string
	hook     func(context.Context, string) error
}

func fixture() *fakeClient {
	return &fakeClient{items: map[string][]lsp.CallHierarchyItem{}, symbols: map[string][]lsp.DocumentSymbol{}, outgoing: map[string][]lsp.CallHierarchyOutgoingCall{}, incoming: map[string][]lsp.CallHierarchyIncomingCall{}, errors: map[string]error{}}
}
func (f *fakeClient) called(ctx context.Context, key string) error {
	f.calls = append(f.calls, key)
	if f.hook != nil {
		if e := f.hook(ctx, key); e != nil {
			return e
		}
	}
	return f.errors[key]
}
func (f *fakeClient) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	key := "symbols:" + p.TextDocument.URI
	return f.symbols[p.TextDocument.URI], f.called(ctx, key)
}
func (f *fakeClient) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	key := prepareKey(p)
	return f.items[key], f.called(ctx, key)
}
func (f *fakeClient) IncomingCalls(ctx context.Context, i lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	key := "in:" + i.Name
	return f.incoming[i.Name], f.incoming[i.Name] == nil, f.called(ctx, key)
}
func (f *fakeClient) OutgoingCalls(ctx context.Context, i lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	key := "out:" + i.Name
	return f.outgoing[i.Name], f.outgoing[i.Name] == nil, f.called(ctx, key)
}
func prepareKey(p lsp.PrepareCallHierarchyParams) string { b, _ := json.Marshal(p); return string(b) }
func item(name string, line uint32) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: name, Kind: 12, URI: "file:///a.go", Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 20}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 2}, End: lsp.Position{Line: line, Character: 6}}, Data: json.RawMessage(`{"opaque":"` + name + `"}`)}
}
func target(id string, i lsp.CallHierarchyItem) Target {
	l, c := i.SelectionRange.Start.Line, i.SelectionRange.Start.Character
	return Target{ID: id, Locator: Locator{URI: i.URI, Line: &l, Character: &c}, DownDepth: 3, UpDepth: 3}
}
func (f *fakeClient) add(i lsp.CallHierarchyItem) {
	p := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: i.URI}, Position: i.SelectionRange.Start}
	f.items[prepareKey(p)] = []lsp.CallHierarchyItem{i}
	f.symbols[i.URI] = append(f.symbols[i.URI], lsp.DocumentSymbol{Name: i.Name, Kind: i.Kind, Range: i.Range, SelectionRange: i.SelectionRange})
}
func (f *fakeClient) edge(a, b lsp.CallHierarchyItem) {
	sites := []lsp.Range{{Start: a.SelectionRange.Start, End: a.SelectionRange.End}}
	f.outgoing[a.Name] = append(f.outgoing[a.Name], lsp.CallHierarchyOutgoingCall{To: b, FromRanges: sites})
	f.incoming[b.Name] = append(f.incoming[b.Name], lsp.CallHierarchyIncomingCall{From: a, FromRanges: sites})
}
func request(a lsp.CallHierarchyItem, rest ...lsp.CallHierarchyItem) Request {
	r := Request{Mode: Slice, Context: AcquisitionContext{ID: "test", SessionID: "fake", Generation: 1, PositionEncoding: "utf-16"}, Root: target("root", a), Limits: Limits{MaxNodes: 100, MaxRequests: 100, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	for _, i := range rest {
		r.RequiredTargets = append(r.RequiredTargets, target(i.Name, i))
	}
	return r
}
func run(t *testing.T, f *fakeClient, r Request) Result {
	t.Helper()
	got, err := Acquire(context.Background(), f, r)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func TestTopmostSiblingExpansionDistinctSeedsSharesGlobalRequestBudget(t *testing.T) {
	f := fixture()
	a, b, peer := item("a", 0), item("b", 4), item("peer", 2)
	for _, i := range []lsp.CallHierarchyItem{a, b, peer} {
		f.add(i)
	}
	container := lsp.DocumentSymbol{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, SelectionRange: lsp.Range{}, Children: []lsp.DocumentSymbol{
		{Name: a.Name, Kind: a.Kind, Range: a.Range, SelectionRange: a.SelectionRange},
		{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
		{Name: b.Name, Kind: b.Kind, Range: b.Range, SelectionRange: b.SelectionRange},
	}}
	f.symbols[a.URI] = []lsp.DocumentSymbol{container}
	r := request(a, b)
	r.TopmostSiblings = true
	// Two seed prepares, then documentSymbol + sibling prepare for the first seed.
	// The second documentSymbol must observe the same exhausted global budget.
	r.Limits.MaxRequests = 4
	one := run(t, f, r)
	f2 := fixture()
	for _, i := range []lsp.CallHierarchyItem{a, b, peer} {
		f2.add(i)
	}
	f2.symbols[a.URI] = []lsp.DocumentSymbol{container}
	two := run(t, f2, r)
	if !reflect.DeepEqual(one, two) || one.Usage.Requests != 4 || len(one.Graph.SiblingCandidates) != 1 || len(one.Graph.Edges) != 0 || one.AcquisitionComplete {
		t.Fatalf("ASSERT_TOPMOST_MULTISEED_SHARED_REQUEST_BUDGET_DOCUMENT_SYMBOL_PREPARE: usage=%#v siblings=%d edges=%d complete=%t deterministic=%t calls=%v", one.Usage, len(one.Graph.SiblingCandidates), len(one.Graph.Edges), one.AcquisitionComplete, reflect.DeepEqual(one, two), f.calls)
	}
	blocked := false
	for _, rec := range one.Requests {
		if rec.Method == "textDocument/documentSymbol" && rec.Outcome == "BUDGET_BLOCKED" {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("ASSERT_TOPMOST_MULTISEED_PARTIAL_SECOND_SEED: requests=%#v", one.Requests)
	}
	if len(one.Omissions) != 2 || one.Omissions[0].Reason != OmissionRequestBudgetBlocked || one.Omissions[1].Reason != OmissionRequestBudgetBlocked || one.Omissions[0].RequestID == one.Omissions[1].RequestID {
		t.Fatalf("ASSERT_TOPMOST_REQUEST_BUDGET_TYPED_OMISSION: %#v", one.Omissions)
	}
}

func TestTopmostSiblingExactNodeCapacityIsComplete(t *testing.T) {
	f := fixture()
	seed, peer := item("seed", 0), item("peer", 2)
	f.add(seed)
	f.add(peer)
	f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
		{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange},
		{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
	}}}
	r := request(seed)
	r.TopmostSiblings = true
	r.Limits.MaxNodes = 2

	got, err := Acquire(context.Background(), f, r)
	if err != nil || !got.AcquisitionComplete || got.Usage.Nodes != r.Limits.MaxNodes || len(got.Graph.SiblingCandidates) != 1 {
		t.Fatalf("ASSERT_TOPMOST_EXACT_NODE_CAPACITY_COMPLETE: err=%v complete=%t usage=%#v siblings=%d", err, got.AcquisitionComplete, got.Usage, len(got.Graph.SiblingCandidates))
	}
	if err := ValidateResult(got); err != nil {
		t.Fatalf("ASSERT_TOPMOST_EXACT_NODE_CAPACITY_VALID: %v", err)
	}
	got.AcquisitionComplete = false
	if err := ValidateResult(got); err == nil {
		t.Fatal("ASSERT_TOPMOST_EXACT_NODE_CAPACITY_FALSE_INCOMPLETE_REJECTED")
	}
}

func TestTopmostSiblingRejectedAdmissionHasTypedOmission(t *testing.T) {
	f := fixture()
	seed, peer := item("seed", 0), item("peer", 2)
	f.add(seed)
	f.add(peer)
	f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
		{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange},
		{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
	}}}
	r := request(seed)
	r.TopmostSiblings = true
	r.Limits.MaxNodes = 1

	got := run(t, f, r)
	if got.AcquisitionComplete || got.Usage.Nodes != r.Limits.MaxNodes || len(got.Omissions) != 1 || got.Omissions[0].Reason != OmissionNodeAdmissionBlocked || got.Omissions[0].NodeID != node(peer).ID {
		t.Fatalf("ASSERT_TOPMOST_REJECTED_ADMISSION_TYPED_OMISSION: complete=%t usage=%#v omissions=%#v", got.AcquisitionComplete, got.Usage, got.Omissions)
	}
	if err := ValidateResult(got); err != nil {
		t.Fatalf("ASSERT_TOPMOST_REJECTED_ADMISSION_VALID: %v", err)
	}
	for _, mutation := range []string{"missing", "reason", "node"} {
		t.Run(mutation, func(t *testing.T) {
			copy := got
			copy.Omissions = append([]Omission(nil), got.Omissions...)
			switch mutation {
			case "missing":
				copy.Omissions = nil
			case "reason":
				copy.Omissions[0].Reason = "OTHER"
			case "node":
				copy.Omissions[0].NodeID = "substituted"
			}
			if err := ValidateResult(copy); err == nil {
				t.Fatalf("ASSERT_TOPMOST_REJECTED_ADMISSION_OMISSION_SUBSTITUTION_REJECTED_%s", mutation)
			}
		})
	}
}

func TestTopmostSiblingFailedPrepareMarksAcquisitionIncomplete(t *testing.T) {
	f := fixture()
	seed, peer := item("seed", 0), item("peer", 2)
	f.add(seed)
	f.add(peer)
	f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
		{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange},
		{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
	}}}
	peerPrepare := prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: peer.SelectionRange.Start})
	f.errors[peerPrepare] = errors.New("sibling preparation failed")
	r := request(seed)
	r.TopmostSiblings = true

	got, err := Acquire(context.Background(), f, r)
	if err != nil || got.AcquisitionComplete || len(got.Graph.SiblingCandidates) != 0 {
		t.Fatalf("ASSERT_TOPMOST_FAILED_SIBLING_PREPARE_INCOMPLETE: err=%v complete=%t siblings=%d", err, got.AcquisitionComplete, len(got.Graph.SiblingCandidates))
	}
	found := false
	for _, rec := range got.Requests {
		found = found || (rec.Method == "textDocument/prepareCallHierarchy" && rec.Outcome != "SUCCESS" && rec.Reason == "sibling preparation failed")
	}
	if !found {
		t.Fatalf("ASSERT_TOPMOST_FAILED_SIBLING_PREPARE_TYPED_RECORD: requests=%#v", got.Requests)
	}
	if len(got.Omissions) != 1 || got.Omissions[0].Reason != OmissionReason("PREPARATION_FAILED") {
		t.Fatalf("ASSERT_TOPMOST_FAILED_SIBLING_PREPARE_TYPED_OMISSION: %#v", got.Omissions)
	}
	if err := ValidateResult(got); err != nil {
		t.Fatalf("ASSERT_TOPMOST_FAILED_SIBLING_PREPARE_VALID: %v", err)
	}
}

func TestTopmostSiblingMatchAndDeclarationOmissionsAreTypedAndReplayBound(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason OmissionReason
		setup  func(*fakeClient, lsp.CallHierarchyItem)
	}{
		{name: "no_match", reason: OmissionNoMatch, setup: func(f *fakeClient, peer lsp.CallHierarchyItem) {
			delete(f.items, prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: peer.SelectionRange.Start}))
		}},
		{name: "ambiguous_match", reason: OmissionAmbiguousMatch, setup: func(f *fakeClient, peer lsp.CallHierarchyItem) {
			key := prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: peer.SelectionRange.Start})
			f.items[key] = []lsp.CallHierarchyItem{peer, peer}
		}},
		{name: "invalid_declaration", reason: OmissionInvalidDeclaration, setup: func(_ *fakeClient, _ lsp.CallHierarchyItem) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixture()
			seed, peer := item("seed", 0), item("peer", 2)
			f.add(seed)
			f.add(peer)
			declaration := lsp.DocumentSymbol{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange}
			if tc.reason == OmissionInvalidDeclaration {
				declaration.Range.End = lsp.Position{Line: 1}
			}
			f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
				{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange}, declaration,
			}}}
			tc.setup(f, peer)
			r := request(seed)
			r.TopmostSiblings = true
			got := run(t, f, r)
			if got.AcquisitionComplete || len(got.Omissions) != 1 || got.Omissions[0].Reason != tc.reason || got.Omissions[0].RequestID == "" {
				t.Fatalf("ASSERT_TOPMOST_%s_TYPED_OMISSION: complete=%t omissions=%#v", tc.name, got.AcquisitionComplete, got.Omissions)
			}
			if err := ValidateResult(got); err != nil {
				t.Fatalf("ASSERT_TOPMOST_%s_REPLAY_VALID: %v", tc.name, err)
			}
			for _, mutation := range []string{"missing", "reason", "request"} {
				mutated := got
				mutated.Omissions = append([]Omission(nil), got.Omissions...)
				switch mutation {
				case "missing":
					mutated.Omissions = nil
				case "reason":
					mutated.Omissions[0].Reason = OmissionNodeAdmissionBlocked
				case "request":
					mutated.Omissions[0].RequestID = "request-unknown"
				}
				if err := ValidateResult(mutated); err == nil {
					t.Fatalf("ASSERT_TOPMOST_%s_REPLAY_MUTATION_REJECTED_%s", tc.name, mutation)
				}
			}
		})
	}
}

func TestPartialAcquisitionFreezesHistoricalGraphDiagnosticAndTypedAccounting(t *testing.T) {
	f := fixture()
	a, b, peer := item("a", 0), item("b", 4), item("peer", 2)
	for _, i := range []lsp.CallHierarchyItem{a, b, peer} {
		f.add(i)
	}
	f.symbols[a.URI] = []lsp.DocumentSymbol{{
		Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
			{Name: a.Name, Kind: a.Kind, Range: a.Range, SelectionRange: a.SelectionRange},
			{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
			{Name: b.Name, Kind: b.Kind, Range: b.Range, SelectionRange: b.SelectionRange},
		},
	}}
	r := request(a, b)
	r.TopmostSiblings = true
	r.Limits.MaxRequests = 4
	got := run(t, f, r)

	t.Run("historical graph diagnostic", func(t *testing.T) {
		const historical = "MULTI_TARGET_ACQUISITION_PARTIAL: consult coordinator accounting"
		found := false
		for _, diagnostic := range got.Graph.Diagnostics {
			if diagnostic.Phase == "prepare" && diagnostic.Message == historical {
				found = true
			}
		}
		if !found {
			t.Fatalf("ASSERT_PARTIAL_ACQUISITION_HISTORICAL_DIAGNOSTIC: diagnostics=%#v", got.Graph.Diagnostics)
		}
	})

	t.Run("typed accounting", func(t *testing.T) {
		if got.AcquisitionComplete || len(got.Targets) != 2 || got.Usage.Requests != 4 || got.Usage.PrepareProbes != 3 || len(got.Graph.SiblingCandidates) != 1 {
			t.Fatalf("ASSERT_PARTIAL_ACQUISITION_TYPED_ACCOUNTING: complete=%t targets=%d usage=%#v siblings=%d", got.AcquisitionComplete, len(got.Targets), got.Usage, len(got.Graph.SiblingCandidates))
		}
		if got.Targets[0].Resolution.Status != Resolved || got.Targets[0].Admission != Admitted || got.Targets[1].Resolution.Status != Resolved || got.Targets[1].Admission != Admitted {
			t.Fatalf("ASSERT_PARTIAL_ACQUISITION_TARGET_ACCOUNTING: targets=%#v", got.Targets)
		}
	})
}

func TestFlatTopmostSiblingCorrespondenceUsesPreparedServerIdentity(t *testing.T) {
	seed, peer := item("seed", 0), item("peer", 2)
	setup := func(items []lsp.CallHierarchyItem) (*fakeClient, Request) {
		f := fixture()
		f.add(seed)
		f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: peer.SelectionRange.Start})] = items
		for character := uint32(0); character < peer.SelectionRange.Start.Character; character++ {
			position := lsp.Position{Line: peer.Range.Start.Line, Character: character}
			f.errors[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: position})] = errors.New("json-rpc error 0: identifier not found")
		}
		f.symbols[seed.URI] = []lsp.DocumentSymbol{
			{Name: "example." + seed.Name, Kind: 6, Range: seed.Range, SelectionRange: seed.Range, Flat: true},
			{Name: "example." + peer.Name, Kind: 6, Range: peer.Range, SelectionRange: peer.Range, Flat: true},
		}
		r := request(seed)
		r.TopmostSiblings = true
		return f, r
	}

	t.Run("exact", func(t *testing.T) {
		f, r := setup([]lsp.CallHierarchyItem{peer})
		got := run(t, f, r)
		if len(got.Graph.SiblingCandidates) != 1 {
			t.Fatalf("ASSERT_FLAT_SIBLING_RECONCILES_EXACT_PREPARED_IDENTITY: siblings=%#v calls=%v", got.Graph.SiblingCandidates, f.calls)
		}
		candidate := got.Graph.SiblingCandidates[0]
		if candidate.Declaration == nil || candidate.Declaration.Name != "example."+peer.Name || candidate.Declaration.Kind != 6 || candidate.Declaration.Range != toRange(peer.Range) || candidate.Declaration.SelectionRange != toRange(peer.SelectionRange) || candidate.Candidate.ID != node(peer).ID {
			t.Fatalf("ASSERT_FLAT_SIBLING_RETAINS_FUSED_DECLARATION_AND_PREPARED_IDENTITY: %#v", candidate)
		}
	})

	for _, tc := range []struct {
		name  string
		items []lsp.CallHierarchyItem
	}{
		{name: "selection-outside-declaration", items: []lsp.CallHierarchyItem{func() lsp.CallHierarchyItem {
			value := peer
			value.SelectionRange.Start.Line = 100
			value.SelectionRange.End.Line = 100
			return value
		}()}},
		{name: "ambiguous", items: []lsp.CallHierarchyItem{peer, peer}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, r := setup(tc.items)
			got, err := Acquire(context.Background(), f, r)
			if len(got.Graph.SiblingCandidates) != 0 || got.AcquisitionComplete {
				t.Fatalf("ASSERT_FLAT_SIBLING_FAILS_CLOSED_%s: err=%v siblings=%#v complete=%t", tc.name, err, got.Graph.SiblingCandidates, got.AcquisitionComplete)
			}
		})
	}
}

func TestFlatTopmostSiblingIdentifierMissExhaustionHasFailureOmission(t *testing.T) {
	seed, peer := item("seed", 0), item("peer", 2)
	setup := func() (*fakeClient, Request) {
		f := fixture()
		f.add(seed)
		f.add(peer)
		f.symbols[seed.URI] = []lsp.DocumentSymbol{
			{Name: "example." + seed.Name, Kind: 6, Range: seed.Range, SelectionRange: seed.Range, Flat: true},
			{Name: "example." + peer.Name, Kind: 6, Range: peer.Range, SelectionRange: peer.Range, Flat: true},
		}
		for probe := uint32(0); probe < MaxPrepareProbes; probe++ {
			position := peer.Range.Start
			position.Character += probe
			params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: peer.URI}, Position: position}
			f.errors[prepareKey(params)] = errors.New("json-rpc error 0: identifier not found")
		}
		r := request(seed)
		r.TopmostSiblings = true
		return f, r
	}

	f, r := setup()
	got, err := Acquire(context.Background(), f, r)
	if err != nil || got.AcquisitionComplete || len(got.Graph.SiblingCandidates) != 0 || len(got.Omissions) != 1 {
		t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_TYPED_OMISSION: err=%v complete=%t siblings=%d omissions=%#v", err, got.AcquisitionComplete, len(got.Graph.SiblingCandidates), got.Omissions)
	}
	omission := got.Omissions[0]
	if omission.Reason != OmissionPreparationFailed || omission.NodeID == "" || omission.RequestID == "" {
		t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_FAILURE_IDENTITY: %#v", omission)
	}
	failedRequest := false
	for _, record := range got.Requests {
		failedRequest = failedRequest || (record.ID == omission.RequestID && record.Method == "textDocument/prepareCallHierarchy" && record.Outcome == "FAILED")
	}
	if !failedRequest {
		t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_FAILED_REQUEST: omission=%#v requests=%#v", omission, got.Requests)
	}
	if err := ValidateResult(got); err != nil {
		t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_VALID: %v", err)
	}
	f2, _ := setup()
	replayed, err := Acquire(context.Background(), f2, r)
	if err != nil || !reflect.DeepEqual(got, replayed) {
		t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_DETERMINISTIC: err=%v equal=%t", err, reflect.DeepEqual(got, replayed))
	}
	for _, mutation := range []string{"reason", "request"} {
		mutated := got
		mutated.Omissions = append([]Omission(nil), got.Omissions...)
		switch mutation {
		case "reason":
			mutated.Omissions[0].Reason = OmissionNoMatch
		case "request":
			mutated.Omissions[0].RequestID = "request-unknown"
		}
		if err := ValidateResult(mutated); err == nil {
			t.Fatalf("ASSERT_FLAT_SIBLING_IDENTIFIER_MISS_EXHAUSTION_MUTATION_REJECTED_%s", mutation)
		}
	}
}

func TestTopmostSiblingCorrespondenceUsesSelectionIdentity(t *testing.T) {
	seed := item("seed", 0)
	declaration := lsp.DocumentSymbol{
		Name:           "GetSchoolFilterModel",
		Kind:           6,
		Range:          lsp.Range{Start: lsp.Position{Line: 3}, End: lsp.Position{Line: 9}},
		SelectionRange: lsp.Range{Start: lsp.Position{Line: 4, Character: 8}, End: lsp.Position{Line: 4, Character: 28}},
	}
	prepared := lsp.CallHierarchyItem{
		Name:           "GetSchoolFilterModel(string schoolId)",
		Kind:           declaration.Kind,
		URI:            seed.URI,
		Range:          lsp.Range{Start: lsp.Position{Line: 4, Character: 4}, End: lsp.Position{Line: 8}},
		SelectionRange: declaration.SelectionRange,
	}
	setup := func() (*fakeClient, Request) {
		f := fixture()
		f.add(seed)
		f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "container", Kind: 5, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
			{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange},
			declaration,
		}}}
		params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start}
		f.items[prepareKey(params)] = []lsp.CallHierarchyItem{prepared}
		r := request(seed)
		r.TopmostSiblings = true
		return f, r
	}

	t.Run("different names and full ranges correspond", func(t *testing.T) {
		f, r := setup()
		got, err := Acquire(context.Background(), f, r)
		if err != nil || len(got.Graph.SiblingCandidates) != 1 {
			t.Fatalf("ASSERT_SIBLING_SELECTION_IDENTITY_ACCEPTED: err=%v siblings=%#v calls=%v", err, got.Graph.SiblingCandidates, f.calls)
		}
		wantPrepare := prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})
		preparedAtSelectionStart := false
		for _, call := range f.calls {
			preparedAtSelectionStart = preparedAtSelectionStart || call == wantPrepare
		}
		if !preparedAtSelectionStart {
			t.Fatalf("ASSERT_SIBLING_PREPARE_AT_EXACT_SELECTION_START: want=%s calls=%v", wantPrepare, f.calls)
		}
		gotDeclaration := got.Graph.SiblingCandidates[0].Declaration
		if gotDeclaration.Name != declaration.Name || gotDeclaration.Range != toRange(declaration.Range) || gotDeclaration.SelectionRange != toRange(declaration.SelectionRange) || got.Graph.SiblingCandidates[0].Candidate.Name != prepared.Name || got.Graph.SiblingCandidates[0].Candidate.Range != toRange(prepared.Range) {
			t.Fatalf("ASSERT_SIBLING_RETAINS_DISTINCT_DECLARATION_AND_PREPARED_EVIDENCE: %#v", got.Graph.SiblingCandidates[0])
		}
	})

	cases := []struct {
		name   string
		mutate func(*fakeClient)
	}{
		{"uri", func(f *fakeClient) {
			p := prepared
			p.URI = "file:///other.go"
			f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})] = []lsp.CallHierarchyItem{p}
		}},
		{"selection", func(f *fakeClient) {
			p := prepared
			p.SelectionRange.Start.Character++
			f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})] = []lsp.CallHierarchyItem{p}
		}},
		{"kind", func(f *fakeClient) {
			p := prepared
			p.Kind++
			f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})] = []lsp.CallHierarchyItem{p}
		}},
		{"ambiguity", func(f *fakeClient) {
			p := prepared
			p.Name += " duplicate"
			f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})] = []lsp.CallHierarchyItem{prepared, p}
		}},
		{"empty", func(f *fakeClient) {
			f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.URI}, Position: declaration.SelectionRange.Start})] = []lsp.CallHierarchyItem{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, r := setup()
			tc.mutate(f)
			got, err := Acquire(context.Background(), f, r)
			if err == nil && (len(got.Graph.SiblingCandidates) != 0 || got.AcquisitionComplete) {
				t.Fatalf("ASSERT_SIBLING_CORRESPONDENCE_REJECTS_%s: siblings=%#v complete=%t", tc.name, got.Graph.SiblingCandidates, got.AcquisitionComplete)
			}
		})
	}
}

func TestTopmostSiblingExpansionSharedEvidenceNodeAndTimeBudgets(t *testing.T) {
	seed, peer := item("seed", 0), item("peer", 2)
	setup := func() *fakeClient {
		f := fixture()
		f.add(seed)
		f.add(peer)
		f.symbols[seed.URI] = []lsp.DocumentSymbol{{Name: "document", Kind: 2, Range: lsp.Range{End: lsp.Position{Line: 20}}, Children: []lsp.DocumentSymbol{
			{Name: seed.Name, Kind: seed.Kind, Range: seed.Range, SelectionRange: seed.SelectionRange},
			{Name: peer.Name, Kind: peer.Kind, Range: peer.Range, SelectionRange: peer.SelectionRange},
		}}}
		return f
	}
	baseRequest := request(seed)
	baseRequest.TopmostSiblings = true
	baseline := run(t, setup(), baseRequest)
	var siblingBefore int
	for _, rec := range baseline.Requests {
		if rec.Method == "textDocument/prepareCallHierarchy" && rec.Before.Requests > 1 {
			siblingBefore = rec.Before.EvidenceBytes
		}
	}
	if siblingBefore == 0 || len(baseline.Graph.SiblingCandidates) != 1 {
		t.Fatalf("ASSERT_TOPMOST_EVIDENCE_BASELINE: before=%d siblings=%d", siblingBefore, len(baseline.Graph.SiblingCandidates))
	}
	t.Run("evidence", func(t *testing.T) {
		r := baseRequest
		r.Limits.MaxEvidenceBytes = siblingBefore
		got := run(t, setup(), r)
		if len(got.Graph.SiblingCandidates) != 0 || got.Usage.EvidenceBytes > siblingBefore || got.AcquisitionComplete {
			t.Fatalf("ASSERT_TOPMOST_SHARED_EVIDENCE_BUDGET_DOCUMENT_SYMBOL_PREPARE: usage=%#v siblings=%d complete=%t", got.Usage, len(got.Graph.SiblingCandidates), got.AcquisitionComplete)
		}
	})
	t.Run("node", func(t *testing.T) {
		r := baseRequest
		r.Limits.MaxNodes = 1
		got := run(t, setup(), r)
		if len(got.Graph.SiblingCandidates) != 0 || got.Usage.Nodes != 1 || got.AcquisitionComplete {
			t.Fatalf("ASSERT_TOPMOST_SHARED_NODE_BUDGET_DOCUMENT_SYMBOL_PREPARE: usage=%#v siblings=%d complete=%t", got.Usage, len(got.Graph.SiblingCandidates), got.AcquisitionComplete)
		}
	})
	t.Run("request-time", func(t *testing.T) {
		f := setup()
		f.hook = func(ctx context.Context, key string) error {
			if key == "symbols:"+seed.URI {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		}
		r := baseRequest
		r.Limits.RequestTimeout = time.Millisecond
		got := run(t, f, r)
		if got.AcquisitionComplete || len(got.Graph.SiblingCandidates) != 0 {
			t.Fatalf("ASSERT_TOPMOST_SHARED_REQUEST_TIME_BUDGET_DOCUMENT_SYMBOL_PREPARE: complete=%t siblings=%d", got.AcquisitionComplete, len(got.Graph.SiblingCandidates))
		}
	})
}

func TestCoordinatorConnectedChain(t *testing.T) {
	f := fixture()
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	for _, i := range []lsp.CallHierarchyItem{a, b, c} {
		f.add(i)
	}
	f.edge(a, b)
	f.edge(b, c)
	got := run(t, f, request(a, c))
	if len(got.Targets) != 2 {
		t.Fatalf("every requested target retains accounting: got %d", len(got.Targets))
	}
	if got.Targets[1].Connection.Status != "FOUND" || len(got.Targets[1].Connection.Path.Nodes) != 3 {
		t.Fatalf("connected chain retains intermediate witness: %+v", got.Targets[1].Connection)
	}
	if len(got.Graph.Edges) != 2 {
		t.Fatalf("only native edges retained: %d", len(got.Graph.Edges))
	}
	if err := got.Graph.ValidateReferences(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got.Graph)
	if err != nil {
		t.Fatal(err)
	}
	if err = graph.ValidateSemanticBundle(raw); err != nil {
		t.Fatal(err)
	}
	for _, rec := range got.Requests {
		if rec.Attempted && !rec.CaptureComplete {
			t.Fatalf("complete fixture captures every request: %+v", rec)
		}
	}
}
func TestCoordinatorDisconnectedIsNotInvented(t *testing.T) {
	f := fixture()
	a, c := item("StartImport", 0), item("ImportSurveyFromWorkbook", 1)
	f.add(a)
	f.add(c)
	got := run(t, f, request(a, c))
	if len(got.Targets) != 2 {
		t.Fatal("all targets accounted")
	}
	if got.Targets[1].Connection.Status != "NOT_FOUND_IN_RETAINED_GRAPH" || len(got.Graph.Edges) != 0 {
		t.Fatal("endpoint presence must not fabricate connectivity")
	}
	if got.Targets[0].Outgoing.Status != SuccessEmpty {
		t.Fatal("successful null is empty expansion")
	}
}
func TestCoordinatorAliasSharedBudget(t *testing.T) {
	f := fixture()
	a := item("a", 0)
	f.add(a)
	r := request(a, a)
	got := run(t, f, r)
	if len(got.Targets) != 2 {
		t.Fatal("aliases retain requested rows")
	}
	if len(f.calls) != 3 {
		t.Fatalf("aliases share preparation and both direction queries: %v", f.calls)
	}
	if got.Targets[1].Connection.Status != "FOUND" || len(got.Targets[1].Connection.Path.Nodes) != 1 {
		t.Fatal("zero hop witness")
	}
	if got.Usage.Requests != 3 || got.Usage.Nodes != 1 {
		t.Fatal("shared work not multiplied")
	}
}
func TestCoordinatorRootFailureDoesNotStopRequired(t *testing.T) {
	f := fixture()
	a, c := item("a", 0), item("c", 1)
	f.add(c)
	got := run(t, f, request(a, c))
	if len(got.Targets) != 2 {
		t.Fatal("missing root accounted")
	}
	if got.Targets[0].Resolution.Status != Missing || got.Targets[1].Resolution.Status != Resolved {
		t.Fatal("required acquisition survives missing root")
	}
	if got.Targets[1].Connection.Status != "NOT_EVALUABLE" {
		t.Fatal("unresolved endpoint is not disconnected")
	}
}
func TestCoordinatorNodeCapNoDangling(t *testing.T) {
	f := fixture()
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	f.add(a)
	f.add(c)
	f.edge(a, b)
	r := request(a, c)
	r.Limits.MaxNodes = 2
	got := run(t, f, r)
	if len(got.Targets) != 2 {
		t.Fatal("cap retains accounting")
	}
	if len(got.Graph.Nodes) != 2 || len(got.Graph.Edges) != 0 {
		t.Fatal("all roots admitted before neighbors; no dangling edge at cap")
	}
	if got.Targets[0].Outgoing.Status != Partial {
		t.Fatal("node cap expansion is partial")
	}
}
func TestCoordinatorCancellationAndErrors(t *testing.T) {
	f := fixture()
	a := item("a", 0)
	f.add(a)
	f.errors["out:a"] = errors.New("SESSION_GENERATION_MISMATCH")
	got := run(t, f, request(a))
	if len(got.Targets) != 1 {
		t.Fatal("failure accounted")
	}
	if got.Targets[0].Outgoing.Status != Failed {
		t.Fatal("restart failure is not empty")
	}
	found := false
	for _, rec := range got.Requests {
		if rec.Reason == "SESSION_GENERATION_MISMATCH" {
			found = true
		}
	}
	if !found {
		t.Fatal("exact client failure retained")
	}
}
func TestCoordinatorDeterminism(t *testing.T) {
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	makeF := func() *fakeClient {
		f := fixture()
		for _, i := range []lsp.CallHierarchyItem{a, b, c} {
			f.add(i)
		}
		f.edge(a, b)
		f.edge(b, c)
		return f
	}
	r := request(a, c, b)
	r.Limits.MaxRequests = 6
	one, two := run(t, makeF(), r), run(t, makeF(), r)
	if !reflect.DeepEqual(one, two) {
		t.Fatal("identical inputs retain deterministic allocation and records")
	}
	if len(one.Targets) != 3 {
		t.Fatal("all target rows survive resource exhaustion")
	}
}
