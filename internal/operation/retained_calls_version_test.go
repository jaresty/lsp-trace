package operation

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRetainedCallsExplicitVersionRejectsCrossFamily(t *testing.T) {
	// A syntactically valid request must not silently ignore explicit V2 selection.
	request, err := json.Marshal(map[string]any{
		"input":   map[string]any{"schema_version": "lsp-trace.graph-provenance.v1"},
		"version": "v2",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, failure := ExportRetainedCallsHandler(context.Background(), Request{Input: request})
	if failure == nil {
		t.Fatal("ASSERT_EXPLICIT_V2_EXPORT_VERSION_DISPATCH: explicit v2 was ignored")
	}
	if failure.Code != FailureInvalidInput {
		t.Fatalf("ASSERT_EXPLICIT_V2_EXPORT_WRONG_FAMILY_REJECTED: %+v", failure)
	}
}
