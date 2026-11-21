package inventorysuccessor

import (
	"context"
	"encoding/json"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/sessionruntime"
	"testing"
)

// These private synthetic metadata states deliberately have no kernel/runtime.
// A rejecting call must retire before anything could reach either missing owner.
func TestRetainedMixedRefusalRetires(t *testing.T) {
	for _, mode := range []string{"mismatch", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			r := &privateRetainedMixedInputV1{acquisition: &privateDiscoveryOperationV2{binding: PrivateDiscoveryBindingV2{Revision: "revision"}}, descriptor: sessionruntime.PrivateDiscoveryGroupingQuery{Method: "textDocument/references", URI: "file:///work/a.go"}, occurrence: "original"}
			q := PrivateMixedInputRequestV2{retained: r, URI: r.descriptor.URI, OccurrenceID: r.occurrence, Revision: "revision", Privacy: "PRIVATE", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Kinds: []pa.RelationKind{pa.TypedReferencesSymbol}}
			op := PrivateMixedInputOperationV2{state: &privateMixedInputOperationV2{request: q, owner: &PrivateManagedInputOwnerV2{}, retainedContext: context.Background()}}
			bad := q
			ctx := context.Background()
			if mode == "mismatch" {
				bad.URI = "file:///other.go"
			} else {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			if _, e := ExecuteMixedInputV2(ctx, op, bad); e == nil || !op.state.used {
				t.Fatal("ASSERT_RETAINED_M_RETIRE: refusal did not retire")
			}
			if _, e := ExecuteMixedInputV2(context.Background(), op, q); e == nil {
				t.Fatal("ASSERT_RETAINED_M_RETIRE: replay succeeded")
			}
		})
	}
}
func TestRetainedCompletionCannotBeDecoded(t *testing.T) {
	var input PrivateManagedGroupingInput
	if e := json.Unmarshal([]byte(`{"sourceConfirmed":true,"retainedCompletion":{"head":"fake"}}`), &input); e != nil {
		t.Fatal(e)
	}
	if input.sourceConfirmed || input.retainedCompletion != nil {
		t.Fatal("decoded completion authority")
	}
	if _, e := input.SourceOriginalsV1(); e == nil {
		t.Fatal("decoded source originals")
	}
}
