package adr0011methodresult

import (
	"errors"
	"strings"
	"testing"
)

func TestADR0011CanonicalRecordBound(t *testing.T) {
	// Quoted ASCII contributes two JSON bytes and the canonical LF contributes one.
	if raw, err := chainBytes(strings.Repeat("x", chainMaxBytes-3)); err != nil || len(raw) != chainMaxBytes {
		t.Fatalf("ASSERT_ADR0011_CANONICAL_AT_BOUND: len=%d err=%v", len(raw), err)
	}
	if raw, err := chainBytes(strings.Repeat("x", chainMaxBytes-2)); !errors.Is(err, ErrChain) || raw != nil {
		t.Fatalf("ASSERT_ADR0011_CANONICAL_ONE_OVER_ZERO: len=%d err=%v", len(raw), err)
	}
	if raw, err := chainBytes(make(chan int)); !errors.Is(err, ErrChain) || raw != nil {
		t.Fatalf("ASSERT_ADR0011_CANONICAL_MARSHAL_ZERO: len=%d err=%v", len(raw), err)
	}
}
