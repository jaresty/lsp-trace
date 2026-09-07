package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/retainedpath"
)

func TestFailedQueryCacheReplay(t *testing.T) {
	a := item("a", 0)
	for _, mode := range []Mode{Slice, Incoming} {
		f := fixture()
		f.add(a)
		f.errors["out:a"] = errors.New("failed outgoing")
		f.errors["in:a"] = errors.New("failed incoming")
		r := request(a, a)
		r.Mode = mode
		got := run(t, f, r)
		if len(f.calls) != 2 || got.Usage.Requests != 2 {
			t.Fatal("failed query cache must not repeat request")
		}
		d := got.Targets[1].Outgoing
		if mode == Incoming {
			d = got.Targets[1].Incoming
		}
		if d.Status != Failed || len(d.Expansions) != 1 || !d.Expansions[0].Cached {
			t.Fatal("cached failure retains target attribution")
		}
	}
}
func TestOpaqueMalformedEvidenceFailsRatherThanEmpty(t *testing.T) {
	f := fixture()
	a := item("a", 0)
	a.Data = json.RawMessage(`{invalid`)
	f.add(a)
	got := run(t, f, request(a))
	if got.Targets[0].Resolution.Status != ResolutionFailed || got.Requests[0].Outcome != "CAPTURE_FAILED" || got.Requests[0].CaptureComplete {
		t.Fatal("malformed opaque evidence cannot become empty/budget success")
	}
}
func TestDiamondTieAndResponseOrder(t *testing.T) {
	a, b, c, d := item("a", 0), item("b", 1), item("c", 2), item("d", 3)
	makeFixture := func(reverse bool) *fakeClient {
		f := fixture()
		for _, i := range []lsp.CallHierarchyItem{a, b, c, d} {
			f.add(i)
		}
		if reverse {
			f.edge(a, c)
			f.edge(a, b)
		} else {
			f.edge(a, b)
			f.edge(a, c)
		}
		f.edge(b, d)
		f.edge(c, d)
		return f
	}
	r := request(a, d)
	got := run(t, makeFixture(false), r)
	other := run(t, makeFixture(true), r)
	if !reflect.DeepEqual(got.Targets, other.Targets) || !reflect.DeepEqual(got.Graph, other.Graph) {
		t.Fatal("neighbor insertion order cannot change admission, depth or witness")
	}
	nodes, edges := PathInput(got.Graph, r.Context.ID)
	budget := &retainedpath.Budget{Context: context.Background(), Left: r.Limits.MaxPathWork}
	want, e := retainedpath.Search(nodes, edges, node(a).ID, node(d).ID, budget)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Targets[1].Connection.Path, want.Path) || got.Usage.PathWork != r.Limits.MaxPathWork-budget.Left {
		t.Fatal("native diamond uses exact kernel lexical tie/work")
	}
}
func TestZeroDepthAndZeroSiteGroups(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := fixtureForEdge(a, b)
	f.outgoing[a.Name][0].FromRanges = []lsp.Range{}
	f.incoming[b.Name][0].FromRanges = []lsp.Range{}
	got := run(t, f, request(a, b))
	if len(got.Targets[1].Connection.Path.OccurrenceIDs) != 1 || len(got.Targets[1].Connection.Path.OccurrenceIDs[0]) != 0 {
		t.Fatal("zero-site group remains witnessed with an empty occurrence list")
	}
	f = fixture()
	f.add(a)
	r := request(a, a)
	r.Root.DownDepth = 0
	r.Root.UpDepth = 0
	r.RequiredTargets[0].DownDepth = 0
	r.RequiredTargets[0].UpDepth = 0
	got = run(t, f, r)
	if len(f.calls) != 1 || got.Targets[0].Outgoing.Status != Frontier || got.Targets[0].Incoming.Status != Frontier || got.Targets[1].Connection.Status != "FOUND" {
		t.Fatal("zero hop need not imply expansion")
	}
}
func TestDuplicateResponseRowsDoNotInflateObservation(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := fixtureForEdge(a, b)
	f.edge(a, b)
	got := run(t, f, request(a, b))
	if len(got.Graph.Edges) != 1 || len(got.Graph.Edges[0].CallSites) != 1 || len(got.EdgeObservations) != 2 {
		t.Fatal("duplicate rows and alias replay do not inflate native support")
	}
}
func TestCancelledCacheStopsExpansion(t *testing.T) {
	a, b := item("a", 0), item("b", 1)
	f := fixtureForEdge(a, b)
	ctx, cancel := context.WithCancel(context.Background())
	f.hook = func(_ context.Context, key string) error {
		if key == "out:b" {
			cancel()
			return context.Canceled
		}
		return nil
	}
	got, e := Acquire(ctx, f, request(a, b))
	if e != nil {
		t.Fatal(e)
	}
	if got.AcquisitionComplete || got.Targets[1].Connection.Status != "INCOMPLETE" {
		t.Fatal("cancellation retained independently of graph connectivity")
	}
}
