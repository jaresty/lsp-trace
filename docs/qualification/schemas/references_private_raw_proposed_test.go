package schemas_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Shape-only checks of an unapproved proposal. No capture, replay, privacy,
// aggregate accounting, file permissions or qualification is established here.
func TestADR0011ReferencesPrivateRawProposedShape(t *testing.T) {
	data, err := os.ReadFile("adr0011-references-private-raw.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
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
	base := map[string]any{
		"version": "lsp-trace.adr0011.references-private-raw.v1", "role": "OWNER_ONLY_MALFORMED_RESULT_REPLAY",
		"privacyPolicyId": digest, "methodPolicyId": digest, "schemaDigest": digest,
		"implementationDigest": digest, "targetDigest": digest, "method": "textDocument/references",
		"queryOccurrenceId": "q", "sessionId": "s", "generation": 1, "requestKey": "k",
		"invocationId": "i", "paramsDigest": digest, "writeDigest": digest, "readDigest": digest,
		"sourceDigest": digest, "revisionDigest": digest, "responsePresence": "RESULT_PRESENT",
		"reason": "MALFORMED_RESULT", "rawResultDigest": digest, "rawResultBytes": 1048576,
		"access": "OWNER_ONLY", "rawResultSelector": "adr0011-references-private-raw-v1-" + strings.Repeat("a", 64) + ".bin",
	}
	missingSelector := make(map[string]any, len(base))
	for key, value := range base {
		if key != "rawResultSelector" {
			missingSelector[key] = value
		}
	}
	if err := schema.Validate(missingSelector); err == nil {
		t.Error("ASSERT_SELECTOR_MISSING_REJECTED: missing selector accepted")
	}
	if err := schema.Validate(base); err != nil {
		t.Fatalf("ASSERT_RAW_BOUNDARY_ACCEPTED: %v", err)
	}
	cases := []struct {
		name, key string
		value     any
		remove    bool
	}{
		{"over_result", "rawResultBytes", 1048577, false},
		{"ASSERT_SELECTOR_PATH", "rawResultSelector", "dir/adr0011-references-private-raw-v1-" + strings.Repeat("a", 64) + ".bin", false},
		{"ASSERT_SELECTOR_SUFFIX", "rawResultSelector", "adr0011-references-private-raw-v1-" + strings.Repeat("a", 64) + ".json", false},
		{"ASSERT_SELECTOR_UPPER", "rawResultSelector", "adr0011-references-private-raw-v1-" + strings.Repeat("A", 64) + ".bin", false},
		{"zero_result", "rawResultBytes", 0, false},
		{"public_access", "access", "PUBLIC", false},
		{"wrong_role", "role", "PRIVATE_DIAGNOSTIC_ONLY", false},
		{"missing_request", "requestKey", nil, true},
		{"raw_canary", "rawResult", "SECRET_CANARY", false},
		{"missing_result", "responsePresence", "RESULT_ABSENT", false},
		{"success_receipt", "reason", "SUCCESS", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := make(map[string]any, len(base)+1)
			for k, v := range base {
				candidate[k] = v
			}
			if tc.remove {
				delete(candidate, tc.key)
			} else {
				candidate[tc.key] = tc.value
			}
			if err := schema.Validate(candidate); err == nil {
				t.Fatalf("ASSERT_RAW_%s_REJECTED: invalid manifest accepted", tc.name)
			}
		})
	}
}
