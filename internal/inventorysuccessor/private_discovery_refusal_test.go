package inventorysuccessor

import (
	"errors"
	"lsp-trace/sessionruntime"
	"strings"
	"testing"
)

func TestRefusal53OperationForwarding(t *testing.T) {
	d := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoverySource, Currency: sessionruntime.PrivateDiscoveryRetainedBytes, Requested: 16777232, Remaining: 8302352, AmountsAvailable: true, CleanupUncertain: true}
	e := privateDiscoveryOperationFailureV2(d, true)
	got, ok := PrivateDiscoveryOperationRefusalV2(e)
	if !ok || got != d || !errors.Is(e, ErrPrivateDiscoveryOperationV2) || !errors.Is(e, sessionruntime.ErrPrivateDiscovery) {
		t.Fatal("ASSERT_OPERATION_CLOSED_FORWARDING")
	}
	if strings.Contains(e.Error(), "8302352") {
		t.Fatal("ASSERT_STATIC_ERROR")
	}
	got.Requested = 0
	again, _ := PrivateDiscoveryOperationRefusalV2(e)
	if again != d {
		t.Fatal("ASSERT_DIAGNOSTIC_COPY")
	}
	for _, bad := range []sessionruntime.PrivateDiscoveryRefusal{{}, {Stage: 255}} {
		if privateDiscoveryOperationFailureV2(bad, true) != ErrPrivateDiscoveryOperationV2 {
			t.Fatal("ASSERT_OPERATION_REJECTS_INVALID")
		}
	}
	if privateDiscoveryOperationFailureV2(d, false) != ErrPrivateDiscoveryOperationV2 {
		t.Fatal("ASSERT_OPERATION_UNKNOWN_UNCHANGED")
	}
}
