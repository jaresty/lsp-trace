package graphprovenance

import (
	"strings"
	"testing"
)

func TestRepairExternalValidCarrierInventory(t *testing.T) {
	_, e := reviewEnvelope(t)
	n := 0
	for _, b := range e.Bindings {
		if b.AnchorStatus == "INVALID_COORDINATES" {
			n++
			t.Errorf("valid baseline carrier labeled invalid: %+v", b)
		}
		if strings.Contains(b.Pointer, "/position") {
			t.Logf("position %+v", b)
		}
	}
	t.Logf("invalid=%d bindings=%d", n, len(e.Bindings))
}
