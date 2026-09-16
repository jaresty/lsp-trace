package schema

import (
	"crypto/sha256"
	"encoding/hex"
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

func TestVCSSymbolChurnV2SchemaBytesRemainImmutable(t *testing.T) {
	raw, err := BytesFor(FamilyVCSSymbolChurn, "v2")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != "e074186e1b7563f97f4ebdbb27bcb6c0ce7208382ae03902c494dc603c12782e" {
		t.Fatalf("ASSERT_VCS_SYMBOL_CHURN_V2_SCHEMA_BYTES_IMMUTABLE: sha256=%s", got)
	}
}

func TestVCSSymbolChurnV3SchemaRegisteredAndValidatesArtifact(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"schema_version": "lsp-trace.vcs-symbol-churn-sidecar.v3", "graph_artifact_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "from_revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "to_revision": "cccccccccccccccccccccccccccccccccccccccc", "old_acquisition": []any{}, "new_acquisition": []any{}, "authority": 0, "source_graph_complete": "UNKNOWN", "attribution": "HISTORICAL_SYMBOL_RANGE", "cross_revision_identity": "NOT_EVALUATED", "line_count": 0, "attributed_line_count": 0, "ambiguous_line_count": 0, "unmatched_line_count": 0, "lines": []any{}, "historical_symbol_metrics": []any{}, "current_symbol_metrics": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	detected, err := ValidateFor(raw, FamilyVCSSymbolChurn, "v3")
	if err != nil || detected != "lsp-trace.vcs-symbol-churn-sidecar.v3" {
		t.Fatalf("ASSERT_VCS_SYMBOL_CHURN_V3_SCHEMA_REGISTERED: detected=%q err=%v", detected, err)
	}
}
