package inventoryworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
	"lsp-trace/internal/publication"
)

// This fixture retains actual Native V5 publications and executes the admitted
// Program C/instability path. No provider, process server, runtime acquisition,
// imported historical store, or positive qualification test token is involved.
func multiSeedRetainedCheckpointFixtureV2(t *testing.T) (Request, string) {
	t.Helper()
	ctx := context.Background()
	pubDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(pubDir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(pubDir)
	if err != nil {
		t.Fatal(err)
	}
	authority := captureset.NativeV5Authority()
	constituents := []captureset.Constituent{}
	payloads := [][]byte{}
	targets := []captureset.Target{}
	entries := []captureset.LedgerEntry{}
	for i := 0; i < 4; i++ {
		value := multiSeedControlledNativeV2(t, i)
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		retained, err := graphprovenance.CaptureV5(raw, fmt.Sprintf("controlled-%d", i), 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
		if err != nil {
			t.Fatal(err)
		}
		// All six runs are computed by the actual runner below. The fixture also
		// checks that each Native is independently admitted before publication.
		outcome, failure := programc.Compute(retained, 1)
		if failure != nil {
			t.Fatalf("controlled Native admission %d: %v", i, failure)
		}
		if i == 2 && outcome.Outcome != "EMPTY" || i != 2 && outcome.Outcome != "COMPLETE" {
			t.Fatalf("controlled Native class %d: %s", i, outcome.Outcome)
		}
		constituent, err := authority.Constituent(retained)
		if err != nil {
			t.Fatal(err)
		}
		seed := fmt.Sprintf("controlled-seed-%d", i)
		targets = append(targets, captureset.Target{CensusOrdinal: i, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: workflowDigest([]byte(seed))})
		entries = append(entries, captureset.LedgerEntry{Ordinal: i, Identity: seed, Disposition: "processed"})
		constituents = append(constituents, constituent)
		payloads = append(payloads, retained)
	}
	ledgerValue := captureset.Ledger{Denominator: 4, Entries: entries}
	batches := []captureset.Batch{}
	for i := 0; i < 4; i++ {
		batches = append(batches, captureset.Batch{Ordinal: i, TargetStart: i, TargetCount: 1, ConstituentIndex: i})
	}
	manifest, err := captureset.PrepareWithBatches(targets, constituents, batches, ledgerValue, ledgerValue, "controlled-census", "controlled-duplicates")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = captureset.AssociateBatches(manifest, constituents)
	if err != nil {
		t.Fatal(err)
	}
	published := captureset.NewPublisher(root).PublishCaptureSet(manifest, payloads, authority)
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	receiptWire := map[string]any{
		"schema_version": "lsp-trace.census-source-partition-complete.private.v1", "authority": 0, "accepted": false, "completeness": "UNKNOWN", "source_graph_complete": "UNKNOWN", "cross_chunk_calls": "NOT_COMPUTED", "feature_identity": "UNRESOLVED", "implementation_decision": "UNISSUED", "production_admission": "REFUSED_MISSING_EVIDENCE", "effects": "NONE", "reviews": "NOT_REQUESTED", "absence_inference": "PROHIBITED", "aggregate_capture": "NOT_MATERIALIZED", "all_chunks_succeeded": true, "all_publications_verified": true, "chunk_count": 1, "source_count": 4, "target_count_sum": 4, "publication_byte_length_sum": int64(published.Receipt.ByteLength), "global_selected_paths_digest": "sha256:" + strings.Repeat("1", 64), "partition_plan_sha256": "sha256:" + strings.Repeat("2", 64), "preflight_receipt_sha256": "sha256:" + strings.Repeat("3", 64),
		"chunks": []any{map[string]any{"ordinal": 1, "source_count": 4, "target_count": 4, "batch_count": len(manifest.Batches), "publication_selector": published.Receipt.Selector, "publication_digest": published.Receipt.ArtifactSHA256, "publication_byte_length": int64(published.Receipt.ByteLength), "verification_status": "VERIFIED"}},
	}
	ledgerWire := map[string]any{
		"schema_version": "lsp-trace.census-constituent-availability.private.v1", "authority": 0, "accepted": false, "completeness": "UNKNOWN", "source_graph_complete": "UNKNOWN", "cross_chunk_calls": "NOT_COMPUTED", "feature_identity": "UNRESOLVED", "effects": "NONE", "absence_inference": "PROHIBITED", "aggregate_capture": "NOT_MATERIALIZED", "chunk_count": 1, "available_constituent_count": 1, "source_manifest_present_count": 1,
		"rows": []any{map[string]any{"ordinal": 1, "expected_source_count": 4, "publication_selector": published.Receipt.Selector, "publication_digest": published.Receipt.ArtifactSHA256, "publication_byte_length": int64(published.Receipt.ByteLength), "source_manifest": "controlled.sources.json", "source_manifest_present": true, "valid_replica_count": 1, "replicas": []any{map[string]any{"path": filepath.Join(pubDir, filepath.FromSlash(published.Receipt.Selector)), "sha256": published.Receipt.ArtifactSHA256, "byte_length": int64(published.Receipt.ByteLength), "valid": true}}}},
	}
	receiptRaw, _ := json.Marshal(receiptWire)
	ledgerRaw, _ := json.Marshal(ledgerWire)
	receiptPath := filepath.Join(pubDir, "controlled-receipt.json")
	ledgerPath := filepath.Join(pubDir, "controlled-ledger.json")
	if err = os.WriteFile(receiptPath, receiptRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ledgerPath, ledgerRaw, 0600); err != nil {
		t.Fatal(err)
	}
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	assertMultiSeedAvailabilityCountsChunksV2(t, receiptPath, ledgerPath)
	opens := 0
	request := productionWorkflowRequest(t, exactInput(t, receiptPath), exactInput(t, ledgerPath), stateDir, 8, &opens)
	result, err := Run(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProgramCStatus != censuscontinuation.ProgramCTransitionComplete {
		t.Fatalf("CONTROLLED_FOUR_CONSTITUENTS_TERMINAL: %+v", result)
	}
	return request, stateDir
}

func assertMultiSeedAvailabilityCountsChunksV2(t *testing.T, receiptPath, ledgerPath string) {
	t.Helper()
	source := &censuscontinuation.AuthoritativeProgramCTransitionPlanSource{Receipt: exactInput(t, receiptPath), Ledger: exactInput(t, ledgerPath)}
	raw, err := source.Load(context.Background())
	var plan struct {
		Records []json.RawMessage `json:"records"`
	}
	if err != nil || json.Unmarshal(raw, &plan) != nil || len(plan.Records) != 4 {
		t.Fatalf("AVAILABILITY_COUNTS_CHUNKS_NOT_BATCHES: one chunk must admit four records: count=%d err=%v", len(plan.Records), err)
	}
	raw, err = os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var ledger map[string]any
	if err = json.Unmarshal(raw, &ledger); err != nil {
		t.Fatal(err)
	}
	ledger["available_constituent_count"] = 4
	bad, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(t.TempDir(), "four-availability-for-one-chunk.json")
	if err = os.WriteFile(badPath, bad, 0600); err != nil {
		t.Fatal(err)
	}
	source.Ledger = exactInput(t, badPath)
	if _, err = source.Load(context.Background()); !errors.Is(err, censuscontinuation.ErrInvalidProgramCTransition) {
		t.Fatalf("AVAILABILITY_COUNTS_CHUNKS_NOT_BATCHES: four availability entries must refuse: %v", err)
	}
	t.Log("AVAILABILITY_COUNTS_CHUNKS_NOT_BATCHES PASS: availability=1 admits four batch records; availability=4 refuses")
}

func TestMultiSeedV2AvailabilityCountsChunksNotBatches(t *testing.T) {
	multiSeedRetainedCheckpointFixtureV2(t)
}

func multiSeedControlledNativeV2(t *testing.T, kind int) graph.Result {
	t.Helper()
	pairs := 1
	if kind == 2 {
		pairs = 0
	}
	if kind == 3 {
		pairs = 64
	}
	nodes := []graph.Node{}
	edges := []graph.Edge{}
	for p := 0; p < pairs; p++ {
		pair := []graph.Node{}
		for j := 0; j < 2; j++ {
			line := uint32(2*p + j)
			r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
			n := graph.NewNode(graph.Item{Name: fmt.Sprintf("node-%d-%03d-%d", kind, p, j), Kind: 12, URI: fmt.Sprintf("file:///controlled/%d.go", kind), Range: r, SelectionRange: r})
			nodes = append(nodes, n)
			pair = append(pair, n)
		}
		edges = append(edges, graph.Edge{CallerNodeID: pair[0].ID, CalleeNodeID: pair[1].ID, CallSites: []graph.Range{{Start: graph.Position{Line: uint32(2 * p)}, End: graph.Position{Line: uint32(2 * p), Character: 1}}}})
	}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: nodes, Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	seeds := []graph.InvocationSeed{}
	results := []graph.SeedResult{}
	if kind != 2 {
		allNodes := []string{}
		allRelations := []string{}
		for _, n := range nodes {
			allNodes = append(allNodes, n.ID)
		}
		for _, e := range edges {
			allRelations = append(allRelations, e.RelationID)
		}
		for j := 0; j < 2; j++ {
			label := fmt.Sprintf("group-%d", j)
			seed := graph.InvocationSeed{Label: label, At: fmt.Sprintf("%d.go:%d:1", kind, j+1), ResolvedURI: fmt.Sprintf("file:///controlled/%d.go", kind), ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
			seeds = append(seeds, seed)
			prepared := []string{nodes[0].ID}
			reached := append([]string{}, allNodes...)
			relations := append([]string{}, allRelations...)
			if kind == 1 && j == 0 {
				prepared = []string{nodes[1].ID}
				reached = []string{nodes[0].ID}
			}
			if j == 1 {
				relations = []string{}
			}
			results = append(results, graph.SeedResult{Label: label, PreparedTargetIDs: prepared, ReachedNodeIDs: reached, ReachedRelationIDs: relations})
		}
	}
	return graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///controlled", Server: graph.ServerInvocation{Command: "controlled-no-provider"}, LanguageID: "go", Seeds: seeds, Provenance: graph.InvocationProvenance{InvocationID: fmt.Sprintf("controlled-%d", kind), SourceRevision: "controlled-test", ServerVersion: "controlled-v1"}}, Nodes: nodes, Edges: edges, Seeds: results, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
}

func assertMultiSeedRetainedAccountingV2(t *testing.T, e censuscontinuation.ProgramCMultiSeedQualificationEvidenceV2, x programcrepresentative.QualificationSubjectExpectedContextV2) (uint64, uint64) {
	t.Helper()
	s := programcrepresentative.QualificationSubjectV2{CorpusIdentity: x.CorpusIdentity, AdapterContract: x.AdapterContract, ReceiptInventory: x.ReceiptInventory, TerminalProgramCHead: x.TerminalProgramCHead}
	constituents, objects, err := e.ReconstructCheckpointConstituentsV2(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	mechanics, _, err := e.ReconstructQualifiedMechanicsV2(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]int{}
	runs, d, g := 0, uint64(0), uint64(0)
	for _, c := range constituents {
		classes[c.Classification]++
		if len(c.Runs) != 6 {
			t.Fatal("ACTUAL_SIX_RUNS_PER_CONSTITUENT")
		}
		for j, r := range c.Runs {
			runs++
			if r.Seed != uint64(j/3+1) || r.ReplayOrdinal != uint8(j%3+1) {
				t.Fatal("ACTUAL_24_RUN_IDENTITIES")
			}
			switch c.Classification {
			case "COMPLETE":
				d += r.CommunityCount
			case "EMPTY":
				if r.Outcome != "EMPTY" || r.CommunityCount != 0 || r.CommunityArtifact != (programcrepresentative.ArtifactBindingDTO{}) {
					t.Fatal("EMPTY_EXACT_ZERO_COMMUNITY_BINDING")
				}
			case "COMMITTED_INCOMPLETE_MATCHING_RESOURCE":
				if r.Outcome != "COMPLETE" || r.CommunityCount == 0 || objects[r.CommunityArtifact] == nil {
					t.Fatal("RESOURCE_EXCLUDED_RETAINS_COMPLETE_DIAGNOSTICS")
				}
			default:
				t.Fatal("UNKNOWN_CONSTITUENT_CLASS")
			}
		}
	}
	for _, m := range mechanics {
		ctx := m.SelectionContext()
		if len(ctx.SeedGroups) != 2 {
			t.Fatal("COMPLETE_TWO_SEED_GROUP_CONTEXT")
		}
		g += uint64(len(ctx.SeedGroups))
		if len(ctx.SeedGroups[0].RelationIDs) == 0 || len(ctx.SeedGroups[1].RelationIDs) != 0 {
			t.Fatal("ELIGIBLE_GROUPS_HAVE_DIFFERENT_CLOSURES")
		}
	}
	if runs != 24 || classes["COMPLETE"] != 2 || classes["EMPTY"] != 1 || classes["COMMITTED_INCOMPLETE_MATCHING_RESOURCE"] != 1 || d != uint64(len(mechanics)) || g != 2*d {
		t.Fatalf("ACTUAL_24_RUN_D_G_CONSERVATION: runs=%d classes=%v D=%d mechanics=%d G=%d", runs, classes, d, len(mechanics), g)
	}
	t.Logf("ACTUAL_24_RUN_D_G_CONSERVATION PASS runs=%d D=%d G=%d; excluded/EMPTY contribute no mechanics", runs, d, g)
	return d, g
}

func TestMultiSeedV2ConnectedCheckpoint(t *testing.T) {
	t.Log("procedure: actual retained four-constituent Native publications -> six runs each -> terminal reconstruction -> v2 expected context -> subject -> qualified denominator/lineage -> three separate CAS operations with reopen")
	request, stateDir := multiSeedRetainedCheckpointFixtureV2(t)
	before := workflowStateSnapshot(t, stateDir)
	root, err := request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	verification, err := programctransitionhost.NewCompletionVerificationRequest(context.Background(), CompletionRequest{StateRoot: root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger, WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(context.Background(), verification)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		t.Fatal(err)
	}
	wantD, wantG := assertMultiSeedRetainedAccountingV2(t, evidence, expected)
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	if len(expected.ReceiptInventory) != 4 || expected.TerminalProgramCHead.ConstituentCount != 4 || expected.TerminalProgramCHead.CommittedPrefix != 4 {
		t.Fatal("CONNECTED_COMPLETE_FOUR_RECEIPT_CONTEXT")
	}
	implementationSource := controlledCheckpointImplementationReaderV2{root: filepath.Clean(filepath.Join("..", ".."))}
	subject, err := BuildProgramCMultiSeedQualificationSubjectV2(context.Background(), request, implementationSource, programcrepresentative.MaxArtifactCanonicalBytesV2)
	if errors.Is(err, programcrepresentative.ErrMultiSeedV2ReviewRequired) {
		if subject.Trusted() || subject.Identity() != "" {
			t.Fatal("REVIEW_BARRIER_PARTIAL_SUBJECT_EXPOSURE")
		}
		after := workflowStateSnapshot(t, stateDir)
		if len(after) != len(before) {
			t.Fatal("REVIEW_BARRIER_STATE_WRITE")
		}
		for k, v := range before {
			if after[k] != v {
				t.Fatal("REVIEW_BARRIER_STATE_MUTATION")
			}
		}
		t.Fatalf("CONNECTED_CHECKPOINT_NOT_GREEN: actual retained four-constituent v2 reconstruction succeeded; %v; verifier and downstream operation apparatus is implemented but production qualification remains unissued", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	runControlledMultiSeedCheckpointAssertionsV2(t, request, stateDir, subject, implementationSource, wantD, wantG)
}
