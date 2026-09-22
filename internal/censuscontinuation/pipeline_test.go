package censuscontinuation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv4"
)

type countingWorker struct {
	calls int
	err   error
}

type cappedStore struct {
	*MemoryStore
	max int64
}

func (s cappedStore) MaxObjectBytes() int64 { return s.max }

func TestWorkerPrivateDiagnosticPropagatesTypedTerminal(t *testing.T) {
	result := Result{}
	workerPrivateDiagnostic{stage: "RUN", code: "BACKEND_FAILURE", subcode: "ADAPTER_PROTOCOL"}.apply(&result)
	if result.PrivateStage != "RUN" {
		t.Fatalf("ASSERT_WORKER_PRIVATE_STAGE_PROPAGATED: got %q", result.PrivateStage)
	}
	if result.PrivateCode != "BACKEND_FAILURE" {
		t.Fatalf("ASSERT_WORKER_PRIVATE_CODE_PROPAGATED: got %q", result.PrivateCode)
	}
	field := reflect.ValueOf(result).FieldByName("PrivateSubcode")
	if !field.IsValid() || field.String() != "ADAPTER_PROTOCOL" {
		t.Fatalf("ASSERT_WORKER_PRIVATE_SUBCODE_PROPAGATED: field_present=%t", field.IsValid())
	}
}

func (s cappedStore) Put(ctx context.Context, raw []byte) (string, error) {
	if int64(len(raw)) > s.max {
		return "", errors.New("object exceeds cap")
	}
	return s.MemoryStore.Put(ctx, raw)
}

func testContract(t *testing.T) ContinuationContract {
	t.Helper()
	c, err := NewContinuationContract(ContractInput{
		ContinuationID:         digestOf([]byte("continuation")),
		ProfileID:              digestOf([]byte("profile")),
		WorkerPinID:            digestOf([]byte("worker")),
		ProgramCSeed:           9,
		CaptureLimits:          v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 4 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 100, MaxWork: 10_000},
		PacketSourcePolicy:     sourceprojection.Policy{PolicyID: "continuation-test", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true},
		PacketResolveLimits:    retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100},
		PacketMaxResponseBytes: 1 << 20,
		DescribeDeadlineMS:     1000,
		RetryPolicy:            RetryPolicy{MaxAttemptsPerRequest: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (w *countingWorker) Run(_ context.Context, request describerequest.Record, attempt string) (describeworker.RunResult, error) {
	w.calls++
	pin := digestOf([]byte("pin"))
	binding := describeworker.IdentityBinding{RequestRecordID: request.RecordID, MessageID: request.Envelope.MessageID, AttemptID: attempt, WorkerSHA256: pin, ModelSHA256: pin, LibrarySHA256: pin, SandboxExecutableSHA256: pin, SandboxProfileSHA256: pin, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: digestOf([]byte(request.Envelope.Prompt)), GrammarSHA256: pin, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1}}
	response, err := describeworker.NewResponseRecord(binding, describeworker.SemanticResponse{Verdict: "COMPLETE", TargetRole: "target", NearestOutwardConsumer: "unresolved", ConsumerNeed: "unknown", ProvidedBehavior: "bounded", BoundaryContribution: "unknown", Limitations: []string{"bounded fixture"}, Citations: []string{"target source"}})
	if err != nil {
		w.err = err
		return describeworker.RunResult{}, err
	}
	invocation, err := describeworker.NewInvocationRecord(binding, describeworker.StatusSucceeded, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, response)
	w.err = err
	return describeworker.RunResult{Invocation: invocation, Response: response}, err
}

func TestHandoffPersistencePreflightReportsCanonicalBytesAndStoreCap(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	const capBytes = int64(1 << 20)
	result := Run(context.Background(), Request{Handoff: h, Workspace: f.workspace, Worker: &countingWorker{}, Store: cappedStore{MemoryStore: NewMemoryStore(), max: capBytes}, Contract: testContract(t)})
	if result.PrivateCode != "HANDOFF_PERSIST" || result.HandoffBytes != int64(len(raw)) || result.MaxObjectBytes != capBytes {
		t.Fatalf("ASSERT_HANDOFF_PERSIST_PREFLIGHT bytes=%d want=%d cap=%d want_cap=%d code=%s", result.HandoffBytes, len(raw), result.MaxObjectBytes, capBytes, result.PrivateCode)
	}
}

func TestContinuationContractCaptureCeilingAndHistoricalCompatibility(t *testing.T) {
	base := testContract(t)
	raw, err := base.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := ParseContinuationContract(raw); err != nil || parsed.ID() != base.ID() {
		t.Fatalf("ASSERT_HISTORICAL_CONTRACT_STILL_VERIFIES: id=%q err=%v", parsed.ID(), err)
	}
	input := ContractInput{ContinuationID: digestOf([]byte("ceiling")), ProfileID: digestOf([]byte("profile")), WorkerPinID: digestOf([]byte("worker")), ProgramCSeed: 9, CaptureLimits: ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "ceiling", BodyRequested: true, MaxBytes: 1, MaxRanges: 1, MaxObjects: 1, MaxWork: 1, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: 1, MaxLogicalSelections: 1}, PacketMaxResponseBytes: 1, DescribeDeadlineMS: 1, RetryPolicy: RetryPolicy{MaxAttemptsPerRequest: 1}}
	if _, err := NewContinuationContract(input); err != nil {
		t.Fatalf("ASSERT_CAPTURE_CEILING_EXACT_ACCEPTED: %v", err)
	}
	input.CaptureLimits.MaxArtifactBytes = MaxCaptureBytes + 1
	if _, err := NewContinuationContract(input); err == nil {
		t.Fatal("ASSERT_CAPTURE_CEILING_LIMIT_PLUS_ONE_REJECTED_BEFORE_WRITE")
	}
}

