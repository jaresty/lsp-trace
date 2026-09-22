package continuationhost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func descriptorCause(t *testing.T, err error) DescriptorPublicationCause {
	t.Helper()
	var publicationErr *DescriptorPublicationError
	if !errors.As(err, &publicationErr) {
		t.Fatalf("ASSERT_DESCRIPTOR_PRIVATE_FIXED_ENUM: err=%v", err)
	}
	return publicationErr.Cause()
}

func TestDescriptorPublicationFreshIdempotentCollisionAndCeiling(t *testing.T) {
	root, dir := testRoot(t)
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	internal := "sha256:" + strings.Repeat("a", 64) + ":1"
	input := DescriptorInput{CatalogSelector: internal, CheckpointSelector: internal, CompositeSelector: internal}
	first, err := store.PublishDescriptor(context.Background(), input)
	if err != nil || !IsPublicDescriptorSelector(first) {
		t.Fatalf("ASSERT_DESCRIPTOR_FRESH_PUBLICATION: selector=%q err=%v", first, err)
	}
	second, err := store.PublishDescriptor(context.Background(), input)
	if err != nil || second != first {
		t.Fatalf("ASSERT_DESCRIPTOR_IDENTICAL_IDEMPOTENCE: first=%q second=%q err=%v", first, second, err)
	}
	digest, _ := descriptorDigest(first)
	path := filepath.Join(dir, filepath.FromSlash(descriptorNamespace+"/"+digest+".json"))
	if err := os.WriteFile(path, []byte("conflict"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PublishDescriptor(context.Background(), input); descriptorCause(t, err) != DescriptorCauseImmutableCollision {
		t.Fatalf("ASSERT_DESCRIPTOR_CONFLICT_FAILS_CLOSED: cause=%s", descriptorCause(t, err))
	}

	limited, err := NewStore(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limited.PublishDescriptor(context.Background(), input); descriptorCause(t, err) != DescriptorCauseByteCeiling {
		t.Fatalf("ASSERT_DESCRIPTOR_BYTE_CEILING: cause=%s", descriptorCause(t, err))
	}
	if _, err := store.PublishDescriptor(context.Background(), DescriptorInput{}); descriptorCause(t, err) != DescriptorCauseCanonicalization {
		t.Fatalf("ASSERT_DESCRIPTOR_CANONICALIZATION_CAUSE: cause=%s", descriptorCause(t, err))
	}
}

func TestDescriptorPublicationClassifiesNamespaceAndRootIdentityWithoutPaths(t *testing.T) {
	t.Run("catalog namespace", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		root, err := publication.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		if err := os.Mkdir(filepath.Join(dir, "continuations"), 0o755); err != nil {
			t.Fatal(err)
		}
		store, err := NewStore(root, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		internal := "sha256:" + strings.Repeat("a", 64) + ":1"
		_, err = store.PublishDescriptor(context.Background(), DescriptorInput{internal, internal, internal})
		if got := descriptorCause(t, err); got != DescriptorCauseCatalogNamespace {
			t.Fatalf("ASSERT_DESCRIPTOR_CATALOG_NAMESPACE_CAUSE: cause=%s", got)
		}
		if strings.Contains(err.Error(), dir) {
			t.Fatal("ASSERT_DESCRIPTOR_PRIVATE_CAUSE_HIDES_PATH")
		}
	})
	t.Run("root identity", func(t *testing.T) {
		root, dir := testRoot(t)
		store, err := NewStore(root, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		moved := dir + ".moved"
		if err := os.Rename(dir, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(dir); _ = os.Rename(moved, dir) })
		internal := "sha256:" + strings.Repeat("a", 64) + ":1"
		_, err = store.PublishDescriptor(context.Background(), DescriptorInput{internal, internal, internal})
		if got := descriptorCause(t, err); got != DescriptorCauseRootIdentity {
			t.Fatalf("ASSERT_DESCRIPTOR_ROOT_IDENTITY_CAUSE: cause=%s", got)
		}
	})
}

func TestPublicDescriptorSelectorRoundTripAndRejectsSubstitution(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Chmod(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	internal := "sha256:" + strings.Repeat("a", 64) + ":1"
	public, err := store.PublishDescriptor(context.Background(), DescriptorInput{CatalogSelector: internal, CheckpointSelector: internal, CompositeSelector: internal})
	if err != nil {
		t.Fatal(err)
	}
	if !IsPublicDescriptorSelector(public) {
		t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_SELECTOR %q", public)
	}
	if _, err := store.ResolveDescriptor(context.Background(), public); err == nil || !strings.Contains(err.Error(), "checkpoint unavailable") {
		t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_UNAVAILABLE_PRECEDES_STOP_DUPLICATE err=%v", err)
	}
	distinctDuplicate, err := store.PublishDescriptor(context.Background(), DescriptorInput{CatalogSelector: internal, CheckpointSelector: internal, CompositeSelector: "sha256:" + strings.Repeat("b", 64) + ":1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveDescriptor(context.Background(), distinctDuplicate); err == nil || !strings.Contains(err.Error(), "target identity mismatch") {
		t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_DUPLICATE_TARGET_COMPARISON err=%v", err)
	}
	for _, bad := range []string{"../" + public, strings.Replace(public, "g-", "g-b", 1), internal} {
		if IsPublicDescriptorSelector(bad) {
			t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_SUBSTITUTION_REJECTED %q", bad)
		}
		if _, err := store.ResolveDescriptor(context.Background(), bad); err == nil {
			t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_TRAVERSAL_REJECTED %q", bad)
		}
	}
}
