package candidatepublication

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programctestfixture"
	"lsp-trace/internal/publication"
)

const (
	testCandidateMemberA   = "3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c"
	testCandidateMemberB   = "4a4c524d563487befdff3720d59fb28740155453ebe6625b782f7612ef9e445f"
	testCandidateSingleton = "020b42e6f4356621d0dbfd50da9ee760ede90980ce036960fd05dafa7113c80b"
)

func testBuiltCandidateBytes(t testing.TB, seed uint64, members []string, representative string, v2 bool) (censuscontinuation.CandidateGroupArtifact, []byte) {
	t.Helper()
	outcome, failure := programc.Compute(programctestfixture.ValidV5(t), seed)
	if failure != nil {
		t.Fatal(failure)
	}
	boundary, err := programc.ComputeBoundary(outcome, programc.BoundaryRequest{PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	profile := censuscontinuation.FrozenCandidateGroupProfile()
	if v2 {
		profile = censuscontinuation.CandidateGroupPrivateV2Profile()
	}
	artifact, err := censuscontinuation.BuildCandidateGroup(censuscontinuation.CandidateGroupInput{Outcome: outcome, Boundary: boundary, CommunityMembers: members, RepresentativeID: representative, Bounds: profile.Bounds, ResourceProfile: profile, Interpretation: censuscontinuation.HostInterpretation{Status: censuscontinuation.InterpretationUnresolved, HostID: "caller-host", ModelID: "caller-model", ContextID: "caller-context", Attempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := artifact.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return artifact, raw
}

func testDistinctPublishInput(t testing.TB, predecessor string, variant string) PublishRequest {
	t.Helper()
	var artifact censuscontinuation.CandidateGroupArtifact
	var raw []byte
	switch variant {
	case "pair":
		artifact, raw = testBuiltCandidateBytes(t, 19, []string{testCandidateMemberA, testCandidateMemberB}, testCandidateMemberA, false)
	case "singleton":
		artifact, raw = testBuiltCandidateBytes(t, 19, []string{testCandidateSingleton}, testCandidateSingleton, false)
	case "pair-v2":
		artifact, raw = testBuiltCandidateBytes(t, 19, []string{testCandidateMemberA, testCandidateMemberB}, testCandidateMemberA, true)
	default:
		t.Fatalf("unknown candidate variant %q", variant)
	}
	req := testPublishInput(artifact, raw)
	req.PredecessorSelector = predecessor
	return req
}

func TestPrivateAdapterOwnerAdvanceIsRepeatableAndPredecessorCAS(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	initial := testDistinctPublishInput(t, "g-0000000000000000000000000000000000000000000000000000000000000000.selector.json", "pair")
	first, err := adapter.PublishCandidateGeneration(context.Background(), initial)
	if err != nil {
		t.Fatal(err)
	}
	advancedA, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: first.Generation, VerificationSelector: first.VerificationSelector, PredecessorSelector: initial.PredecessorSelector})
	if err != nil || !advancedA.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_FIRST_ADVANCE_COMMITS: advanced=%+v err=%v", advancedA, err)
	}
	currentA, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := testDistinctPublishInput(t, advancedA.Selector, "singleton")
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: second.Generation, VerificationSelector: second.VerificationSelector, PredecessorSelector: currentA.Selector}); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_REJECTS_SELECTOR_ONLY_PREDECESSOR_TOKEN")
	}
	advancedB, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: second.Generation, VerificationSelector: second.VerificationSelector, PredecessorSelector: currentA.Selector, PredecessorDigest: currentA.SelectorDigest, PredecessorByteLength: currentA.SelectorByteLength})
	if err != nil || !advancedB.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_ACCEPTS_EXACT_CALLER_HELD_PREDECESSOR_TOKEN: advanced=%+v err=%v current=%+v", advancedB, err, currentA)
	}
	if _, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: first.Generation, VerificationSelector: first.VerificationSelector, PredecessorSelector: initial.PredecessorSelector}); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_STALE_PREDECESSOR_LOSES")
	}
}

