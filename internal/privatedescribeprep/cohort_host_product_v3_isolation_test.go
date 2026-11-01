//go:build darwin || linux

package privatedescribeprep

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// Existing original-owned V2 fixture, not an owner manufactured from V3 data.
// The controlled physical peer presents the NEW protocol under the old binding;
// the unchanged old production backend must refuse before admitting its payload.
func TestLegacyCohortPhysicalRefusesHostProductV3(t *testing.T) {
	h, i, _ := cohortHandoffTest(t)
	req := cohortHandoffRequestTest(h)
	frame := i.frame(req, 4096)
	response := CohortHandoffResponsePrivateV2{Binding: frame.Binding, PayloadSHA256: pilot.Digest([]byte("[]")), PayloadByteLength: 2, Payload: []byte("[]")}
	response.Binding.Protocol = "PRIVATE_V3_OWN_AST_EVIDENCE_CAPSULE_1"
	raw, _ := json.Marshal(response)
	done := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, e := i.shared.root.Lstat("request-000001.json"); e == nil {
				done <- i.shared.publish("response-000001.json", raw)
				return
			}
			time.Sleep(time.Millisecond)
		}
		done <- errors.New("controlled wait deadline")
	}()
	_, e := i.ExchangeCohort(context.Background(), req, 4096)
	if writeErr := <-done; writeErr != nil {
		t.Fatal(writeErr)
	}
	var terminal interface{ TerminalHandoffFailure() }
	if e == nil || !errors.As(e, &terminal) || !i.shared.failed {
		t.Fatal("ASSERT_LEGACY_PHYSICAL_REFUSES_NEW_V3_PROTOCOL", e)
	}
	t.Log("ASSERT_LEGACY_PHYSICAL_REFUSES_NEW_V3_PROTOCOL: PASS")
}
