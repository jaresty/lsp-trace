package adr0011acquisition

import (
	"context"
	"errors"
	"testing"
)

func TestADR0011DisabledOwnerCannotAcquire(t *testing.T) {
	owner := NewDisabled(nil)
	receipt, err := owner.Acquire(context.Background(), Query{SessionID: "synthetic", Generation: 1, URI: "file:///a.go", OccurrenceID: "q", Revision: "r"})
	if !errors.Is(err, ErrDisabled) || receipt != nil {
		t.Fatalf("ASSERT_ADR0011_OWNER_DISABLED_ZERO: receipt=%+v err=%v", receipt, err)
	}
}
