package sourcecontinuation

import (
	"context"
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	"testing"
	"time"
)

func TestSelectedConsumerV3ZeroDTOAndCancellation(t *testing.T) {
	ctx := context.Background()
	b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 1<<20)
	q := CallerDescribeBindingV3{AttemptID: "test", Interpreter: "test", Deadline: time.Now().Add(time.Second), Limits: dw.SuppliedResponseLimits{MaxAdapterBytes: 1024, MaxSemanticBytes: 1024}, Budget: b}
	var copied PrivateSelectedResultV2
	if e := json.Unmarshal([]byte(`{"unit":{"ID":"forged"},"state":{}}`), &copied); e != nil {
		t.Fatal(e)
	}
	for _, r := range []PrivateSelectedResultV2{{}, copied} {
		if _, e := PrepareSelectedDescribeV3(ctx, r, q); e == nil {
			t.Fatal("ASSERT_V3_DTO_ZERO_BEFORE_HOST")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := PrepareSelectedDescribeV3(canceled, copied, q); e == nil {
		t.Fatal("ASSERT_V3_CANCEL_BEFORE_HOST")
	}
	var bound BoundSelectedDescriptionV3
	if _, e := bound.Evidence(ctx); e == nil {
		t.Fatal("ASSERT_V3_ZERO_BOUND_NO_EVIDENCE")
	}
	t.Log("ASSERT_V3_DTO_ZERO_BEFORE_HOST: PASS; ASSERT_V3_CANCEL_BEFORE_HOST: PASS; ASSERT_V3_ZERO_BOUND_NO_EVIDENCE: PASS")
}
