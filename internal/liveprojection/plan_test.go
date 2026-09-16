package liveprojection

import (
	"slices"
	"testing"

	"lsp-trace/internal/sourceprojection"
)

const (
	assertPrivacyBeforeAcquisition = "ASSERT_PLAN_PRIVACY_BEFORE_ACQUISITION"
	assertUniqueDocuments          = "ASSERT_PLAN_UNIQUE_DOCUMENTS"
	assertTargetFirstLexical       = "ASSERT_PLAN_TARGET_FIRST_THEN_LEXICAL"
)

func TestPlanDocumentsPrivacyDeduplicationAndCanonicalOrder(t *testing.T) {
	t.Log(assertPrivacyBeforeAcquisition, assertUniqueDocuments, assertTargetFirstLexical)
	const target = "file:///workspace/target.go"
	candidates := []sourceprojection.Candidate{
		{LogicalSourceID: "file:///workspace/z.go", PrivacyClassification: "PUBLIC"},
		{LogicalSourceID: target, PrivacyClassification: "PUBLIC"},
		{LogicalSourceID: "file:///workspace/private.go", PrivacyClassification: "PRIVATE"},
		{LogicalSourceID: "file:///workspace/a.go", PrivacyClassification: "PUBLIC"},
		{LogicalSourceID: "file:///workspace/z.go", PrivacyClassification: "PUBLIC"},
		{LogicalSourceID: "", PrivacyClassification: "PUBLIC"},
	}

	got := PlanDocuments(candidates, target)
	want := []string{target, "file:///workspace/a.go", "file:///workspace/z.go"}
	if slices.Contains(got, "file:///workspace/private.go") || slices.Contains(got, "") {
		t.Fatalf("%s: got=%q", assertPrivacyBeforeAcquisition, got)
	}
	seen := make(map[string]struct{}, len(got))
	for _, uri := range got {
		if _, duplicate := seen[uri]; duplicate {
			t.Fatalf("%s: got=%q", assertUniqueDocuments, got)
		}
		seen[uri] = struct{}{}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s: got=%q want=%q", assertTargetFirstLexical, got, want)
	}
}

func TestPlanDocumentsOrdersLexicallyWhenTargetIsIneligible(t *testing.T) {
	t.Log(assertTargetFirstLexical)
	const target = "file:///workspace/target.go"
	got := PlanDocuments([]sourceprojection.Candidate{
		{LogicalSourceID: "file:///workspace/z.go", PrivacyClassification: "PUBLIC"},
		{LogicalSourceID: target, PrivacyClassification: "PRIVATE"},
		{LogicalSourceID: "file:///workspace/a.go", PrivacyClassification: "PUBLIC"},
	}, target)
	want := []string{"file:///workspace/a.go", "file:///workspace/z.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("%s: ineligible target got=%q want=%q", assertTargetFirstLexical, got, want)
	}
}
