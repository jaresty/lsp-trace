//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestVariableCallerV2IntegratedTransportWitness(t *testing.T) {
	for _, kind := range []string{"integrated-notification-suffix", "integrated-request-suffix"} {
		t.Run(kind, func(t *testing.T) {
			_, path, git := variableOwnedConfig(t, 1, 0)
			factory, starts, child, _ := variableOwnedFactory(t, kind)
			r, err := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
			<-child.done
			if err != nil || !r.Acquisition.Complete {
				t.Fatalf("ASSERT_INTEGRATED_CAPTURE: err=%v complete=%v", err, r.Acquisition.Complete)
			}
			f := r.Cleanup.LifecyclePrivateV2
			if starts.Load() != 1 || child.shutdownRequests.Load() != 1 || child.teardowns.Load() != 1 || r.Cleanup.StopCalls != 1 || r.Cleanup.ShutdownCalls != 1 || r.Cleanup.TerminalReads != 1 || !r.Cleanup.ShutdownJoined {
				t.Fatal("ASSERT_INTEGRATED_ONE_LIFECYCLE")
			}
			child.fixtureMu.Lock()
			defer child.fixtureMu.Unlock()
			if child.fixtureMaxReaders != 1 || child.fixtureReaders != 0 {
				t.Fatal("ASSERT_INTEGRATED_NO_READER_OVERLAP")
			}
			if child.fixtureFinalID == "" || child.fixtureShutdownID == "" || child.fixtureFinalID == child.fixtureShutdownID || f.ShutdownRequestID != child.fixtureShutdownID {
				t.Fatal("ASSERT_INTEGRATED_OWNED_IDS")
			}
			final, shutdown, ack := -1, -1, -1
			for i, e := range child.fixtureEvents {
				switch e.Kind {
				case "FINAL_RESPONSE_COALESCED":
					final = i
				case "OWN_SHUTDOWN_READ":
					shutdown = i
				case "ACK_AFTER_OWN_SHUTDOWN":
					ack = i
				}
				t.Logf("OWNED_TRANSITION kind=%s id=%s active=%d bytes=%d", e.Kind, e.ID, e.ActiveReaders, e.Bytes)
			}
			if final < 0 || shutdown <= final || ack <= shutdown {
				t.Fatal("ASSERT_INTEGRATED_ACK_AFTER_REQUEST")
			}
			if f.ShutdownRead != sessionruntime.ShutdownMatchedPrivateV2 || f.ReadFrames != 1 || !f.ReaderJoined || f.RawReadBytes != uint64(f.ReadBytes) {
				t.Fatal("ASSERT_INTEGRATED_FRESH_SHUTDOWN_READER")
			}
			// This is an owned read-ahead witness, not a claim of provider receipt or
			// fidelity. The suffix was supplied in the final response's single pipe
			// write, but is not visible to the fresh shutdown reader. No dispatcher or
			// persistent-reader repair is introduced here.
			t.Logf("ASSERT_INTEGRATED_ACK_AFTER_REQUEST: PASS finalID=%s shutdownID=%s coalescedBytes=%d maxReaders=%d suffixAbsentFromShutdown=true", child.fixtureFinalID, child.fixtureShutdownID, child.fixtureCoalescedBytes, child.fixtureMaxReaders)
		})
	}
}

func TestVariableCallerV2AcquisitionPartialOrdering(t *testing.T) {
	_, path, git := variableOwnedConfig(t, 3, 0)
	factory, _, _, _ := variableOwnedFactory(t, "capture-second-failed")
	r, err := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
	a := r.Acquisition
	if err == nil || !a.DiscoveryComplete || a.Complete || a.ExpectedBatches != 3 || a.AttemptedCaptures != 2 || a.CompletedCaptures != 1 || len(a.Batches) != 3 {
		t.Fatalf("ASSERT_PARTIAL_ACQUISITION_COUNTS: err=%v a=%+v", err, a)
	}
	for i, want := range []string{"SUCCESS_OBSERVED", "CAPTURE_FAILED", "NOT_ATTEMPTED"} {
		if a.Batches[i].Ordinal != i || a.Batches[i].Disposition != want {
			t.Fatalf("ASSERT_PARTIAL_ACQUISITION_ORDER[%d]: %+v", i, a.Batches)
		}
	}
	if !a.Batches[1].ErrorObserved || a.Batches[2].ErrorObserved {
		t.Fatal("ASSERT_PARTIAL_CAPTURE_ERROR")
	}
	t.Log("ASSERT_PARTIAL_ACQUISITION_ORDER: PASS")
}
