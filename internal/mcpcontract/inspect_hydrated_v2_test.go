package mcpcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"testing"

	hi "lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/operation"
	ri "lsp-trace/internal/retainedinspection"
)

func TestHydratedV2CompositionPreservesV1BytesAndRegistersSuccessors(t *testing.T) {
	v1IDs := []string{hi.InputSchemaID, hi.SchemaID, HydratedEnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json"), HydratedEnvelopeID("https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json")}
	before := map[string][]byte{}
	hashes := map[string][32]byte{}
	for _, id := range v1IDs {
		b, err := SchemaJSON(id)
		if err != nil {
			t.Fatal(err)
		}
		before[id] = append([]byte(nil), b...)
		hashes[id] = sha256.Sum256(b)
	}
	base, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(base)
	composed := WithHydratedInspection(WithRetainedCalls(base))
	count := 0
	var tool ToolContract
	for _, candidate := range composed.Tools {
		if candidate.Name == HydratedTool {
			count++
			tool = candidate
		}
	}
	if count != 1 {
		t.Fatalf("operation 41 registration count=%d", count)
	}
	if tool.InputSchemaID != ri.InputSchemaV3ID {
		t.Fatalf("successor input not advertised: %s", tool.InputSchemaID)
	}
	for _, id := range []string{ri.ArtifactEnvelopeSchemaID, ri.ArtifactEnvelopeSchemaV3ID, ri.DomainErrorEnvelopeSchemaID} {
		if !containsString(tool.EnvelopeSchemaIDs, id) {
			t.Fatalf("missing envelope %s", id)
		}
	}
	for _, id := range []string{hi.SchemaID, ri.SourceProjectionSchemaID, ri.SourceProjectionSchemaV3ID} {
		if !containsString(tool.ArtifactSchemaIDs, id) {
			t.Fatalf("missing artifact %s", id)
		}
	}
	for _, id := range append(v1IDs, ri.InputSchemaID, ri.InputSchemaV3ID, ri.ArtifactEnvelopeSchemaID, ri.ArtifactEnvelopeSchemaV3ID, ri.DomainErrorEnvelopeSchemaID) {
		if _, err := SchemaJSON(id); err != nil {
			t.Fatalf("schema %s: %v", id, err)
		}
	}
	for _, id := range v1IDs {
		after, err := SchemaJSON(id)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before[id], after) || hashes[id] != sha256.Sum256(after) {
			t.Fatalf("predecessor schema changed: %s", id)
		}
	}
	afterBase, _ := json.Marshal(base)
	if !bytes.Equal(original, afterBase) {
		t.Fatal("composition mutated predecessor manifest")
	}
}

func TestHydratedV2SchemaJSONValidatesEnvelopes(t *testing.T) {
	artifact := []byte(`{"envelope_version":"1","envelope_schema_id":"` + ri.ArtifactEnvelopeSchemaID + `","tool":"` + HydratedTool + `","request_id":"r","outcome":"COMPLETE","operation_status":"SUCCEEDED","isError":false,"artifact_schema_id":"` + ri.SourceProjectionSchemaID + `","content":"{}"}`)
	if err := ValidateJSON(ri.ArtifactEnvelopeSchemaID, artifact); err != nil {
		t.Fatal(err)
	}
	domain := []byte(`{"envelope_version":"1","envelope_schema_id":"` + ri.DomainErrorEnvelopeSchemaID + `","tool":"` + HydratedTool + `","request_id":"r","outcome":"DOMAIN_ERROR","operation_status":"FAILED","isError":true,"phase":"ASSEMBLE","state":"ASSEMBLY_FAILED"}`)
	if err := ValidateJSON(ri.DomainErrorEnvelopeSchemaID, domain); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{
		bytes.Replace(artifact, []byte(`"content":"{}"`), []byte(`"content":{}`), 1),
		bytes.Replace(artifact, []byte(ri.SourceProjectionSchemaID), []byte(hi.SchemaID), 1),
		bytes.Replace(domain, []byte(`"DOMAIN_ERROR"`), []byte(`"COMPLETE"`), 1),
		bytes.Replace(domain, []byte(`"ASSEMBLY_FAILED"`), []byte(`"FICTIONAL"`), 1),
	} {
		if err := ValidateJSON(map[bool]string{true: ri.ArtifactEnvelopeSchemaID, false: ri.DomainErrorEnvelopeSchemaID}[bytes.Contains(bad, []byte(`"content"`))], bad); err == nil {
			t.Fatalf("adversarial envelope accepted: %s", bad)
		}
	}
}

func TestOperationValidatorUsesHydratedV2UnionWithoutNarrowingLegacy(t *testing.T) {
	validator, err := NewOperationInputValidator()
	if err != nil {
		t.Fatal(err)
	}
	legacy := json.RawMessage(`{"input":"{}","node_ids":["node"]}`)
	projection := json.RawMessage(`{"mode":"RETAINED_SOURCE_PROJECTION","retained_source_evidence":{"inline_snapshot_v2":"{}"},"selection":{"target":{"graph_subject_id":"subject","logical_source_id":"file:///source.go"},"selections":[{"graph_subject_id":"subject","logical_source_id":"file:///source.go"}]},"projection":{"body":"INCLUDE","privacy_policy_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","limits":{"max_source_bytes":1024,"max_ranges":8,"max_objects":8,"max_work":1024,"max_response_bytes":4096}},"resolve_limits":{"max_distinct_objects":8,"max_unique_source_bytes":1024,"max_logical_selections":8}}`)
	for _, input := range []json.RawMessage{legacy, projection} {
		if err := validator.ValidateOperationInput(operation.InspectHydrated, input); err != nil {
			t.Fatalf("ASSERT_OPERATION_41_V2_UNION_PREFLIGHT: %v", err)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