func TestContinuationContractPersistedBeforeFirstCheckpoint(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	contract := testContract(t)
	result := Run(context.Background(), Request{Handoff: h, Workspace: "/workspace", Worker: &countingWorker{}, Store: store, Contract: contract})
	if result.CheckpointID == "" {
		t.Fatalf("ASSERT_CONTINUATION_CONTRACT_PERSISTED: no committed checkpoint: %+v", result)
	}
	chain, err := VerifyChain(context.Background(), store, result.CheckpointID)
	if err != nil || len(chain) == 0 {
		t.Fatalf("ASSERT_CONTINUATION_CONTRACT_PERSISTED: chain: %v", err)
	}
	var contractRef ArtifactRef
	for _, ref := range chain[0].Artifacts() {
		if ref.Kind == "continuation_contract" {
			contractRef = ref
		}
	}
	contractBytes, err := store.Get(context.Background(), contractRef.ID)
	if err != nil {
		t.Fatalf("ASSERT_CONTINUATION_CONTRACT_PERSISTED: %v", err)
	}
	parsed, err := ParseContinuationContract(contractBytes)
	if err != nil || parsed.ID() != contract.ID() {
		t.Fatalf("ASSERT_CONTINUATION_CONTRACT_PERSISTED: parse=%v id=%q", err, parsed.ID())
	}
}

func TestProgramCWitnessReplacesOpaqueStore(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	result := Run(context.Background(), Request{Handoff: h, Workspace: "/workspace", Worker: &countingWorker{}, Store: NewMemoryStore(), Contract: testContract(t)})
	if result.Err != nil && strings.Contains(result.Err.Error(), "PROGRAM_C_STORE") {
		t.Fatalf("ASSERT_PROGRAM_C_WITNESS_REPLACES_OPAQUE_STORE: %+v", result)
	}
}

