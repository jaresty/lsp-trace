package acquisitionorchestration

import (
	"strings"
	"testing"

	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/operation"
)

func TestBoundedTraversalReceiptFailsClosed(t *testing.T) {
	for name, value := range map[string]any{"missing": nil, "wrong-type": true} {
		t.Run(name, func(t *testing.T) {
			if _, err := boundedTraversalReceipt(operation.Result{Value: value}); err == nil || !strings.Contains(err.Error(), "bounded traversal status unavailable") {
				t.Fatalf("ASSERT_MISSING_BOUNDED_TRAVERSAL_RECEIPT_REJECTED: err=%v", err)
			}
			t.Log("ASSERT_MISSING_BOUNDED_TRAVERSAL_RECEIPT_REJECTED: PASS")
		})
	}
}

func TestBoundedTraversalReceiptPreservesStatus(t *testing.T) {
	for _, want := range []bool{false, true} {
		got, err := boundedTraversalReceipt(operation.Result{Value: acquisitionengine.BoundedTraversalStatus{Complete: want}})
		if err != nil || got != want {
			t.Fatalf("ASSERT_BOUNDED_TRAVERSAL_RECEIPT_STATUS_PRESERVED: got=%t want=%t err=%v", got, want, err)
		}
	}
	t.Log("ASSERT_BOUNDED_TRAVERSAL_RECEIPT_STATUS_PRESERVED: PASS")
}
