package censuscontinuation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/v5sourcesnapshotv6"
)

func terminalCaptureState(t *testing.T) (*MemoryStore, pipelineState) {
	t.Helper()
	store := NewMemoryStore()
	artifacts := []ArtifactRef{
		{Kind: "continuation_contract", ID: digestOf([]byte("contract")), Digest: digestOf([]byte("contract")), ByteLength: 1},
		{Kind: "handoff", ID: digestOf([]byte("handoff")), Digest: digestOf([]byte("handoff")), ByteLength: 1},
		{Kind: "program_c", ID: digestOf([]byte("program")), Digest: digestOf([]byte("program")), ByteLength: 1},
	}
	checkpoint, err := NewCheckpoint(CheckpointInput{Stage: StageProgramCComputed, CensusID: digestOf([]byte("census")), HandoffID: digestOf([]byte("handoff-id")), ContractID: digestOf([]byte("contract-id")), ProfileID: digestOf([]byte("profile")), Artifacts: artifacts, Status: StatusRunning})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := checkpoint.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	selector, err := store.Put(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, pipelineState{checkpoint: checkpoint, checkpointSelector: selector, artifacts: artifacts, contractID: checkpoint.ContractID(), profileID: checkpoint.ProfileID(), handoff: CommittedHandoff{wire: handoffWire{CensusID: checkpoint.CensusID(), HandoffID: checkpoint.HandoffID()}}}
}

type terminalPutFailureStore struct{ *MemoryStore }

func (s terminalPutFailureStore) Put(context.Context, []byte) (string, error) {
	return "", errors.New("injected terminal store failure")
}

func TestFailCapturePersistsTypedTerminalForNonLimitFailure(t *testing.T) {
	store, state := terminalCaptureState(t)
	prior := state.checkpointSelector
	result := failCapture(context.Background(), store, &state, managedPreparationError(ManagedPreparationAvailability, "managed document preparation unavailable: /private/source.go"))
	if result.CheckpointID == prior {
		t.Fatal("ASSERT_CAPTURE_EXIT_ADVANCES_TO_TERMINAL_CHECKPOINT")
	}
	raw, err := store.Get(context.Background(), result.CheckpointID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := ParseCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, ok := checkpoint.CaptureFailureDiagnostic()
	if !ok || checkpoint.Status() != StatusFailedCapture || diagnostic.Category != "availability" || diagnostic.FailedField != "source_preparation" || diagnostic.Invariant != "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT" || diagnostic.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" || diagnostic.Recovery != "RESTART_FROM_PRESERVED_CENSUS_COMMIT" {
		t.Fatalf("ASSERT_NON_LIMIT_CAPTURE_TYPED_PRIVATE_SAFE: checkpoint=%+v diagnostic=%+v", checkpoint, diagnostic)
	}
	if diagnostic.Observed != 0 || diagnostic.Limit != 0 {
		t.Fatalf("ASSERT_NON_LIMIT_CAPTURE_OMITS_FALSE_LIMIT: %+v", diagnostic)
	}
}

func TestFailCaptureTerminalStoreFailureIsSeparateAndDoesNotFabricateDiagnosis(t *testing.T) {
	base, state := terminalCaptureState(t)
	prior := state.checkpointSelector
	result := failCapture(context.Background(), terminalPutFailureStore{MemoryStore: base}, &state, errors.New("managed document preparation unavailable"))
	var persistErr *TerminalCheckpointPersistError
	if !errors.As(result.Err, &persistErr) || result.PrivateCode != "TERMINAL_CHECKPOINT_PERSIST" || result.CheckpointID != prior {
		t.Fatalf("ASSERT_TERMINAL_CHECKPOINT_STORE_FAILURE_TYPED_SEPARATELY: result=%+v err=%v", result, result.Err)
	}
	if _, ok := result.CaptureFailureDiagnostic(); ok {
		t.Fatal("ASSERT_TERMINAL_CHECKPOINT_STORE_FAILURE_DOES_NOT_FABRICATE_CAPTURE_DIAGNOSIS")
	}
}

type retainedOverlayStore struct {
	root string
	mem  *MemoryStore
}

func (s retainedOverlayStore) Put(ctx context.Context, raw []byte) (string, error) {
	return s.mem.Put(ctx, raw)
}

func (s retainedOverlayStore) Get(ctx context.Context, selector string) ([]byte, error) {
	if raw, err := s.mem.Get(ctx, selector); err == nil {
		return raw, nil
	}
	return os.ReadFile(filepath.Join(s.root, strings.TrimPrefix(selector, "sha256:")))
}

type retainedOnlyPreparer struct{ entered *bool }

func (p retainedOnlyPreparer) PrepareManagedDocuments(context.Context, string, string, uint64, string, []string, ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
	*p.entered = true
	return nil, errors.New("managed document preparation unavailable")
}

func TestExactRetainedV6ReplayCaptureBoundary(t *testing.T) {
	root := os.Getenv("LSP_TRACE_RETAINED_V6_CARDINALITY_ROOT")
	if root == "" {
		t.Skip("set LSP_TRACE_RETAINED_V6_CARDINALITY_ROOT for immutable exact replay")
	}
	objects := filepath.Join(root, "continuations", "objects", "sha256")
	store := retainedOverlayStore{root: objects, mem: NewMemoryStore()}
	preparerEntered, resolverEntered := false, false
	deps := &FreshCaptureDependencies{
		Context:  context.Background(),
		Preparer: retainedOnlyPreparer{entered: &preparerEntered},
		Resolver: v5sourcesnapshotv6.ResolverFunc(func(context.Context, v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
			resolverEntered = true
			return v5sourcesnapshotv6.ResolveResult{}, errors.New("resolver unavailable")
		}),
		Limits: ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 32, MaxWork: 100, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20},
	}
	result := Resume(context.Background(), ResumeRequest{Selector: "sha256:ef6eee9fd4be6ab27cfcd4efb7ed8caf1e89ff7acd981c8fa1b7809756aaf97d", UseCommittedWorkspace: true, Store: store, FreshCapture: deps})
	diagnostic, ok := result.CaptureFailureDiagnostic()
	if !ok {
		t.Fatalf("ASSERT_EXACT_REPLAY_TYPED_TERMINAL: result=%+v", result)
	}
	if diagnostic.Category != "availability" || diagnostic.FailedField != "source_preparation" || diagnostic.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" || resolverEntered {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_UNAVAILABLE_TYPED_AND_NO_RESOLVER diagnostic=%+v resolver_entered=%t", diagnostic, resolverEntered)
	}
	t.Logf("EXACT_REPLAY stage=%s status=%s category=%s field=%s invariant=%s action=%s recovery=%s preparer_entered=%t resolver_entered=%t", diagnostic.Stage, result.Status, diagnostic.Category, diagnostic.FailedField, diagnostic.Invariant, diagnostic.CallerAction, diagnostic.Recovery, preparerEntered, resolverEntered)
}
