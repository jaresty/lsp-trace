package operation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	hi "lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/publication"
)

func TestInspectHydratedImmutableIngress(t *testing.T) {
	raw, err := os.ReadFile("../hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	schemaID := "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v2.schema.json"
	sum := sha256.Sum256(raw)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	generation := "g-" + hex.EncodeToString(sum[:])
	focus := hi.DefaultRequest()
	focus.NodeIDs = []string{"unknown"}

	t.Run("verified selector", func(t *testing.T) {
		root, err := publication.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		published := publication.NewPublisher().PublishVerifiedGeneration(root, raw, schemaID)
		if published.Failure != nil {
			t.Fatal(published.Failure)
		}
		p := published.Receipt
		focus.PublicationSelector = &hi.PublicationSelector{Selector: p.VerificationSelector, ArtifactDigest: p.Digest, ArtifactByteLength: p.ByteLength, ArtifactSchemaID: p.ArtifactSchemaID, PublicationMechanism: p.PublicationMechanism, Generation: p.Generation, VerificationSelector: p.VerificationSelector}
		request, _ := json.Marshal(focus)
		result, failure := InspectHydratedHandler(context.Background(), Request{Input: request, PublicationRoot: root})
		if failure != nil || !bytes.Contains(result.Artifact, []byte(`"schema_version":"lsp-trace.inspect-hydrated.v1"`)) {
			t.Fatalf("ASSERT_HYDRATED_VERIFIED_SELECTOR: failure=%v", failure)
		}
		focus.PublicationSelector.ArtifactDigest = "sha256:" + strings.Repeat("0", 64)
		request, _ = json.Marshal(focus)
		if _, failure = InspectHydratedHandler(context.Background(), Request{Input: request, PublicationRoot: root}); failure == nil || failure.Code != "DIGEST_MISMATCH" || strings.Contains(failure.Error(), root.Path()) {
			t.Fatalf("ASSERT_HYDRATED_SELECTOR_TAMPER_PRIVACY: %v", failure)
		}
		focus.PublicationSelector = nil
	})

	t.Run("content addressed", func(t *testing.T) {
		root, err := publication.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: strings.TrimPrefix(digest, "sha256:"), Bytes: raw, ArtifactSchemaID: schemaID})
		if published.Failure != nil {
			t.Fatal(published.Failure)
		}
		focus.ContentAddressedArtifact = &hi.ContentAddressedArtifact{ID: digest, ArtifactByteLength: uint64(len(raw)), ArtifactSchemaID: schemaID, Generation: generation}
		request, _ := json.Marshal(focus)
		result, failure := InspectHydratedHandler(context.Background(), Request{Input: request, ArtifactStore: root})
		if failure != nil || len(result.Artifact) == 0 {
			t.Fatalf("ASSERT_HYDRATED_CONTENT_ADDRESS: failure=%v", failure)
		}
	})
}

func TestInspectHydratedInlineCapUnchanged(t *testing.T) {
	r := hi.DefaultRequest()
	r.Input = string(bytes.Repeat([]byte(" "), (1<<20)+1))
	r.NodeIDs = []string{"unknown"}
	request, _ := json.Marshal(r)
	if _, failure := InspectHydratedHandler(context.Background(), Request{Input: request}); failure == nil || failure.Code != FailureInvalidInput {
		t.Fatalf("ASSERT_HYDRATED_INLINE_ONE_MIB_CAP: %v", failure)
	}
}
