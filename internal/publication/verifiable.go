package publication

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"runtime"

	"lsp-trace/internal/verification"
)

const VerifiedGenerationMechanism = "atomic_no_replace_with_verified_generation"

// PublishVerifiedGeneration publishes a verifier-compatible immutable generation
// and exposes it only after artifact and receipt bytes have been installed. The
// caller-owned flat publication remains a separate operation.
func (p *Publisher) PublishVerifiedGeneration(root *Root, artifact []byte, artifactSchemaID string) Result {
	if p == nil || root == nil || artifact == nil || artifactSchemaID == "" {
		return failure("generation", CodePublicationFailed, true, false, errors.New("invalid verified generation input"))
	}
	sum := sha256.Sum256(artifact)
	generation := "g-" + hex.EncodeToString(sum[:])
	artifactSelector := generation + "/artifact.json"
	receiptSelector := generation + "/receipt.json"
	selectorPath := generation + ".selector.json"

	publishedArtifact := p.Publish(Request{Root: root, Selector: artifactSelector, Bytes: artifact, ArtifactSchemaID: artifactSchemaID})
	if publishedArtifact.Failure != nil {
		if publishedArtifact.Failure.Code == CodeTargetExists && existingVerifiedGeneration(root, generation, selectorPath, artifact) == nil {
			return Result{Receipt: &Receipt{
				Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(artifact)),
				ArtifactSchemaID: artifactSchemaID, PublicationMechanism: VerifiedGenerationMechanism,
				Generation: generation, VerificationSelector: selectorPath,
			}}
		}
		return publishedArtifact
	}
	checked, err := syncGeneration(root, generation)
	if err != nil {
		return failure("generation-sync", CodePublicationFailed, false, true, err)
	}
	if rootChecked, syncErr := root.SyncDirectory(); syncErr != nil {
		return failure("root-sync", CodePublicationFailed, false, true, syncErr)
	} else if !rootChecked {
		checked = false
	}
	durability := verification.DirectoryDurabilityUnavailable
	if checked {
		durability = verification.DirectoryDurabilityChecked
	}
	receiptBytes, err := verification.ReceiptBytes(artifact, durability)
	if err != nil {
		return failure("receipt", CodePublicationFailed, false, true, err)
	}
	publishedReceipt := p.Publish(Request{Root: root, Selector: receiptSelector, Bytes: receiptBytes, ArtifactSchemaID: "lsp-trace.byte-custody-receipt.v1"})
	if publishedReceipt.Failure != nil {
		return publishedReceipt
	}
	if _, err := syncGeneration(root, generation); err != nil {
		return failure("generation-sync", CodePublicationFailed, false, true, err)
	}
	selectorBytes, err := json.Marshal(verification.Selector{Generation: generation})
	if err != nil {
		return failure("selector", CodePublicationFailed, false, true, err)
	}
	selectorBytes = append(selectorBytes, '\n')
	publishedSelector := p.Publish(Request{Root: root, Selector: selectorPath, Bytes: selectorBytes, ArtifactSchemaID: "lsp-trace.generation-selector.v1"})
	if publishedSelector.Failure != nil {
		return publishedSelector
	}
	if _, err := root.SyncDirectory(); err != nil {
		return failure("root-sync", CodePublicationFailed, false, true, err)
	}
	publishedArtifact.Receipt.Generation = generation
	publishedArtifact.Receipt.VerificationSelector = selectorPath
	publishedArtifact.Receipt.PublicationMechanism = VerifiedGenerationMechanism
	return publishedArtifact
}

func existingVerifiedGeneration(root *Root, generation, selectorPath string, artifact []byte) error {
	retained, err := root.ReadSelector(generation+"/artifact.json", int64(len(artifact))+1)
	if err != nil || !bytes.Equal(retained, artifact) {
		return errors.New("existing verified generation artifact mismatch")
	}
	receipt, err := root.ReadSelector(generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(retained, receipt) != nil {
		return errors.New("existing verified generation receipt mismatch")
	}
	selector, err := root.ReadSelector(selectorPath, 4096)
	if err != nil {
		return err
	}
	decoded, err := verification.DecodeSelector(selector)
	if err != nil || decoded.Generation != generation {
		return errors.New("existing verified generation selector mismatch")
	}
	return nil
}

func syncGeneration(root *Root, generation string) (bool, error) {
	if root == nil || root.handle == nil {
		return false, errors.New("publication root is closed")
	}
	before, err := root.handle.Lstat(generation)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return false, errors.New("verified generation is not a no-follow directory")
	}
	file, err := root.handle.Open(generation)
	if err != nil {
		return false, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return false, errors.New("verified generation identity changed")
	}
	return syncDirectory(file, runtime.GOOS)
}
