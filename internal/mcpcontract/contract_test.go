package mcpcontract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStage1Contract(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatalf("P1_MANIFEST: LoadManifest: %v", err)
	}
	if err := ValidateManifest(manifest); err != nil {
		t.Errorf("P1_MANIFEST: %v", err)
	}
	for _, registration := range manifest.Schemas {
		if err := ValidateSchemaIdentity(registration); err != nil {
			t.Errorf("P2_SCHEMA_IDENTITY[%s]: %v", registration.ID, err)
		}
	}
	capabilityTool := findTool(manifest, "lsp_trace_v1_capabilities")
	if capabilityTool == nil {
		t.Fatal("P3_STRUCTURAL_VALIDATION: capabilities tool missing")
	}
	if err := ValidateJSON(capabilityTool.InputSchemaID, []byte(`{}`)); err != nil {
		t.Errorf("P3_STRUCTURAL_VALIDATION: valid input rejected: %v", err)
	}
	if err := ValidateJSON(capabilityTool.InputSchemaID, []byte(`{"extra":true}`)); err == nil {
		t.Error("P3_STRUCTURAL_VALIDATION: closed input accepted unknown field")
	}
	complete := []byte(`{"envelope_version":"1","envelope_schema_id":"https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-result.v1.schema.json","tool":"lsp_trace_v1_capabilities","request_id":"manager-1","outcome":"COMPLETE","operation_status":"SUCCEEDED","isError":false,"result":{"manifest_version":"1"}}`)
	if err := ValidateEnvelopeExclusive(complete); err != nil {
		t.Errorf("P4_ENVELOPE_EXCLUSIVE: %v", err)
	}
	if err := ValidateTranscripts(manifest); err != nil {
		t.Errorf("P5_TRANSCRIPTS: %v", err)
	}
}

func findTool(manifest *Manifest, name string) *ToolContract {
	for i := range manifest.Tools {
		if manifest.Tools[i].Name == name {
			return &manifest.Tools[i]
		}
	}
	return nil
}

func TestManifestIsSoleExactAuthority(t *testing.T) {
	const (
		coverageAssertion = "manifest contains all twelve recognized canonical tools with seven advertised enabled and five unadvertised reserved"
		aliasAssertion    = "canonical names and aliases are globally unique"
		inputAssertion    = "each enabled canonical tool has an exact dedicated closed input schema"
	)
	for _, assertion := range []string{coverageAssertion, aliasAssertion, inputAssertion} {
		t.Log("ASSERTION: " + assertion)
	}
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(manifest.Tools); got != 12 {
		t.Errorf("%s: got %d", coverageAssertion, got)
	}
	seen := map[string]string{}
	enabled, reserved := 0, 0
	inputIDs := map[string]bool{}
	for _, tool := range manifest.Tools {
		for _, name := range append([]string{tool.Name}, tool.Aliases...) {
			if prior := seen[name]; prior != "" {
				t.Errorf("%s: %q also owned by %s", aliasAssertion, name, prior)
			}
			seen[name] = tool.Name
		}
		if tool.Advertised && tool.Availability == "ENABLED" {
			enabled++
			if inputIDs[tool.InputSchemaID] {
				t.Errorf("%s: shared %s", inputAssertion, tool.InputSchemaID)
			}
			inputIDs[tool.InputSchemaID] = true
			raw, err := SchemaJSON(tool.InputSchemaID)
			if err != nil {
				t.Errorf("%s: %s: %v", inputAssertion, tool.Name, err)
				continue
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil || schema["additionalProperties"] != false {
				t.Errorf("%s: %s is not closed", inputAssertion, tool.Name)
			}
		} else if !tool.Advertised && tool.Availability == "NOT_IMPLEMENTED" {
			reserved++
		}
	}
	if enabled != 7 || reserved != 5 {
		t.Errorf("%s: enabled=%d reserved=%d", coverageAssertion, enabled, reserved)
	}
}

func TestNegativeControls(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("P1_MANIFEST", func(t *testing.T) {
		wrong := *manifest
		wrong.Tools = wrong.Tools[:len(wrong.Tools)-1]
		if err := ValidateManifest(&wrong); err == nil || !strings.Contains(err.Error(), "canonical tool count") {
			t.Fatalf("wrong tool count accepted: %v", err)
		}
	})
	t.Run("P2_SCHEMA_IDENTITY", func(t *testing.T) {
		err := ValidateSchemaIdentity(SchemaRegistration{ID: "https://example.test/schema.json", Family: "bad", Layer: "artifact", Path: "bad.json"})
		if err == nil || !strings.Contains(err.Error(), "versioned") {
			t.Fatalf("unversioned identity accepted: %v", err)
		}
	})
	t.Run("P3_GLOBAL_ALIAS_UNIQUENESS", func(t *testing.T) {
		wrong := *manifest
		wrong.Tools = append([]ToolContract(nil), manifest.Tools...)
		wrong.Tools[1].Aliases = []string{wrong.Tools[0].Name}
		if err := ValidateManifest(&wrong); err == nil || !strings.Contains(err.Error(), "name or alias") {
			t.Fatalf("duplicate canonical/alias accepted: %v", err)
		}
	})
	t.Run("P4_ENVELOPE_EXCLUSIVE", func(t *testing.T) {
		wrong := []byte(`{"envelope_version":"1","envelope_schema_id":"wrong","tool":"lsp_trace_v1_capabilities","request_id":"manager-1","outcome":"COMPLETE","operation_status":"SUCCEEDED","isError":false,"result":{}}`)
		if err := ValidateEnvelopeExclusive(wrong); err == nil || !strings.Contains(err.Error(), "exactly named schema") {
			t.Fatalf("wrong envelope identity accepted: %v", err)
		}
	})
	t.Run("P5_TRANSCRIPTS", func(t *testing.T) {
		wrong := []byte("{ \"error\":{},\"structuredContent\":{}}\n")
		if err := ValidateTranscript(wrong, true); err == nil {
			t.Fatal("noncanonical pre-envelope operation envelope accepted")
		}
	})
}
