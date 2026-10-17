package retainedcalls

import (
	"errors"
	"testing"
)

func TestRetainedCallsV2RejectsGraphProvenanceV4WithoutLoss(t *testing.T) {
	_, err := ExportV2([]byte(`{"schema_version":"lsp-trace.graph-provenance.v4"}`))
	if !errors.Is(err, ErrGraphProvenanceV4Unsupported) {
		t.Fatalf("ASSERT_RETAINED_CALLS_V2_V4_TYPED_REFUSAL: %v", err)
	}
}
