package inventoryworkflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programctransitionhost"
	"lsp-trace/internal/publication"
)

type exactFileReader struct {
	path  string
	reads *int
}

func (r exactFileReader) ReadBounded(_ context.Context, limit int64) ([]byte, error) {
	if r.reads != nil {
		*r.reads++
	}
	info, err := os.Lstat(r.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, censuscontinuation.ErrInvalidProgramCTransition
	}
	return os.ReadFile(r.path)
}

func exactInput(t *testing.T, path string) censuscontinuation.ExactProgramCTransitionInput {
	return countedExactInput(t, path, nil)
}

func countedExactInput(t *testing.T, path string, reads *int) censuscontinuation.ExactProgramCTransitionInput {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return censuscontinuation.ExactProgramCTransitionInput{Reader: exactFileReader{path: path, reads: reads}, ByteLength: int64(len(raw)), SHA256: workflowDigest(raw)}
}

func workflowDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func productionWorkflowRequest(t *testing.T, receipt, ledger censuscontinuation.ExactProgramCTransitionInput, stateDir string, maxTransitions int, opens *int) Request {
	t.Helper()
	bindings := censuscontinuation.ProgramCTransitionBindings{
		RepositoryCommit: strings.Repeat("a", 40), RepositoryTree: strings.Repeat("b", 40),
		FrozenCorpusSHA256: "sha256:" + strings.Repeat("c", 64), CensusReceiptSHA256: receipt.SHA256, LedgerSHA256: ledger.SHA256,
		AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3,
		ParametersSHA256: "sha256:" + strings.Repeat("d", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("e", 64),
	}
	source := &censuscontinuation.AuthoritativeProgramCTransitionPlanSource{Receipt: receipt, Ledger: ledger}
	planRaw, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bindings.PlanSHA256 = workflowDigest(planRaw)
	bindingsRaw, err := json.Marshal(bindings)
	if err != nil {
		t.Fatal(err)
	}
	identity := inventorysuccessor.WorkflowIdentity{
		Profile: "CALLS_ONLY_FROZEN_CENSUS_V1", Contract: "adr0012-private-successor-v1",
		CensusReceiptSHA256: receipt.SHA256, AvailabilityLedgerSHA256: ledger.SHA256,
		SourcePinSHA256: bindings.FrozenCorpusSHA256, ProgramCPlanSHA256: bindings.PlanSHA256,
		ProgramCBindingsSHA256: workflowDigest(bindingsRaw), PrivacyPolicySHA256: "sha256:" + strings.Repeat("f", 64),
		RetentionPolicySHA256: "sha256:" + strings.Repeat("1", 64), ResourcePolicySHA256: bindings.ResourcePolicySHA256,
		ReplayPolicySHA256: "sha256:" + strings.Repeat("2", 64), QualificationPolicySHA256: "sha256:" + strings.Repeat("3", 64),
	}
	return Request{
		ProgramC: programctransitionhost.RepeatedRequest{
			Receipt: receipt, Ledger: ledger, WorkflowKey: "production-workflow", HeadKey: "production-workflow",
			Bindings: bindings, MaxTransitions: maxTransitions,
			OpenState: func() (*publication.Root, error) { *opens++; return publication.OpenRoot(stateDir) },
		},
		SuccessorWorkflowKey: "production-successor", SuccessorHeadKey: "production-successor", Identity: identity,
	}
}

func TestRunBoundedProductionCustodyToGapLedger(t *testing.T) {
	publicationDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil || !filepath.IsAbs(publicationDir) {
		t.Fatalf("resolve publication root: path=%q err=%v", publicationDir, err)
	}
	if err = os.Chmod(publicationDir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(publicationDir)
	if err != nil {
		t.Fatal(err)
	}
	nativeResult, err := json.Marshal(graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: "bounded", SourceRevision: "rev", ServerVersion: "v"}},
		Summary:       graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := graphprovenance.CaptureV5(nativeResult, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	authority := captureset.NativeV5Authority()
	constituent, err := authority.Constituent(exact)
	if err != nil {
		t.Fatal(err)
	}
	ledgerValue := captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "bounded", Disposition: "processed"}}}
	seed := "bounded-seed"
	manifest, err := captureset.Prepare([]captureset.Target{{CensusOrdinal: 0, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: workflowDigest([]byte(seed))}}, []captureset.Constituent{constituent}, ledgerValue, ledgerValue, "census", "duplicates")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = captureset.AssociateBatches(manifest, []captureset.Constituent{constituent})
	if err != nil {
		t.Fatal(err)
	}
	published := captureset.NewPublisher(root).PublishCaptureSet(manifest, [][]byte{exact}, authority)
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	receiptWire := map[string]any{
		"schema_version": "lsp-trace.census-source-partition-complete.private.v1", "authority": 0, "accepted": false,
		"completeness": "UNKNOWN", "source_graph_complete": "UNKNOWN", "cross_chunk_calls": "NOT_COMPUTED", "feature_identity": "UNRESOLVED",
		"implementation_decision": "UNISSUED", "production_admission": "REFUSED_MISSING_EVIDENCE", "effects": "NONE", "reviews": "NOT_REQUESTED",
		"absence_inference": "PROHIBITED", "aggregate_capture": "NOT_MATERIALIZED", "all_chunks_succeeded": true, "all_publications_verified": true,
		"chunk_count": 1, "source_count": 1, "target_count_sum": 1, "publication_byte_length_sum": int64(published.Receipt.ByteLength),
		"global_selected_paths_digest": "sha256:" + strings.Repeat("1", 64), "partition_plan_sha256": "sha256:" + strings.Repeat("2", 64), "preflight_receipt_sha256": "sha256:" + strings.Repeat("3", 64),
		"chunks": []any{map[string]any{"ordinal": 1, "source_count": 1, "target_count": 1, "batch_count": len(manifest.Batches), "publication_selector": published.Receipt.Selector, "publication_digest": published.Receipt.ArtifactSHA256, "publication_byte_length": int64(published.Receipt.ByteLength), "verification_status": "VERIFIED"}},
	}
	ledgerWire := map[string]any{
		"schema_version": "lsp-trace.census-constituent-availability.private.v1", "authority": 0, "accepted": false,
		"completeness": "UNKNOWN", "source_graph_complete": "UNKNOWN", "cross_chunk_calls": "NOT_COMPUTED", "feature_identity": "UNRESOLVED", "effects": "NONE",
		"absence_inference": "PROHIBITED", "aggregate_capture": "NOT_MATERIALIZED", "chunk_count": 1, "available_constituent_count": 1, "source_manifest_present_count": 1,
		"rows": []any{map[string]any{"ordinal": 1, "expected_source_count": 1, "publication_selector": published.Receipt.Selector, "publication_digest": published.Receipt.ArtifactSHA256, "publication_byte_length": int64(published.Receipt.ByteLength), "source_manifest": "bounded.sources.json", "source_manifest_present": true, "valid_replica_count": 1, "replicas": []any{map[string]any{"path": filepath.Join(publicationDir, filepath.FromSlash(published.Receipt.Selector)), "sha256": published.Receipt.ArtifactSHA256, "byte_length": int64(published.Receipt.ByteLength), "valid": true}}}},
	}
	receiptRaw, _ := json.Marshal(receiptWire)
	ledgerRaw, _ := json.Marshal(ledgerWire)
	receiptPath := filepath.Join(publicationDir, "bounded-receipt.json")
	ledgerPath := filepath.Join(publicationDir, "bounded-ledger.json")
	if err = os.WriteFile(receiptPath, receiptRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ledgerPath, ledgerRaw, 0600); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	if err = os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	opens, receiptReads, ledgerReads := 0, 0, 0
	request := productionWorkflowRequest(t, countedExactInput(t, receiptPath, &receiptReads), countedExactInput(t, ledgerPath, &ledgerReads), stateDir, 2, &opens)
	result, err := Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	gap, err := inventorysuccessor.ValidateDescribePreparationGapLedger(result.GapLedger)
	if err != nil || result.Outcome != inventorysuccessor.OutcomeWaitingEvidence || result.ProgramCStatus != censuscontinuation.ProgramCTransitionComplete || result.ProgramCTransitions != 2 || opens != 4 || receiptReads != 6 || ledgerReads != 6 || gap.Authority != 0 || gap.Accepted {
		t.Fatalf("ASSERT_BOUNDED_PRODUCTION_WORKFLOW result=%+v opens=%d receipt_reads=%d ledger_reads=%d gap=%+v err=%v", result, opens, receiptReads, ledgerReads, gap, err)
	}
	reopened, err := publication.OpenRoot(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := inventorysuccessor.NewHeads(reopened)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	headRaw, err := heads.Load(context.Background(), request.SuccessorHeadKey)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	revision, err := inventorysuccessor.ParseHeadRevision(headRaw)
	if err != nil || revision != result.Successor.Head {
		reopened.Close()
		t.Fatalf("ASSERT_BOUNDED_SUCCESSOR_REOPEN revision=%+v want=%+v err=%v", revision, result.Successor.Head, err)
	}
	rebuilt, err := inventorysuccessor.BuildDescribePreparationGapLedger(context.Background(), inventorysuccessor.GapLedgerRequest{WorkflowKey: request.SuccessorWorkflowKey, HeadKey: request.SuccessorHeadKey, Identity: request.Identity, Heads: heads})
	closeErr := reopened.Close()
	if err != nil || closeErr != nil || !bytes.Equal(rebuilt, result.GapLedger) {
		t.Fatalf("ASSERT_BOUNDED_LEDGER_REBUILDS_FROM_RETAINED_HEAD rebuilt=%d returned=%d err=%v close=%v", len(rebuilt), len(result.GapLedger), err, closeErr)
	}
	second, err := Run(context.Background(), request)
	if err != nil || second.Outcome != inventorysuccessor.OutcomeWaitingEvidence || second.ProgramCStatus != censuscontinuation.ProgramCTransitionComplete || second.ProgramCTransitions != 1 || opens != 7 || receiptReads != 11 || ledgerReads != 11 || !bytes.Equal(second.GapLedger, result.GapLedger) {
		t.Fatalf("ASSERT_BOUNDED_NEW_TOP_LEVEL_REVERIFIES_NO_REPUBLISH second=%+v opens=%d receipt_reads=%d ledger_reads=%d err=%v", second, opens, receiptReads, ledgerReads, err)
	}
}

