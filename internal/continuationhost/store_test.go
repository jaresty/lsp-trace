package continuationhost

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

func testRoot(t *testing.T) (*publication.Root, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

func TestStorePersistsExactADR0007ProgramCSizeWithoutDuplicateObjects(t *testing.T) {
	root, _ := testRoot(t)
	store, err := NewStore(root, 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	const programCBytes = 24_693_581
	raw := bytes.Repeat([]byte{'x'}, programCBytes)
	capture := v5sourcesnapshotv6.CaptureResult{Raw: raw, Objects: []sourceobject.Object{{Bytes: append([]byte(nil), raw...)}}}
	encoded, err := json.Marshal(capture)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(encoded)) > store.MaxObjectBytes() {
		t.Fatalf("ASSERT_ADR0007_PROGRAM_C_CAPTURE_UNDER_64MIB encoded=%d limit=%d", len(encoded), store.MaxObjectBytes())
	}
	selector, err := store.Put(context.Background(), encoded)
	if err != nil {
		t.Fatalf("ASSERT_ADR0007_PROGRAM_C_CAPTURE_PERSISTS: %v", err)
	}
	if got, err := store.Get(context.Background(), selector); err != nil || !bytes.Equal(got, encoded) {
		t.Fatalf("ASSERT_ADR0007_PROGRAM_C_CAPTURE_REPLAYS selector=%s err=%v", selector, err)
	}
}

func TestStoreRoundTripIdempotentAndConcurrent(t *testing.T) {
	root, _ := testRoot(t)
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"stage":"catalog"}`)
	first, err := store.Put(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(context.Background(), append([]byte(nil), payload...))
	if err != nil || second != first {
		t.Fatalf("ASSERT_IDEMPOTENT_PUT first=%q second=%q err=%v", first, second, err)
	}
	got, err := store.Get(context.Background(), first)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("ASSERT_ROUND_TRIP got=%q err=%v", got, err)
	}
	got[0] ^= 1
	again, err := store.Get(context.Background(), first)
	if err != nil || !bytes.Equal(again, payload) {
		t.Fatalf("ASSERT_DEFENSIVE_GET got=%q err=%v", again, err)
	}

	const n = 16
	selectors := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s, e := store.Put(context.Background(), payload); selectors <- s; errs <- e }()
	}
	wg.Wait()
	close(selectors)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("ASSERT_CONCURRENT_PUT: %v", e)
		}
	}
	for s := range selectors {
		if s != first {
			t.Fatalf("ASSERT_CONCURRENT_SELECTOR got=%q want=%q", s, first)
		}
	}
}

func TestStoreHonors64MiBObjectCeilingBeforeWrite(t *testing.T) {
	root, dir := testRoot(t)
	const maxObjectBytes = 64 << 20
	store, err := NewStore(root, maxObjectBytes)
	if err != nil {
		t.Fatal(err)
	}

	belowLimit := bytes.Repeat([]byte{'a'}, 17_692_927)
	selector, err := store.Put(context.Background(), belowLimit)
	if err != nil {
		t.Fatalf("ASSERT_64_MIB_CONTINUATION_OBJECT_ACCEPTED: %v", err)
	}
	stored, err := store.Get(context.Background(), selector)
	if err != nil || !bytes.Equal(stored, belowLimit) {
		t.Fatalf("ASSERT_64_MIB_CONTINUATION_OBJECT_ROUND_TRIP bytes=%d err=%v", len(stored), err)
	}

	oversized := bytes.Repeat([]byte{'b'}, maxObjectBytes+1)
	oversizedPath := filepath.Join(dir, filepath.FromSlash(objectPath(strings.TrimPrefix(Digest(oversized), "sha256:"))))
	if _, err := store.Put(context.Background(), oversized); err == nil || err.Error() != "continuationhost: object rejected" {
		t.Fatalf("ASSERT_OVER_64_MIB_CONTINUATION_OBJECT_REJECTED: %v", err)
	}
	if _, err := os.Lstat(oversizedPath); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_OVER_64_MIB_CONTINUATION_OBJECT_REJECTED_BEFORE_WRITE: %v", err)
	}
}

func TestStoreRejectsUnsafeSelectorsAndCorruption(t *testing.T) {
	root, dir := testRoot(t)
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	selector, err := store.Put(context.Background(), []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../escape", "/absolute", "continuations/objects/alias", selector + "/extra", strings.ToUpper(selector)} {
		if _, err := store.Get(context.Background(), bad); err == nil {
			t.Fatalf("ASSERT_UNSAFE_SELECTOR_REJECTED %q", bad)
		}
	}
	hexDigest := strings.TrimPrefix(selector, "sha256:")
	legacySelector := CanonicalSelector([]byte("original"))
	if got, err := store.Get(context.Background(), legacySelector); err != nil || string(got) != "original" {
		t.Fatalf("ASSERT_LEGACY_LENGTH_SELECTOR_READ: got=%q err=%v", got, err)
	}
	path := filepath.Join(dir, filepath.FromSlash(objectPath(hexDigest)))
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), selector); err == nil {
		t.Fatal("ASSERT_DIGEST_LENGTH_MISMATCH_REJECTED")
	}

	other, _ := testRoot(t)
	otherStore, err := NewStore(other, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	wanted := CanonicalSelector([]byte("target"))
	linkPath := filepath.Join(other.Path(), filepath.FromSlash(wanted))
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := otherStore.Get(context.Background(), wanted); err == nil {
		t.Fatal("ASSERT_SYMLINK_REJECTED")
	}
}

func TestDescriptorIsSingleOpaquePublicRecord(t *testing.T) {
	root, dir := testRoot(t)
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	catalog, _ := store.Put(context.Background(), []byte(`{"catalog":true}`))
	checkpoint, _ := store.Put(context.Background(), []byte(`{"checkpoint":true}`))
	composite, _ := store.Put(context.Background(), []byte(`{"composite":true}`))
	descriptor, err := store.PublishDescriptor(context.Background(), DescriptorInput{CatalogSelector: catalog, CheckpointSelector: checkpoint, CompositeSelector: composite})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := descriptorDigest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := root.ReadSelector(descriptorNamespace+"/"+digest+".json", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveDescriptor(context.Background(), descriptor); err == nil || !strings.Contains(err.Error(), "catalog invalid") {
		t.Fatalf("ASSERT_PUBLIC_DESCRIPTOR_TYPED_CATALOG_REQUIRED err=%v", err)
	}
	if bytes.Contains(raw, []byte(dir)) || bytes.Contains(raw, []byte("continuations/objects")) || bytes.Contains(raw, []byte("invocation")) || bytes.Contains(raw, []byte("response")) {
		t.Fatalf("ASSERT_DESCRIPTOR_HIDES_INTERNALS %s", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 4 || decoded["schema_version"] != DescriptorSchemaVersion {
		t.Fatalf("ASSERT_SINGLE_DESCRIPTOR_SHAPE %#v", decoded)
	}
	for _, key := range []string{"catalog_selector", "checkpoint_selector", "composite_selector"} {
		value, ok := decoded[key].(string)
		if !ok || !IsCanonicalSelector(value) {
			t.Fatalf("ASSERT_OPAQUE_SELECTOR %s=%#v", key, decoded[key])
		}
	}
}
