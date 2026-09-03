package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	semanticfilter "lsp-trace/internal/filter"
	"lsp-trace/internal/schema"
)

func filterInspectionFixture(t *testing.T) (string, []byte) {
	t.Helper()
	_, paths := inspectFixture(t)
	artifact, _, _ := strings.Cut(paths, "\x00")
	stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--all-seeds", "--json"})
	if code != 0 || stderr != "" {
		t.Fatalf("prepare inspection: code=%d stderr=%q", code, stderr)
	}
	path := filepath.Join(t.TempDir(), "inspection.json")
	data := []byte(stdout)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path, data
}

func TestFilterHelpAndModeErrors(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"filter", "--help"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "--compare-seeds") {
		t.Fatalf("ASSERT_FILTER_HELP_AND_MODE_ERRORS: help code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func assertFilterFailure(t *testing.T, name string, args []string, want string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		stdout, stderr, code := captureRun(t, args)
		if code != 1 || stdout != "" || !strings.Contains(stderr, want) {
			t.Fatalf("ASSERT_FILTER_%s: code=%d stdout=%q stderr=%q want=%q", name, code, stdout, stderr, want)
		}
	})
}

func decodeFilter(t *testing.T, stdout string) semanticfilter.Projection {
	t.Helper()
	var got semanticfilter.Projection
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode filter projection: %v", err)
	}
	return got
}

func TestFilterCLIParsingAndValidationBeforeRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	assertFilterFailure(t, "CLI_ZERO_OPERANDS_BEFORE_READ", []string{"filter", missing}, "exactly two --compare-seeds")
	assertFilterFailure(t, "CLI_ONE_OPERAND_BEFORE_READ", []string{"filter", missing, "--compare-seeds", "left"}, "exactly two --compare-seeds")
	assertFilterFailure(t, "CLI_THREE_OPERANDS_BEFORE_READ", []string{"filter", missing, "--compare-seeds", "left", "--compare-seeds", "right", "--compare-seeds", "third"}, "exactly two --compare-seeds")
	assertFilterFailure(t, "CLI_EMPTY_OPERAND_BEFORE_READ", []string{"filter", missing, "--compare-seeds", "", "--compare-seeds", "right"}, "must be nonempty")
	assertFilterFailure(t, "CLI_EQUAL_OPERANDS_BEFORE_READ", []string{"filter", missing, "--compare-seeds", "same", "--compare-seeds", "same"}, "must be distinct")
	assertFilterFailure(t, "CLI_TRAILING_ARGUMENT_BEFORE_READ", []string{"filter", missing, "--compare-seeds", "left", "--compare-seeds", "right", "extra"}, filterUsage)
}