func TestProgramCWitnessRejectsRecomputationMismatch(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	program, err := ReconstructProgramC(h, testContract(t).ProgramCSeed())
	if err != nil {
		t.Fatal(err)
	}
	witness, err := NewProgramCWitness(program)
	if err != nil {
		t.Fatal(err)
	}
	witness.Outcome.ClaimCeiling += " tampered"
	witness.WitnessID = identityJSON("lsp-trace:census-continuation-program-c-witness:v1", witness, func(v *ProgramCWitness) { v.WitnessID = "" })
	raw, err := witness.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifyProgramCWitness(program, raw); err == nil || !strings.Contains(err.Error(), "recomputation mismatch") {
		t.Fatalf("ASSERT_PROGRAM_C_WITNESS_RECOMPUTATION_MISMATCH: %v", err)
	}
}

func TestSourceBearingV4FreshSuccessThroughCatalogComposite(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.direct.Representatives.Nominations) == 0 {
		t.Fatalf("ASSERT_V4_FIXTURE_PROGRAM_C_NOMINATION: state=%s outcome=%s candidates=%d", f.direct.Representatives.State, f.direct.Outcome.Outcome, len(f.direct.Candidates))
	}
	contract := testContract(t)
	capture, err := CaptureSnapshots(h, f.direct, f.workspace, h.PositionEncoding(), contract.CaptureLimits())
	if err != nil {
		pc := f.direct.Admission.Artifact.Constituents[0]
		projection, _ := h.Projection()
		t.Fatalf("ASSERT_V4_FIXTURE_TARGET_CUSTODY: %v program=(%s,%d) envelope=(%s,%d)", err, pc.GraphSHA256, pc.GraphByteLength, digestBytes(projection.Constituents[0].Raw), len(projection.Constituents[0].Raw))
	}
	snapshots, lookup, err := ReplaySnapshots(capture)
	if err != nil {
		t.Fatalf("ASSERT_V4_FIXTURE_REPLAY: %v", err)
	}
	if _, err := targetpacket.Build(targetpacket.Request{Census: f.direct, Snapshots: snapshots, Lookup: lookup, Policy: contract.PacketSourcePolicy(), ResolveLimits: contract.PacketResolveLimits(), MaxResponseBytes: contract.PacketMaxResponseBytes()}); err != nil {
		var artifact v5sourcesnapshotv4.Artifact
		_ = json.Unmarshal(snapshots[0].Raw, &artifact)
		constituent := f.direct.Admission.Artifact.Constituents[snapshots[0].ConstituentOrdinal]
		t.Fatalf("ASSERT_V4_FIXTURE_PACKET: %v cause=%v admission=(%s,%d) snapshot=(%s,%d)", err, errors.Unwrap(err), constituent.SHA256, constituent.ByteLength, artifact.ConstituentGraphDigest, artifact.ConstituentGraphByteLength)
	}
	store := NewMemoryStore()
	worker := &countingWorker{}
	result := Run(context.Background(), Request{Handoff: h, Workspace: f.workspace, Worker: worker, Store: store, Contract: testContract(t), FreshCapture: testFreshCapture(t, f)})
	if worker.calls == 0 {
		t.Fatalf("ASSERT_V4_FULL_FRESH_SUCCESS_NONEMPTY_DESCRIBE_REQUEST: result=%+v", result)
	}
	if result.Status != StatusComplete || result.CensusID != h.CensusID() || result.HandoffID != h.HandoffID() || result.CheckpointID == "" || result.Composite.ID() == "" || result.Composite.CatalogID() == "" {
		t.Fatalf("ASSERT_V4_FULL_FRESH_SUCCESS_THROUGH_CATALOG_COMPOSITE: %+v worker_err=%v", result, worker.err)
	}
	chain, err := VerifyChain(context.Background(), store, result.CheckpointID)
	if err != nil || len(chain) < len(stageOrder) || chain[len(chain)-1].Stage() != StageCatalogCommitted || chain[len(chain)-1].Status() != StatusComplete {
		t.Fatalf("ASSERT_V4_FULL_FRESH_SUCCESS_CHAIN: %v %#v", err, chain)
	}
	kinds := map[string]bool{}
	for _, ref := range chain[len(chain)-1].Artifacts() {
		kinds[ref.Kind] = true
	}
	for _, kind := range []string{"program_c", "snapshots", "packets", "requests", "describe_records", "catalog", "composite"} {
		if !kinds[kind] {
			t.Fatalf("ASSERT_V4_FULL_FRESH_SUCCESS_ARTIFACT_%s", kind)
		}
	}
}

