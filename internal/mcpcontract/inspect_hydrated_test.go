package mcpcontract

import (
	"bytes"
	"encoding/json"
	hi "lsp-trace/internal/hydratedinspection"
	"testing"
)

func TestHydratedContractAdditive(t *testing.T) {
	historical, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(historical)
	previous := WithRetainedCalls(historical)
	hydrated := WithHydratedInspection(previous)
	exportedV2 := WithRetainedCallsV2Export(hydrated)
	runtime := WithRetainedCallsV2Verifier(exportedV2)
	after, _ := json.Marshal(historical)
	if !bytes.Equal(before, after) || len(historical.Tools) != 13 || len(previous.Tools) != 20 || len(hydrated.Tools) != 21 || len(exportedV2.Tools) != 22 || len(runtime.Tools) != 23 {
		t.Fatalf("PUBLIC_ADDITIVE_CONTRACT FAIL: historical=%d previous=%d hydrated=%d exported_v2=%d runtime=%d", len(historical.Tools), len(previous.Tools), len(hydrated.Tools), len(exportedV2.Tools), len(runtime.Tools))
	}
	for _, id := range []string{hi.InputSchemaID, hi.SchemaID, HydratedEnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json"), HydratedEnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json")} {
		if _, err := SchemaJSON(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateJSON(hi.InputSchemaID, []byte(`{"input":"{}","node_ids":["unknown"],"include_bodies":false}`)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"input":{},"node_ids":[]}`, `{"input":"{}","position_encoding":"guess"}`, `{"input":"{}","core_policy":{"max_work":1.5}}`, `{"input":"{}","output_selector":"no-publication"}`} {
		if err := ValidateJSON(hi.InputSchemaID, []byte(raw)); err == nil {
			t.Fatal("PUBLIC_ADDITIVE_CONTRACT FAIL: loose input")
		}
	}
	t.Log("PUBLIC_ADDITIVE_CONTRACT PASS: historical=13 previous=20 hydrated=21 exported_v2=22 runtime=23; no publication schema")
}
