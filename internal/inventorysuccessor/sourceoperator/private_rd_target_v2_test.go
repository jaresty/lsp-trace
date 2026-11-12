//go:build darwin || linux

package sourceoperator

import (
	"context"
	"testing"
	"time"

	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	pa "lsp-trace/internal/programcadmission"
)

// DATA ONLY validation and ZERO-AUTHORITY branch probes. No owner bootstrap,
// provider operation, reservation, native positive admission or qualification.
func TestSelectedV2RDTargetInput(t *testing.T) {
	for _, mode := range []string{"", "RD"} {
		for _, kinds := range [][]pa.RelationKind{{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}, {pa.TypedResolvesToDefinition, pa.TypedReferencesSymbol}} {
			if !privateRDTargetInputV2(inv.PrivateMixedInputRequestV2{Kinds: kinds}, inv.PrivateSelectedConfigurationV2{Mode: mode}) {
				t.Fatal("RD_INPUT_ACCEPTS_ONLY_FIXED_MASK", mode, kinds)
			}
		}
	}
	for _, kinds := range [][]pa.RelationKind{nil, {pa.TypedReferencesSymbol}, {pa.TypedResolvesToDefinition}, {pa.TypedCalls, pa.TypedReferencesSymbol}, {pa.TypedCalls, pa.TypedResolvesToDefinition}, {pa.TypedReferencesSymbol, pa.TypedReferencesSymbol}, {pa.TypedResolvesToDefinition, pa.TypedResolvesToDefinition}, {pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition, pa.TypedCalls}, {pa.RelationKind("unknown"), pa.TypedResolvesToDefinition}} {
		if privateRDTargetInputV2(inv.PrivateMixedInputRequestV2{Kinds: kinds}, inv.PrivateSelectedConfigurationV2{}) {
			t.Fatal("RD_INPUT_REJECTS_MASK", kinds)
		}
	}
	valid := inv.PrivateMixedInputRequestV2{Kinds: []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}}
	for _, mode := range []string{"R", "D", "C", "CR", "CD", "CRD", "rd", "RD "} {
		if privateRDTargetInputV2(valid, inv.PrivateSelectedConfigurationV2{Mode: mode}) {
			t.Fatal("RD_INPUT_REJECTS_MODE", mode)
		}
	}
	valid.Timing = &inv.PrivateSourceTimingFrameV2{}
	if privateRDTargetInputV2(valid, inv.PrivateSelectedConfigurationV2{}) {
		t.Fatal("RD_INPUT_REJECTS_TIMING")
	}
	t.Log("RD_INPUT_FIXED_ADMISSION: PASS; DATA_ONLY")
}

func TestSelectedV2RDTargetBeforeEffects(t *testing.T) {
	capture := inv.PrivateMixedInputRequestV2{Kinds: []pa.RelationKind{pa.TypedReferencesSymbol, pa.TypedResolvesToDefinition}}
	capture.Start.Deadline = time.Now().Add(time.Second)
	owner := &inv.PrivateManagedInputOwnerV2{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, cx := range []context.Context{nil, ctx, context.Background()} {
		got, err := PrepareRDTargetV2(cx, owner, capture, inv.PrivateSelectedConfigurationV2{})
		if err == nil || got.Available() {
			t.Fatal("RD_ZERO_AUTHORITY_REFUSES", err)
		}
	}
	// A malformed exact selector must never be examined/copied or used when
	// both policies are requested. No valid operation is constructed here.
	q := privateSelectedRequestV2{Owner: owner, Capture: capture, DTarget: true, Endpoint: &sc.PrivateSelectedEndpointV2{RowPosition: 1001}}
	got, err := privatePrepareSelectedKindsV2(context.Background(), q)
	if err == nil || got.Available() {
		t.Fatal("RD_BOTH_POLICIES_REFUSE")
	}
	for _, mode := range []string{"C", "R", "D", "CRD"} {
		got, err := PrepareRDTargetV2(context.Background(), owner, capture, inv.PrivateSelectedConfigurationV2{Mode: mode})
		if err == nil || got.Available() {
			t.Fatal("RD_WRONG_MODE_REFUSES", mode)
		}
	}
	t.Log("RD_ZERO_AUTHORITY_REFUSALS: PASS; NO_NATIVE_POSITIVE")
}

func TestSelectedV2RDTargetScalarSnapshot(t *testing.T) {
	// The request and policy are value parameters, not a borrowed *bool. This
	// language-level snapshot test is not an inflight/captured-operation test.
	original := privateSelectedRequestV2{DTarget: true}
	copied := original
	original.DTarget = false
	if !copied.DTarget {
		t.Fatal("RD_SCALAR_SNAPSHOT_STABLE")
	}
	copied.DTarget = false
	if original.DTarget {
		t.Fatal("RD_SCALAR_SNAPSHOT_NOT_ALIASED")
	}
	t.Log("RD_SCALAR_SNAPSHOT_STABLE: PASS; DATA_ONLY_NOT_INFLIGHT")
}