func TestPrivateAdapterConcurrentAdvanceHasOneWinnerPerPredecessor(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	const contenders = 8
	var wg sync.WaitGroup
	results := make(chan error, contenders)
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: input.PredecessorSelector})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_CONCURRENT_CAS_ONE_WINNER: successes=%d", success)
	}
}

func TestPrivateAdapterQualificationReceiptIsItselfVerifiedGenerationAndBoundByCurrent(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if published.QualificationReceiptVerificationSelector == "" || published.QualificationReceiptDigest == "" || published.QualificationReceiptByteLength == 0 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_QUALIFICATION_RECEIPT_VERIFIED_GENERATION_IDENTITY: %+v", published)
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: input.PredecessorSelector})
	if err != nil {
		t.Fatal(err)
	}
	current, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current.QualificationReceiptVerificationSelector != published.QualificationReceiptVerificationSelector || current.QualificationReceiptDigest != published.QualificationReceiptDigest || current.QualificationReceiptByteLength != published.QualificationReceiptByteLength || advanced.QualificationReceiptVerificationSelector == "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_CURRENT_BINDS_QUALIFICATION_VERIFIED_RECEIPT: current=%+v published=%+v advanced=%+v", current, published, advanced)
	}
}

func TestPrivateAdapterCanonicalQualificationReceiptFieldTamperMatrix(t *testing.T) {
	fields := []string{"candidate_digest", "candidate_byte_length", "grouping_policy_id", "grouping_policy_digest", "qualification_id", "source_revision", "generation", "predecessor_selector", "representative", "authority", "accepted", "completeness", "feature_identity_status"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			root, dir := testPrivateRoot(t)
			adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			artifact, raw := testCandidateBytes(t)
			input := testPublishInput(artifact, raw)
			published, err := adapter.PublishCandidateGeneration(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			receiptPath := filepath.Join(dir, filepath.FromSlash(published.QualificationReceiptSelector))
			receiptRaw, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(receiptRaw, &doc); err != nil {
				t.Fatal(err)
			}
			doc[field] = tamperedValue(doc[field])
			tampered, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(receiptPath, append(tampered, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.GetCandidateGeneration(context.Background(), published.Generation); err == nil {
				t.Fatalf("ASSERT_PRIVATE_ADAPTER_RECEIPT_FIELD_TAMPER_REJECTED: field=%s", field)
			}
		})
	}
}

func tamperedValue(v any) any {
	switch x := v.(type) {
	case string:
		return x + "-tampered"
	case float64:
		return x + 1
	case bool:
		return !x
	case map[string]any:
		x["inferred"] = true
		return x
	default:
		return "tampered"
	}
}

func TestPrivateAdapterAdvanceRequiresCallerHeldExactPredecessorToken(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	initial := testDistinctPublishInput(t, "g-0000000000000000000000000000000000000000000000000000000000000000.selector.json", "pair")
	first, err := adapter.PublishCandidateGeneration(context.Background(), initial)
	if err != nil {
		t.Fatal(err)
	}
	advancedA, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: first.Generation, VerificationSelector: first.VerificationSelector, PredecessorAbsent: true})
	if err != nil || !advancedA.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_INITIAL_ABSENT_PREDECESSOR_COMMITS: advanced=%+v err=%v", advancedA, err)
	}
	currentA, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := testDistinctPublishInput(t, currentA.Selector, "singleton")
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: second.Generation, VerificationSelector: second.VerificationSelector, PredecessorSelector: currentA.Selector}); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_REJECTS_SELECTOR_ONLY_PREDECESSOR_TOKEN")
	}
	advancedB, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: second.Generation, VerificationSelector: second.VerificationSelector, PredecessorSelector: currentA.Selector, PredecessorDigest: currentA.SelectorDigest, PredecessorByteLength: currentA.SelectorByteLength})
	if err != nil || !advancedB.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_ACCEPTS_EXACT_CALLER_HELD_PREDECESSOR_TOKEN: advanced=%+v err=%v current=%+v", advancedB, err, currentA)
	}
}

