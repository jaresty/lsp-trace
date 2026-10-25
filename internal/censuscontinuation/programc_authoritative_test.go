package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
)

type programCFixtureReader struct {
	raw   []byte
	calls int
}

type programCFixtureFileReader struct{ path string }

func (r programCFixtureFileReader) ReadBounded(_ context.Context, limit int64) ([]byte, error) {
	info, err := os.Lstat(r.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, ErrInvalidProgramCTransition
	}
	return os.ReadFile(r.path)
}

type uncertainProgramCHeads struct {
	inner       censussourcecontinuation.HeadStore
	calls       int
	uncertainAt int
}

func (h *uncertainProgramCHeads) Load(ctx context.Context, key string) ([]byte, error) {
	return h.inner.Load(ctx, key)
}
func (h *uncertainProgramCHeads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	h.calls++
	result := h.inner.CompareAndSwap(ctx, key, expected, raw)
	if h.calls == h.uncertainAt && result.Outcome == censussourcecontinuation.CASCommitted {
		result.Outcome = censussourcecontinuation.CASCommitUncertain
	}
	return result
}

func (r *programCFixtureReader) ReadBounded(context.Context, int64) ([]byte, error) {
	r.calls++
	return append([]byte(nil), r.raw...), nil
}

func TestProgramCTransitionRepositoryRevisionExactLowerHex(t *testing.T) {
	valid := strings.Repeat("a", 40)
	if !transitionValidRepositoryRevision(valid) {
		t.Fatal("ASSERT_PROGRAM_C_TRANSITION_REVISION_VALID")
	}
	for _, invalid := range []string{"", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("A", 40), strings.Repeat("g", 40), valid + "\n"} {
		if transitionValidRepositoryRevision(invalid) {
			t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_REVISION_REJECT value=%q", invalid)
		}
	}
}

func TestAuthoritativeProgramCTransitionPlanSourceRejectsBeforeVerify(t *testing.T) {
	receipt := &programCFixtureReader{raw: []byte(`{"schema_version":"wrong"}`)}
	ledger := &programCFixtureReader{raw: []byte(`{}`)}
	verifies := 0
	source := AuthoritativeProgramCTransitionPlanSource{
		Receipt: ExactProgramCTransitionInput{Reader: receipt, ByteLength: int64(len(receipt.raw)), SHA256: transitionSHA(receipt.raw)},
		Ledger:  ExactProgramCTransitionInput{Reader: ledger, ByteLength: int64(len(ledger.raw)), SHA256: transitionSHA(ledger.raw)},
		VerifyPublication: func(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
			verifies++
			return captureset.Manifest{}, errors.New("must not verify")
		},
	}
	if _, err := source.Load(context.Background()); !errors.Is(err, ErrInvalidProgramCTransition) {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_AUTHORITATIVE_STRICT_SCHEMA err=%v", err)
	}
	if verifies != 0 {
		t.Fatalf("ASSERT_PROGRAM_C_TRANSITION_REJECTS_BEFORE_VERIFY calls=%d", verifies)
	}
}

func TestCanonicalProgramCTransitionPlanSourceIsExplicitlyNonAuthoritative(t *testing.T) {
	var _ ProgramCTransitionPlanSource = NonAuthoritativeCanonicalProgramCTransitionPlanSource{}
}

