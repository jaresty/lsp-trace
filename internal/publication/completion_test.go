package publication

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

const (
	assertCompletionExactBytes = "ASSERT_COMPLETION_PUBLISHES_EXACT_CANONICAL_BYTES"
	assertCompletionNoReplace  = "ASSERT_COMPLETION_PUBLISHES_NO_REPLACE"
	assertCompletionDigest     = "ASSERT_COMPLETION_DIGEST_OFFLINE_VERIFIABLE"
	assertCompletionLength     = "ASSERT_COMPLETION_LENGTH_OFFLINE_VERIFIABLE"
	assertCompletionSelector   = "ASSERT_COMPLETION_PRESERVES_SELECTOR"
	assertCompletionGeneration = "ASSERT_COMPLETION_PRESERVES_GENERATION"
	assertCompletionReferences = "ASSERT_COMPLETION_PRESERVES_REFERENCES"
	assertCompletionNoAlias    = "ASSERT_COMPLETION_REFERENCES_DEFENSIVE_COPY"
	assertCompletionFailure    = "ASSERT_COMPLETION_FAILURE_HAS_NO_SUCCESS"
	assertCompletionNeutral    = "ASSERT_COMPLETION_TRANSPORT_NEUTRAL"
)

func completionFixture(t *testing.T) (*Root, string, AdmittedArtifact) {
	t.Helper()
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, "nested/artifact.json", AdmittedArtifact{
		CanonicalBytes: []byte("{\"canonical\":true}\n"), ArtifactSchemaID: "artifact.v4",
		Generation: "generation-7", ArtifactReferences: []string{"graph", "receipt"},
	}
}

func completeFixture(t *testing.T) (*Root, string, AdmittedArtifact, CompletionResult) {
	t.Helper()
	root, selector, admitted := completionFixture(t)
	result := Complete(NewPublisher(), root, selector, admitted)
	if result.Failure != nil || result.Completion == nil {
		t.Fatalf("completion failed: %#v", result)
	}
	return root, selector, admitted, result
}

func TestCompletionPublishesExactCanonicalBytes(t *testing.T) {
	root, selector, admitted, _ := completeFixture(t)
	got, err := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(selector)))
	if err != nil || !bytes.Equal(got, admitted.CanonicalBytes) {
		t.Fatalf("%s: bytes=%q err=%v", assertCompletionExactBytes, got, err)
	}
}

func TestCompletionPublicationIsImmutable(t *testing.T) {
	root, selector, admitted, _ := completeFixture(t)
	before, _ := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(selector)))
	second := Complete(NewPublisher(), root, selector, admitted)
	after, _ := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(selector)))
	if second.Failure == nil || second.Failure.Code != CodeTargetExists || !bytes.Equal(before, after) {
		t.Fatalf("%s: second=%#v before=%q after=%q", assertCompletionNoReplace, second, before, after)
	}
}

func publishedFixture(t *testing.T) ([]byte, CompletionResult) {
	t.Helper()
	root, selector, _, result := completeFixture(t)
	published, err := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(selector)))
	if err != nil {
		t.Fatal(err)
	}
	return published, result
}

func TestCompletionDigestMatchesPublishedBytes(t *testing.T) {
	published, result := publishedFixture(t)
	sum := sha256.Sum256(published)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if result.Completion.Digest != want {
		t.Fatalf("%s: got=%q want=%q", assertCompletionDigest, result.Completion.Digest, want)
	}
}

func TestCompletionLengthMatchesPublishedBytes(t *testing.T) {
	published, result := publishedFixture(t)
	if result.Completion.ByteLength != uint64(len(published)) {
		t.Fatalf("%s: got=%d want=%d", assertCompletionLength, result.Completion.ByteLength, len(published))
	}
}

func TestCompletionPreservesSelector(t *testing.T) {
	_, selector, _, result := completeFixture(t)
	if result.Completion.Selector != selector {
		t.Fatalf("%s: got=%q want=%q", assertCompletionSelector, result.Completion.Selector, selector)
	}
}

func TestCompletionPreservesGeneration(t *testing.T) {
	_, _, admitted, result := completeFixture(t)
	if result.Completion.Generation != admitted.Generation {
		t.Fatalf("%s: got=%q want=%q", assertCompletionGeneration, result.Completion.Generation, admitted.Generation)
	}
}

func TestCompletionPreservesReferences(t *testing.T) {
	_, _, admitted, result := completeFixture(t)
	if !slices.Equal(result.Completion.ArtifactReferences, admitted.ArtifactReferences) {
		t.Fatalf("%s: got=%q want=%q", assertCompletionReferences, result.Completion.ArtifactReferences, admitted.ArtifactReferences)
	}
}

func TestCompletionDefensivelyCopiesReferences(t *testing.T) {
	_, _, admitted, result := completeFixture(t)
	admitted.ArtifactReferences[0] = "mutated"
	if result.Completion.ArtifactReferences[0] == "mutated" {
		t.Fatalf("%s: completion aliases admitted references", assertCompletionNoAlias)
	}
}

func TestCompletionRejectsInvalidAndDoesNotSynthesizeSuccessOnPublicationFailure(t *testing.T) {
	root, selector, admitted := completionFixture(t)
	for _, bad := range []AdmittedArtifact{{}, {CanonicalBytes: []byte("x")}} {
		result := Complete(NewPublisher(), root, selector, bad)
		if result.Completion != nil || result.Failure == nil || result.Failure.Code != CodeCompletionInputInvalid {
			t.Fatalf("%s: invalid result=%#v", assertCompletionFailure, result)
		}
	}
	if first := Complete(NewPublisher(), root, selector, admitted); first.Failure != nil {
		t.Fatal(first.Failure)
	}
	failed := Complete(NewPublisher(), root, selector, admitted)
	if failed.Completion != nil || failed.Failure == nil || !errors.Is(failed.Failure, os.ErrExist) {
		t.Fatalf("%s: failed result=%#v", assertCompletionFailure, failed)
	}
}

func TestCompletionDomainValuesAreTransportNeutral(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(AdmittedArtifact{}), reflect.TypeOf(Completion{}), reflect.TypeOf(CompletionResult{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			if name == "JSONRPC" || name == "RequestID" || name == "Envelope" || name == "Tool" {
				t.Fatalf("%s: %s.%s", assertCompletionNeutral, typ.Name(), name)
			}
		}
	}
}