func TestPrivateAdapterDistinctSuccessorsFromExistingPredecessorExactlyOneWinner(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	initial := testDistinctPublishInput(t, "g-0000000000000000000000000000000000000000000000000000000000000000.selector.json", "pair")
	first, err := adapter.PublishCandidateGeneration(context.Background(), initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: first.Generation, VerificationSelector: first.VerificationSelector, PredecessorAbsent: true}); err != nil {
		t.Fatal(err)
	}
	pred, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	leftInput := testDistinctPublishInput(t, pred.Selector, "singleton")
	left, err := adapter.PublishCandidateGeneration(context.Background(), leftInput)
	if err != nil {
		t.Fatal(err)
	}
	rightInput := testDistinctPublishInput(t, pred.Selector, "pair-v2")
	right, err := adapter.PublishCandidateGeneration(context.Background(), rightInput)
	if err != nil {
		t.Fatal(err)
	}
	requests := []AdvanceRequest{
		{Generation: left.Generation, VerificationSelector: left.VerificationSelector, ManifestSelector: left.ManifestSelector, ManifestDigest: left.ManifestDigest, ManifestByteLength: left.ManifestByteLength, PredecessorSelector: pred.Selector, PredecessorDigest: pred.SelectorDigest, PredecessorByteLength: pred.SelectorByteLength},
		{Generation: right.Generation, VerificationSelector: right.VerificationSelector, ManifestSelector: right.ManifestSelector, ManifestDigest: right.ManifestDigest, ManifestByteLength: right.ManifestByteLength, PredecessorSelector: pred.Selector, PredecessorDigest: pred.SelectorDigest, PredecessorByteLength: pred.SelectorByteLength},
	}
	success := 0
	var winner PublishedGeneration
	for _, req := range requests {
		advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), req)
		if err == nil {
			success++
			winner = advanced
			continue
		}
		if !errors.Is(err, publication.ErrStalePredecessor) {
			t.Fatalf("ASSERT_PRIVATE_ADAPTER_EXISTING_PREDECESSOR_LOSER_IS_STALE: err=%v", err)
		}
	}
	if success != 1 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_EXISTING_PREDECESSOR_EXACTLY_ONE_SUCCESSOR_WINS: success=%d pred=%+v", success, pred)
	}
	current, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current.Generation != winner.Generation {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_CURRENT_MATCHES_WINNER: current=%+v winner=%+v", current, winner)
	}
}

func TestPrivateAdapterCurrentSelectorBindsVerifiedManifestIdentity(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if published.ManifestSelector == "" || published.ManifestVerificationSelector == "" || published.ManifestDigest == "" || published.ManifestByteLength == 0 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PUBLISH_EXPOSES_VERIFIED_MANIFEST_IDENTITY: %+v", published)
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, ManifestSelector: published.ManifestSelector, ManifestDigest: published.ManifestDigest, ManifestByteLength: published.ManifestByteLength, PredecessorAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	current, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current.ManifestSelector != published.ManifestSelector || current.ManifestVerificationSelector != published.ManifestVerificationSelector || current.ManifestDigest != published.ManifestDigest || current.ManifestByteLength != published.ManifestByteLength || advanced.ManifestSelector != published.ManifestSelector {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_CURRENT_BINDS_MANIFEST_IDENTITY: published=%+v advanced=%+v current=%+v", published, advanced, current)
	}
}

func TestPrivateAdapterCurrentSelectorStrictSemanticValidation(t *testing.T) {
	root, dir := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"selector":"../escape","generation":"G-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","verification_selector":"../escape","qualification_receipt_selector":"../escape"}` + "\n")
	currentPath := filepath.Join(dir, filepath.FromSlash(currentSelector))
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte(strings.ReplaceAll(string(bad), `\"`, `"`)), 0o600); err != nil {
		t.Fatal(err)
	}
	if current, err := adapter.CurrentCandidateGeneration(context.Background()); err == nil || current.Generation != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_MALFORMED_CURRENT_FAILS_CLOSED: current=%+v err=%v", current, err)
	}
}
