package censuscontinuation

import (
	"sync"
	"testing"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
)

func TestRetrySelectionConcurrentSmall(t *testing.T) {
	contract := testContract(t)
	request := describerequest.Record{RecordID: "request", Envelope: describerequest.Envelope{MessageID: "message"}}
	pin := digestOf([]byte("race-pin"))
	binding := describeworker.IdentityBinding{RequestRecordID: request.RecordID, MessageID: request.Envelope.MessageID, AttemptID: attemptID(contract.ID(), request.RecordID, 1), WorkerSHA256: pin, ModelSHA256: pin, LibrarySHA256: pin, SandboxExecutableSHA256: pin, SandboxProfileSHA256: pin, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: pin, GrammarSHA256: pin, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1}}
	first, err := describeworker.NewInvocationRecord(binding, describeworker.StatusBackendFailure, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, describeworker.ResponseRecord{})
	if err != nil {
		t.Fatal(err)
	}
	binding.AttemptID = attemptID(contract.ID(), request.RecordID, 2)
	final, err := describeworker.NewInvocationRecord(binding, describeworker.StatusBackendFailure, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, describeworker.ResponseRecord{})
	if err != nil {
		t.Fatal(err)
	}
	state := pipelineState{contract: contract, requests: []describerequest.Record{request}, invocations: []describeworker.InvocationRecord{final, first}}

	var wait sync.WaitGroup
	errors := make(chan error, 16)
	for i := 0; i < cap(errors); i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			selected, _, err := selectCatalogAttempts(&state, true)
			if err == nil && selected[request.RecordID].ID() != final.ID() {
				t.Errorf("ASSERT_RETRY_SELECTION_CONCURRENT_FINAL: got=%s want=%s", selected[request.RecordID].ID(), final.ID())
			}
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("ASSERT_RETRY_SELECTION_CONCURRENT: %v", err)
		}
	}
}