func TestAuthoritativeProgramCTransitionExactFixtureCanonical395(t *testing.T) {
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	if receiptPath == "" || ledgerPath == "" {
		t.Skip("set LSP_TRACE_PROGRAM_C_RECEIPT and LSP_TRACE_PROGRAM_C_LEDGER")
	}
	exact := func(path string) ExactProgramCTransitionInput {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return ExactProgramCTransitionInput{Reader: programCFixtureFileReader{path: path}, ByteLength: int64(len(raw)), SHA256: transitionSHA(raw)}
	}
	source := &AuthoritativeProgramCTransitionPlanSource{Receipt: exact(receiptPath), Ledger: exact(ledgerPath)}
	first, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_LOAD_FIRST err=%v", err)
	}
	firstDigest := transitionSHA(first)
	second, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_LOAD_SECOND err=%v", err)
	}
	plan, err := decodeProgramCTransitionPlan(first)
	if err != nil || !bytes.Equal(first, second) || transitionSHA(second) != firstDigest || len(plan.Records) != 395 || source.resolver == nil || len(source.resolver.bySelector) != 295 {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_CANONICAL_395 records=%d publications=%d digest=%s err=%v", len(plan.Records), len(source.resolver.bySelector), firstDigest, err)
	}
	ledgerRaw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var ledger programCAvailabilityLedger
	if decodeProgramCObservedWire(ledgerRaw, &ledger) != nil {
		t.Fatal("ASSERT_PROGRAM_C_EXACT_FIXTURE_LEDGER_DECODE")
	}
	type rankedConstituent struct {
		publicationByteLength int64
		receiptOrdinal        int
		batchOrdinal          int
		publicationSelector   string
		constituentSelector   string
	}
	var ranked []rankedConstituent
	replicaCount := 0
	for _, row := range ledger.Rows {
		replicaCount += len(row.Replicas)
		verified, ok := source.resolver.bySelector[row.PublicationSelector]
		if !ok {
			t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_LEDGER_PUBLICATION selector=%s", row.PublicationSelector)
		}
		for _, batch := range verified.manifest.Batches {
			ranked = append(ranked, rankedConstituent{row.PublicationByteLength, row.Ordinal, batch.Ordinal, row.PublicationSelector, verified.manifest.Constituents[batch.ConstituentIndex].ImmutableSelector})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].publicationByteLength != ranked[j].publicationByteLength {
			return ranked[i].publicationByteLength < ranked[j].publicationByteLength
		}
		if ranked[i].receiptOrdinal != ranked[j].receiptOrdinal {
			return ranked[i].receiptOrdinal < ranked[j].receiptOrdinal
		}
		return ranked[i].batchOrdinal < ranked[j].batchOrdinal
	})
	if replicaCount != 299 || len(ranked) != len(plan.Records) {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_ACCOUNTING records=%d ranked=%d publications=%d replicas=%d", len(plan.Records), len(ranked), len(source.resolver.bySelector), replicaCount)
	}
	for i, want := range ranked {
		got := plan.Records[i]
		if got.PublicationSelector != want.publicationSelector || got.ConstituentSelector != want.constituentSelector {
			t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_LEDGER_ORDER index=%d got_publication=%s got_constituent=%s want_bytes=%d want_receipt_ordinal=%d want_batch_ordinal=%d want_publication=%s want_constituent=%s", i, got.PublicationSelector, got.ConstituentSelector, want.publicationByteLength, want.receiptOrdinal, want.batchOrdinal, want.publicationSelector, want.constituentSelector)
		}
	}
	for selector, verified := range source.resolver.bySelector {
		if verified.selector != selector || verified.binaryDigest == "sha256:"+strings.TrimSuffix(strings.TrimPrefix(selector, captureset.CaptureSetSelectorPrefix), ".bundle") || verified.binaryLength != int64(len(verified.bundle)) || transitionSHA(verified.bundle) != verified.binaryDigest {
			t.Fatalf("ASSERT_PROGRAM_C_EXACT_FIXTURE_BINARY_IDENTITY selector=%s", selector)
		}
	}
	firstRanked := ranked[0]
	t.Logf("ASSERT_PROGRAM_C_EXACT_FIXTURE_395 records=%d publications=%d replicas=%d first_bytes=%d first_receipt_ordinal=%d first_batch_ordinal=%d first_publication=%s first_constituent=%s plan_sha256=%s", len(plan.Records), len(source.resolver.bySelector), replicaCount, firstRanked.publicationByteLength, firstRanked.receiptOrdinal, firstRanked.batchOrdinal, firstRanked.publicationSelector, firstRanked.constituentSelector, firstDigest)
}

