package provisionalfeaturecatalog_test

import (
	"context"
	"encoding/json"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"testing"
	"time"
)

type fakeNativeV3 struct{ called *bool }

func (f fakeNativeV3) Evidence(context.Context) (cat.SelectedEvidenceV3, error) {
	*f.called = true
	return cat.SelectedEvidenceV3{ID: "forged"}, nil
}
func TestNativeCatalogV3RefusesAdaptersAndZero(t *testing.T) {
	ctx := context.Background()
	b, _ := cat.NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
	called := false
	if _, e := cat.BuildNativeV3(ctx, b, "actor", []cat.NativeBoundDescriptionV3{fakeNativeV3{&called}}); e == nil || called {
		t.Fatal("ASSERT_V3_FAKE_ADAPTER_BEFORE_DISPATCH")
	}
	if _, e := cat.BuildNativeV3(ctx, b, "actor", []cat.NativeBoundDescriptionV3{sc.BoundSelectedDescriptionV3{}}); e == nil {
		t.Fatal("ASSERT_V3_ZERO_NATIVE_REFUSED")
	}
	if _, e := cat.BuildNativeV3(ctx, nil, "actor", nil); e == nil {
		t.Fatal("ASSERT_V3_NIL_BUDGET_REFUSED")
	}
	base, e := cat.BuildNativeV3(ctx, b, "actor", nil)
	if e != nil {
		t.Fatal(e)
	}
	encode := func(d cat.NativeDecisionV3) []byte { raw, _ := json.Marshal(d); return raw }
	decision := cat.NativeDecisionV3{Predecessor: base.Revision(), Actor: "actor", Reason: "no eligible source in fixture", Affected: []string{}, Groups: []cat.NativeGroupV3{}}
	foreign := decision
	foreign.Predecessor = "foreign"
	if _, e = cat.ApplyNativeDecisionV3(ctx, base, encode(foreign), false); e == nil {
		t.Fatal("ASSERT_V3_FOREIGN_PREDECESSOR_REFUSED")
	}
	unknown := decision
	unknown.Groups = []cat.NativeGroupV3{{ID: "x", Label: "x", Reason: "x", Members: []string{"forged"}}}
	if _, e = cat.ApplyNativeDecisionV3(ctx, base, encode(unknown), false); e == nil {
		t.Fatal("ASSERT_V3_UNKNOWN_MEMBER_REFUSED")
	}
	proposed, e := cat.ApplyNativeDecisionV3(ctx, base, encode(decision), false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = cat.ApplyNativeDecisionV3(ctx, base, encode(decision), false); e == nil {
		t.Fatal("ASSERT_V3_DUPLICATE_PROPOSAL_REFUSED")
	}
	review := decision
	review.Predecessor = proposed.Revision()
	if _, e = cat.ApplyNativeDecisionV3(ctx, proposed, encode(review), true); e == nil {
		t.Fatal("ASSERT_V3_SAME_REVIEWER_REFUSED")
	}
	review.Actor = "reviewer"
	corrected, e := cat.ApplyNativeDecisionV3(ctx, proposed, encode(review), true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = cat.ApplyNativeDecisionV3(ctx, proposed, encode(review), true); e == nil {
		t.Fatal("ASSERT_V3_DUPLICATE_CORRECTION_REFUSED")
	}
	raw, e := corrected.Bytes(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var w cat.NativeCatalogWireV3
	if cat.DecodeNativeV3(raw, &w) != nil || w.Status != "PROVISIONAL" || w.Authority != 0 || w.Accepted || w.Completeness != "UNKNOWN" || w.Absence != "PROHIBITED" {
		t.Fatal("ASSERT_V3_EMPTY_CATALOG_CEILING")
	}
	t.Log("ASSERT_V3_FAKE_ADAPTER_BEFORE_DISPATCH: PASS; ASSERT_V3_ZERO_NATIVE_REFUSED: PASS; ASSERT_V3_NIL_BUDGET_REFUSED: PASS; ASSERT_V3_FOREIGN_PREDECESSOR_REFUSED: PASS; ASSERT_V3_UNKNOWN_MEMBER_REFUSED: PASS; ASSERT_V3_DUPLICATE_PROPOSAL_REFUSED: PASS; ASSERT_V3_SAME_REVIEWER_REFUSED: PASS; ASSERT_V3_DUPLICATE_CORRECTION_REFUSED: PASS; ASSERT_V3_EMPTY_CATALOG_CEILING: PASS")
}
func TestNativeCatalogV3StrictDecisionAndBudget(t *testing.T) {
	for _, raw := range []string{`null`, `{"actor":"a","actor":"b"}`, `{"unknown":true}`, `{} {}`} {
		var d cat.NativeDecisionV3
		if cat.DecodeNativeV3([]byte(raw), &d) == nil {
			t.Fatal("ASSERT_V3_STRICT_DECISION", raw)
		}
	}
	b, _ := cat.NewCallerBudgetV3(time.Now().Add(time.Minute), 100)
	if b.Reserve(context.Background(), 100) != nil {
		t.Fatal("ASSERT_V3_BUDGET_EQUALITY")
	}
	if b.Reserve(context.Background(), 1) == nil {
		t.Fatal("ASSERT_V3_BUDGET_OVERRUN")
	}
	if !b.Exhausted() {
		t.Fatal("ASSERT_V3_EXHAUSTION_STICKY")
	}
	s, _ := cat.NewCallerBudgetV3(time.Now().Add(time.Minute), 1024)
	for i := 0; i < 4; i++ {
		_, cancel, e := s.Exchange(context.Background(), 30*time.Second)
		if e != nil {
			t.Fatal(e)
		}
		cancel()
	}
	if _, _, e := s.Exchange(context.Background(), time.Nanosecond); e == nil {
		t.Fatal("ASSERT_V3_SEMANTIC_AGGREGATE")
	}
	t.Log("ASSERT_V3_STRICT_DECISION: PASS; ASSERT_V3_BUDGET_EQUALITY: PASS; ASSERT_V3_BUDGET_OVERRUN: PASS; ASSERT_V3_EXHAUSTION_STICKY: PASS; ASSERT_V3_SEMANTIC_AGGREGATE: PASS")
}
