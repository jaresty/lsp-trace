package provisionalfeaturecatalog_test

import (
	"context"
	"encoding/json"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"testing"
	"time"
)

func TestNativeCatalogSameThreadReviewSequenceV3(t *testing.T) {
	ctx := context.Background()
	budget, _ := cat.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	base, err := cat.BuildNativeV3(ctx, budget, "caller", nil, cat.ReviewModeSameThreadV3)
	if err != nil {
		t.Fatal(err)
	}
	decision := func(predecessor string) []byte {
		raw, _ := json.Marshal(cat.NativeDecisionV3{Predecessor: predecessor, Actor: "caller", Reason: "controlled sequential pass, not authenticated identity", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
		return raw
	}
	if _, err = cat.ApplyNativeDecisionV3(ctx, base, decision(base.Revision()), true); err == nil {
		t.Fatal("ASSERT_REVIEW_SEPARATE_CATALOG_PASS", "base accepted as reviewed")
	}
	proposed, err := cat.ApplyNativeDecisionV3(ctx, base, decision(base.Revision()), false)
	if err != nil {
		t.Fatal("ASSERT_REVIEW_SEPARATE_CATALOG_PASS", err)
	}
	corrected, err := cat.ApplyNativeDecisionV3(ctx, proposed, decision(proposed.Revision()), true)
	if err != nil {
		t.Fatal("ASSERT_REVIEW_SEPARATE_CATALOG_PASS", err)
	}
	if _, err = cat.ApplyNativeDecisionV3(ctx, proposed, decision(proposed.Revision()), true); err == nil {
		t.Fatal("ASSERT_REVIEW_SEPARATE_CATALOG_PASS", "proposal correction replayed")
	}
	if _, err = cat.ApplyNativeDecisionV3(ctx, corrected, decision(corrected.Revision()), true); err == nil {
		t.Fatal("ASSERT_REVIEW_SEPARATE_CATALOG_PASS", "second correction accepted")
	}
	t.Log("ASSERT_REVIEW_SEPARATE_CATALOG_PASS: PASS")
}