func TestV5ReplayAfterWorkspaceDeletionAndEndpointSidecarTamper(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := CaptureSnapshots(h, f.direct, f.workspace, h.PositionEncoding(), testContract(t).CaptureLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.workspace); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReplaySnapshots(capture); err != nil {
		t.Fatalf("ASSERT_CONTINUATION_V5_REPLAY_AFTER_WORKSPACE_DELETION: %v", err)
	}
	original := capture.Constituents[0].EndpointOutcomes[0]
	capture.Constituents[0].EndpointOutcomes[0].Status = EndpointStatusSourceUnavailable
	capture.Constituents[0].EndpointOutcomes[0].Code = SourceCodeExactEndpointUnavailable
	capture.Constituents[0].EndpointOutcomes[0].OutcomeID = identityJSON("lsp-trace:census-endpoint-capture-outcome:v1", capture.Constituents[0].EndpointOutcomes[0], func(v *EndpointCaptureOutcome) { v.OutcomeID = "" })
	if _, _, err := ReplaySnapshots(capture); err == nil {
		t.Fatal("ASSERT_CHECKPOINT_V5_SIDECAR_SEMANTIC_RECOMPUTATION")
	}
	capture.Constituents[0].EndpointOutcomes[0] = original
	if _, _, err := ReplaySnapshots(capture); err != nil {
		t.Fatalf("ASSERT_CHECKPOINT_V5_SIDECAR_RESTORE: %v", err)
	}
}

func TestHistoricalV5AllSelectedEndpointsUnavailableCommitsSidecar(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.workspace, "a.go")); err != nil {
		t.Fatal(err)
	}
	capture, err := CaptureSnapshots(h, f.direct, f.workspace, h.PositionEncoding(), testContract(t).CaptureLimits())
	if err != nil {
		t.Fatalf("ASSERT_ALL_UNAVAILABLE_CAPTURE: %v", err)
	}
	snapshots, lookup, err := ReplaySnapshots(capture)
	if err != nil {
		t.Fatalf("ASSERT_ALL_UNAVAILABLE_REPLAY: %v", err)
	}
	contract := testContract(t)
	packets, err := targetpacket.Build(targetpacket.Request{Census: f.direct, Snapshots: snapshots, Lookup: lookup, Policy: contract.PacketSourcePolicy(), ResolveLimits: contract.PacketResolveLimits(), MaxResponseBytes: contract.PacketMaxResponseBytes()})
	if err != nil {
		var failure *targetpacket.Failure
		if errors.As(err, &failure) {
			t.Fatalf("ASSERT_ALL_UNAVAILABLE_PACKET stage=%s code=%s reason=%v", failure.Stage, failure.Code, errors.Unwrap(failure))
		}
		t.Fatal("ASSERT_ALL_UNAVAILABLE_PACKET stage=UNKNOWN code=UNKNOWN")
	}
	if len(packets.Packets) != 0 || len(packets.PreparationFailures) == 0 {
		t.Fatalf("ASSERT_ALL_UNAVAILABLE_ACCOUNTING packets=%d failures=%d", len(packets.Packets), len(packets.PreparationFailures))
	}
}

