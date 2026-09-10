package mcpcontract

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRetainedRelationsCompositionPreservesManifestAndToolCardinality(t *testing.T) {
	historical, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(historical)
	base := WithRetainedCalls(historical)
	composed := WithRetainedRelations(base)
	after, _ := json.Marshal(historical)
	if !bytes.Equal(before, after) || len(composed.Tools) != len(base.Tools) {
		t.Fatalf("ASSERT_RETAINED_RELATIONS_ADDITIVE_CARDINALITY: base=%d composed=%d", len(base.Tools), len(composed.Tools))
	}
	var found bool
	for _, tool := range composed.Tools {
		if tool.Name != "lsp_trace_v1_export_retained_calls" {
			continue
		}
		found = tool.InputSchemaID == RetainedRelationsInputID && len(tool.ArtifactSchemaIDs) == 2 && tool.ArtifactSchemaIDs[1] == RetainedRelationsArtifactID
	}
	if !found {
		t.Fatal("ASSERT_RETAINED_RELATIONS_EXISTING_TOOL_COMPOSITION")
	}
	for name, raw := range map[string][]byte{
		"historical": []byte(`{"input":{}}`),
		"v3":         []byte(`{"input":{},"version":"v3"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateJSON(RetainedRelationsInputID, raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}