// TestRunFrozenCorpusProductionCustodyToGapLedger is an opt-in, long-running
// qualification. The pre-reuse attempt timed out during repeated authoritative
// replica verification. Invocation-scoped reuse then exposed a truthful
// transition-46 matching-resource stop. Accepted ADR 0013 v3 custody preserves
// that incompleteness and advances the structural prefix exactly once. A
// completed passing invocation of this exact test establishes this boundary.
func TestRunFrozenCorpusProductionCustodyToGapLedger(t *testing.T) {
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	if receiptPath == "" || ledgerPath == "" {
		t.Skip("set LSP_TRACE_PROGRAM_C_RECEIPT and LSP_TRACE_PROGRAM_C_LEDGER to reproduce the opt-in frozen 395-constituent qualification")
	}
	receipt := exactInput(t, receiptPath)
	ledger := exactInput(t, ledgerPath)
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	opens := 0
	result, err := Run(context.Background(), productionWorkflowRequest(t, receipt, ledger, stateDir, programctransitionhost.MaxRepeatedTransitions, &opens))
	if err != nil {
		t.Fatal(err)
	}
	gap, err := inventorysuccessor.ValidateDescribePreparationGapLedger(result.GapLedger)
	if err != nil || result.Outcome != inventorysuccessor.OutcomeWaitingEvidence ||
		result.ProgramCStatus != censuscontinuation.ProgramCTransitionComplete || result.ProgramCTransitions != 396 || opens != 398 ||
		gap.Authority != 0 || gap.Accepted || gap.FeatureIdentity != "UNRESOLVED" || gap.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" {
		t.Fatalf("ASSERT_FROZEN_PRODUCTION_WORKFLOW result=%+v opens=%d gap=%+v err=%v", result, opens, gap, err)
	}
}
