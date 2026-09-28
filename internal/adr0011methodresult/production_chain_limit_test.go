package adr0011methodresult

import (
	"errors"
	"strings"
	"testing"
)

func TestADR0011CanonicalRecordBound(t *testing.T) {
	if raw, err := chainBytes(strings.Repeat("x", chainMaxBytes)); !errors.Is(err, ErrChain) || raw != nil {
		t.Fatalf("ASSERT_ADR0011_CANONICAL_OVERBOUND_ZERO: len=%d err=%v", len(raw), err)
	}
	if raw, err := chainBytes(make(chan int)); !errors.Is(err, ErrChain) || raw != nil {
		t.Fatalf("ASSERT_ADR0011_CANONICAL_MARSHAL_ZERO: len=%d err=%v", len(raw), err)
	}
}
