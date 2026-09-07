package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/retainedpath"
	"lsp-trace/internal/schema"
)

func TestExactResolution(t *testing.T) {
	for _, name := range []string{"symbol", "namesake", "wrong-source", "wrong-selection", "malformed-symbol", "positional-ambiguous", "duplicate-identities", "symbol-range-bound", "global-probe-bound", "root-ambiguous"} {
		t.Run(name, func(t *testing.T) {
			f := fixture()
			a, b := item("a", 0), item("a", 1)
			f.add(a)
			r := request(a)
			r.Root.Locator = Locator{URI: a.URI, Symbol: a.Name}
			want := Resolved
			switch name {
			case "namesake", "root-ambiguous":
				f.add(b)
				want = Ambiguous
			case "wrong-source":
				bad := a
				bad.URI = "file:///b.go"
				f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.URI}, Position: a.SelectionRange.Start})] = []lsp.CallHierarchyItem{bad}
				want = ResolutionFailed
			case "wrong-selection":
				bad := a
				bad.SelectionRange.End.Character++
				f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.URI}, Position: a.SelectionRange.Start})] = []lsp.CallHierarchyItem{bad}
				want = ResolutionFailed
			case "malformed-symbol":
				f.symbols[a.URI][0].Range.End = lsp.Position{}
				want = ResolutionFailed
			case "positional-ambiguous":
				r.Root = target("root", a)
				b := a
				b.Detail = "distinct"
				f.items[prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.URI}, Position: a.SelectionRange.Start})] = []lsp.CallHierarchyItem{a, b}
				r.Limits.MaxNodes = 1
				want = Ambiguous
			case "duplicate-identities":
				p := prepareKey(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: a.URI}, Position: a.SelectionRange.Start})
				f.items[p] = []lsp.CallHierarchyItem{a, a}
			case "symbol-range-bound":
				f.items = map[string][]lsp.CallHierarchyItem{}
				f.symbols[a.URI][0].Range.End.Character = 6
				want = Missing
			case "global-probe-bound":
				f.items = map[string][]lsp.CallHierarchyItem{}
				f.symbols[a.URI][0].Range.End.Character = 1000
				r.RequiredTargets = []Target{target("required", b)}
				want = ResolutionBlocked
			}
			if name == "root-ambiguous" {
				b.Name = "valid"
				b.URI = "file:///b.go"
				f.add(b)
				r.RequiredTargets = []Target{target("required", b)}
			}
			got := run(t, f, r)
			if got.Targets[0].Resolution.Status != want {
				t.Fatalf("exact resolution %s: got %+v want %s", name, got.Targets[0].Resolution, want)
			}
			if name == "root-ambiguous" && got.Targets[1].Resolution.Status != Resolved {
				t.Fatal("root ambiguity must not stop required resolution")
			}
			if name == "symbol-range-bound" && got.Usage.PrepareProbes != 4 {
				t.Fatalf("probe must stay within range: %d", got.Usage.PrepareProbes)
			}
			if name == "global-probe-bound" && (got.Usage.PrepareProbes != 65 || got.Targets[1].Resolution.Status != ResolutionBlocked) {
				t.Fatal("65 probes shared globally")
			}
			if want == Resolved {
				rr := got.Targets[0].Resolution
				if !reflect.DeepEqual(*rr.Prepared, a) || rr.Identity.ID != node(a).ID {
					t.Fatal("exact identity and opaque data preserved")
				}
			}
		})
	}
}
func TestSourceQualifiedNames(t *testing.T) {
	f := fixture()
	a, b := item("same", 0), item("same", 0)
	b.URI = "file:///b.go"
	f.add(a)
	f.add(b)
	r := request(a, b)
	got := run(t, f, r)
	if len(got.Graph.Nodes) != 2 || got.Targets[0].Resolution.Identity.ID == got.Targets[1].Resolution.Identity.ID {
		t.Fatal("same spelling different URI must not coalesce")
	}
}
func TestBudgetAccounting(t *testing.T) {
	for _, name := range []string{"requests-zero", "requests-resolution-only", "bytes-zero", "bytes-partial-response", "path-zero", "nodes-zero", "nodes-one", "shared-directions"} {
		t.Run(name, func(t *testing.T) {
			f := fixture()
			a, b := item("a", 0), item("b", 1)
			f.add(a)
			f.add(b)
			f.edge(a, b)
			r := request(a, b)
			switch name {
			case "requests-zero":
				r.Limits.MaxRequests = 0
			case "requests-resolution-only":
				r.Limits.MaxRequests = 2
			case "bytes-zero":
				r.Limits.MaxEvidenceBytes = 0
			case "bytes-partial-response":
				r.Limits.MaxEvidenceBytes = 120
			case "path-zero":
				r.Limits.MaxPathWork = 0
			case "nodes-zero":
				r.Limits.MaxNodes = 0
			case "nodes-one":
				r.Limits.MaxNodes = 1
			case "shared-directions":
				r.Limits.MaxRequests = 4
			}
			got := run(t, f, r)
			u := got.Usage
			l := r.Limits
			if u.Requests > l.MaxRequests || u.Nodes > l.MaxNodes || u.EvidenceBytes > l.MaxEvidenceBytes || u.PathWork > l.MaxPathWork || len(got.Targets) != 2 {
				t.Fatal("shared hard limits and denominator")
			}
			attempts := 0
			for _, rec := range got.Requests {
				if rec.Attempted {
					attempts++
				}
			}
			if attempts != len(f.calls) || attempts != u.Requests {
				t.Fatal("all attempts charged, no cached/fabricated request")
			}
			if name == "path-zero" && got.Targets[1].Connection.Status != "INCOMPLETE" {
				t.Fatal("pathwork exhaustion is not not-found")
			}
			if name == "nodes-one" && (got.Targets[1].Resolution.Status != Resolved || got.Targets[1].Admission != AdmissionBlocked) {
				t.Fatal("resolution independent of node admission")
			}
			if name == "bytes-partial-response" && (got.Requests[0].Outcome != "CAPTURE_INCOMPLETE" || got.Requests[0].CaptureComplete) {
				t.Fatal("byte cap rejects incomplete response capture")
			}
			if name == "shared-directions" {
				for _, call := range f.calls {
					if strings.HasPrefix(call, "in:") {
						t.Fatal("outgoing/incoming cannot multiply max requests")
					}
				}
			}
			raw, e := json.Marshal(got.Graph)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = schema.Validate(raw, "v3"); e != nil {
				t.Fatalf("native schema survives %s: %v", name, e)
			}
		})
	}
}
func TestAncestorDepthAliasCache(t *testing.T) {
	f := fixture()
	a, b, c, d := item("a", 0), item("b", 1), item("c", 2), item("d", 3)
	for _, i := range []lsp.CallHierarchyItem{a, b, c, d} {
		f.add(i)
	}
	f.edge(a, b)
	f.edge(b, c)
	f.edge(c, d)
	r := request(a, b)
	r.Root.DownDepth = 1
	r.RequiredTargets[0].DownDepth = 3
	got := run(t, f, r)
	if !has(got.Targets[0].Outgoing.FrontierIDs, node(b).ID) {
		t.Fatal("ancestor frontier retained at target-local depth")
	}
	if len(got.Targets[1].Outgoing.Layers) != 3 || !has(got.Targets[1].Outgoing.Layers[2].NodeIDs, node(d).ID) {
		t.Fatal("shallower alias traverses its own depth without reprepare")
	}
	seen := map[string]int{}
	for _, call := range f.calls {
		seen[call]++
	}
	for call, n := range seen {
		if n != 1 {
			t.Fatalf("query cache replay must not issue %s %d times", call, n)
		}
	}
	if !has(got.Targets[0].Incoming.StartIDs, node(b).ID) || !has(got.Targets[1].Incoming.StartIDs, node(d).ID) {
		t.Fatal("incoming starts are actual frontier/empty leaves")
	}
}
func TestTargetRoundRobinAndRootAdmission(t *testing.T) {
	f := fixture()
	a, b, c, d := item("a", 0), item("b", 1), item("c", 2), item("d", 3)
	f.add(a)
	f.add(c)
	f.edge(a, b)
	f.edge(c, d)
	r := request(a, c)
	r.Limits.MaxRequests = 4
	got := run(t, f, r)
	if len(f.calls) != 4 || f.calls[2] != "out:a" || f.calls[3] != "out:c" {
		t.Fatalf("round robin starts before descendants: %v", f.calls)
	}
	if got.Targets[0].Outgoing.Expansions[0].Status != SuccessNonempty || got.Targets[1].Outgoing.Expansions[0].Status != SuccessNonempty {
		t.Fatal("both target queues served")
	}
}
func TestErrorsNullAndMalformed(t *testing.T) {
	for _, name := range []string{"null", "error", "unsupported", "malformed-site", "malformed-item"} {
		t.Run(name, func(t *testing.T) {
			f := fixture()
			a, b := item("a", 0), item("b", 1)
			f.add(a)
			want := SuccessEmpty
			switch name {
			case "error":
				f.errors["out:a"] = errors.New("provider broke")
				want = Failed
			case "unsupported":
				f.errors["out:a"] = errors.New("json-rpc error -32601: unsupported")
				want = Unsupported
			case "malformed-site":
				f.edge(a, b)
				f.outgoing[a.Name][0].FromRanges[0].End = lsp.Position{}
				want = Partial
			case "malformed-item":
				f.edge(a, b)
				f.outgoing[a.Name][0].To.Range.End = lsp.Position{}
				want = Partial
			}
			got := run(t, f, request(a))
			if got.Targets[0].Outgoing.Status != want {
				t.Fatalf("outcome distinction: got %s want %s", got.Targets[0].Outgoing.Status, want)
			}
			if len(got.Graph.Edges) != 0 {
				t.Fatal("malformed or absent evidence must not create edges")
			}
		})
	}
}
func TestContextDeadlinesAndCancellation(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := fixture()
	f.add(a)
	f.add(b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, e := Acquire(ctx, f, request(a, b))
	if e != nil {
		t.Fatal(e)
	}
	if len(f.calls) != 0 || len(got.Targets) != 2 || got.Targets[1].Resolution.Status != ResolutionBlocked {
		t.Fatal("cancelled unattempted rows retained")
	}
	f = fixture()
	f.add(a)
	f.hook = func(ctx context.Context, key string) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 20*time.Millisecond {
			t.Fatal("per-request deadline missing")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	r := request(a)
	r.Limits.RequestTimeout = time.Millisecond
	got = run(t, f, r)
	if got.Targets[0].Resolution.Status != ResolutionFailed || got.Requests[0].Reason != context.DeadlineExceeded.Error() {
		t.Fatal("per-request timeout is a charged failed request")
	}
}
func TestIncomingDirectionAndKernelParity(t *testing.T) {
	f := fixture()
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	for _, i := range []lsp.CallHierarchyItem{a, b, c} {
		f.add(i)
	}
	f.edge(a, b)
	f.edge(b, c)
	r := request(c, a)
	r.Mode = Incoming
	got := run(t, f, r)
	if got.Targets[1].Connection.Status != "FOUND" {
		t.Fatal("incoming connects required to root")
	}
	for _, call := range f.calls {
		if strings.HasPrefix(call, "out:") {
			t.Fatal("incoming must not issue outgoing")
		}
	}
	nodes, edges := PathInput(got.Graph, r.Context.ID)
	budget := &retainedpath.Budget{Context: context.Background(), Left: r.Limits.MaxPathWork}
	want, e := retainedpath.Search(nodes, edges, node(a).ID, node(c).ID, budget)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Targets[1].Connection.Path, want.Path) || got.Usage.PathWork != r.Limits.MaxPathWork-budget.Left {
		t.Fatal("same kernel exact witness and work")
	}
	for _, occ := range want.Path.OccurrenceIDs {
		if len(occ) != 1 || !strings.HasPrefix(occ[0], "/edges/") {
			t.Fatal("native occurrence pointer required")
		}
	}
}
func TestPathPartialAcquisitionAndSharedWork(t *testing.T) {
	f := fixture()
	a, b, c := item("a", 0), item("b", 1), item("c", 2)
	f.add(a)
	f.add(b)
	f.add(c)
	f.errors["out:a"] = errors.New("partial")
	got := run(t, f, request(a, b, c))
	if got.AcquisitionComplete || got.Targets[1].Connection.Status != "NOT_FOUND_IN_RETAINED_GRAPH" {
		t.Fatal("exhaustive retained negative independent from failed acquisition")
	}
	r := request(a, a, a)
	r.RequiredTargets[0].ID = "one"
	r.RequiredTargets[1].ID = "two"
	r.Limits.MaxPathWork = 1
	got = run(t, f, r)
	if got.Targets[1].Connection.Status != "FOUND" || got.Targets[2].Connection.Status != "INCOMPLETE" || got.Usage.PathWork != 1 {
		t.Fatal("zero-hop queries share pathwork")
	}
}
func TestInvalidRequestNoClientCalls(t *testing.T) {
	for _, name := range []string{"duplicate", "reserved-root", "empty", "partial-position", "both", "relative", "noncanonical", "too-many", "generation", "encoding"} {
		t.Run(name, func(t *testing.T) {
			a := item("a", 0)
			f := fixture()
			r := request(a, a)
			switch name {
			case "duplicate":
				r.RequiredTargets = append(r.RequiredTargets, r.RequiredTargets[0])
			case "reserved-root":
				r.RequiredTargets[0].ID = "root"
			case "empty":
				r.RequiredTargets[0].ID = ""
			case "partial-position":
				r.Root.Locator.Character = nil
			case "both":
				r.Root.Locator.Symbol = "a"
			case "relative":
				r.Root.Locator.URI = "a.go"
			case "noncanonical":
				r.Root.Locator.URI = "file:///x/../a.go"
			case "too-many":
				for i := 0; i < 64; i++ {
					x := target(fmt.Sprint(i), a)
					r.RequiredTargets = append(r.RequiredTargets, x)
				}
			case "generation":
				r.Context.Generation = 0
			case "encoding":
				r.Context.PositionEncoding = "unknown"
			}
			if _, e := Acquire(context.Background(), f, r); e == nil || len(f.calls) != 0 {
				t.Fatal("invalid request must fail before acquisition")
			}
		})
	}
}
func TestNoSupportInflationAndValidator(t *testing.T) {
	f := fixture()
	a, b := item("a", 0), item("b", 1)
	f.add(a)
	f.add(b)
	f.edge(a, b)
	r := request(a, b, b)
	r.RequiredTargets[1].ID = "alias"
	got := run(t, f, r)
	if len(got.Graph.Edges) != 1 {
		t.Fatal("native group dedup")
	}
	observed := map[string]bool{}
	for _, o := range got.EdgeObservations {
		key := o.RequestID + o.RelationID
		if observed[key] {
			t.Fatal("cache must not multiply evidence observations")
		}
		observed[key] = true
	}
	if len(observed) != 2 {
		t.Fatalf("one outgoing and one incoming response, not aliases: %d", len(observed))
	}
	for _, mutation := range []string{"missing-row", "endpoint", "path", "work", "request", "resolved", "missing-observations", "false-observation", "duplicate-observation", "budget-context", "false-empty", "missing-expansion", "completeness"} {
		t.Run(mutation, func(t *testing.T) {
			copy := run(t, fixtureForEdge(a, b), r)
			switch mutation {
			case "missing-row":
				copy.Targets = copy.Targets[:2]
			case "endpoint":
				copy.Targets[1].Connection.To = node(a).ID
			case "path":
				copy.Targets[1].Connection.Path.GroupIDs[0] = "fake"
			case "work":
				copy.Usage.PathWork++
			case "request":
				copy.Targets[0].Outgoing.Expansions[0].RequestID = "missing"
			case "resolved":
				copy.Targets[1].Resolution.Identity.ID = "fake"
			case "missing-observations":
				copy.EdgeObservations = nil
			case "false-observation":
				copy.EdgeObservations[0].RequestID = copy.Targets[0].Resolution.RequestIDs[0]
			case "duplicate-observation":
				copy.EdgeObservations = append(copy.EdgeObservations, copy.EdgeObservations[0])
			case "budget-context":
				copy.Requests[0].Before.Requests++
			case "false-empty":
				copy.Targets[0].Outgoing.Expansions[0].Status = SuccessEmpty
				copy.Targets[0].Outgoing.Status = aggregate(copy.Targets[0].Outgoing.Expansions)
			case "missing-expansion":
				copy.Targets[0].Outgoing = DirectionResult{Status: ExpansionNotApplicable}
			case "completeness":
				copy.AcquisitionComplete = !copy.AcquisitionComplete
			}
			if ValidateResult(copy) == nil {
				t.Fatal("validator must reject substituted join")
			}
		})
	}
}
func fixtureForEdge(a, b lsp.CallHierarchyItem) *fakeClient {
	f := fixture()
	f.add(a)
	f.add(b)
	f.edge(a, b)
	return f
}

type supplyClient struct {
	*fakeClient
	version int
}

func (s *supplyClient) PrepareDocument(_ context.Context, _ AcquisitionContext, l Locator) (Supply, error) {
	s.version++
	return Supply{URI: l.URI, Observation: json.RawMessage(fmt.Sprintf(`{"version":%d}`, s.version))}, nil
}
func TestSupplyObservationsDoNotFreezeURI(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := &supplyClient{fakeClient: fixtureForEdge(a, b)}
	got, e := Acquire(context.Background(), f, request(a, b))
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Supplies) != 2 || bytesEqual(got.Supplies[0].Observation, got.Supplies[1].Observation) {
		t.Fatal("same URI versions preserved as distinct supply facts")
	}
	if got.Usage.Requests != len(f.calls)+2 {
		t.Fatal("supply attempts share budget")
	}
}
func bytesEqual(a, b []byte) bool { return string(a) == string(b) }
func TestNativeGraphSchema(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	got := run(t, fixtureForEdge(a, b), request(a, b))
	raw, e := json.Marshal(got.Graph)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = schema.Validate(raw, "v3"); e != nil {
		t.Fatal(e)
	}
	if got.Graph.Slice != nil {
		t.Fatal("must not manufacture historical single-at slice")
	}
	if e = graph.ValidateSemanticBundle(raw); e != nil {
		t.Fatal(e)
	}
}
