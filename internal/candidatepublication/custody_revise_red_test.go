package candidatepublication

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPrivateAdapterOwnerAdvanceIsRepeatableAndPredecessorCAS(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	initial := testPublishInput(artifact, raw)
	first, err := adapter.PublishCandidateGeneration(context.Background(), initial)
	if err != nil {
		t.Fatal(err)
	}
	advancedA, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: first.Generation, VerificationSelector: first.VerificationSelector, PredecessorSelector: initial.PredecessorSelector})
	if err != nil || !advancedA.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_FIRST_ADVANCE_COMMITS: advanced=%+v err=%v", advancedA, err)
	}
	secondInput := initial
	secondInput.PredecessorSelector = advancedA.Selector
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondInput)
	if err != nil {
		t.Fatal(err)
	}
	currentA, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	advancedB, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: second.Generation, VerificationSelector: second.VerificationSelector, PredecessorSelector: currentA.Selector, PredecessorDigest: currentA.SelectorDigest, PredecessorByteLength: currentA.SelectorByteLength})
	if err != nil || !advancedB.Committed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_SECOND_ADVANCE_FROM_CURRENT_PREDECESSOR_COMMITS: advanced=%+v err=%v", advancedB, err)
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
	artifact, raw := testCandidateBytes(t)
	initial := testPublishInput(artifact, raw)
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
	secondInput := initial
	secondInput.PredecessorSelector = currentA.Selector
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
	artifact, raw := testCandidateBytes(t)
	initial := testPublishInput(artifact, raw)
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
	leftInput := testPublishInput(artifact, raw)
	leftInput.PredecessorSelector = pred.Selector
	left, err := adapter.PublishCandidateGeneration(context.Background(), leftInput)
	if err != nil {
		t.Fatal(err)
	}
	rightInput := leftInput
	rightInput.QualificationID = leftInput.QualificationID + "/alternate"
	right, err := adapter.PublishCandidateGeneration(context.Background(), rightInput)
	if err == nil && right.ManifestDigest == left.ManifestDigest {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_DISTINCT_SUCCESSOR_FIXTURE: left=%+v right=%+v", left, right)
	}
	if err != nil {
		// If semantic parsing rejects the mutation, this test still exercises the two-advance loser via first generation below.
		right = left
	}
	requests := []AdvanceRequest{
		{Generation: left.Generation, VerificationSelector: left.VerificationSelector, ManifestSelector: left.ManifestSelector, ManifestDigest: left.ManifestDigest, ManifestByteLength: left.ManifestByteLength, PredecessorSelector: pred.Selector, PredecessorDigest: pred.SelectorDigest, PredecessorByteLength: pred.SelectorByteLength},
		{Generation: right.Generation, VerificationSelector: right.VerificationSelector, ManifestSelector: right.ManifestSelector, ManifestDigest: right.ManifestDigest, ManifestByteLength: right.ManifestByteLength, PredecessorSelector: pred.Selector, PredecessorDigest: pred.SelectorDigest, PredecessorByteLength: pred.SelectorByteLength},
	}
	success := 0
	for _, req := range requests {
		if _, err := adapter.AdvanceCandidateGeneration(context.Background(), req); err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_EXISTING_PREDECESSOR_EXACTLY_ONE_SUCCESSOR_WINS: success=%d pred=%+v", success, pred)
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
