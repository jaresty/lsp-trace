package censuscontinuation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

type countingWorkerV2 struct{ calls int }

func (w *countingWorkerV2) RunV2(_ context.Context, request describerequest.Record, packet targetpacket.Packet, attempt string) (describeworker.RunResultV2, error) {
	w.calls++
	pin := digestOf([]byte("pin"))
	host := describeworker.ResponseHostV2{
		RequestRecordID: request.RecordID, MessageID: request.Envelope.MessageID, AttemptID: attempt,
		Consumer:   describeworker.ConsumerIdentityV2{Resolution: describeworker.ConsumerUnresolvedV2, AlternativeID: request.Lineage.AlternativeID, AlternativeOrdinal: request.Lineage.AlternativeOrdinal},
		Pins:       describeworker.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin, PromptSHA256: digestOf([]byte(request.Envelope.Prompt))},
		Provenance: describeworker.HostProvenanceV2{PacketID: packet.PacketID, RequestLineageIdentity: request.Lineage.LineageIdentity},
		Custody:    describeworker.HostCustodyV2{GraphDigest: packet.Custody.GraphDigest, CaptureID: packet.Custody.CaptureID},
	}
	response, err := describeworker.NewResponseRecordV2([]byte(`{"verdict":"COMPLETE","target_role":"target","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"bounded","consumer_relative":false},"boundary_contribution":"unknown","limitations":["bounded fixture"]}`), host)
	if err != nil {
		return describeworker.RunResultV2{}, err
	}
	binding := describeworker.IdentityBinding{RequestRecordID: request.RecordID, MessageID: request.Envelope.MessageID, AttemptID: attempt, WorkerSHA256: pin, ModelSHA256: pin, LibrarySHA256: pin, SandboxExecutableSHA256: pin, SandboxProfileSHA256: pin, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: host.Pins.PromptSHA256, GrammarSHA256: pin, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1}}
	invocation, err := describeworker.NewInvocationRecordV2(binding, describeworker.StatusSucceeded, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: response.RawStdoutLength()}, response)
	return describeworker.RunResultV2{Invocation: invocation, Response: response}, err
}

func testContractV2(t *testing.T) ContinuationContract {
	t.Helper()
	c, err := NewContinuationContract(ContractInput{ContinuationID: digestOf([]byte("continuation-v2")), ProfileID: digestOf([]byte("profile-v2")), WorkerPinID: digestOf([]byte("worker-v2")), ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 4 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 100, MaxWork: 10_000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "continuation-v2-test", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: RetryPolicy{MaxAttemptsPerRequest: 2}, ResponseVersion: describeworker.ResponseVersionV2})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPipelineV2FreshResumeNoV1NoRerunIdentityConvergence(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	worker := &countingWorkerV2{}
	fresh := Run(context.Background(), Request{Handoff: h, Workspace: f.workspace, WorkerV2: worker, Store: store, Contract: testContractV2(t), FreshCapture: testFreshCapture(t, f)})
	if fresh.Err != nil || fresh.Status != StatusComplete || worker.calls == 0 {
		t.Fatalf("ASSERT_PIPELINE_V2_FRESH: %+v calls=%d", fresh, worker.calls)
	}
	calls := worker.calls
	resumed := Resume(context.Background(), ResumeRequest{Selector: fresh.CheckpointID, Store: store, WorkerV2: worker})
	if resumed.Err != nil || resumed.Status != StatusComplete || worker.calls != calls || resumed.CatalogSelector != fresh.CatalogSelector || resumed.Composite.ID() != fresh.Composite.ID() {
		t.Fatalf("ASSERT_PIPELINE_V2_RESUME_IDENTITY_NO_RERUN: fresh=%+v resumed=%+v calls=%d/%d", fresh, resumed, calls, worker.calls)
	}
	if _, err := VerifyChain(context.Background(), store, fresh.CheckpointID); err != nil {
		t.Fatalf("ASSERT_PIPELINE_V2_VERIFY_CHAIN: %v", err)
	}
}

func TestPipelineV2ZeroRequestsAndVersionRejection(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.workspace, "a.go")); err != nil {
		t.Fatal(err)
	}
	worker := &countingWorkerV2{}
	request := Request{Handoff: h, Workspace: f.workspace, WorkerV2: worker, Store: NewMemoryStore(), Contract: testContractV2(t), FreshCapture: testUnavailableFreshCapture(t, f)}
	result := Run(context.Background(), request)
	if result.Err != nil || result.Status != StatusComplete || worker.calls != 0 {
		t.Fatalf("ASSERT_PIPELINE_V2_ZERO_REQUESTS: %+v calls=%d", result, worker.calls)
	}
	rejected := Run(context.Background(), Request{Handoff: h, Workspace: f.workspace, Worker: &countingWorker{}, Store: NewMemoryStore(), Contract: testContractV2(t)})
	if rejected.Err == nil || rejected.PrivateCode != "WORKER_V2_REQUIRED" {
		t.Fatalf("ASSERT_PIPELINE_V2_REJECTS_V1_WORKER: %+v", rejected)
	}
}
