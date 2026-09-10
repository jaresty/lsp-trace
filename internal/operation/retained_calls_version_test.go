package operation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRetainedCallsExplicitVersionV3RejectsCrossFamily(t *testing.T) {
	// A syntactically valid request must dispatch V3 to strict graph-provenance V5 admission.
	request, err := json.Marshal(map[string]any{
		"input":   map[string]any{"schema_version": "lsp-trace.graph-provenance.v1"},
		"version": "v3",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, failure := ExportRetainedCallsHandler(context.Background(), Request{Input: request})
	if failure == nil || !strings.Contains(failure.Error(), `verified graph-provenance v5 required`) {
		t.Fatalf("ASSERT_EXPLICIT_EXPORT_VERSION_DISPATCH: %+v", failure)
	}
}
