package incomingops

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
)

type mutableFailureCausePrivate struct{ message string }

func (e *mutableFailureCausePrivate) Error() string { return e.message }

func TestProductionPrivateResultFailureCauseIsImmutablePresentation(t *testing.T) {
	original := &mutableFailureCausePrivate{message: "mutable cause"}
	result := newProductionPrivateFailureResult(&operation.Failure{
		Code:        "FAIL",
		Diagnostics: []string{"diagnostic"},
		Err:         original,
	})

	first := result.FailurePrivate()
	if first == nil || first.Err == nil || first.Err == original {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_VALUE_ISOLATED: got=%#v", first)
	}
	if _, ok := first.Err.(*mutableFailureCausePrivate); ok {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_VALUE_ISOLATED: retained type %T", first.Err)
	}
	if got := first.Err.Error(); got != "mutable cause" {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_VALUE_ISOLATED: got=%q", got)
	}
	if errors.Is(first.Err, original) {
		t.Fatal("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_IDENTITY_NOT_PRESERVED: errors.Is retained original identity")
	}
	var mutable *mutableFailureCausePrivate
	if errors.As(first.Err, &mutable) {
		t.Fatal("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_IDENTITY_NOT_PRESERVED: errors.As retained original type")
	}

	first.Code = "MUTATED"
	first.Diagnostics[0] = "mutated"
	original.message = "changed after retention"
	second := result.FailurePrivate()
	if second.Code != "FAIL" || !reflect.DeepEqual(second.Diagnostics, []string{"diagnostic"}) {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_CODE_DIAGNOSTICS_ISOLATED: got=%#v", second)
	}
	if second.Err == nil || second.Err.Error() != "mutable cause" || first.Err.Error() != "mutable cause" {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_CAUSE_STABLE_AFTER_SOURCE_MUTATION: first=%v second=%v", first.Err, second.Err)
	}

	if got := newProductionPrivateFailureResult(&operation.Failure{Code: "NIL"}).FailurePrivate(); got == nil || got.Err != nil {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_NIL_CAUSE: got=%#v", got)
	}
	if got := newProductionPrivateFailureResult(nil).FailurePrivate(); got != nil {
		t.Fatalf("ASSERT_PRODUCTION_PRIVATE_FAILURE_NIL_CAUSE: nil failure got=%#v", got)
	}
}

func TestProductionPrivateResultCompleteMutationIsolation(t *testing.T) {
	incoming := lsp.CallHierarchyIncomingCall{From: lsp.CallHierarchyItem{Name: "source", Tags: []int{1}, Data: json.RawMessage(`{"incoming":true}`), Range: lsp.Range{Start: lsp.Position{Line: 3}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 4}}}, FromRanges: []lsp.Range{{Start: lsp.Position{Line: 5}}}}
	outgoing := lsp.CallHierarchyOutgoingCall{To: lsp.CallHierarchyItem{Name: "target", Tags: []int{2}, Data: json.RawMessage(`{"outgoing":true}`), Range: lsp.Range{Start: lsp.Position{Line: 6}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 7}}}, FromRanges: []lsp.Range{{Start: lsp.Position{Line: 8}}}}
	result := newProductionPrivateFailureResult(&operation.Failure{Code: "FAIL", Diagnostics: []string{"diagnostic"}, Err: errors.New("cause")})
	result.traversal = operation.Result{Value: cloneIncomingPrivate([]lsp.CallHierarchyIncomingCall{incoming}), Artifact: []byte("artifact")}
	result.outgoing = cloneOutgoingPrivate([]lsp.CallHierarchyOutgoingCall{outgoing})
	result.sealedArtifact = []byte("sealed")

	firstTraversal := result.TraversalPrivate()
	firstIncoming := firstTraversal.Value.([]lsp.CallHierarchyIncomingCall)
	firstIncoming[0].From.Name = "mutated"
	firstIncoming[0].From.Tags[0] = 99
	firstIncoming[0].From.Data[0] = 'X'
	firstIncoming[0].FromRanges[0].Start.Line = 99
	firstTraversal.Artifact[0] = 'X'
	secondTraversal := result.TraversalPrivate()
	secondIncoming := secondTraversal.Value.([]lsp.CallHierarchyIncomingCall)
	if secondIncoming[0].From.Name != "source" || secondIncoming[0].From.Tags[0] != 1 || string(secondIncoming[0].From.Data) != `{"incoming":true}` || secondIncoming[0].FromRanges[0].Start.Line != 5 || string(secondTraversal.Artifact) != "artifact" {
		t.Fatal("ASSERT_PRODUCTION_PRIVATE_INCOMING_DEEP_IMMUTABLE")
	}

	firstOutgoing := result.OutgoingPrivate()
	firstOutgoing[0].To.Name = "mutated"
	firstOutgoing[0].To.Tags[0] = 99
	firstOutgoing[0].To.Data[0] = 'X'
	firstOutgoing[0].FromRanges[0].Start.Line = 99
	secondOutgoing := result.OutgoingPrivate()
	if secondOutgoing[0].To.Name != "target" || secondOutgoing[0].To.Tags[0] != 2 || string(secondOutgoing[0].To.Data) != `{"outgoing":true}` || secondOutgoing[0].FromRanges[0].Start.Line != 8 {
		t.Fatal("ASSERT_PRODUCTION_PRIVATE_OUTGOING_DEEP_IMMUTABLE")
	}

	sealed := result.SealedArtifactPrivate()
	sealed[0] = 'X'
	failure := result.FailurePrivate()
	failure.Code = "MUTATED"
	failure.Diagnostics[0] = "mutated"
	if string(result.SealedArtifactPrivate()) != "sealed" || result.FailurePrivate().Code != "FAIL" || result.FailurePrivate().Diagnostics[0] != "diagnostic" {
		t.Fatal("ASSERT_PRODUCTION_PRIVATE_SEALED_FAILURE_IMMUTABLE")
	}

	typeOf := reflect.TypeOf(result)
	for i := 0; i < typeOf.NumField(); i++ {
		if typeOf.Field(i).PkgPath == "" {
			t.Fatalf("ASSERT_PRODUCTION_PRIVATE_NO_EXPORTED_STATE: %s", typeOf.Field(i).Name)
		}
	}
}
