package adr0011methodresult

import (
	"errors"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestADR0011UnissuedPublicationFailsClosed(t *testing.T) {
	// Even caller-supplied receipts cannot bypass the absent final readback gate.
	receipt := &TargetReceipt{Selector: "caller-supplied", Digest: "caller-supplied"}
	if got, err := PublishReferences(nil, receipt, sessionruntime.OwnedMethodPair{}, sessionruntime.OwnedMethodPair{}, ChainExpectation{}); got != nil || !errors.Is(err, ErrChain) {
		t.Fatalf("ASSERT_ADR0011_UNISSUED_OCCURRENCES: receipt=%v err=%v", got, err)
	}
	if got, err := PublishReferenceDiagnostic(nil, receipt, sessionruntime.OwnedMethodPair{}, sessionruntime.OwnedMethodPair{}, ChainExpectation{}); got != nil || !errors.Is(err, ErrChain) {
		t.Fatalf("ASSERT_ADR0011_UNREDACTED_DIAGNOSTIC_BLOCKED: receipt=%v err=%v", got, err)
	}
}
