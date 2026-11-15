package main

import (
	"bytes"
	"lsp-trace/internal/censusresult"
	"lsp-trace/sessionruntime"
	"testing"
)

func TestRefusal53BridgeMapping(t *testing.T) {
	d := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoverySource, Currency: sessionruntime.PrivateDiscoveryRetainedBytes, Requested: 16777232, Remaining: 8302352, AmountsAvailable: true, CleanupUncertain: true}
	out, e := censusInventoryDiscoveryDiagnostic(d)
	if e != nil {
		t.Fatal("ASSERT_BRIDGE_MAPPING", e)
	}
	raw, e := marshalCensusCLIDiagnostic(out)
	if e != nil {
		t.Fatal(e)
	}
	got, e := censusresult.DecodeDiagnostic(raw)
	if e != nil || got.Stage != censusresult.StageDiscovery || got.Code != censusresult.CodeDiscoveryFailed || got.Retry || got.PrivateDiscoveryRefusal == nil || *got.PrivateDiscoveryRefusal != d {
		t.Fatal("ASSERT_BRIDGE_NO_RETRY", e)
	}
	for _, secret := range []string{"URI", "path", "source.go", "opaque", "error_text"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("ASSERT_NO_RAW_LEAKAGE")
		}
	}
	if _, e := censusInventoryDiscoveryDiagnostic(sessionruntime.PrivateDiscoveryRefusal{Stage: 255}); e == nil {
		t.Fatal("ASSERT_BRIDGE_VALIDATES")
	}
}