func TestFilterRejectsInputFamiliesAndUsesExactSeedLookup(t *testing.T) {
	path, _ := filterInspectionFixture(t)
	assertFilterFailure(t, "EXACT_SEED_LOOKUP_CASE_SENSITIVE", []string{"filter", path, "--compare-seeds", "Chosen", "--compare-seeds", "other"}, `seed label "Chosen" not found`)

	for _, tc := range []struct {
		name, body string
	}{
		{"UNKNOWN_JSON", `{}`},
		{"GRAPH_FAMILY", `{"schema_version":"lsp-trace.graph.v3"}`},
		{"SINGLE_SEED_INSPECTION", `{"inspection_schema_version":"lsp-trace.inspect.v1","projection_kind":"SEED_INSPECTION","authority":"NON_AUTHORITATIVE_DERIVED_VIEW"}`},
		{"CONFLICTING_GRAPH_MARKER", `{"inspection_schema_version":"lsp-trace.inspect.v1","projection_kind":"ALL_SEEDS","authority":"NON_AUTHORITATIVE_DERIVED_VIEW","schema_version":"lsp-trace.graph.v3"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(input, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			assertFilterFailure(t, tc.name, []string{"filter", input, "--compare-seeds", "left", "--compare-seeds", "right"}, "input must be lsp-trace.inspect.v1 ALL_SEEDS")
		})
	}
}

func runFilterFixture(t *testing.T, assertion string) (string, string, []byte) {
	t.Helper()
	path, before := filterInspectionFixture(t)
	stdout, stderr, code := captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code != 0 || stderr != "" {
		t.Fatalf("%s: filter fixture code=%d stderr=%q", assertion, code, stderr)
	}
	return path, stdout, before
}

func TestFilterPreservesOptionalExecutionBundleCustody(t *testing.T) {
	path, data := filterInspectionFixture(t)
	var inspection map[string]any
	if err := json.Unmarshal(data, &inspection); err != nil {
		t.Fatal(err)
	}
	identity := inspection["artifact_identity"].(map[string]any)
	delete(identity, "execution_bundle_id")
	withoutBundle, _ := json.Marshal(inspection)
	if err := os.WriteFile(path, withoutBundle, 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code != 0 || stderr != "" {
		t.Fatalf("ASSERT_FILTER_OPTIONAL_EXECUTION_BUNDLE_ABSENCE: code=%d stderr=%q", code, stderr)
	}
	var projection map[string]any
	_ = json.Unmarshal([]byte(stdout), &projection)
	if _, present := projection["input_identity"].(map[string]any)["execution_bundle_id"]; present {
		t.Fatalf("ASSERT_FILTER_OPTIONAL_EXECUTION_BUNDLE_ABSENCE: invented execution bundle")
	}

	identity["execution_bundle_id"] = "not-a-digest"
	malformed, _ := json.Marshal(inspection)
	if err := os.WriteFile(path, malformed, 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "execution_bundle_id") {
		t.Fatalf("ASSERT_FILTER_MALFORMED_EXECUTION_BUNDLE_REJECTED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestFilterTypedPartitionsAndOrder(t *testing.T) {
	_, stdout, _ := runFilterFixture(t, "ASSERT_FILTER_TYPED_PARTITIONS_AND_ORDER")
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	nodes := got["partitions"].(map[string]any)["nodes"].(map[string]any)
	if len(nodes["shared"].([]any)) != 0 || len(nodes["left_only"].([]any)) != 2 || len(nodes["right_only"].([]any)) != 0 {
		t.Fatalf("ASSERT_FILTER_TYPED_PARTITIONS_AND_ORDER: nodes=%v", nodes)
	}
}

func TestFilterSchemaSemanticAccounting(t *testing.T) {
	_, stdout, _ := runFilterFixture(t, "ASSERT_FILTER_SCHEMA_SEMANTIC_ACCOUNTING")
	if _, err := schema.ValidateFor([]byte(stdout), schema.FamilyFilter, "v1"); err != nil {
		t.Fatalf("valid output: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(stdout), &got)
	got["accounting"].(map[string]any)["nodes"].(map[string]any)["pair_universe_count"] = float64(99)
	mutated, _ := json.Marshal(got)
	if _, err := schema.ValidateFor(mutated, schema.FamilyFilter, "v1"); err == nil || !strings.Contains(err.Error(), "semantic validation") {
		t.Fatalf("ASSERT_FILTER_SCHEMA_SEMANTIC_ACCOUNTING: mutation accepted: %v", err)
	}
}

func TestFilterRejectsCommonAccountingMismatch(t *testing.T) {
	_, stdout, _ := runFilterFixture(t, "ASSERT_FILTER_COMMON_ACCOUNTING_RECONCILES")
	var got map[string]any
	_ = json.Unmarshal([]byte(stdout), &got)
	got["accounting"].(map[string]any)["requested_seed_count"] = float64(999)
	mutated, _ := json.Marshal(got)
	if _, err := schema.ValidateFor(mutated, schema.FamilyFilter, "v1"); err == nil || !strings.Contains(err.Error(), "common seed accounting") {
		t.Fatalf("ASSERT_FILTER_COMMON_ACCOUNTING_RECONCILES: mutation accepted: %v", err)
	}
}

func TestFilterRejectsStateReferenceMismatch(t *testing.T) {
	_, stdout, _ := runFilterFixture(t, "ASSERT_FILTER_STATE_MATCHES_REFERENCE_COUNTS")
	var got map[string]any
	_ = json.Unmarshal([]byte(stdout), &got)
	got["seeds"].([]any)[0].(map[string]any)["state"] = "SUCCESSFUL_EMPTY"
	mutated, _ := json.Marshal(got)
	if _, err := schema.ValidateFor(mutated, schema.FamilyFilter, "v1"); err == nil || !strings.Contains(err.Error(), "state and reference counts") {
		t.Fatalf("ASSERT_FILTER_STATE_MATCHES_REFERENCE_COUNTS: mutation accepted: %v", err)
	}
}

func TestFilterSchemaVersionAuthorityPolicyAndClaimCeiling(t *testing.T) {
	_, stdout, _ := runFilterFixture(t, "ASSERT_FILTER_CONTRACT_CONSTANTS")
	got := decodeFilter(t, stdout)
	if got.FilterSchemaVersion != "lsp-trace.filter.v1" {
		t.Fatalf("ASSERT_FILTER_SCHEMA_VERSION: %q", got.FilterSchemaVersion)
	}
	if got.ProjectionKind != "SEED_EVIDENCE_COMPARISON" {
		t.Fatalf("ASSERT_FILTER_PROJECTION_KIND: %q", got.ProjectionKind)
	}
	if got.Authority != "TOOL_DERIVED_SET_PROJECTION" {
		t.Fatalf("ASSERT_FILTER_AUTHORITY: %q", got.Authority)
	}
	if got.SupportContribution != 0 {
		t.Fatalf("ASSERT_FILTER_SUPPORT_CONTRIBUTION: %d", got.SupportContribution)
	}
	if got.NativeSemanticsPolicy != "PRESERVE_WITHOUT_AUTHORITY_UPGRADE" {
		t.Fatalf("ASSERT_FILTER_NATIVE_SEMANTICS_POLICY: %q", got.NativeSemanticsPolicy)
	}
	wantSupports := []string{"EXACT_REFERENCE_INTERSECTION", "EXACT_REFERENCE_DIFFERENCE"}
	if !reflect.DeepEqual(got.ClaimCeiling.Supports, wantSupports) {
		t.Fatalf("ASSERT_FILTER_CLAIM_CEILING_SUPPORTS: got=%v want=%v", got.ClaimCeiling.Supports, wantSupports)
	}
	wantDoesNotSupport := []string{"SHARED_FEATURE_PURPOSE", "DISTINCT_FEATURE_PURPOSE", "FEATURE_IDENTITY", "WORKFLOW_IDENTITY", "MERGE_OR_SPLIT_DISPOSITION", "INDEPENDENT_OBSERVATION", "EVIDENTIARY_SUPPORT", "CONFIDENCE", "COVERAGE", "RUNTIME_BEHAVIOR", "ACCEPTANCE"}
	if !reflect.DeepEqual(got.ClaimCeiling.DoesNotSupport, wantDoesNotSupport) {
		t.Fatalf("ASSERT_FILTER_CLAIM_CEILING_DOES_NOT_SUPPORT: got=%v want=%v", got.ClaimCeiling.DoesNotSupport, wantDoesNotSupport)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"SCHEMA_VERSION", func(v map[string]any) { v["filter_schema_version"] = "lsp-trace.filter.v2" }},
		{"AUTHORITY", func(v map[string]any) { v["authority"] = "UPGRADED" }},
		{"SUPPORT", func(v map[string]any) { v["support_contribution"] = float64(1) }},
		{"NATIVE_POLICY", func(v map[string]any) { v["native_semantics_policy"] = "REPLACE" }},
		{"CLAIM_SUPPORTS", func(v map[string]any) {
			v["claim_ceiling"].(map[string]any)["supports"] = []any{"EXACT_REFERENCE_INTERSECTION"}
		}},
		{"CLAIM_DOES_NOT_SUPPORT", func(v map[string]any) { v["claim_ceiling"].(map[string]any)["does_not_support"] = []any{"ACCEPTANCE"} }},
	}
	for _, tc := range mutations {
		t.Run("reject_mutated_"+tc.name, func(t *testing.T) {
			var candidate map[string]any
			encoded, _ := json.Marshal(raw)
			_ = json.Unmarshal(encoded, &candidate)
			tc.mutate(candidate)
			mutated, _ := json.Marshal(candidate)
			if _, err := schema.ValidateFor(mutated, schema.FamilyFilter, "v1"); err == nil {
				t.Fatalf("ASSERT_FILTER_REJECT_MUTATED_%s: accepted", tc.name)
			}
		})
	}
}

func TestFilterDynamicNoServerBehavior(t *testing.T) {
	path, _ := filterInspectionFixture(t)
	t.Setenv("PATH", t.TempDir())
	stdout, stderr, code := captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"filter_schema_version":"lsp-trace.filter.v1"`) {
		t.Fatalf("ASSERT_FILTER_DYNAMIC_NO_SERVER: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestFilterDeterministicReadOnlyAuthority(t *testing.T) {
	path, stdout1, before := runFilterFixture(t, "ASSERT_FILTER_DETERMINISTIC_READ_ONLY_AUTHORITY")
	stdout2, stderr2, code2 := captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code2 != 0 || stderr2 != "" || stdout1 != stdout2 {
		t.Fatalf("ASSERT_FILTER_DETERMINISTIC_READ_ONLY_AUTHORITY: code=%d stderr=%q equal=%v", code2, stderr2, stdout1 == stdout2)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("ASSERT_FILTER_DETERMINISTIC_READ_ONLY_AUTHORITY: input changed err=%v", err)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(stdout1), &got)
	if got["authority"] != "TOOL_DERIVED_SET_PROJECTION" || got["support_contribution"] != float64(0) {
		t.Fatalf("ASSERT_FILTER_DETERMINISTIC_READ_ONLY_AUTHORITY: authority=%v support=%v", got["authority"], got["support_contribution"])
	}
}
