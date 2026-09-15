package schema

import (
	"encoding/json"
	"testing"
)

func TestVCSSymbolChurnV2SchemaRegisteredAndValidatesArtifact(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"schema_version": "lsp-trace.vcs-symbol-churn-sidecar.v2", "graph_artifact_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "from_revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "to_revision": "cccccccccccccccccccccccccccccccccccccccc", "old_acquisition": []any{}, "new_acquisition": []any{}, "authority": 0, "source_graph_complete": "UNKNOWN", "attribution": "HISTORICAL_SYMBOL_RANGE", "cross_revision_identity": "NOT_EVALUATED", "line_count": 0, "attributed_line_count": 0, "ambiguous_line_count": 0, "unmatched_line_count": 0, "lines": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	detected, err := ValidateFor(raw, FamilyVCSSymbolChurn, "v2")
	if err != nil || detected != "lsp-trace.vcs-symbol-churn-sidecar.v2" {
		t.Fatalf("ASSERT_VCS_SYMBOL_CHURN_V2_SCHEMA_REGISTERED: detected=%q err=%v", detected, err)
	}
}
