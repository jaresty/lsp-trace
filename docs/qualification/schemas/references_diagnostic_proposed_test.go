package schemas_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// This only checks an unregistered draft shape against matrix row 33.
// It does not establish owner observations, privacy, replay, or qualification.
func TestADR0011ReferencesDiagnosticPostEvaluationWorkLimitShape(t *testing.T) {
	raw, err := os.ReadFile("adr0011-references-diagnostic.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	id := document["$id"].(string)
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(id, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	d := map[string]any{
		"version": "lsp-trace.adr0011.references-diagnostic.v1", "role": "PRIVATE_DIAGNOSTIC_ONLY",
		"privacyPolicyId": digest, "methodPolicyId": digest, "schemaDigest": digest,
		"implementationDigest": digest, "targetDigest": digest, "method": "textDocument/references",
		"queryOccurrenceId": "q", "sessionId": "s", "generation": 1, "requestKey": "k",
		"invocationId": "i", "paramsDigest": digest, "writeDigest": digest, "readDigest": digest,
		"sourceDigest": digest, "revisionDigest": digest, "responsePresence": "RESULT_PRESENT",
		"reason": "POST_EVALUATION_WORK_LIMIT", "outcome": "PARTIAL", "queryDisposition": "LIMITED",
		"counts": map[string]any{"declared": 1, "begun": 1, "terminal": 1, "elements": 2,
			"elementBegun": 1, "elementTerminal": 1, "parsed": 0, "admitted": 0,
			"diagnosticValidPrefix": 1},
		"error": map[string]any{"codePresence": "ABSENT"},
	}
	if err := schema.Validate(d); err != nil {
		t.Fatalf("ASSERT_R11_D_POST_WORK_PARTIAL_LIMITED: valid row-33 record rejected: %v", err)
	}
	d["outcome"] = "RESOURCE_LIMIT"
	if err := schema.Validate(d); err == nil {
		t.Fatal("ASSERT_R11_D_POST_WORK_REJECT_RESOURCE_LIMIT: row-33 misclassification accepted")
	}
	// A server-error D is allowed only with an explicitly authorized exact signed code.
	// Without that policy, the response is WITHHELD outside this D shape.
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var serverError map[string]any
	if err := json.Unmarshal(encoded, &serverError); err != nil {
		t.Fatal(err)
	}
	serverError["reason"] = "MATCHED_SERVER_ERROR"
	serverError["responsePresence"] = "MATCHED_ERROR"
	serverError["outcome"] = "PROVIDER_FAILURE"
	serverError["queryDisposition"] = "NONE"
	serverError["counts"] = map[string]any{"declared": 1, "begun": 0, "terminal": 0, "elements": "UNKNOWN", "elementBegun": 0, "elementTerminal": 0, "parsed": 0, "admitted": 0, "diagnosticValidPrefix": 0}
	serverError["error"] = map[string]any{"codePresence": "ABSENT"}
	if err := schema.Validate(serverError); err == nil {
		t.Fatal("ASSERT_SERVER_ERROR_CODE_WITHHELD: code-erased D accepted")
	}
	serverError["error"] = map[string]any{"codePresence": "PRESENT", "code": 0}
	if err := schema.Validate(serverError); err != nil {
		t.Fatalf("ASSERT_SERVER_ERROR_ZERO_AUTHORIZED_SHAPE: %v", err)
	}
}
