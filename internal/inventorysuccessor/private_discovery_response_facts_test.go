package inventorysuccessor

import (
	"encoding/json"
	"errors"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
	"testing"
	"unsafe"
)

// Only the pure Execute handoff helper is exercised: no kernel, initialization,
// reservation, grants, root, Starter, Manager, source opens or native process.
func TestResponse76OperationHandoff(t *testing.T) {
	f := lspwire.ResponseFacts{State: lspwire.ResponseValidJSON, Envelope: lspwire.ResponseObject, Error: lspwire.ResponseObject, ID: lspwire.ResponseNumber, Association: lspwire.ResponseIDMatched, CodeState: lspwire.ResponseCodeValid}
	d := sessionruntime.PrivateDiscoveryResponseFacts{Available: true, Count: 1}
	d.Records[0] = sessionruntime.PrivateDiscoveryResponseRecord{Ordinal: 1, ReadAttempts: 1, LastDisposition: sessionruntime.PrivateResponseSelected, LastFrame: f, Selected: f}
	refusal := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoveryRead, Currency: sessionruntime.PrivateDiscoveryResponseBytes, Requested: 1, AmountsAvailable: true, CleanupUncertain: true}
	for _, known := range []bool{false, true} {
		slot := new(privateDiscoveryOperationResponseFailureV2)
		e := slot.capture(refusal, known, d)
		got, ok := PrivateDiscoveryOperationResponseFactsV2(e)
		if !ok || got != d {
			t.Fatal("P2 operation handoff lost facts")
		}
		if !errors.Is(e, ErrPrivateDiscoveryOperationV2) || errors.Is(e, sessionruntime.ErrPrivateDiscovery) != known {
			t.Fatal("P3 error identity changed")
		}
		rd, rk := PrivateDiscoveryOperationRefusalV2(e)
		if rk != known || known && rd != refusal {
			t.Fatal("P3 refusal lost")
		}
		dcopy := d
		dcopy.Records[0].Selected.Code = 5
		got.Records[0].Selected.Code = 9
		again, _ := PrivateDiscoveryOperationResponseFactsV2(e)
		if again.Records[0].Selected.Code != 0 {
			t.Fatal("P2 borrowed error handoff")
		}
		if _, err := json.Marshal(again); err != nil {
			t.Fatal("P5 serialized facts", err)
		}
		op := PrivateDiscoveryOperationV2{state: &privateDiscoveryOperationV2{executed: true, observation: sessionruntime.PrivateDiscoveryObservation{ResponseFacts: d}}}
		snap := op.ResponseFacts()
		snap.Records[0].Selected.Code = 4
		if op.ResponseFacts() != d {
			t.Fatal("P2 operation snapshot aliases")
		}
	}
	if unsafe.Sizeof(privateDiscoveryOperationResponseFailureV2{}) >= 32<<20 {
		t.Fatal("P4 fixed operation metadata allowance exceeded")
	}
	if _, ok := PrivateDiscoveryOperationResponseFactsV2(ErrPrivateDiscoveryOperationV2); ok {
		t.Fatal("P2 invented observation")
	}
}
