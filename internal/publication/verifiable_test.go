package publication

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lsp-trace/internal/verification"
)

func TestPublishVerifiedGenerationProducesExactVerifierMaterial(t *testing.T) {
	dir := t.TempDir()
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	artifact := []byte("{\"schema_version\":\"fixture\"}\n")
	result := NewPublisher().PublishVerifiedGeneration(root, artifact, "fixture")
	if result.Failure != nil {
		t.Fatal(result.Failure)
	}
	if result.Receipt.Generation == "" || result.Receipt.VerificationSelector == "" || result.Receipt.PublicationMechanism != VerifiedGenerationMechanism {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_RECEIPT: %+v", result.Receipt)
	}
	selectorBytes, err := root.ReadSelector(result.Receipt.VerificationSelector, 4096)
	if err != nil {
		t.Fatal(err)
	}
	selector, err := verification.DecodeSelector(selectorBytes)
	if err != nil || selector.Generation != result.Receipt.Generation {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_SELECTOR: selector=%+v err=%v", selector, err)
	}
	publishedArtifact, err := root.ReadSelector(selector.Generation+"/artifact.json", 1<<20)
	if err != nil || !bytes.Equal(publishedArtifact, artifact) {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_ARTIFACT: equal=%v err=%v", bytes.Equal(publishedArtifact, artifact), err)
	}
	receipt, err := root.ReadSelector(selector.Generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(publishedArtifact, receipt) != nil {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_CUSTODY: err=%v verify=%v", err, verification.VerifyReceipt(publishedArtifact, receipt))
	}
	var receiptDoc map[string]any
	if err := json.Unmarshal(receipt, &receiptDoc); err != nil {
		t.Fatal(err)
	}
	wantDurability := verification.DirectoryDurabilityChecked
	if runtime.GOOS == "windows" {
		wantDurability = verification.DirectoryDurabilityUnavailable
	}
	if receiptDoc["directory_durability"] != wantDurability {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_DURABILITY: %v", receiptDoc)
	}
}

func TestPublishVerifiedGenerationReusesOnlyExactCompleteGeneration(t *testing.T) {
	dir := t.TempDir()
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := NewPublisher()
	artifact := []byte("same")
	first := publisher.PublishVerifiedGeneration(root, artifact, "fixture")
	if first.Failure != nil {
		t.Fatal(first.Failure)
	}
	second := publisher.PublishVerifiedGeneration(root, artifact, "fixture")
	if second.Failure != nil || second.Receipt.VerificationSelector != first.Receipt.VerificationSelector {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_EXACT_REUSE: first=%+v second=%+v", first, second)
	}
	if err := os.WriteFile(filepath.Join(dir, first.Receipt.Generation, "receipt.json"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	third := publisher.PublishVerifiedGeneration(root, artifact, "fixture")
	if third.Failure == nil || third.Failure.Code != CodeTargetExists {
		t.Fatalf("ASSERT_VERIFIED_GENERATION_CORRUPT_REUSE_REJECTED: %+v", third)
	}
}
