package acquisition

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"testing"
)

// Adapted from the retained independent-copy probes: correctness assertions
// deliberately reverse their original defect-observation success criteria.
func TestReceiptTamperingRejected(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	for _, kind := range []string{"prepared-data", "resolution-status", "request-owner", "cached-flag", "depth", "invented-callsite", "supply", "pathwork-zero"} {
		t.Run(kind, func(t *testing.T) {
			r := run(t, fixtureForEdge(a, b), request(a, b))
			switch kind {
			case "prepared-data":
				r.Targets[0].Resolution.Prepared.Data = json.RawMessage(`{"forged":true}`)
				r.Targets[0].Resolution.Identity.Data = json.RawMessage(`{"forged":true}`)
			case "resolution-status":
				r = run(t, fixture(), request(a))
				r.Targets[0].Resolution.Status = Ambiguous
			case "request-owner":
				r.Requests[0].TargetID = "b"
			case "cached-flag":
				r.Targets[0].Outgoing.Expansions[0].Cached = true
			case "depth":
				d := &r.Targets[0].Outgoing
				d.Expansions[1].Depth = 2
				d.Layers = append(d.Layers, graph.SliceLayer{Depth: 2, NodeIDs: d.Layers[1].NodeIDs})
				d.Layers[1].NodeIDs = []string{}
			case "invented-callsite":
				r.Graph.Edges[0].CallSites = append(r.Graph.Edges[0].CallSites, graph.Range{Start: graph.Position{Line: 9}, End: graph.Position{Line: 9, Character: 1}})
				r.Graph.Canonicalize()
				for i := range r.Targets[1].Connection.Path.OccurrenceIDs {
					r.Targets[1].Connection.Path.OccurrenceIDs[i] = []string{"/edges/0/call_sites/0", "/edges/0/call_sites/1"}
				}
			case "supply":
				r.Supplies = []Supply{{URI: "file:///unrequested.go", Observation: json.RawMessage(`{"version":999}`)}}
			case "pathwork-zero":
				r.Targets[1].Connection.Work = 0
				r.Usage.PathWork = 0
				r.Request.Limits.MaxPathWork = 0
			}
			if err := ValidateResult(r); err == nil {
				t.Fatalf("ASSERT_REJECT_%s: accepted tampered receipt", kind)
			} else {
				t.Logf("ASSERT_REJECT_%s: rejected: %v", kind, err)
			}
		})
	}
}

func TestCancellationCauseIsSticky(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Acquire(ctx, fixtureForEdge(a, b), request(a, b))
	if err != nil {
		t.Fatal(err)
	}
	r.Requests[1].Reason = context.DeadlineExceeded.Error()
	r.Targets[1].Resolution.Reason = context.DeadlineExceeded.Error()
	for i := range r.Graph.Seeds {
		if r.Graph.Seeds[i].Label == "b" {
			r.Graph.Seeds[i].Failure.Message = string(ResolutionBlocked) + ":" + context.DeadlineExceeded.Error() + ":" + string(NotApplicable)
		}
	}
	r.Graph.Canonicalize()
	if ValidateResult(r) == nil {
		t.Fatal("ASSERT_CANCEL_CAUSE: accepted changed global cause")
	}
	t.Log("ASSERT_CANCEL_CAUSE: rejected changed global cause")
}

func TestTypedNilRangesArePartial(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	for _, direction := range []Mode{Slice, Incoming} {
		f := fixtureForEdge(a, b)
		r := request(a)
		if direction == Incoming {
			r = request(b)
			r.Mode = Incoming
			f.incoming[b.Name][0].FromRanges = nil
		} else {
			f.outgoing[a.Name][0].FromRanges = nil
		}
		got := run(t, f, r)
		d := got.Targets[0].Outgoing
		if direction == Incoming {
			d = got.Targets[0].Incoming
		}
		if d.Expansions[0].Status != Partial || len(got.Graph.Edges) != 0 {
			t.Errorf("ASSERT_TYPED_NIL_%s: status=%s edges=%d", direction, d.Expansions[0].Status, len(got.Graph.Edges))
		} else {
			t.Logf("ASSERT_TYPED_NIL_%s: rejected row", direction)
		}
	}
}

