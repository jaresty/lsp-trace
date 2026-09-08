package operation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRetainedCallsExplicitVersionRejectsCrossFamily(t *testing.T) {
	// A syntactically valid request must not silently ignore explicit V2 selection.
	request, err := json.Marshal(map[string]any{
		"input":   map[string]any{"schema_version": "lsp-trace.graph-provenance.v1"},
		"version": "v3",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, failure := ExportRetainedCallsHandler(context.Background(), Request{Input: request})
	if failure == nil || !strings.Contains(failure.Error(), `unsupported retained-calls version "v3"`) {
		t.Fatalf("ASSERT_EXPLICIT_EXPORT_VERSION_DISPATCH: %+v", failure)
	}
}
