package acquisition

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

func frozenFixturePrivate() (*fakeClient, Request, lsp.CallHierarchyItem, lsp.CallHierarchyItem) {
	f := fixture()
	a, b := item("A", 0), item("B", 4)
	b.URI = "file:///outside.go"
	f.add(a)
	f.edge(a, b)
	r := request(a)
	r.Root.DownDepth = 1
	r.Root.UpDepth = 1
	r.FrozenQueryURIsPrivate = [6]string{"file:///a.go", "file:///b.go", "file:///c.go", "file:///d.go", "file:///e.go", "file:///f.go"}
	return f, r, a, b
}

func TestFrozenQueryPrivateSchedulingAndReconstruction(t *testing.T) {
	f, r, a, b := frozenFixturePrivate()
	got, err := Acquire(context.Background(), f, r)
	if err != nil {
		t.Fatal(err)
	}
	// Observe forbidden scheduling directly, including when a competing reading
	// would treat the outside endpoint as merely a depth stop and then query it.
	for _, call := range f.calls {
		if call == "in:B" || call == "out:B" || call == "in:A" {
			t.Fatalf("ASSERT_FROZEN_QUERY_NO_EXTERNAL_OR_RESEED: %s", call)
		}
	}
	t.Log("ASSERT_FROZEN_QUERY_NO_EXTERNAL_OR_RESEED: PASS")
	if len(got.Requests) != 2 || got.Targets[0].Incoming.Expansions[0].RequestID != "" || got.Targets[0].Incoming.Expansions[0].Status != Frontier || got.Targets[0].Incoming.Expansions[0].Reason != FrozenQueryStopPrivate || !reflect.DeepEqual(got.Targets[0].Incoming.StartIDs, []string{node(b).ID}) {
		t.Fatal("ASSERT_FROZEN_QUERY_NO_FAKE_REQUEST")
	}
	if len(got.Graph.Edges) != 1 || len(got.Graph.Frontier) != 2 || got.Graph.Summary.Complete || len(got.Omissions) != 0 {
		t.Fatal("ASSERT_FROZEN_QUERY_RETAINED_PARTIAL")
	}
	for _, boundary := range got.Graph.Frontier {
		if boundary.Reason != graph.ExternalURI || !strings.Contains(boundary.Message, "originating_observations=request-000002") {
			t.Fatal("ASSERT_FROZEN_QUERY_ORIGIN")
		}
	}
	for _, kind := range []string{"inside-policy-stop", "no-declaration", "changed-origin", "changed-direction"} {
		bad := got
		bad.Graph.Frontier = append([]graph.Boundary(nil), got.Graph.Frontier...)
		switch kind {
		case "inside-policy-stop":
			bad.Request.FrozenQueryURIsPrivate[5] = b.URI
		case "no-declaration":
			bad.Request.FrozenQueryURIsPrivate = [6]string{}
		case "changed-origin":
			bad.Graph.Frontier[0].Message = "external-to-frozen-queryset originating_observations=request-FAKE"
		case "changed-direction":
			bad.Graph.Frontier[0].Message = "external-to-frozen-queryset direction=FAKE"
		}
		if ValidateResult(bad) == nil {
			t.Fatalf("ASSERT_FROZEN_QUERY_TAMPER: %s", kind)
		}
	}
	// Ordinary requests still issue incoming(B), not incoming(A).
	plain := r
	plain.FrozenQueryURIsPrivate = [6]string{}
	f2 := fixture()
	f2.add(a)
	f2.edge(a, b)
	ordinary := run(t, f2, plain)
	if len(ordinary.Graph.Frontier) != 0 || !reflect.DeepEqual(f2.calls, []string{f2.calls[0], "out:A", "in:B"}) {
		t.Fatal("ASSERT_FROZEN_QUERY_ZERO_COMPATIBILITY")
	}
	// The declaration is never a new public/acquisition wire field.
	raw, _ := json.Marshal(r)
	var decoded Request
	_ = json.Unmarshal(raw, &decoded)
	if decoded.FrozenQueryURIsPrivate != [6]string{} {
		t.Fatal("ASSERT_FROZEN_QUERY_PRIVATE_ONLY")
	}
}

func TestFrozenQueryPrivateInsideContextAndFiniteBudgets(t *testing.T) {
	f, r, a, b := frozenFixturePrivate()
	b.URI = "file:///f.go" // context is queryable, even though it is not a seed
	f.outgoing[a.Name] = nil
	f.incoming[b.Name] = nil
	f.edge(a, b)
	got := run(t, f, r)
	if !reflect.DeepEqual(got.Targets[0].Incoming.StartIDs, []string{node(b).ID}) || len(got.Graph.Frontier) != 0 || f.calls[len(f.calls)-1] != "in:B" {
		t.Fatal("ASSERT_FROZEN_QUERY_CONTEXT_QUERIED")
	}
	for _, n := range []int{0, 1, 2} {
		f, r, _, _ := frozenFixturePrivate()
		r.Limits.MaxRequests = n
		got := run(t, f, r)
		if got.Usage.Requests > n || got.AcquisitionComplete {
			t.Fatal("ASSERT_FROZEN_QUERY_FINITE_BUDGET")
		}
	}
	for _, mutation := range []func(*Request){func(r *Request) { r.FrozenQueryURIsPrivate[0] = "" }, func(r *Request) { r.FrozenQueryURIsPrivate[1] = r.FrozenQueryURIsPrivate[0] }, func(r *Request) { r.Root.Locator.URI = "file:///outside.go" }} {
		f, r, _, _ := frozenFixturePrivate()
		mutation(&r)
		if _, err := Acquire(context.Background(), f, r); err == nil || len(f.calls) != 0 {
			t.Fatal("ASSERT_FROZEN_QUERY_PREFLIGHT")
		}
	}
}
