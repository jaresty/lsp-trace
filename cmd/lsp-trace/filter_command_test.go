package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func runFilterFixture(t *testing.T, assertion string) (string, string, []byte) {
	t.Helper()
	path, before := filterInspectionFixture(t)
	stdout, stderr, code := captureRun(t, []string{"filter", path, "--compare-seeds", "chosen", "--compare-seeds", "other", "--json"})
	if code != 0 || stderr != "" {
		t.Fatalf("%s: filter fixture code=%d stderr=%q", assertion, code, stderr)
	}
	return path, stdout, before
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