func TestSupportedUnionAndMissingSite(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := fixtureForEdge(a, b)
	extra := lsp.Range{Start: lsp.Position{Line: 8}, End: lsp.Position{Line: 8, Character: 1}}
	f.incoming[b.Name][0].FromRanges = append(f.incoming[b.Name][0].FromRanges, extra, extra)
	r := run(t, f, request(a, b))
	if len(r.Graph.Edges) != 1 || len(r.Graph.Edges[0].CallSites) != 2 {
		t.Fatal("canonical union must retain two distinct sites")
	}
	r.Graph.Edges[0].CallSites = r.Graph.Edges[0].CallSites[:1]
	r.Graph.Canonicalize()
	for i := range r.Targets[1].Connection.Path.OccurrenceIDs {
		r.Targets[1].Connection.Path.OccurrenceIDs[i] = []string{"/edges/0/call_sites/0"}
	}
	if ValidateResult(r) == nil {
		t.Fatal("ASSERT_MISSING_SITE: accepted missing site")
	}
	t.Log("ASSERT_MISSING_SITE: rejected; duplicate repeated-response union accepted")
}

func TestCancellationSearchPrefixes(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	for _, work := range []int{0, 1, 2, 3} {
		r := run(t, fixtureForEdge(a, b), request(a, b))
		required := r.Targets[1].Connection.Work
		r.Targets[1].Connection.Status = "INCOMPLETE"
		r.Targets[1].Connection.Reason = "CANCELLED"
		r.Targets[1].Connection.Path = emptyPath()
		r.Targets[1].Connection.Work = work
		r.Usage.PathWork = work
		err := ValidateResult(r)
		if (err == nil) != (work < required) {
			t.Fatalf("ASSERT_CANCEL_PREFIX_%d: needed=%d error=%v", work, required, err)
		}
		t.Logf("ASSERT_CANCEL_PREFIX_%d: correct", work)
	}
	r := run(t, fixtureForEdge(a, b), request(a, b, a))
	r.Targets[1].Connection.Status = "INCOMPLETE"
	r.Targets[1].Connection.Reason = "CANCELLED"
	r.Targets[1].Connection.Path = emptyPath()
	r.Usage.PathWork -= r.Targets[1].Connection.Work
	r.Targets[1].Connection.Work = 0
	if ValidateResult(r) == nil {
		t.Fatal("ASSERT_CANCEL_STICKY: accepted later FOUND")
	}
	t.Log("ASSERT_CANCEL_STICKY: rejected later FOUND")
}

func TestWireRangeArrayContract(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	for _, kind := range []string{"null", "missing", "scalar", "object", "empty", "top-null"} {
		t.Run(kind, func(t *testing.T) {
			client := NewWireClient(func(_ context.Context, w WireRequest) (json.RawMessage, error) {
				switch w.Method {
				case "textDocument/prepareCallHierarchy":
					return json.Marshal([]lsp.CallHierarchyItem{a})
				case "callHierarchy/outgoingCalls":
					row := map[string]any{"to": b}
					switch kind {
					case "null":
						row["fromRanges"] = nil
					case "scalar":
						row["fromRanges"] = 1
					case "object":
						row["fromRanges"] = map[string]any{}
					case "empty":
						row["fromRanges"] = []any{}
					case "top-null":
						return json.RawMessage("null"), nil
					}
					return json.Marshal([]any{row})
				default:
					return json.RawMessage("null"), nil
				}
			})
			r := request(a)
			r.Root.DownDepth = 1
			r.Root.UpDepth = 0
			got, err := Acquire(context.Background(), client, r)
			if err != nil {
				t.Fatal(err)
			}
			want := Failed
			edges := 0
			if kind == "empty" {
				want = SuccessNonempty
				edges = 1
			}
			if kind == "top-null" {
				want = SuccessEmpty
			}
			if got.Targets[0].Outgoing.Expansions[0].Status != want || len(got.Graph.Edges) != edges {
				t.Fatalf("ASSERT_RANGE_ARRAY_%s: status=%s edges=%d", kind, got.Targets[0].Outgoing.Expansions[0].Status, len(got.Graph.Edges))
			}
			t.Logf("ASSERT_RANGE_ARRAY_%s: correct", kind)
		})
	}
}
