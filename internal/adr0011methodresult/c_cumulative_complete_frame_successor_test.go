package adr0011methodresult

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/adr0011cobserve"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// Successor to the immutable historical RED-only test archived in testdata.
// Shared pinned fixture, delivery and marker helpers are independently active in
// c_cumulative_fixture_helpers_test.go and use package-local exact originals.
func TestCCumulativeCompleteOriginalFrameSuccessor(t *testing.T) {
	manifest, wires := cCumulativePins(t)
	fixtureRoot, assets := cTrackedABAssets(t)
	if !t.Run("independent-A-B-controls", TestCObservedB4SuccessorEquivalenceAndOrder) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: independent A/B controls")
	}
	for _, name := range []string{"at-cap", "plus-one"} {
		if !t.Run(name, func(t *testing.T) {
			in, occurrence, sources, wants, _, req, owner, profile := composedFixture(t, assets, fixtureRoot, "A")
			if len(req.Params) != manifest.FixtureRequestParams.Length || cCumulativeSHA(req.Params) != manifest.FixtureRequestParams.SHA || len(in.Write.RequestFrame) != manifest.FixtureRequestFrame.Length || cCumulativeSHA(in.Write.RequestFrame) != manifest.FixtureRequestFrame.SHA || !bytes.Equal(wires["at-cap"][5], bridgeAssetBytes(t, fixtureRoot, "A/response.frame", assets)) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: request/response A pins")
			}
			req.MaxBytes = manifest.Limits.MaxBytes
			req.MaxMessages = manifest.Limits.MaxMessages
			req.CaptureDefinitionResponseFrameMaxBytes = manifest.Limits.CaptureDefinitionResponseFrameMaxBytes
			req.CaptureMethodRequestFrameMaxBytes = manifest.Limits.CaptureMethodRequestFrameMaxBytes
			req.Deadline = time.Now().Add(12 * time.Second)
			child, delivered := cCumulativeChild(in.Write.RequestFrame, wires[name])
			manager, started := cFixtureManager(t, profile, child, req)
			probe := &cCumulativeMarkers{}
			result, lease := manager.RoundTripPrivateB4(adr0011cobserve.With(context.Background(), probe.observe), req, owner)
			cExactWrite(t, child)
			if result.Failure != "" {
				_ = child.stdout.Close()
			}
			var d cCumulativeDelivery
			select {
			case d = <-delivered:
			case <-time.After(4 * time.Second):
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s producer join timeout", name)
			}
			for i, wire := range wires[name] {
				if d.bytes[i] != len(wire) || d.errs[i] != nil {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame %d delivery=%d/%d err=%v", name, i, d.bytes[i], len(wire), d.errs[i])
				}
			}
			t.Logf("complete-delivery: %s bytes=%v total=%d", name, d.bytes, manifest.Cases[name].Total)
			if result.RequestMessages != 1 || result.RequestBytes <= 0 || result.Failure == session.RequestTimeout || result.Failure == session.SessionPoisoned || result.Failure == session.SessionCrashed || result.ServerError != nil {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s request/transport: failure=%s messages=%d", name, result.Failure, result.RequestMessages)
			}
			root, dir := cPrivateRoot(t)
			raw, _ := retentionFixture(t)
			var decision PrivateB4Decision
			var pubErr error
			continued := cContinue(result, lease, func() {
				decision = cObservedB4(probe.observe, manager, lease, cSelection(started, result, owner), in, occurrence, sources)
				if decision.Status == DefinitionBridgeCandidateItems {
					_, pubErr = publishPrivateCandidateObserved(func() { adr0011cobserve.Notify(probe.observe, adr0011cobserve.PrivateCandidatePublishEntry) }, root, "synthetic.json", raw)
				}
			})
			m := probe.snapshot()
			if m.ambiguous || m.decode[4] != 1 || m.retained[4] < 2 {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s fifth marker unreachable/ambiguous decoded=%v retained=%v", name, m.decode, m.retained)
			}
			if name == "at-cap" {
				responseRead, responseOK := result.CompletedResponseRead()
				if result.Failure != "" || lease == (sessionruntime.B4DefinitionLease{}) || result.Messages != 6 || len(result.Notifications) != 5 || !responseOK || responseRead.FrameBytes != 163 || responseRead.FrameSHA256 != "sha256:"+manifest.FixtureResponseFrame.SHA || !continued || decision.Status != DefinitionBridgeCandidateItems || len(decision.Candidates) != len(wants) || pubErr != nil || m.decoded != 6 || m.decode[5] != 1 || m.retained[5] < 2 || m.b4 != 1 || m.publish != 1 {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: at-cap six-message selection/lease/marker/B4 prerequisite: failure=%s messages=%d notifications=%d decoded=%v retained=%v b4=%d publish=%d decision=%s", result.Failure, result.Messages, len(result.Notifications), m.decode, m.retained, m.b4, m.publish, decision.Status)
				}
				if _, err := os.Stat(filepath.Join(dir, "synthetic.json")); err != nil {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: at-cap private publication: %v", err)
				}
				t.Logf("at-cap-success: decoded=%v retained=%v b4=%d publish=%d", m.decode, m.retained, m.b4, m.publish)
				return
			}
			// Unlike the historical RED-only test, zero sixth entries with the typed
			// cumulative failure is the success branch. Capture-cap denial alone cannot
			// satisfy this conjunction: sixth decode must also be absent.
			_, responseOK := result.CompletedResponseRead()
			if result.Failure != session.ResourceExhausted || lease != (sessionruntime.B4DefinitionLease{}) || continued || result.Messages != 5 || len(result.Notifications) != 5 || responseOK || m.decoded != 5 || m.decode[5] != 0 || m.retained[5] != 0 || m.b4 != 0 || m.publish != 0 || pubErr != nil {
				t.Errorf("cumulative-plus-one-boundary: sixth decoded=%d retained=%d failure=%s messages=%d notifications=%d matched=%v lease=%v continued=%v b4=%d publish=%d", m.decode[5], m.retained[5], result.Failure, result.Messages, len(result.Notifications), responseOK, lease != (sessionruntime.B4DefinitionLease{}), continued, m.b4, m.publish)
			} else {
				t.Logf("cumulative-plus-one-boundary: PASS sixth decoded=0 retained=0 failure=%s messages=5 lease=false b4=0 publish=0", result.Failure)
			}
			if _, err := os.Stat(filepath.Join(dir, "synthetic.json")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("cumulative-publication-absent: %v", err)
			}
		}) && name == "at-cap" {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: at-cap prerequisite")
		}
	}
}