func TestAllSelectedEndpointsUnavailableCommitsSidecarAndDegradedCatalog(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	worker := &countingWorkerV2{}
	request := testUnavailableManagedRequest(t, f, Request{Handoff: h, Workspace: f.workspace, WorkerV2: worker, Store: store, Contract: testContractV2(t)})
	result := Run(context.Background(), request)
	if result.Status != StatusComplete || result.Err != nil || worker.calls != 0 {
		t.Fatalf("ASSERT_MANAGED_ALL_UNAVAILABLE_DEGRADED_CATALOG result=%+v worker_calls=%d", result, worker.calls)
	}
	chain, err := VerifyChain(context.Background(), store, result.CheckpointID)
	if err != nil {
		t.Fatalf("ASSERT_MANAGED_ALL_UNAVAILABLE_CHAIN: %v", err)
	}
	var snapshots, catalog bool
	for _, checkpoint := range chain {
		for _, artifact := range checkpoint.Artifacts() {
			switch artifact.Kind {
			case "snapshots":
				snapshots = true
			case "catalog":
				catalog = true
			}
		}
	}
	if !snapshots || !catalog || result.CatalogSelector == "" || result.Composite.ID() == "" {
		t.Fatalf("ASSERT_MANAGED_ALL_UNAVAILABLE_IDENTITIES snapshots=%t catalog=%t selector=%q composite=%q", snapshots, catalog, result.CatalogSelector, result.Composite.ID())
	}
	resumed := Resume(context.Background(), ResumeRequest{Selector: result.CheckpointID, Store: store, WorkerV2: &countingWorkerV2{}})
	if resumed.Status != StatusComplete || resumed.Err != nil || resumed.CatalogSelector != result.CatalogSelector || resumed.Composite.ID() != result.Composite.ID() {
		t.Fatalf("ASSERT_MANAGED_ALL_UNAVAILABLE_RESUME_IDENTITY result=%+v resumed=%+v", result, resumed)
	}
}

func TestResumeFromEveryCommittedStage(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	fresh := Run(context.Background(), Request{Handoff: h, Workspace: f.workspace, Worker: &countingWorker{}, Store: store, Contract: testContract(t), FreshCapture: testFreshCapture(t, f)})
	if fresh.Status != StatusComplete {
		t.Fatalf("ASSERT_RESUME_EVERY_STAGE_FRESH: %+v", fresh)
	}
	chain, err := VerifyChain(context.Background(), store, fresh.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	stageCounts := map[Stage]int{}
	for _, checkpoint := range chain {
		stageCounts[checkpoint.Stage()]++
		if stageCounts[checkpoint.Stage()] > 1 {
			continue
		}
		raw, err := checkpoint.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		workspace := ""
		if stageIndex(checkpoint.Stage()) < stageIndex(StageSnapshotsCaptured) {
			workspace = f.workspace
		}
		resumed := Resume(context.Background(), ResumeRequest{Selector: digestOf(raw), Workspace: workspace, Store: store, Worker: &countingWorker{}, FreshCapture: testFreshCapture(t, f)})
		if resumed.Status != StatusComplete || resumed.Err != nil {
			t.Fatalf("ASSERT_RESUME_STAGE_%s: %+v", checkpoint.Stage(), resumed)
		}
	}
	for _, stage := range stageOrder {
		if stageCounts[stage] == 0 {
			t.Fatalf("ASSERT_RESUME_STAGE_COUNTER_%s", stage)
		}
	}
}

func TestResumeBeforeCaptureRequiresFreshWorkspaceWithoutAmbientFallback(t *testing.T) {
	f := newFixture(t)
	h, _ := BuildHandoff(f.input)
	store := NewMemoryStore()
	hb, _ := h.Bytes()
	ref, _ := putArtifact(context.Background(), store, "handoff", hb)
	contract := testContract(t)
	contractBytes, _ := contract.Bytes()
	contractRef, _ := putArtifact(context.Background(), store, "continuation_contract", contractBytes)
	cp, err := NewCheckpoint(CheckpointInput{Stage: StageCensusCommitted, CensusID: h.CensusID(), HandoffID: h.HandoffID(), ContractID: contract.ID(), ProfileID: contract.ProfileID(), Artifacts: []ArtifactRef{contractRef, ref}, Status: StatusRunning})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := putCheckpoint(context.Background(), store, cp)
	if err != nil {
		t.Fatal(err)
	}
	result := Resume(context.Background(), ResumeRequest{Selector: selector, Store: store, Worker: &countingWorker{}})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "workspace") {
		t.Fatalf("ASSERT_NO_AMBIENT_RESUME: %+v", result)
	}
}
