package adr0011methodresult_test

import (
	"context"
	"testing"

	methodresult "lsp-trace/internal/adr0011methodresult"
)

func TestPrivateManagedGroupingExternalEntry(t *testing.T) {
	manager, request := methodresult.PrivateManagedGroupingFixtureForExternalTest(t)
	result, err := methodresult.AcquirePrivateManagedGroupingInput(context.Background(), manager, request)
	if err != nil || len(result.Input.Contributions()) != 4 || len(result.Predecessors) != 4 {
		t.Fatalf("ASSERT_PRIVATE_EXTERNAL_OWNED_ENTRY: %v", err)
	}
	if err := result.Replay(request.Root); err != nil {
		t.Fatalf("ASSERT_PRIVATE_EXTERNAL_NATIVE_REPLAY: %v", err)
	}
	// Copying even valid public snapshots loses the private issued originals.
	forged := methodresult.PrivateManagedGroupingInput{Input: result.Input, TypedInput: result.TypedInput, Predecessors: result.Predecessors, Ledger: result.Ledger, Dispositions: result.Dispositions}
	if forged.Replay(request.Root) == nil {
		t.Fatal("ASSERT_PRIVATE_EXTERNAL_CUSTODY_LOSS_REFUSED")
	}
	t.Log("ASSERT_PRIVATE_EXTERNAL_OWNED_ENTRY: PASS")
	t.Log("ASSERT_PRIVATE_EXTERNAL_CUSTODY_LOSS_REFUSED: PASS")
}
