package directedwalk_test

import (
	"reflect"
	"testing"

	"lsp-trace/internal/directedwalk"
)

func mixedRequest() directedwalk.Request {
	return directedwalk.Request{
		Root: "A", Kinds: []directedwalk.Kind{directedwalk.Calls, directedwalk.ReferencesSymbol, directedwalk.ResolvesToDefinition},
		DownDepth: 3, UpDepth: 0, MaxNodes: 3, MaxCandidates: 8, MaxWork: directedwalk.MaxWorkLimit,
		Candidates: []directedwalk.Candidate{
			{ID: "call-1", Kind: directedwalk.Calls, From: "A", To: "B"},
			{ID: "def-1", Kind: directedwalk.ResolvesToDefinition, From: "B", To: "C"},
			{ID: "ref-1", Kind: directedwalk.ReferencesSymbol, From: "C", To: "D"},
			{ID: "ref-2", Kind: directedwalk.ReferencesSymbol, From: "C", To: "D"},
			{ID: "far", Kind: directedwalk.Calls, From: "X", To: "Y"},
		},
	}
}

func byID(result directedwalk.Result) map[string]directedwalk.Disposition {
	out := make(map[string]directedwalk.Disposition)
	for _, o := range result.Occurrences {
		out[o.Candidate.ID] = o
	}
	return out
}

func byKind(result directedwalk.Result) map[directedwalk.Kind]directedwalk.KindCounts {
	out := make(map[directedwalk.Kind]directedwalk.KindCounts)
	for _, c := range result.Counts {
		out[c.Kind] = c
	}
	return out
}

func TestTypedWalkSharedNodeBudgetAndPerKindAccounting(t *testing.T) {
	req := mixedRequest()
	original := append([]directedwalk.Candidate(nil), req.Candidates...)
	got, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 3 || got.Nodes[0].ID != "A" || got.Nodes[1].ID != "B" || got.Nodes[2].ID != "C" || got.NodesOmitted != 1 {
		t.Fatalf("ASSERT_TYPED_GLOBAL_BUDGET: nodes=%v omitted=%d", got.Nodes, got.NodesOmitted)
	}
	o := byID(got)
	if o["call-1"].State != directedwalk.Witnessed || o["def-1"].State != directedwalk.Witnessed ||
		o["ref-1"].State != directedwalk.NodeBound || o["ref-2"].State != directedwalk.NodeBound || o["far"].State != directedwalk.OutsideFrontier ||
		len(o["ref-1"].NodeBoundSteps) != 1 || len(o["ref-2"].NodeBoundSteps) != 1 ||
		!reflect.DeepEqual(req.Candidates, original) {
		t.Fatalf("ASSERT_TYPED_PER_KIND: occurrences=%v", got.Occurrences)
	}
	counts := byKind(got)
	if counts[directedwalk.Calls].Total != 2 || counts[directedwalk.Calls].Witnessed != 1 || counts[directedwalk.Calls].OutsideFrontier != 1 ||
		counts[directedwalk.ResolvesToDefinition].Witnessed != 1 || counts[directedwalk.ReferencesSymbol].Total != 2 ||
		counts[directedwalk.ReferencesSymbol].NodeBound != 2 || counts[directedwalk.ReferencesSymbol].DownNodeBound != 2 {
		t.Fatalf("ASSERT_TYPED_PER_KIND: counts=%v", got.Counts)
	}
	t.Log("ASSERT_TYPED_GLOBAL_BUDGET: PASS")
	t.Log("ASSERT_TYPED_PER_KIND: PASS")
}

func TestTypedWalkIncomingUsesOnlyExistingReverseEndpoints(t *testing.T) {
	req := mixedRequest()
	req.Root, req.UpDepth, req.DownDepth, req.MaxNodes = "D", 3, 0, 4
	got, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	wantNodes := []string{"D", "C", "B", "A"}
	for i, want := range wantNodes {
		if i >= len(got.Nodes) || got.Nodes[i].ID != want || got.Nodes[i].UpDepth == nil || *got.Nodes[i].UpDepth != i {
			t.Fatalf("ASSERT_TYPED_UP_REVERSE: nodes=%v", got.Nodes)
		}
	}
	o := byID(got)
	for id, depth := range map[string]int{"call-1": 3, "def-1": 2, "ref-1": 1, "ref-2": 1} {
		v := o[id]
		if v.State != directedwalk.Witnessed || len(v.Witnesses) != 1 || v.Witnesses[0] != (directedwalk.Step{Direction: directedwalk.Up, Depth: depth}) {
			t.Fatalf("ASSERT_TYPED_UP_REVERSE: id=%s occurrence=%v", id, v)
		}
	}
	if byKind(got)[directedwalk.ReferencesSymbol].UpWitnesses != 2 {
		t.Fatalf("ASSERT_TYPED_UP_REVERSE: counts=%v", got.Counts)
	}
	t.Log("ASSERT_TYPED_UP_REVERSE: PASS")
}

