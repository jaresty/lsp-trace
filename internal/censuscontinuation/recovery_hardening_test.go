package censuscontinuation

import (
	"context"
	"sync"
	"testing"
)

func TestRecoveryChainRejectsStageSkip(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	contract := testContract(t)
	contractBytes, err := contract.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	contractRef, err := putArtifact(ctx, store, "continuation_contract", contractBytes)
	if err != nil {
		t.Fatal(err)
	}
	handoffRef, err := putArtifact(ctx, store, "handoff", []byte(`{"fixture":"handoff"}`))
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewCheckpoint(CheckpointInput{
		Stage:      StageCensusCommitted,
		CensusID:   digestOf([]byte("census")),
		HandoffID:  digestOf([]byte("handoff")),
		ContractID: contract.ID(),
		ProfileID:  contract.ProfileID(),
		Artifacts:  []ArtifactRef{contractRef, handoffRef},
		Status:     StatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	firstSelector, err := putCheckpoint(ctx, store, first)
	if err != nil {
		t.Fatal(err)
	}
	programRef, err := putArtifact(ctx, store, "program_c", []byte(`{"fixture":"program"}`))
	if err != nil {
		t.Fatal(err)
	}
	snapshotsRef, err := putArtifact(ctx, store, "snapshots", []byte(`{"fixture":"snapshots"}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewCheckpoint(CheckpointInput{
		PriorCheckpointID: firstSelector,
		Stage:             StageSnapshotsCaptured,
		CensusID:          first.CensusID(),
		HandoffID:         first.HandoffID(),
		ContractID:        first.ContractID(),
		ProfileID:         first.ProfileID(),
		Artifacts:         []ArtifactRef{contractRef, handoffRef, programRef, snapshotsRef},
		Status:            StatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := putCheckpoint(ctx, store, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChain(ctx, store, selector); err == nil {
		t.Fatal("ASSERT_RECOVERY_CHAIN_STAGE_SKIP_REJECTED")
	}
}

func TestConcurrentResumeConvergesWithoutRerunningCompletedStages(t *testing.T) {
	fixture := newFixture(t)
	handoff, err := BuildHandoff(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	fresh := Run(context.Background(), Request{Handoff: handoff, Workspace: fixture.workspace, Contract: testContract(t), Worker: &countingWorker{}, Store: store, FreshCapture: testFreshCapture(t, fixture)})
	if fresh.Status != StatusComplete {
		t.Fatalf("ASSERT_CONCURRENT_RESUME_FIXTURE: %+v", fresh)
	}
	chain, err := VerifyChain(context.Background(), store, fresh.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	var selector string
	for _, checkpoint := range chain {
		if checkpoint.Stage() == StageDescribeComplete && checkpoint.Status() == StatusRunning {
			raw, err := checkpoint.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			selector = digestOf(raw)
			break
		}
	}
	if selector == "" {
		t.Fatal("ASSERT_CONCURRENT_RESUME_DESCRIBE_CHECKPOINT")
	}

	const callers = 8
	start := make(chan struct{})
	results := make(chan Result, callers)
	var wait sync.WaitGroup
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			worker := &countingWorker{}
			result := Resume(context.Background(), ResumeRequest{Selector: selector, Store: store, Worker: worker})
			if worker.calls != 0 {
				t.Errorf("ASSERT_CONCURRENT_RESUME_COMPLETED_WORK_NOT_RERUN: calls=%d", worker.calls)
			}
			results <- result
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	var checkpointID string
	for result := range results {
		if result.Status != StatusComplete || result.Err != nil {
			t.Fatalf("ASSERT_CONCURRENT_RESUME_COMPLETE: %+v", result)
		}
		if checkpointID == "" {
			checkpointID = result.CheckpointID
		}
		if result.CheckpointID != checkpointID {
			t.Fatalf("ASSERT_CONCURRENT_RESUME_IDENTITY_CONVERGENCE: got=%s want=%s", result.CheckpointID, checkpointID)
		}
	}
}

func TestRecoveryChainRejectsContractSubstitution(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	original := testContract(t)
	substituteInput := original.wire
	substituteInput.WorkerPinID = digestOf([]byte("foreign-worker"))
	substituteInput.ContractID = identityJSON("lsp-trace:census-continuation-contract:v1", substituteInput, func(v *contractWire) { v.ContractID = "" })
	substitute := ContinuationContract{wire: substituteInput}
	if err := substitute.Validate(); err != nil {
		t.Fatal(err)
	}
	substituteBytes, err := substitute.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	contractRef, err := putArtifact(ctx, store, "continuation_contract", substituteBytes)
	if err != nil {
		t.Fatal(err)
	}
	handoffRef, err := putArtifact(ctx, store, "handoff", []byte(`{"fixture":"handoff"}`))
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := NewCheckpoint(CheckpointInput{
		Stage:      StageCensusCommitted,
		CensusID:   digestOf([]byte("census")),
		HandoffID:  digestOf([]byte("handoff")),
		ContractID: original.ID(),
		ProfileID:  original.ProfileID(),
		Artifacts:  []ArtifactRef{contractRef, handoffRef},
		Status:     StatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := putCheckpoint(ctx, store, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChain(ctx, store, selector); err == nil {
		t.Fatal("ASSERT_RECOVERY_CHAIN_CONTRACT_SUBSTITUTION_REJECTED")
	}
}
