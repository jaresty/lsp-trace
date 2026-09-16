package sourceobject

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
)

const (
	assertIdentityOnlyAPI   = "ASSERT_SOURCE_OBJECT_IDENTITY_ONLY_API"
	assertContextNeutral    = "ASSERT_SOURCE_OBJECT_CONTEXT_NEUTRAL_IDEMPOTENCE"
	assertExactRoundTrip    = "ASSERT_SOURCE_OBJECT_SHA256_EXACT_BYTE_ROUND_TRIP"
	assertTypedMissing      = "ASSERT_SOURCE_OBJECT_MISSING_TYPED"
	assertTypedCorrupt      = "ASSERT_SOURCE_OBJECT_CORRUPT_TYPED"
	assertTypedLimit        = "ASSERT_SOURCE_OBJECT_LIMIT_TYPED"
	assertTypedCustody      = "ASSERT_SOURCE_OBJECT_CUSTODY_TYPED"
	assertOverflowBoundary  = "ASSERT_SOURCE_OBJECT_VERIFIED_READ_LOOKAHEAD"
	assertConcurrentPublish = "ASSERT_SOURCE_OBJECT_CONCURRENT_VERIFIED_IDEMPOTENCE"
)

func testStore(t *testing.T, max int64) (string, *Store) {
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
	store, err := New(root, max)
	if err != nil {
		t.Fatal(err)
	}
	return dir, store
}

// publishForContract keeps the guard applicable to both the rejected API and
// the repaired identity-only API. Context values are supplied only when the
// candidate still exposes a second caller-controlled argument.
func publishForContract(t *testing.T, store *Store, raw []byte, context [3]string) (Identity, error) {
	t.Helper()
	method := reflect.ValueOf(store).MethodByName("Publish")
	args := []reflect.Value{reflect.ValueOf(raw)}
	if method.Type().NumIn() == 2 {
		metadata := reflect.New(method.Type().In(1)).Elem()
		for i, name := range []string{"SchemaID", "MediaType", "Encoding"} {
			field := metadata.FieldByName(name)
			if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
				field.SetString(context[i])
			}
		}
		args = append(args, metadata)
	}
	out := method.Call(args)
	id, _ := out[0].Interface().(Identity)
	var err error
	if !out[1].IsNil() {
		err, _ = out[1].Interface().(error)
	}
	return id, err
}

func TestSourceObjectIdentityOnlyAPI(t *testing.T) {
	_, store := testStore(t, 1024)
	if got := reflect.ValueOf(store).MethodByName("Publish").Type().NumIn(); got != 1 {
		t.Fatalf("%s Publish input count=%d want=1 exact bytes only", assertIdentityOnlyAPI, got)
	}
	if _, ok := reflect.TypeOf(Object{}).FieldByName("Metadata"); ok {
		t.Fatalf("%s Object exposes caller context", assertIdentityOnlyAPI)
	}
}