func TestTypedWalkSharedBudgetAcrossOppositeFrontiers(t *testing.T) {
	req := mixedRequest()
	req.Root, req.UpDepth, req.DownDepth, req.MaxNodes = "B", 1, 1, 2
	got, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 || got.Nodes[0].ID != "B" || got.Nodes[1].ID != "A" || got.NodesOmitted != 1 ||
		byID(got)["call-1"].State != directedwalk.Witnessed || byID(got)["def-1"].State != directedwalk.NodeBound {
		t.Fatalf("ASSERT_TYPED_GLOBAL_BUDGET: nodes=%v occurrences=%v omitted=%d", got.Nodes, got.Occurrences, got.NodesOmitted)
	}
	t.Log("ASSERT_TYPED_GLOBAL_BUDGET: PASS")
}

func TestTypedWalkSelectorZeroDepthAndPermutation(t *testing.T) {
	req := mixedRequest()
	req.Kinds = []directedwalk.Kind{directedwalk.Calls, directedwalk.ResolvesToDefinition}
	req.DownDepth, req.UpDepth = 0, 0
	got, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || got.Unselected != 2 || len(got.Occurrences) != 3 || byID(got)["call-1"].State != directedwalk.OutsideFrontier {
		t.Fatalf("ASSERT_TYPED_EXPLICIT_SELECTOR: result=%v", got)
	}
	req = mixedRequest()
	baseline, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(req.Candidates)-1; i < j; i, j = i+1, j-1 {
		req.Candidates[i], req.Candidates[j] = req.Candidates[j], req.Candidates[i]
	}
	req.Kinds[0], req.Kinds[2] = req.Kinds[2], req.Kinds[0]
	permuted, err := directedwalk.Walk(req)
	if err != nil || !reflect.DeepEqual(permuted, baseline) {
		t.Fatalf("ASSERT_TYPED_PERMUTATION: baseline=%v permuted=%v err=%v", baseline, permuted, err)
	}
	t.Log("ASSERT_TYPED_EXPLICIT_SELECTOR: PASS")
	t.Log("ASSERT_TYPED_PERMUTATION: PASS")
}

func TestTypedWalkRetainsCycleOccurrence(t *testing.T) {
	req := mixedRequest()
	req.Candidates = append(req.Candidates, directedwalk.Candidate{ID: "cycle", Kind: directedwalk.ReferencesSymbol, From: "C", To: "A"})
	got, err := directedwalk.Walk(req)
	if err != nil || byID(got)["cycle"].State != directedwalk.Witnessed ||
		!reflect.DeepEqual(byID(got)["cycle"].Witnesses, []directedwalk.Step{{Direction: directedwalk.Down, Depth: 3}}) {
		t.Fatalf("ASSERT_TYPED_CYCLE: cycle=%v err=%v", byID(got)["cycle"], err)
	}
	t.Log("ASSERT_TYPED_CYCLE: PASS")
}

func TestTypedWalkRejectsInvalidOrImplicitSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*directedwalk.Request)
	}{
		{"missing selector", func(r *directedwalk.Request) { r.Kinds = nil }},
		{"unknown selector", func(r *directedwalk.Request) { r.Kinds = []directedwalk.Kind{"CALL"} }},
		{"duplicate selector", func(r *directedwalk.Request) { r.Kinds = append(r.Kinds, directedwalk.Calls) }},
		{"invalid kind", func(r *directedwalk.Request) { r.Candidates[0].Kind = "DEFINITION" }},
		{"duplicate occurrence ID", func(r *directedwalk.Request) { r.Candidates[1].ID = "call-1" }},
		{"missing occurrence ID", func(r *directedwalk.Request) { r.Candidates[0].ID = "" }},
		{"missing endpoint", func(r *directedwalk.Request) { r.Candidates[0].To = "" }},
		{"missing root", func(r *directedwalk.Request) { r.Root = "" }},
		{"zero node budget", func(r *directedwalk.Request) { r.MaxNodes = 0 }},
		{"negative depth", func(r *directedwalk.Request) { r.UpDepth = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := mixedRequest()
			tc.change(&req)
			if got, err := directedwalk.Walk(req); err == nil || len(got.Nodes) != 0 || len(got.Occurrences) != 0 {
				t.Fatalf("ASSERT_TYPED_VALIDATION: result=%v err=%v", got, err)
			}
			t.Log("ASSERT_TYPED_VALIDATION: PASS")
		})
	}
}
