package main

import (
	"context"
	inventory "lsp-trace/internal/applicationinventory"
	"testing"
	"time"
)

func TestApplicationInventoryV3DefaultOff(t *testing.T) {
	calls := 0
	q := inventory.Request{Manifest: inventory.Manifest{Scope: "controlled fixture", Provenance: "supplied lead", Leads: []inventory.Lead{{ID: "one"}}}, Interpreter: "actor", MaxNativeAttempts: 1, NativeRetainedUpper: 4096, HostWindow: time.Second, Preparer: func(context.Context, inventory.Lead) (inventory.NativeResult, error) {
		calls++
		return inventory.NativeResult{}, nil
	}}
	if _, e := runApplicationInventoryV3(context.Background(), false, q); e == nil || calls != 0 {
		t.Fatal("ASSERT_V3_DEFAULT_OFF_NO_EFFECT")
	}
	t.Log("ASSERT_V3_DEFAULT_OFF_NO_EFFECT: PASS")
}