func TestSourceObjectContextNeutralIdempotence(t *testing.T) {
	_, store := testStore(t, 1024)
	raw := []byte("same exact bytes")
	first, err := publishForContract(t, store, raw, [3]string{"schema/a", "text/plain", "utf-8"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := publishForContract(t, store, raw, [3]string{"schema/b", "application/octet-stream", "opaque"})
	if err != nil || second != first {
		t.Fatalf("%s first=%+v second=%+v err=%v", assertContextNeutral, first, second, err)
	}
}

func TestSourceObjectExactMalformedByteRoundTrip(t *testing.T) {
	_, store := testStore(t, 1024)
	raw := []byte{0xff, 0xfe, 0x00, '\r', '\n', 0xc3, 0x28}
	id, err := publishForContract(t, store, raw, [3]string{"ignored", "ignored", "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Digest != "sha256:fe95de587b6bd60d8665ab7eb5a325e7417cf8a9e1eec9a837b984adae05a052" || id.ByteLength != uint64(len(raw)) {
		t.Fatalf("%s identity=%+v", assertExactRoundTrip, id)
	}
	got, err := store.Get(id)
	if err != nil || got.Identity != id || !bytes.Equal(got.Bytes, raw) {
		t.Fatalf("%s got=%+v bytes=%x err=%v", assertExactRoundTrip, got, got.Bytes, err)
	}
}

func TestSourceObjectStoredFailureClassification(t *testing.T) {
	dir, store := testStore(t, 4)
	missing := Identity{Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ByteLength: 1}
	if _, err := store.Get(missing); !IsCode(err, CodeMissing) {
		t.Fatalf("%s err=%T %v", assertTypedMissing, err, err)
	}

	raw := []byte("1234")
	id, err := publishForContract(t, store, raw, [3]string{"schema", "media", "encoding"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, selector(id))
	if err := os.WriteFile(path, []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(id); !IsCode(err, CodeCorrupt) {
		t.Fatalf("%s malformed err=%T %v", assertTypedCorrupt, err, err)
	}

	// A well-formed object stored beneath the wrong identity is corruption, too.
	if err := os.WriteFile(path, encode([]byte("5678")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(id); !IsCode(err, CodeCorrupt) {
		t.Fatalf("%s identity mismatch err=%T %v", assertTypedCorrupt, err, err)
	}

	// Restore a valid object in a fresh store, then exceed the observed envelope limit.
	dir2, store2 := testStore(t, 4)
	id, err = publishForContract(t, store2, raw, [3]string{"schema", "media", "encoding"})
	if err != nil {
		t.Fatal(err)
	}
	oversized := make([]byte, store2.envelopeLimit()+1)
	if err := os.WriteFile(filepath.Join(dir2, selector(id)), oversized, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.Get(id); !IsCode(err, CodeLimit) {
		t.Fatalf("%s err=%T %v", assertTypedLimit, err, err)
	}
}

func TestSourceObjectCustodyFailuresAreTyped(t *testing.T) {
	dir, store := testStore(t, 1024)
	id := Identity{Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ByteLength: 7}
	target := filepath.Join(dir, "outside")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, selector(id))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(id); !IsCode(err, CodePolicy) {
		t.Fatalf("%s symlink err=%T %v", assertTypedCustody, err, err)
	}

	dir2, store2 := testStore(t, 1024)
	raw := []byte("hard-link protected")
	published, err := publishForContract(t, store2, raw, [3]string{"schema", "media", "encoding"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir2, selector(published)), filepath.Join(dir2, "attacker-alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.Get(published); !IsCode(err, CodePolicy) {
		t.Fatalf("%s hard-link err=%T %v", assertTypedCustody, err, err)
	}

	dir3, store3 := testStore(t, 1024)
	modeID, err := publishForContract(t, store3, []byte("mode protected"), [3]string{"schema", "media", "encoding"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir3, selector(modeID)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store3.Get(modeID); !IsCode(err, CodePolicy) {
		t.Fatalf("%s file-mode err=%T %v", assertTypedCustody, err, err)
	}
	if err := os.Chmod(dir3, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store3.Get(modeID); !IsCode(err, CodePolicy) {
		t.Fatalf("%s root-mode err=%T %v", assertTypedCustody, err, err)
	}
}

func TestSourceObjectClassificationDoesNotParseMessages(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code Code
	}{
		{"missing sentinel", os.ErrNotExist, CodeMissing},
		{"permission sentinel", os.ErrPermission, CodePermission},
		{"limit sentinel", publication.ErrBoundFileLimit, CodeLimit},
		{"policy sentinel", publication.ErrBoundFilePolicy, CodePolicy},
		{"lookalike permission text", errors.New("permission denied"), CodePolicy},
		{"lookalike limit text", errors.New("bound file limit exceeded"), CodePolicy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyStoredRead(tc.err); !IsCode(got, tc.code) {
				t.Fatalf("classification err=%q got=%T %v want=%s", tc.err, got, got, tc.code)
			}
		})
	}
}

func TestSourceObjectMaxBytesInt64Boundary(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	maxSafe := int64(math.MaxInt64) - int64(headerBytes+envelopeHashBytes) - 1
	store, err := New(root, maxSafe)
	if err != nil {
		t.Fatalf("%s max-safe rejected: %v", assertOverflowBoundary, err)
	}
	if got := store.envelopeLimit() + 1; got != int64(math.MaxInt64) {
		t.Fatalf("%s max-safe read limit=%d want=%d", assertOverflowBoundary, got, int64(math.MaxInt64))
	}
	if _, err := New(root, maxSafe+1); !IsCode(err, CodeLimit) {
		t.Fatalf("%s first-unsafe err=%T %v", assertOverflowBoundary, err, err)
	}
	t.Logf("%s exact-safe read limit=%d; first-unsafe rejected with %s", assertOverflowBoundary, int64(math.MaxInt64), CodeLimit)
}

func TestSourceObjectConcurrentIdenticalPublication(t *testing.T) {
	_, store := testStore(t, 1024)
	raw := []byte("concurrent exact bytes")
	const count = 12
	ids := make(chan Identity, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := publishForContract(t, store, raw, [3]string{"schema", "media", "encoding"})
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("%s err=%v", assertConcurrentPublish, err)
		}
	}
	var first Identity
	for id := range ids {
		if first == (Identity{}) {
			first = id
		} else if id != first {
			t.Fatalf("%s first=%+v id=%+v", assertConcurrentPublish, first, id)
		}
	}
	got, err := store.Get(first)
	if err != nil || !bytes.Equal(got.Bytes, raw) {
		t.Fatalf("%s get=%x err=%v", assertConcurrentPublish, got.Bytes, err)
	}
}

func TestSourceObjectErrorWrapping(t *testing.T) {
	err := fail(CodeMissing, errors.New("cause"))
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeMissing || typed.Unwrap() == nil {
		t.Fatalf("typed error contract: %v", err)
	}
}
