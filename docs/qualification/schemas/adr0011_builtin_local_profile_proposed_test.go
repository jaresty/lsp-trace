package schemas_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Proposal-only: synthetic records and original-file pins, never a host selector.
func builtinBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func builtinDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func builtinObject(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func builtinCanonical(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func builtinRole(t *testing.T, schema []byte, name string) *jsonschema.Schema {
	t.Helper()
	doc := builtinObject(t, schema)
	id := doc["$id"].(string)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	r, err := c.Compile(id + "#/$defs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func builtinRef(role, uri string) map[string]any {
	return map[string]any{"role": role, "schema_version": uri, "selector": role + ".json", "digest": "sha256:" + strings.Repeat("a", 64), "byte_length": 1}
}
func TestADR0011BuiltinLocalProfilePinnedProposal(t *testing.T) {
	profileBytes := builtinBytes(t, "../profiles/adr0011-builtin-local-references-v1.proposed.json")
	if !bytes.Equal(profileBytes, builtinCanonical(t, builtinObject(t, profileBytes))) {
		t.Fatal("ASSERT_BUILTIN_CANONICAL_PROFILE")
	}
	profile := builtinObject(t, profileBytes)
	schemaBytes := builtinBytes(t, "adr0011-builtin-local-references-v1.proposed.schema.json")
	if err := builtinRole(t, schemaBytes, "profile").Validate(profile); err != nil {
		t.Fatalf("ASSERT_BUILTIN_PROFILE_SCHEMA: %v", err)
	}
	check := func(desc map[string]any, file string) {
		t.Helper()
		b := builtinBytes(t, file)
		if desc["digest"] != builtinDigest(b) || desc["byte_length"] != float64(len(b)) {
			t.Fatalf("ASSERT_PIN_%s", file)
		}
	}
	check(profile["admission_schema"].(map[string]any), "adr0011-production-admission-v1.proposed.schema.json")
	check(profile["historical_schema"].(map[string]any), "adr0011-references-issuance-records.proposed.schema.json")
	check(profile["request_key_successor_schema"].(map[string]any), "adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	check(profile["profile_schema"].(map[string]any), "adr0011-builtin-local-references-v1.proposed.schema.json")
	target := profile["target_query"].(map[string]any)
	check(target["schema"].(map[string]any), "adr0011-references-issuance-records.proposed.schema.json")
	check(target["policy"].(map[string]any), "../policies/adr0011-document-symbol-target-policy-v1.proposed.json")
	policy := builtinObject(t, builtinBytes(t, "../policies/adr0011-document-symbol-target-policy-v1.proposed.json"))
	if policy["selection"] != "UNIQUE_STRICT_MOST_SPECIFIC_NESTED_SELECTION_CONTAINMENT" || policy["crossing_nonminimal"] != "ALLOW_IF_UNIQUE_STRICT_INNER" || policy["disposition_on_nonunique"] != "TARGET_IDENTITY_UNRESOLVED_NO_REFERENCES_WRITE" {
		t.Fatal("ASSERT_REVIEWED_NESTED_TARGET_SELECTION_RULE")
	}
	if target["schema_role_uri"] != target["schema"].(map[string]any)["uri"].(string)+"#/$defs/targetResult" {
		t.Fatal("ASSERT_INDEPENDENT_TARGET_SCHEMA_ROLE")
	}
	for role, path := range map[string]string{"method": "adr0011-references-method-policy-v1.proposed.json", "admission": "adr0011-references-admission-policy-v1.proposed.json", "privacy": "adr0011-references-privacy-policy-v1.proposed.json", "retention": "adr0011-references-retention-policy-v1.proposed.json"} {
		check(profile["policies"].(map[string]any)[role].(map[string]any), filepath.Join("../policies", path))
	}
	admission := builtinBytes(t, "adr0011-production-admission-v1.proposed.schema.json")
	refShape := builtinRole(t, admission, "ref")
	base := profile["historical_schema"].(map[string]any)["uri"].(string) + "#/$defs/"
	roles := profile["historical_roles"].(map[string]any)
	if len(roles) != 20 {
		t.Fatal("ASSERT_TWENTY_HISTORICAL_ROLES")
	}
	for role, def := range roles {
		ref := builtinRef(role, base+def.(string))
		if err := refShape.Validate(ref); err != nil {
			t.Fatalf("ASSERT_HISTORICAL_ROLE_%s: %v", role, err)
		}
		ref["schema_version"] = profile["production_query_declaration_role"]
		if err := refShape.Validate(ref); err == nil {
			t.Fatalf("ASSERT_ROLE_URI_SWAP_%s", role)
		}
	}
	decl := builtinRef("production_query_declaration", profile["production_query_declaration_role"].(string))
	if err := refShape.Validate(decl); err != nil {
		t.Fatalf("ASSERT_DECLARATION_ROLE: %v", err)
	}
	witness := builtinObject(t, builtinCanonical(t, profile["witness_policy"]))
	witness["declaration_ref"] = decl
	if err := builtinRole(t, admission, "attemptWitnessPolicy").Validate(witness); err != nil {
		t.Fatalf("ASSERT_WITNESS_TEMPLATE: %v", err)
	}
	witness["selected_roles"] = []string{"attempt_query_begin", "attempt_initiation"}
	if err := builtinRole(t, admission, "attemptWitnessPolicy").Validate(witness); err == nil {
		t.Fatal("ASSERT_WITNESS_ROLE_ORDER")
	}
	if profile["activation"] != "EXPLICIT_REFERENCES_OPERATION_ONLY" || profile["selection"].(map[string]any)["profile_override_from_request"] != false {
		t.Fatal("ASSERT_NO_IMPLICIT_OR_REQUEST_OVERRIDE")
	}
	bad := builtinObject(t, profileBytes)
	bad["model"] = "latest"
	if err := builtinRole(t, schemaBytes, "profile").Validate(bad); err == nil {
		t.Fatal("ASSERT_NO_LATEST_PROFILE")
	}
}

func TestADR0011BuiltinLocalPlanAndSelectionProposal(t *testing.T) {
	schema := builtinBytes(t, "adr0011-builtin-local-references-v1.proposed.schema.json")
	p := builtinBytes(t, "../profiles/adr0011-builtin-local-references-v1.proposed.json")
	pd := builtinDigest(p)
	profile := builtinObject(t, p)
	d := "sha256:" + strings.Repeat("b", 64)
	measure := map[string]any{"role": "ADR0011_INSTALLED_EXECUTABLE_MEASUREMENT_V1", "version": 1, "profile_digest": pd, "executable_digest": d, "executable_byte_length": 5, "adapter_scope": "LINKED_INTO_INSTALLED_EXECUTABLE_ONLY_V1", "measurement_rule": "PIN_OPENED_EXECUTABLE_FILE_COMPARE_PATH_AT_START_PREWRITE_POSTCAPTURE_V1"}
	if err := builtinRole(t, schema, "implementationMeasurement").Validate(measure); err != nil {
		t.Fatalf("ASSERT_MEASUREMENT_SHAPE: %v", err)
	}
	measured := builtinDigest(builtinCanonical(t, measure))
	ref := builtinRef("prepared_source", "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json#/$defs/preparedSource")
	git := builtinRef("host_git_before", "https://jaresty.github.io/lsp-trace/schemas/adr0011-references-issuance-records.proposed.schema.json#/$defs/hostGit")
	policies := map[string]any{}
	for _, role := range []string{"method", "admission", "privacy", "retention"} {
		policies[role] = profile["policies"].(map[string]any)[role].(map[string]any)["digest"]
	}
	selectedSchema := profile["admission_schema"].(map[string]any)["digest"]
	target := profile["target_query"].(map[string]any)
	targetSchema := target["schema"].(map[string]any)["digest"]
	targetPolicy := target["policy"].(map[string]any)["digest"]
	targetIdentity := map[string]any{"measurement_digest": measured, "target_policy_digest": targetPolicy, "target_schema_digest": targetSchema, "target_role_uri": target["schema_role_uri"]}
	targetImplementation := builtinDigest(append([]byte("ADR0011_DOCUMENT_SYMBOL_TARGET_IMPLEMENTATION_V1\x00"), builtinCanonical(t, targetIdentity)...))
	plan := map[string]any{"role": "ADR0011_LOCAL_REFERENCES_PLAN_V1", "version": 1, "profile_digest": pd, "session_id": "synthetic", "generation": 1, "workspace_uri": "file:///fixture", "query_uri": "file:///fixture/a.go", "line": 1, "character": 2, "encoding": "utf-16", "document_version": 1, "document_byte_length": 3, "document_digest": d, "source_ref": ref, "git_root_uri": "file:///fixture", "git_commit": strings.Repeat("a", 40), "git_before": git, "policy_digests": policies, "schema_digest": selectedSchema, "implementation_digest": measured, "invocation_nonce": strings.Repeat("c", 32), "params_digest": d, "target_schema_digest": targetSchema, "target_policy_digest": targetPolicy, "target_implementation_digest": targetImplementation, "limits": map[string]any{"max_messages": 4, "max_wire_bytes": 4096, "timeout_ms": 1000, "max_work": 1000}}
	if err := builtinRole(t, schema, "plan").Validate(plan); err != nil {
		t.Fatalf("ASSERT_PLAN_SHAPE: %v", err)
	}
	planBytes := builtinCanonical(t, plan)
	planID := builtinDigest(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1\x00"), planBytes...))
	planRef := builtinRef("plan", "https://jaresty.github.io/lsp-trace/schemas/adr0011-builtin-local-references-v1.proposed.schema.json#/$defs/plan")
	planRef["digest"], planRef["byte_length"] = builtinDigest(planBytes), len(planBytes)
	selection := map[string]any{"role": "ADR0011_LOCAL_REFERENCES_SELECTION_V1", "version": 1, "profile_digest": pd, "plan_id": planID, "plan_ref": planRef, "declaration_ref": builtinRef("production_query_declaration", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/productionQueryDeclaration"), "implementation_digest": measured, "policy_digests": policies, "schema_digest": selectedSchema, "root_identity_digest": d, "session_id": "synthetic", "generation": 1, "invocation_nonce": strings.Repeat("c", 32), "expected_custody_ref": builtinRef("attempt_host_custody", "https://jaresty.github.io/lsp-trace/schemas/adr0011-production-admission-v1.proposed.schema.json#/$defs/attemptHostCustody"), "target_schema_digest": targetSchema, "target_policy_digest": targetPolicy, "target_implementation_digest": targetImplementation}
	if err := builtinRole(t, schema, "hostSelection").Validate(selection); err != nil {
		t.Fatalf("ASSERT_SELECTION_SHAPE: %v", err)
	}
	claimed := builtinObject(t, builtinCanonical(t, plan))
	claimed["invocation_nonce"] = strings.Repeat("d", 32)
	changedID := builtinDigest(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1\x00"), builtinCanonical(t, claimed)...))
	if changedID == planID || selection["plan_id"] == changedID {
		t.Fatal("ASSERT_HELD_PLAN_REHASH_REJECT")
	}
	changedTarget := builtinObject(t, builtinCanonical(t, plan))
	changedTarget["target_policy_digest"] = d
	if err := builtinRole(t, schema, "plan").Validate(changedTarget); err != nil {
		t.Fatalf("ASSERT_SCHEMA_VALID_TARGET_SUBSTITUTION: %v", err)
	}
	changedTargetID := builtinDigest(append([]byte("ADR0011_LOCAL_REFERENCES_PLAN_V1\x00"), builtinCanonical(t, changedTarget)...))
	if changedTargetID == planID || changedTargetID == selection["plan_id"] || changedTarget["target_policy_digest"] == targetPolicy {
		t.Fatal("ASSERT_HELD_TARGET_POLICY_REHASH_REJECT")
	}
	bad := builtinObject(t, builtinCanonical(t, selection))
	bad["unrecognized"] = true
	if err := builtinRole(t, schema, "hostSelection").Validate(bad); err == nil {
		t.Fatal("ASSERT_CLOSED_SELECTION")
	}
	for _, role := range []string{"method", "admission", "privacy", "retention"} {
		if policies[role] != profile["policies"].(map[string]any)[role].(map[string]any)["digest"] {
			t.Fatalf("ASSERT_PROFILE_POLICY_%s", role)
		}
	}
}