func TestAuthoritativeProgramCTransitionSourceCountIsAccountingNotBatchDenominator(t *testing.T) {
	selector := "capture-sets/v1/sha256/" + strings.Repeat("a", 64) + ".bundle"
	binaryDigest := "sha256:" + strings.Repeat("b", 64)
	constituentSelector := "graph-provenance-v5/sha256/" + strings.Repeat("c", 64)
	receipt := programCCensusReceipt{
		SchemaVersion: "lsp-trace.census-source-partition-complete.private.v1", Authority: 0, Accepted: false,
		Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED",
		FeatureIdentity: "UNRESOLVED", ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE",
		Effects: "NONE", Reviews: "NOT_REQUESTED", AbsenceInference: "PROHIBITED", AggregateCapture: "NOT_MATERIALIZED",
		AllChunksSucceeded: true, AllPublicationsVerified: true, ChunkCount: 1, SourceCount: 5, TargetCountSum: 1, PublicationByteLengthSum: 1,
		GlobalSelectedPathsDigest: "sha256:" + strings.Repeat("1", 64), PartitionPlanSHA256: "sha256:" + strings.Repeat("2", 64), PreflightReceiptSHA256: "sha256:" + strings.Repeat("3", 64),
		Chunks: []programCReceiptChunk{{Ordinal: 1, SourceCount: 5, TargetCount: 1, BatchCount: 1, PublicationSelector: selector, PublicationDigest: binaryDigest, PublicationByteLength: 1, VerificationStatus: "VERIFIED"}},
	}
	ledger := programCAvailabilityLedger{
		SchemaVersion: "lsp-trace.census-constituent-availability.private.v1", Authority: 0, Accepted: false,
		Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", FeatureIdentity: "UNRESOLVED",
		Effects: "NONE", AbsenceInference: "PROHIBITED", AggregateCapture: "NOT_MATERIALIZED", ChunkCount: 1,
		AvailableConstituentCount: 1, SourceManifestPresentCount: 1,
		Rows: []programCAvailabilityRow{{Ordinal: 1, ExpectedSourceCount: 5, PublicationSelector: selector, PublicationDigest: binaryDigest, PublicationByteLength: 1, SourceManifest: "fixture.sources.json", SourceManifestPresent: true, ValidReplicaCount: 1, Replicas: []programCReplica{{Path: "/replica/" + selector, SHA256: binaryDigest, ByteLength: 1, Valid: true}}}},
	}
	receiptRaw, _ := json.Marshal(receipt)
	ledgerRaw, _ := json.Marshal(ledger)
	source := AuthoritativeProgramCTransitionPlanSource{
		Receipt: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: receiptRaw}, ByteLength: int64(len(receiptRaw)), SHA256: transitionSHA(receiptRaw)},
		Ledger:  ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: ledgerRaw}, ByteLength: int64(len(ledgerRaw)), SHA256: transitionSHA(ledgerRaw)},
		VerifyPublication: func(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
			return captureset.Manifest{Batches: []captureset.Batch{{Ordinal: 0, ConstituentIndex: 0}}, Constituents: []captureset.Constituent{{ImmutableSelector: constituentSelector}}}, nil
		},
	}
	planRaw, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_SOURCE_COUNT_SEPARATE_FROM_BATCH_COUNT err=%v", err)
	}
	plan, err := decodeProgramCTransitionPlan(planRaw)
	if err != nil || len(plan.Records) != 1 {
		t.Fatalf("ASSERT_PROGRAM_C_SOURCE_COUNT5_BATCH_COUNT1_CONSTITUENT1 plan=%+v err=%v", plan, err)
	}
}

