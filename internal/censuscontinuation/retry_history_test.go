package censuscontinuation

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/provisionalfeaturecatalog"
)

type retryWorker struct {
	calls    int
	attempts []string
}

func (w *retryWorker) Run(_ context.Context, request describerequest.Record, attempt string) (describeworker.RunResult, error) {
	w.calls++
	w.attempts = append(w.attempts, attempt)
	pin := digestOf([]byte("retry-pin"))
	binding := describeworker.IdentityBinding{
		RequestRecordID:         request.RecordID,
		MessageID:               request.Envelope.MessageID,
		AttemptID:               attempt,
		WorkerSHA256:            pin,
		ModelSHA256:             pin,
		LibrarySHA256:           pin,
		SandboxExecutableSHA256: pin,
		SandboxProfileSHA256:    pin,
		RuntimeIdentity:         "runtime",
		AdapterIdentity:         "adapter",
		ModelIdentity:           "model",
		PromptSHA256:            digestOf([]byte(request.Envelope.Prompt)),
		GrammarSHA256:           pin,
		Limits:                  describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1},
	}
	if w.calls == 1 {
		invocation, err := describeworker.NewInvocationRecord(binding, describeworker.StatusBackendFailure, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, describeworker.ResponseRecord{})
		if err != nil {
			return describeworker.RunResult{}, err
		}
		return describeworker.RunResult{Invocation: invocation}, errors.New("private worker detail must not escape")
	}
	response, err := describeworker.NewResponseRecord(binding, describeworker.SemanticResponse{Verdict: "COMPLETE", TargetRole: "target", NearestOutwardConsumer: "unresolved", ConsumerNeed: "unknown", ProvidedBehavior: "bounded", BoundaryContribution: "unknown", Limitations: []string{"bounded fixture"}, Citations: []string{"target source"}})
	if err != nil {
		return describeworker.RunResult{}, err
	}
	invocation, err := describeworker.NewInvocationRecord(binding, describeworker.StatusSucceeded, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, response)
	return describeworker.RunResult{Invocation: invocation, Response: response}, err
}

