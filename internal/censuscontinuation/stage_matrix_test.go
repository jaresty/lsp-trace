package censuscontinuation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/targetpacket"
)

type cancelAfterCheckpointStore struct {
	*MemoryStore
	cancel context.CancelFunc
	stage  Stage
}

func (s *cancelAfterCheckpointStore) Put(ctx context.Context, raw []byte) (string, error) {
	id, err := s.MemoryStore.Put(ctx, raw)
	if err != nil {
		return "", err
	}
	if checkpoint, parseErr := ParseCheckpoint(raw); parseErr == nil && checkpoint.Stage() == s.stage && checkpoint.Status() == StatusRunning {
		s.cancel()
	}
	return id, nil
}

func TestEveryStageFailureAndCancellationRecoveryMatrix(t *testing.T) {
	fixture := newRecoveryFixture(t)
	preparedTargets := 0
	for _, batch := range fixture.input.Projection.Batches {
		preparedTargets += len(batch.Targets)
	}
	if got := preparedTargets; got != 2 || len(fixture.input.Projection.Batches) != 2 {
		t.Fatalf("ASSERT_RECOVERY_FIXTURE_TARGET_COUNT: targets=%d batches=%d want_targets=2 want_batches=2", got, len(fixture.input.Projection.Batches))
	}
	handoff, err := BuildHandoff(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	stages := []Stage{StageProgramCComputed, StageSnapshotsCaptured, StagePacketsPrepared, StageRequestsRendered, StageDescribeComplete, StageCatalogAssembled, StageCatalogCommitted}
	const secret = "RAW_SOURCE_PATH_PROMPT_PROVIDER_STDERR"
	for _, stage := range stages {
		stage := stage
		t.Run(string(stage)+"/failure", func(t *testing.T) {
			store := NewMemoryStore()
			result := runWithHooks(context.Background(), Request{Handoff: handoff, Workspace: fixture.workspace, Contract: testContract(t), Worker: &countingWorker{}, Store: store, FreshCapture: testFreshCapture(t, fixture)}, func(_ context.Context, point stageHookPoint, got Stage) error {
				if point == stageHookAfter && got == stage {
					return errors.New(secret)
				}
				return nil
			})
			assertStageRecovery(t, store, fixture, handoff, stage, result, secret)
		})
		t.Run(string(stage)+"/cancel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			store := NewMemoryStore()
			result := runWithHooks(ctx, Request{Handoff: handoff, Workspace: fixture.workspace, Contract: testContract(t), Worker: &countingWorker{}, Store: store, FreshCapture: testFreshCapture(t, fixture)}, func(_ context.Context, point stageHookPoint, got Stage) error {
				if point == stageHookAfter && got == stage {
					cancel()
				}
				return nil
			})
			if result.Status != StatusCancelledAfterCommit {
				t.Fatalf("ASSERT_STAGE_CANCEL_STATUS_%s: %+v", stage, result)
			}
			assertStageRecovery(t, store, fixture, handoff, stage, result, secret)
		})
	}
}

func assertStageRecovery(t *testing.T, store Store, fixture fixture, handoff CommittedHandoff, stage Stage, result Result, secret string) {
	t.Helper()
	if result.CensusID != handoff.CensusID() || result.HandoffID != handoff.HandoffID() || result.CheckpointID == "" || result.Err == nil {
		t.Fatalf("ASSERT_STAGE_RECOVERY_IDENTITY_%s: %+v", stage, result)
	}
	chain, err := VerifyChain(context.Background(), store, result.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	last := chain[len(chain)-1]
	if last.Stage() != stage {
		t.Fatalf("ASSERT_STAGE_RECOVERY_CHECKPOINT_%s: %s", stage, last.Stage())
	}
	raw, _ := last.Bytes()
	if strings.Contains(result.Err.Error(), secret) || strings.Contains(string(raw), secret) {
		t.Fatalf("ASSERT_STAGE_DIAGNOSTIC_REDACTION_%s", stage)
	}
	resumeRequest := ResumeRequest{Selector: result.CheckpointID, Store: store, Worker: &countingWorker{}}
	if stageIndex(stage) < stageIndex(StageSnapshotsCaptured) {
		resumeRequest.Workspace = fixture.workspace
		resumeRequest = testManagedResumeRequest(t, fixture, resumeRequest)
	}
	resumed := Resume(context.Background(), resumeRequest)
	if resumed.Status != StatusComplete || resumed.Err != nil {
		t.Fatalf("ASSERT_STAGE_RESUMABLE_%s: %+v", stage, resumed)
	}
	packetCount, requestCount := recoveryArtifactCounts(t, store, resumed.CheckpointID)
	if packetCount != 2 || requestCount != 2 {
		t.Fatalf("ASSERT_STAGE_RECOVERY_FIXTURE_COUNTS_%s: targets=2 packets=%d requests=%d", stage, packetCount, requestCount)
	}
}

func recoveryArtifactCounts(t *testing.T, store Store, checkpointID string) (int, int) {
	t.Helper()
	chain, err := VerifyChain(context.Background(), store, checkpointID)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := chain[len(chain)-1].Artifacts()
	var packetCount, requestCount int
	for _, artifact := range artifacts {
		raw, err := store.Get(context.Background(), artifact.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch artifact.Kind {
		case "packets":
			var packets targetpacket.Result
			if err := json.Unmarshal(raw, &packets); err != nil {
				t.Fatal(err)
			}
			packetCount = len(packets.Packets)
		case "requests":
			var requests []json.RawMessage
			if err := json.Unmarshal(raw, &requests); err != nil {
				t.Fatal(err)
			}
			requestCount = len(requests)
		}
	}
	return packetCount, requestCount
}

func TestCancellationAfterProgramCCommitPersistsRecovery(t *testing.T) {
	fixture := newFixture(t)
	handoff, err := BuildHandoff(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	store := &cancelAfterCheckpointStore{MemoryStore: NewMemoryStore(), cancel: cancel, stage: StageProgramCComputed}
	result := Run(ctx, Request{Handoff: handoff, Workspace: fixture.workspace, Contract: testContract(t), Worker: &countingWorker{}, Store: store, FreshCapture: testFreshCapture(t, fixture)})
	if result.Status != StatusCancelledAfterCommit || result.Err == nil {
		t.Fatalf("ASSERT_CANCEL_AFTER_PROGRAM_C_COMMIT_RECOVERABLE: result=%+v", result)
	}
	chain, err := VerifyChain(context.Background(), store, result.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	last := chain[len(chain)-1]
	if last.Stage() != StageProgramCComputed || last.Status() != StatusCancelledAfterCommit {
		t.Fatalf("ASSERT_CANCEL_AFTER_PROGRAM_C_COMMIT_CHECKPOINT: stage=%s status=%s", last.Stage(), last.Status())
	}
}
