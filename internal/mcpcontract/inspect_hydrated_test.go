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
	runtime := WithHydratedInspection(previous)
	after, _ := json.Marshal(historical)
	if !bytes.Equal(before, after) || len(historical.Tools) != 13 || len(previous.Tools) != 21 || len(runtime.Tools) != 22 {
		t.Fatal("PUBLIC_ADDITIVE_CONTRACT FAIL")
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
	t.Log("PUBLIC_ADDITIVE_CONTRACT PASS: historical=13 previous=21 runtime=22; no publication schema")
}
