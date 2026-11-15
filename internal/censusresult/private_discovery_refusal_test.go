package censusresult

import (
	"bytes"
	"lsp-trace/sessionruntime"
	"testing"
)

func TestRefusal53DiagnosticDTO(t *testing.T) {
	ordinary, _ := NewDiagnostic(StageDiscovery, nil)
	raw, e := MarshalDiagnostic(ordinary)
	if e != nil || !ordinary.Retry || bytes.Contains(raw, []byte("private_discovery_refusal")) {
		t.Fatal("ASSERT_ORDINARY_UNCHANGED")
	}
	d := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoverySource, Currency: sessionruntime.PrivateDiscoveryRetainedBytes, Requested: 16777232, Remaining: 8302352, AmountsAvailable: true, CleanupUncertain: true}
	out := ordinary
	out.Retry = false
	out.PrivateDiscoveryRefusal = &d
	raw, e = MarshalDiagnostic(out)
	if e != nil {
		t.Fatal("ASSERT_DETAILED_MARSHAL", e)
	}
	got, e := DecodeDiagnostic(raw)
	if e != nil || got.Retry || *got.PrivateDiscoveryRefusal != d {
		t.Fatal("ASSERT_CLOSED_ROUNDTRIP", e)
	}
	for _, change := range []func(*Diagnostic){func(x *Diagnostic) { x.Retry = true }, func(x *Diagnostic) { x.Stage = StageConfig }, func(x *Diagnostic) { x.Detail = "secret source URI" }, func(x *Diagnostic) { v := *x.PrivateDiscoveryRefusal; v.Stage = 255; x.PrivateDiscoveryRefusal = &v }, func(x *Diagnostic) {
		v := *x.PrivateDiscoveryRefusal
		v.AmountsAvailable = false
		x.PrivateDiscoveryRefusal = &v
	}} {
		bad := out
		change(&bad)
		if _, e := MarshalDiagnostic(bad); e == nil {
			t.Fatal("ASSERT_INVALID_DETAIL_REJECTED")
		}
	}
	unknown := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoveryAllSupplied}
	out.PrivateDiscoveryRefusal = &unknown
	if _, e := MarshalDiagnostic(out); e != nil {
		t.Fatal("ASSERT_UNKNOWN_NOT_ZERO_BALANCE", e)
	}
}
