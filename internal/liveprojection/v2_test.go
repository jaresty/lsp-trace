package liveprojection

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/sourceprojection"
)

const (
	assertV2Schema     = "ASSERT_V2_ASSEMBLY_SCHEMA"
	assertV2Documents  = "ASSERT_V2_DOCUMENT_BINDINGS"
	assertV2Accounting = "ASSERT_V2_ACCOUNTING_SEPARATE"
	assertV2Ranges     = "ASSERT_V2_RANGE_ROLES"
	assertV2Neutral    = "ASSERT_V2_GRAPH_NEUTRAL"
	assertV2Private    = "ASSERT_V2_RAW_SUPPLIES_PRIVATE"
	assertV2Response   = "ASSERT_V2_RESPONSE_LIMIT"
)

func v2Fixture(t *testing.T) (CompositionResult, PreparationResult, string, []string) {
	t.Helper()
	prepared, candidates, policy := compositionFixture()
	normalizeV2CandidateIDs(candidates)
	composed := Compose(prepared, "session", 4, candidates, policy)
	if composed.Status != CompositionComplete {
		t.Fatal(composed.Failure)
	}
	prepared.Accounting.Documents.Observed = 2
	prepared.Accounting.Bytes.Observed = 18
	prepared.Accounting.Attempted = 2
	prepared.Accounting.Succeeded = 2
	return composed, prepared, "file:///workspace/target.go", []string{"file:///workspace/target.go", "file:///workspace/caller.go"}
}

func normalizeV2CandidateIDs(candidates []sourceprojection.Candidate) {
	candidates[0].UnitID = "sha256:" + strings.Repeat("a", 64)
	candidates[0].CitationID = "sha256:" + strings.Repeat("b", 64)
	candidates[1].UnitID = "sha256:" + strings.Repeat("c", 64)
	candidates[1].CitationID = "sha256:" + strings.Repeat("d", 64)
	candidates[1].OccurrenceID = "sha256:" + strings.Repeat("e", 64)
}

func TestAssembleV2SchemaDocumentsAccountingAndRanges(t *testing.T) {
	t.Log(assertV2Schema, assertV2Documents, assertV2Accounting, assertV2Ranges, assertV2Neutral)
	composed, prepared, target, selected := v2Fixture(t)
	policyID := "sha256:" + strings.Repeat("f", 64)
	got, err := AssembleV2Bounded(composed, prepared, target, selected, policyID, 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertV2Schema, err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := mcpcontract.ValidateJSON(mcpcontract.SourceProjectionResultV2ID, raw); err != nil {
		t.Fatalf("%s: %v\n%s", assertV2Schema, err, raw)
	}
	if len(got.DocumentBindings) != 2 || got.DocumentBindings[0].URI != target || got.DocumentBindings[0].Role != "TARGET" || got.DocumentBindings[0].Ordinal != 0 || got.DocumentBindings[1].Role != "ADDITIONAL" || got.DocumentBindings[1].Ordinal != 1 {
		t.Fatalf("%s: %+v", assertV2Documents, got.DocumentBindings)
	}
	if got.DocumentAccounting.Candidates != 2 || got.DocumentAccounting.Selected != 2 || got.DocumentAccounting.Acquired != 2 || got.DocumentAccounting.TotalAcquiredBytes != 18 || got.Accounting != composed.Projection.Accounting {
		t.Fatalf("%s: documents=%+v projection=%+v", assertV2Accounting, got.DocumentAccounting, got.Accounting)
	}
	if len(got.Units) != 2 || got.Units[0].EvidenceRange != composed.Candidates[0].EvidenceRange || got.Units[0].ItemRange == nil || *got.Units[0].ItemRange != composed.Candidates[0].ItemRange || got.Units[0].SelectionRange == nil || *got.Units[0].SelectionRange != composed.Candidates[0].SelectionRange || len(got.Citations) != 2 {
		t.Fatalf("%s: units=%+v citations=%+v", assertV2Ranges, got.Units, got.Citations)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 || got.CustodyBinding.SessionID != "session" || got.CustodyBinding.Generation != 4 {
		t.Fatalf("%s: %+v", assertV2Neutral, got)
	}
}

func TestAssembleV2PrivacyAndResponseLimit(t *testing.T) {
	t.Log(assertV2Private, assertV2Response)
	composed, prepared, target, selected := v2Fixture(t)
	policyID := "sha256:" + strings.Repeat("f", 64)
	got, err := AssembleV2Bounded(composed, prepared, target, selected, policyID, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	for _, forbidden := range []string{"LSP_SUPPLIED", "target()", "caller()", "resolutions", "supplies"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("%s: found=%q", assertV2Private, forbidden)
		}
	}
	limited, err := AssembleV2Bounded(composed, prepared, target, selected, policyID, 1)
	if err == nil || limited.SchemaVersion != "" {
		t.Fatalf("%s: result=%+v err=%v", assertV2Response, limited, err)
	}
}
