package censuscontinuation

import (
	"context"
	"testing"
)

func TestStopAfterDescribeRequestsPausesWithoutWorkerAndResumesExactlyOnce(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()

	paused := Run(context.Background(), Request{
		Handoff: h, Workspace: f.workspace, Store: store, Contract: testContractV2(t),
		StopAfter: StopAfterDescribeRequests, FreshCapture: testFreshCapture(t, f),
	})
	if paused.Err != nil || paused.Status != StatusPaused || paused.CheckpointID == "" || paused.RequestCount < 0 {
		t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_PAUSED: %+v", paused)
	}

	replayed := Resume(context.Background(), ResumeRequest{
		Selector: paused.CheckpointID, Store: store, StopAfter: StopAfterDescribeRequests,
	})
	if replayed.Err != nil || replayed.Status != StatusPaused || replayed.CheckpointID != paused.CheckpointID || replayed.RequestCount != paused.RequestCount {
		t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_REPLAY_IDENTITY: paused=%+v replayed=%+v", paused, replayed)
	}

	wrongVersion := Resume(context.Background(), ResumeRequest{Selector: paused.CheckpointID, Store: store, Worker: &countingWorker{}})
	if wrongVersion.Err == nil || wrongVersion.Err.Error() != "invalid resume worker version" {
		t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_REJECTS_V1_RESUME_WORKER: %+v", wrongVersion)
	}

	worker := &countingWorkerV2{}
	completed := Resume(context.Background(), ResumeRequest{Selector: paused.CheckpointID, Store: store, WorkerV2: worker})
	if completed.Err != nil || completed.Status != StatusComplete || worker.calls != paused.RequestCount {
		t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_RESUME_REMAINING_ONCE: completed=%+v calls=%d requests=%d", completed, worker.calls, paused.RequestCount)
	}
}
