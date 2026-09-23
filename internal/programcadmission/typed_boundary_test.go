package programcadmission_test

import (
	"errors"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

func TestTypedStagingCannotBypassCompositeNodeConflict(t *testing.T) {
	a := node("conflicting-typed-node")
	conflict := a
	conflict.Item.Data = []byte(`{"opaque":true}`)
	x := capture(t, "conflict-x", []graph.Node{a}, nil)
	y := capture(t, "conflict-y", []graph.Node{conflict}, nil)
	if _, err := programccompose.Compose([]programccompose.Input{x, y}); programccompose.ErrorBranch(err) != programccompose.BranchNodeConflict {
		t.Fatalf("expected NODE_CONFLICT before typed staging, got %v", err)
	}
}

func TestTypedStagingOnlyProjectsAdmittedCalls(t *testing.T) {
	a, b := node("typed-a"), node("typed-b")
	e := edge(a, b)
	e.CallSites = []graph.Range{{End: graph.Position{Character: 1}}, {End: graph.Position{Character: 2}}}
	x := capture(t, "typed-x", []graph.Node{a, b}, []graph.Edge{e})
	y := capture(t, "typed-y", []graph.Node{a, b}, nil)
	composite, err := programccompose.Compose([]programccompose.Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	got, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != programcadmission.TypedInputVersion || got.CompositeID != admitted.Artifact.CompositeID || got.CompositeOutputSHA256 != admitted.Artifact.CompositeOutputSHA256 || got.ClaimCeiling != programccompose.ClaimCeiling || got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" {
		t.Fatal("typed staging changed composite identity or claim ceiling")
	}
	if len(got.Occurrences) != 2 || got.PairWeights[programcadmission.TypedPair{From: a.ID, To: b.ID}] != 2 {
		t.Fatalf("repeated CALLS occurrences collapsed: %+v", got)
	}
	if got.Occurrences[0].Identity == got.Occurrences[1].Identity || got.Occurrences[0].Kind != programcadmission.TypedCalls || got.Occurrences[1].Kind != programcadmission.TypedCalls {
		t.Fatal("typed occurrence identity or kind lost")
	}
	originalOrdinals := make(map[string]int)
	for _, occurrence := range admitted.Admission.Occurrences() {
		originalOrdinals[occurrence.Identity] = occurrence.Ordinal
		if occurrence.Ordinal < 0 || occurrence.Ordinal >= len(e.CallSites) || occurrence.CallSite != e.CallSites[occurrence.Ordinal] {
			t.Fatal("original call-site ordinal not preserved in opaque admission")
		}
	}
	for _, occurrence := range got.Occurrences {
		if occurrence.Ordinal != originalOrdinals[occurrence.Identity] {
			t.Fatal("typed staging substituted sorted index for original call-site ordinal")
		}
	}
	for _, tc := range []struct {
		name    string
		request programcadmission.TypedInputRequest
		want    error
	}{
		{"missing selection", programcadmission.TypedInputRequest{Calls: admitted.Admission}, programcadmission.ErrTypedSelection},
		{"D/R selector", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}}, programcadmission.ErrTypedSelection},
		{"definition selector", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}}, programcadmission.ErrTypedSelection},
		{"unknown selector", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{"UNKNOWN"}}, programcadmission.ErrTypedSelection},
		{"duplicate selector", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedCalls}}, programcadmission.ErrTypedSelection},
		{"unadmitted reference", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}, MethodCandidates: []programcadmission.MethodCandidate{{Identity: "r", Kind: programcadmission.TypedReferencesSymbol, From: a.ID, To: b.ID}}}, programcadmission.ErrMethodEvidenceUnadmitted},
		{"unadmitted definition", programcadmission.TypedInputRequest{Calls: admitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}, MethodCandidates: []programcadmission.MethodCandidate{{Identity: "d", Kind: programcadmission.TypedResolvesToDefinition, From: a.ID, To: b.ID}}}, programcadmission.ErrMethodEvidenceUnadmitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := programcadmission.PrepareTypedInput(tc.request)
			if !errors.Is(err, tc.want) || len(out.Occurrences) != 0 || len(out.PairWeights) != 0 {
				t.Fatalf("unadmitted evidence reached staging: result=%+v err=%v", out, err)
			}
		})
	}
	if out, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}}); err == nil || len(out.Occurrences) != 0 {
		t.Fatal("zero-value composite admitted")
	}
}