func TestAuthoritativeProgramCTransitionDurableReopenProgression(t *testing.T) {
	native := func(id string) []byte {
		b, _ := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: id, SourceRevision: "rev", ServerVersion: "v"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}})
		raw, err := graphprovenance.CaptureV5(b, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	exact := [][]byte{native("a"), native("b")}
	authority := captureset.NativeV5Authority()
	constituents := make([]captureset.Constituent, len(exact))
	for i := range exact {
		var err error
		constituents[i], err = authority.Constituent(exact[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	ledger := captureset.Ledger{Denominator: 2, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a", Disposition: "processed"}, {Ordinal: 1, Identity: "b", Disposition: "processed"}}}
	targets := make([]captureset.Target, 100)
	for i := range targets {
		seed := fmt.Sprintf("seed-%d", i)
		targets[i] = captureset.Target{CensusOrdinal: i, CanonicalSeedV2: seed, CanonicalSeedV2SHA256: transitionDigest([]byte(seed))}
	}
	manifest, err := captureset.Prepare(targets, constituents, ledger, ledger, "census", "duplicates")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = captureset.AssociateBatches(manifest, constituents)
	if err != nil {
		t.Fatal(err)
	}
	publicationDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil || !filepath.IsAbs(publicationDir) {
		t.Fatalf("resolve publication temp root: path=%q err=%v", publicationDir, err)
	}
	_ = os.Chmod(publicationDir, 0700)
	publicationRoot, err := publication.OpenRoot(publicationDir)
	if err != nil {
		t.Fatal(err)
	}
	defer publicationRoot.Close()
	publisher := captureset.NewPublisher(publicationRoot)
	published := publisher.PublishCaptureSet(manifest, exact, authority)
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	receiptWire := programCCensusReceipt{SchemaVersion: "lsp-trace.census-source-partition-complete.private.v1", Authority: 0, Accepted: false, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", FeatureIdentity: "UNRESOLVED", ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE", Effects: "NONE", Reviews: "NOT_REQUESTED", AbsenceInference: "PROHIBITED", AggregateCapture: "NOT_MATERIALIZED", AllChunksSucceeded: true, AllPublicationsVerified: true, ChunkCount: 1, SourceCount: len(constituents), TargetCountSum: len(targets), PublicationByteLengthSum: int64(published.Receipt.ByteLength), GlobalSelectedPathsDigest: "sha256:" + strings.Repeat("1", 64), PartitionPlanSHA256: "sha256:" + strings.Repeat("2", 64), PreflightReceiptSHA256: "sha256:" + strings.Repeat("3", 64), Chunks: []programCReceiptChunk{{Ordinal: 1, SourceCount: len(constituents), TargetCount: len(targets), BatchCount: len(manifest.Batches), PublicationSelector: published.Receipt.Selector, PublicationDigest: published.Receipt.ArtifactSHA256, PublicationByteLength: int64(published.Receipt.ByteLength), VerificationStatus: "VERIFIED"}}}
	ledgerWire := programCAvailabilityLedger{SchemaVersion: "lsp-trace.census-constituent-availability.private.v1", Authority: 0, Accepted: false, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", FeatureIdentity: "UNRESOLVED", Effects: "NONE", AbsenceInference: "PROHIBITED", AggregateCapture: "NOT_MATERIALIZED", ChunkCount: 1, AvailableConstituentCount: 1, SourceManifestPresentCount: 1, Rows: []programCAvailabilityRow{{Ordinal: 1, ExpectedSourceCount: len(constituents), PublicationSelector: published.Receipt.Selector, PublicationDigest: published.Receipt.ArtifactSHA256, PublicationByteLength: int64(published.Receipt.ByteLength), SourceManifest: "fixture.sources.json", SourceManifestPresent: true, ValidReplicaCount: 1, Replicas: []programCReplica{{Path: filepath.Join(publicationDir, filepath.FromSlash(published.Receipt.Selector)), SHA256: published.Receipt.ArtifactSHA256, ByteLength: int64(published.Receipt.ByteLength), Valid: true}}}}}
	receiptRaw, _ := json.Marshal(receiptWire)
	ledgerRaw, _ := json.Marshal(ledgerWire)
	replicaRoot, ok := validProgramCReplicaPath(ledgerWire.Rows[0].Replicas[0].Path, published.Receipt.Selector)
	if !ok {
		t.Fatalf("ASSERT_AUTHORITATIVE_FIXTURE_REPLICA_PATH path=%s", ledgerWire.Rows[0].Replicas[0].Path)
	}
	probeRoot, err := publication.OpenRoot(replicaRoot)
	if err != nil {
		t.Fatalf("ASSERT_AUTHORITATIVE_FIXTURE_REPLICA_ROOT err=%v", err)
	}
	probeRaw, err := probeRoot.ReadSelector(published.Receipt.Selector, captureset.MaxBundleBytes)
	if err != nil || transitionSHA(probeRaw) != published.Receipt.ArtifactSHA256 {
		probeRoot.Close()
		t.Fatalf("ASSERT_AUTHORITATIVE_FIXTURE_REPLICA_BYTES len=%d digest=%s want_len=%d want_digest=%s err=%v", len(probeRaw), transitionSHA(probeRaw), published.Receipt.ByteLength, published.Receipt.ArtifactSHA256, err)
	}
	if _, err = captureset.NewPublisher(probeRoot).Verify(published.Receipt.Selector, captureset.NativeV5Authority()); err != nil {
		probeRoot.Close()
		t.Fatalf("ASSERT_AUTHORITATIVE_FIXTURE_REPLICA_NATIVE_VERIFY err=%v", err)
	}
	probeRoot.Close()
	source := &AuthoritativeProgramCTransitionPlanSource{Receipt: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: receiptRaw}, ByteLength: int64(len(receiptRaw)), SHA256: transitionSHA(receiptRaw)}, Ledger: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: ledgerRaw}, ByteLength: int64(len(ledgerRaw)), SHA256: transitionSHA(ledgerRaw)}}
	planRaw, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	_ = os.Chmod(stateDir, 0700)
	openStores := func() (*publication.Root, *censussourceassembly.Objects, *censussourceassembly.Heads) {
		root, e := publication.OpenRoot(stateDir)
		if e != nil {
			t.Fatal(e)
		}
		objects, e := censussourceassembly.NewObjects(root)
		if e != nil {
			t.Fatal(e)
		}
		heads, e := censussourceassembly.NewHeadsWithParser(root, "program-c-test/heads/v1", ParseProgramCTransitionHeadRevision)
		if e != nil {
			t.Fatal(e)
		}
		return root, objects, heads
	}
	runner := &transitionRunner{result: transitionResultBytes("STABLE")}
	bindings := transitionBindings(planRaw)
	bindings.CensusReceiptSHA256 = transitionSHA(receiptRaw)
	bindings.LedgerSHA256 = transitionSHA(ledgerRaw)
	root, objects, heads := openStores()
	req := ProgramCTransitionRequest{WorkflowKey: "workflow", HeadKey: "head", Bindings: bindings, PlanSource: source, Objects: objects, Heads: heads, Runner: runner}
	first, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || first.CommittedPrefix != 1 {
		t.Fatalf("ASSERT_AUTHORITATIVE_PREFIX_0_1 got=%+v err=%v", first, err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	root, objects, heads = openStores()
	defer root.Close()
	req.Objects = objects
	req.Heads = heads
	req.Expected = first.Head
	second, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || second.CommittedPrefix != 2 {
		t.Fatalf("ASSERT_AUTHORITATIVE_PREFIX_1_2 got=%+v err=%v", second, err)
	}
	calls := len(runner.seeds)
	req.Expected = second.Head
	done, err := CoordinateProgramCTransition(context.Background(), req)
	if err != nil || done.Status != ProgramCTransitionComplete || len(runner.seeds) != calls {
		t.Fatalf("ASSERT_AUTHORITATIVE_COMPLETE_NO_RECOMPUTE got=%+v err=%v", done, err)
	}

	assertRejectedBeforeCompute := func(name string, candidate *AuthoritativeProgramCTransitionPlanSource) {
		t.Helper()
		before := len(runner.seeds)
		if _, loadErr := candidate.Load(context.Background()); !errors.Is(loadErr, ErrInvalidProgramCTransition) {
			t.Fatalf("%s err=%v", name, loadErr)
		}
		if len(runner.seeds) != before {
			t.Fatalf("%s compute changed", name)
		}
	}
	mutReceipt := receiptWire
	mutReceipt.Chunks = append([]programCReceiptChunk(nil), receiptWire.Chunks...)
	mutReceipt.Chunks[0].PublicationDigest = "sha256:" + strings.Repeat("f", 64)
	mutReceiptRaw, _ := json.Marshal(mutReceipt)
	assertRejectedBeforeCompute("ASSERT_AUTHORITATIVE_RECEIPT_MUTATION_BEFORE_COMPUTE", &AuthoritativeProgramCTransitionPlanSource{Receipt: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: mutReceiptRaw}, ByteLength: int64(len(mutReceiptRaw)), SHA256: transitionSHA(mutReceiptRaw)}, Ledger: source.Ledger})
	mutLedger := ledgerWire
	mutLedger.Rows = append([]programCAvailabilityRow(nil), ledgerWire.Rows...)
	mutLedger.Rows[0].PublicationSelector = records2()[0].PublicationSelector
	mutLedgerRaw, _ := json.Marshal(mutLedger)
	assertRejectedBeforeCompute("ASSERT_AUTHORITATIVE_LEDGER_MUTATION_BEFORE_COMPUTE", &AuthoritativeProgramCTransitionPlanSource{Receipt: source.Receipt, Ledger: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: mutLedgerRaw}, ByteLength: int64(len(mutLedgerRaw)), SHA256: transitionSHA(mutLedgerRaw)}})
	for name, mutate := range map[string]func(*captureset.Manifest){
		"ASSERT_AUTHORITATIVE_MANIFEST_MUTATION_BEFORE_COMPUTE": func(m *captureset.Manifest) { m.Constituents = m.Constituents[:1] },
		"ASSERT_AUTHORITATIVE_BATCH_MUTATION_BEFORE_COMPUTE":    func(m *captureset.Manifest) { m.Batches[0].ConstituentIndex = len(m.Constituents) },
		"ASSERT_AUTHORITATIVE_SELECTOR_MUTATION_BEFORE_COMPUTE": func(m *captureset.Manifest) { m.Constituents[0].ImmutableSelector = "caller-authored" },
	} {
		candidate := *source
		candidate.VerifyPublication = func(string, captureset.ExactBytesAuthority) (captureset.Manifest, error) {
			m := manifest
			m.Batches = append([]captureset.Batch(nil), manifest.Batches...)
			m.Constituents = append([]captureset.Constituent(nil), manifest.Constituents...)
			mutate(&m)
			return m, nil
		}
		assertRejectedBeforeCompute(name, &candidate)
	}
	badDigest := *source
	badDigest.Receipt.SHA256 = "sha256:" + strings.Repeat("0", 64)
	assertRejectedBeforeCompute("ASSERT_AUTHORITATIVE_DIGEST_MUTATION_BEFORE_COMPUTE", &badDigest)

	for name, path := range map[string]string{
		"ASSERT_AUTHORITATIVE_REPLICA_BACKSLASH_BEFORE_COMPUTE": ledgerWire.Rows[0].Replicas[0].Path + `\\alias`,
		"ASSERT_AUTHORITATIVE_REPLICA_TRAVERSAL_BEFORE_COMPUTE": publicationDir + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(publicationDir) + string(filepath.Separator) + filepath.FromSlash(published.Receipt.Selector),
	} {
		mut := ledgerWire
		mut.Rows = append([]programCAvailabilityRow(nil), ledgerWire.Rows...)
		mut.Rows[0].Replicas = append([]programCReplica(nil), ledgerWire.Rows[0].Replicas...)
		mut.Rows[0].Replicas[0].Path = path
		raw, _ := json.Marshal(mut)
		assertRejectedBeforeCompute(name, &AuthoritativeProgramCTransitionPlanSource{Receipt: source.Receipt, Ledger: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: raw}, ByteLength: int64(len(raw)), SHA256: transitionSHA(raw)}})
	}

	aliasRoot := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(publicationDir, aliasRoot); err != nil {
		t.Fatal(err)
	}
	aliasLedger := ledgerWire
	aliasLedger.Rows = append([]programCAvailabilityRow(nil), ledgerWire.Rows...)
	aliasLedger.Rows[0].Replicas = append([]programCReplica(nil), ledgerWire.Rows[0].Replicas...)
	aliasLedger.Rows[0].Replicas[0].Path = filepath.Join(aliasRoot, filepath.FromSlash(published.Receipt.Selector))
	aliasRaw, _ := json.Marshal(aliasLedger)
	assertRejectedBeforeCompute("ASSERT_AUTHORITATIVE_REPLICA_SYMLINK_BEFORE_COMPUTE", &AuthoritativeProgramCTransitionPlanSource{Receipt: source.Receipt, Ledger: ExactProgramCTransitionInput{Reader: &programCFixtureReader{raw: aliasRaw}, ByteLength: int64(len(aliasRaw)), SHA256: transitionSHA(aliasRaw)}})

	bundlePath := ledgerWire.Rows[0].Replicas[0].Path
	bundleRaw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	mutatedBundle := append([]byte(nil), bundleRaw...)
	mutatedBundle[len(mutatedBundle)-1] ^= 1
	if err = os.WriteFile(bundlePath, mutatedBundle, 0600); err != nil {
		t.Fatal(err)
	}
	assertRejectedBeforeCompute("ASSERT_AUTHORITATIVE_BINARY_BYTES_BEFORE_COMPUTE", &AuthoritativeProgramCTransitionPlanSource{Receipt: source.Receipt, Ledger: source.Ledger})
	if err = os.WriteFile(bundlePath, bundleRaw, 0600); err != nil {
		t.Fatal(err)
	}

	root.Close()
	root, objects, heads = openStores()
	linkedRunner := &transitionRunner{result: transitionResultBytes("STABLE")}
	linkedHeads := &uncertainProgramCHeads{inner: heads, uncertainAt: 2}
	linkedReq := ProgramCTransitionRequest{WorkflowKey: "linked", HeadKey: "linked-head", Bindings: bindings, PlanSource: source, Objects: objects, Heads: linkedHeads, Runner: linkedRunner}
	linked, err := CoordinateProgramCTransition(context.Background(), linkedReq)
	if err != nil || linked.Status != ProgramCTransitionIndeterminate {
		t.Fatalf("ASSERT_AUTHORITATIVE_LINKED_CRASH err=%v got=%+v", err, linked)
	}
	linkedCalls := len(linkedRunner.seeds)
	root.Close()
	root, objects, heads = openStores()
	linkedReq.Objects, linkedReq.Heads = objects, heads
	linkedReq.Expected = linked.Head
	recovered, err := CoordinateProgramCTransition(context.Background(), linkedReq)
	if err != nil || recovered.Status != ProgramCTransitionCommitted || len(linkedRunner.seeds) != linkedCalls {
		t.Fatalf("ASSERT_AUTHORITATIVE_LINKED_REOPEN_NO_REPLAY err=%v got=%+v", err, recovered)
	}
	root.Close()

	root, objects, heads = openStores()
	uncertainRunner := &transitionRunner{result: transitionResultBytes("STABLE")}
	uncertainHeads := &uncertainProgramCHeads{inner: heads, uncertainAt: 1}
	uncertainReq := ProgramCTransitionRequest{WorkflowKey: "unreceipted", HeadKey: "unreceipted-head", Bindings: bindings, PlanSource: source, Objects: objects, Heads: uncertainHeads, Runner: uncertainRunner}
	uncertain, err := CoordinateProgramCTransition(context.Background(), uncertainReq)
	if err != nil || uncertain.Status != ProgramCTransitionIndeterminate || len(uncertainRunner.seeds) != 0 {
		t.Fatalf("ASSERT_AUTHORITATIVE_UNRECEIPTED_INDETERMINATE err=%v got=%+v", err, uncertain)
	}
	root.Close()
	root, objects, heads = openStores()
	defer root.Close()
	uncertainReq.Objects, uncertainReq.Heads, uncertainReq.Expected = objects, heads, uncertain.Head
	still, err := CoordinateProgramCTransition(context.Background(), uncertainReq)
	if err != nil || still.Status != ProgramCTransitionIndeterminate || len(uncertainRunner.seeds) != 0 {
		t.Fatalf("ASSERT_AUTHORITATIVE_UNRECEIPTED_REOPEN_NO_REPLAY err=%v got=%+v", err, still)
	}
}