func TestBoundedRetryHistoryFailureResumeAndIdempotence(t *testing.T) {
	f := newFixture(t)
	handoff, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	worker := &retryWorker{}
	failed := Run(context.Background(), Request{Handoff: handoff, Workspace: f.workspace, Contract: testContract(t), Worker: worker, Store: store, FreshCapture: testFreshCapture(t, f)})
	if failed.Status != StatusFailedWorker || failed.CheckpointID == "" {
		t.Fatalf("ASSERT_RETRY_HISTORY_FIRST_FAILURE_PERSISTED: result=%+v", failed)
	}
	chain, err := VerifyChain(context.Background(), store, failed.CheckpointID)
	if err != nil {
		t.Fatalf("ASSERT_RETRY_HISTORY_FIRST_FAILURE_PERSISTED: chain=%v", err)
	}
	last := chain[len(chain)-1]
	if last.Stage() != StageDescribeAttempts || last.Status() != StatusFailedWorker || len(last.wire.Diagnostics) != 1 || last.wire.Diagnostics[0].Code != "WORKER_TERMINAL_FAILURE" {
		t.Fatalf("ASSERT_FAILED_INVOCATION_CHECKPOINTED_BEFORE_TERMINAL_FAILURE: stage=%s status=%s diagnostics=%+v", last.Stage(), last.Status(), last.wire.Diagnostics)
	}
	failureCheckpoint := last
	var describe ArtifactRef
	for _, ref := range last.Artifacts() {
		if ref.Kind == "describe_records" {
			describe = ref
		}
	}
	if describe.ID == "" {
		t.Fatalf("ASSERT_RETRY_HISTORY_FIRST_FAILURE_PERSISTED: missing describe history at stage=%s", last.Stage())
	}
	raw, err := store.Get(context.Background(), describe.ID)
	if err != nil {
		t.Fatal(err)
	}
	invocations, responses, err := decodeWorkerRecords(raw)
	if err != nil || len(invocations) != 1 || len(responses) != 0 || invocations[0].Status() == describeworker.StatusSucceeded {
		t.Fatalf("ASSERT_RETRY_HISTORY_FIRST_FAILURE_PERSISTED: invocations=%d responses=%d err=%v", len(invocations), len(responses), err)
	}
	firstID := invocations[0].ID()
	firstAttempt := invocations[0].Binding().AttemptID
	if err := os.RemoveAll(f.workspace); err != nil {
		t.Fatal(err)
	}

	completed := Resume(context.Background(), ResumeRequest{Selector: failed.CheckpointID, Store: store, Worker: worker})
	if completed.Status != StatusComplete || completed.Err != nil || worker.calls < 2 {
		t.Fatalf("ASSERT_RETRY_HISTORY_RESUME_NEW_ATTEMPT: result=%+v calls=%d", completed, worker.calls)
	}
	chain, err = VerifyChain(context.Background(), store, completed.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	last = chain[len(chain)-1]
	for _, ref := range last.Artifacts() {
		if ref.Kind == "describe_records" {
			describe = ref
		}
	}
	raw, err = store.Get(context.Background(), describe.ID)
	if err != nil {
		t.Fatal(err)
	}
	invocations, responses, err = decodeWorkerRecords(raw)
	if err != nil || len(invocations) < 2 || len(responses) == 0 {
		t.Fatalf("ASSERT_RETRY_HISTORY_RESUME_NEW_ATTEMPT: invocations=%d responses=%d err=%v", len(invocations), len(responses), err)
	}
	if invocations[0].ID() != firstID || invocations[0].Binding().AttemptID != firstAttempt || invocations[1].ID() == firstID || invocations[1].Binding().AttemptID == firstAttempt {
		t.Fatalf("ASSERT_RETRY_HISTORY_RESUME_NEW_ATTEMPT: first=%s second=%s", invocations[0].ID(), invocations[1].ID())
	}

	calls := worker.calls
	again := Resume(context.Background(), ResumeRequest{Selector: completed.CheckpointID, Store: store, Worker: worker})
	if again.Status != StatusComplete || again.Err != nil || worker.calls != calls {
		t.Fatalf("ASSERT_RETRY_HISTORY_SUCCESS_NOT_RERUN: result=%+v calls=%d want=%d", again, worker.calls, calls)
	}

	state := pipelineState{checkpoint: last, checkpointSelector: completed.CheckpointID, artifacts: last.Artifacts(), contractID: last.ContractID(), profileID: last.ProfileID()}
	if err := loadState(context.Background(), store, &state); err != nil {
		t.Fatal(err)
	}
	baseInvocations := append([]describeworker.InvocationRecord(nil), state.invocations...)
	baseResponses := append([]describeworker.ResponseRecord(nil), state.responses...)
	if len(state.requests) < 2 || len(baseInvocations) < 2 {
		t.Fatalf("ASSERT_RETRY_HISTORY_NONEMPTY_MULTI_REQUEST_FIXTURE: requests=%d invocations=%d", len(state.requests), len(baseInvocations))
	}
	cloneState := func() pipelineState {
		clone := state
		clone.invocations = append([]describeworker.InvocationRecord(nil), baseInvocations...)
		clone.responses = append([]describeworker.ResponseRecord(nil), baseResponses...)
		return clone
	}
	makeInvocation := func(t *testing.T, binding describeworker.IdentityBinding, status describeworker.TerminalStatus, response describeworker.ResponseRecord) describeworker.InvocationRecord {
		t.Helper()
		invocation, err := describeworker.NewInvocationRecord(binding, status, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, response)
		if err != nil {
			t.Fatal(err)
		}
		return invocation
	}
	assertRejected := func(t *testing.T, mutate func(*pipelineState)) {
		t.Helper()
		candidate := cloneState()
		mutate(&candidate)
		if _, _, err := selectCatalogAttempts(&candidate, true); err == nil || (err.Error() != "retry history invalid" && err.Error() != "retry history incomplete") {
			t.Fatalf("ASSERT_RETRY_HISTORY_MALFORMED_REJECTED: err=%v", err)
		}
	}

	t.Run("duplicate ordinal", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			candidate.invocations = append(candidate.invocations, candidate.invocations[0])
		})
	})
	t.Run("unlinked request", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			binding := candidate.invocations[0].Binding()
			binding.RequestRecordID = "foreign-request"
			binding.AttemptID = attemptID(candidate.contract.ID(), binding.RequestRecordID, 1)
			candidate.invocations = append(candidate.invocations, makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{}))
		})
	})
	t.Run("skipped ordinal", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			binding := candidate.invocations[0].Binding()
			binding.AttemptID = attemptID(candidate.contract.ID(), binding.RequestRecordID, 2)
			candidate.invocations[0] = makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
		})
	})
	t.Run("attempt beyond bound", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			binding := candidate.invocations[0].Binding()
			binding.AttemptID = attemptID(candidate.contract.ID(), binding.RequestRecordID, candidate.contract.MaxAttempts()+1)
			candidate.invocations[0] = makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
		})
	})
	t.Run("response invocation mismatch", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			candidate.responses[0], candidate.responses[1] = candidate.responses[1], candidate.responses[0]
			candidate.responses = candidate.responses[:1]
		})
	})
	t.Run("response attached to failed invocation", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			requestID := candidate.invocations[0].Binding().RequestRecordID
			for _, request := range candidate.requests {
				if request.RecordID == requestID {
					candidate.requests = []describerequest.Record{request}
					break
				}
			}
			candidate.invocations = candidate.invocations[:1]
			for _, response := range baseResponses {
				if response.Binding().RequestRecordID == requestID {
					candidate.responses = []describeworker.ResponseRecord{response}
					break
				}
			}
		})
	})
	t.Run("retry binding substitution", func(t *testing.T) {
		assertRejected(t, func(candidate *pipelineState) {
			first := candidate.invocations[0].Binding()
			first.AttemptID = attemptID(candidate.contract.ID(), first.RequestRecordID, 1)
			failed := makeInvocation(t, first, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
			second := candidate.invocations[1].Binding()
			second.AttemptID = attemptID(candidate.contract.ID(), second.RequestRecordID, 2)
			second.RuntimeIdentity = "substituted-runtime"
			succeeded := makeInvocation(t, second, describeworker.StatusSucceeded, candidate.invocations[1].Response())
			candidate.invocations = append([]describeworker.InvocationRecord{failed, succeeded}, candidate.invocations[2:]...)
		})
	})
	t.Run("input permutation invariant", func(t *testing.T) {
		permuted := cloneState()
		for left, right := 0, len(permuted.invocations)-1; left < right; left, right = left+1, right-1 {
			permuted.invocations[left], permuted.invocations[right] = permuted.invocations[right], permuted.invocations[left]
		}
		selectedA, _, errA := selectCatalogAttempts(&state, true)
		selectedB, _, errB := selectCatalogAttempts(&permuted, true)
		if errA != nil || errB != nil || len(selectedA) != len(selectedB) {
			t.Fatalf("ASSERT_RETRY_HISTORY_PERMUTATION_INVARIANT: errA=%v errB=%v", errA, errB)
		}
		buildCatalog := func(selected map[string]describeworker.InvocationRecord) provisionalfeaturecatalog.Catalog {
			invocations := make([]describeworker.InvocationRecord, 0, len(state.requests))
			responses := make([]describeworker.ResponseRecord, 0, len(state.requests))
			for _, request := range state.requests {
				invocation := selected[request.RecordID]
				invocations = append(invocations, invocation)
				if response := invocation.Response(); response.ID() != "" {
					responses = append(responses, response)
				}
			}
			catalog, err := provisionalfeaturecatalog.Build(state.packets, state.requests, invocations, responses)
			if err != nil {
				t.Fatal(err)
			}
			return catalog
		}
		for requestID, invocation := range selectedA {
			if selectedB[requestID].ID() != invocation.ID() {
				t.Fatalf("ASSERT_RETRY_HISTORY_PERMUTATION_INVARIANT: request=%s", requestID)
			}
		}
		if buildCatalog(selectedA).ID() != buildCatalog(selectedB).ID() {
			t.Fatal("ASSERT_RETRY_HISTORY_CATALOG_IDENTITY_PERMUTATION_INVARIANT")
		}
	})
	t.Run("concurrent selection", func(t *testing.T) {
		var wait sync.WaitGroup
		errors := make(chan error, 8)
		for i := 0; i < cap(errors); i++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				_, _, err := selectCatalogAttempts(&state, true)
				errors <- err
			}()
		}
		wait.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatalf("ASSERT_RETRY_HISTORY_CONCURRENT_SELECTION: %v", err)
			}
		}
	})
	t.Run("coordinated attempt replacement", func(t *testing.T) {
		binding := invocations[0].Binding()
		binding.RuntimeIdentity = "replacement-runtime"
		replacement := makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
		records, err := encodeWorkerRecords([]describeworker.InvocationRecord{replacement}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ref, err := putArtifact(context.Background(), store, "describe_records", records)
		if err != nil {
			t.Fatal(err)
		}
		artifacts := appendRef(failureCheckpoint.Artifacts(), ref)
		forged, err := NewCheckpoint(CheckpointInput{PriorCheckpointID: failed.CheckpointID, Stage: StageDescribeAttempts, CensusID: failureCheckpoint.CensusID(), HandoffID: failureCheckpoint.HandoffID(), ContractID: failureCheckpoint.ContractID(), ProfileID: failureCheckpoint.ProfileID(), Artifacts: artifacts, Status: StatusFailedWorker, Diagnostics: []Diagnostic{{Code: "WORKER_TERMINAL_FAILURE", Stage: StageDescribeAttempts}}})
		if err != nil {
			t.Fatal(err)
		}
		selector, err := putCheckpoint(context.Background(), store, forged)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = VerifyChain(context.Background(), store, selector); err == nil {
			t.Fatal("ASSERT_RETRY_HISTORY_COORDINATED_REPLACEMENT_REJECTED")
		}
	})
	t.Run("retry policy substitution", func(t *testing.T) {
		forged, err := NewCheckpoint(CheckpointInput{PriorCheckpointID: failed.CheckpointID, Stage: failureCheckpoint.Stage(), CensusID: failureCheckpoint.CensusID(), HandoffID: failureCheckpoint.HandoffID(), ContractID: digestOf([]byte("substituted-policy")), ProfileID: failureCheckpoint.ProfileID(), Artifacts: failureCheckpoint.Artifacts(), Status: StatusFailedWorker, Diagnostics: []Diagnostic{{Code: "WORKER_TERMINAL_FAILURE", Stage: failureCheckpoint.Stage()}}})
		if err != nil {
			t.Fatal(err)
		}
		selector, err := putCheckpoint(context.Background(), store, forged)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = VerifyChain(context.Background(), store, selector); err == nil {
			t.Fatal("ASSERT_RETRY_HISTORY_POLICY_SUBSTITUTION_REJECTED")
		}
	})
	t.Run("final failure selected once", func(t *testing.T) {
		request := state.requests[0]
		binding := baseInvocations[0].Binding()
		binding.RequestRecordID = request.RecordID
		binding.MessageID = request.Envelope.MessageID
		binding.AttemptID = attemptID(state.contract.ID(), request.RecordID, 1)
		first := makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
		binding.AttemptID = attemptID(state.contract.ID(), request.RecordID, 2)
		final := makeInvocation(t, binding, describeworker.StatusBackendFailure, describeworker.ResponseRecord{})
		candidate := state
		candidate.requests = []describerequest.Record{request}
		candidate.invocations = []describeworker.InvocationRecord{first, final}
		candidate.responses = nil
		selected, counts, err := selectCatalogAttempts(&candidate, true)
		if err != nil || len(selected) != 1 || selected[request.RecordID].ID() != final.ID() || counts[request.RecordID] != state.contract.MaxAttempts() {
			t.Fatalf("ASSERT_RETRY_HISTORY_FINAL_FAILURE_SELECTED_ONCE: selected=%d count=%d err=%v", len(selected), counts[request.RecordID], err)
		}
	})
}
