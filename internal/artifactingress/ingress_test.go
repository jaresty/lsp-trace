package artifactingress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

const testSchema = "https://example.invalid/artifact.v1.schema.json"

func artifact(size int) []byte {
	prefix := `{"schema":"` + testSchema + `","padding":"`
	suffix := `"}`
	if size < len(prefix)+len(suffix) {
		size = len(prefix) + len(suffix)
	}
	return []byte(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
}
func validate(schema string, raw []byte) error {
	if schema != testSchema {
		return errors.New("wrong schema id")
	}
	var doc struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Schema != schema {
		return errors.New("semantic schema mismatch")
	}
	return nil
}
func digest(raw []byte) string { s := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(s[:]) }
func code(t *testing.T, err error, want FailureCode) {
	t.Helper()
	var f *Failure
	if !errors.As(err, &f) || f.Code != want {
		t.Fatalf("failure=%v want=%s", err, want)
	}
}
func config(max int64) Config { return Config{MaxBytes: max, Validate: validate} }

func TestInlineExactBoundary(t *testing.T) {
	c := config(8 << 20)
	for _, tc := range []struct {
		name string
		n    int
		want FailureCode
	}{{"equality", InlineMaxBytes, ""}, {"plus-one", InlineMaxBytes + 1, TooLarge}} {
		t.Run(tc.name, func(t *testing.T) {
			raw := artifact(tc.n)
			got, err := c.Inline(raw, testSchema, "g-inline")
			if tc.want != "" {
				code(t, err, tc.want)
				return
			}
			if err != nil || !bytes.Equal(got.Bytes, raw) {
				t.Fatalf("ASSERT_INLINE_BOUNDARY result=FAIL err=%v", err)
			}
		})
	}
}

func TestVerifiedSelectorIngressAndTampering(t *testing.T) {
	dir := t.TempDir()
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw := artifact(2 << 20)
	published := publication.NewPublisher().PublishVerifiedGeneration(root, raw, testSchema)
	if published.Failure != nil {
		t.Fatal(published.Failure)
	}
	c := config(7 << 20)
	c.PublicationRoot = root
	expected := Expected{SchemaID: testSchema, ByteLength: uint64(len(raw)), Generation: published.Receipt.Generation}
	got, err := c.FromSelector(SelectorRequest{Selector: published.Receipt.VerificationSelector, Receipt: *published.Receipt, Expected: expected})
	if err != nil || !bytes.Equal(got.Bytes, raw) || got.Evidence.Generation != published.Receipt.Generation || got.Evidence.SchemaID != testSchema {
		t.Fatalf("ASSERT_SELECTOR_VALID result=FAIL err=%v evidence=%+v", err, got.Evidence)
	}
	cases := []struct {
		name   string
		mutate func(*SelectorRequest)
		want   FailureCode
	}{
		{"selector", func(r *SelectorRequest) { r.Selector = "missing.selector.json" }, SelectorInvalid},
		{"receipt-digest", func(r *SelectorRequest) { r.Receipt.Digest = "sha256:" + strings.Repeat("0", 64) }, DigestMismatch},
		{"receipt-length", func(r *SelectorRequest) { r.Receipt.ByteLength++ }, CustodyFailed},
		{"receipt-schema", func(r *SelectorRequest) { r.Receipt.ArtifactSchemaID = "wrong" }, SchemaMismatch},
		{"receipt-generation", func(r *SelectorRequest) { r.Receipt.Generation = "g-" + strings.Repeat("0", 64) }, SelectorInvalid},
		{"receipt-mechanism", func(r *SelectorRequest) { r.Receipt.PublicationMechanism = "replaceable" }, SelectorInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := SelectorRequest{Selector: published.Receipt.VerificationSelector, Receipt: *published.Receipt, Expected: expected}
			tc.mutate(&r)
			_, e := c.FromSelector(r)
			code(t, e, tc.want)
		})
	}
	selectorPath := filepath.Join(dir, published.Receipt.VerificationSelector)
	selectorBytes, err := os.ReadFile(selectorPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selectorPath, []byte(`{"generation":"tampered"}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = c.FromSelector(SelectorRequest{Selector: published.Receipt.VerificationSelector, Receipt: *published.Receipt, Expected: expected})
	code(t, err, SelectorInvalid)
	if err := os.WriteFile(selectorPath, selectorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(dir, published.Receipt.Generation, "receipt.json")
	receiptBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = c.FromSelector(SelectorRequest{Selector: published.Receipt.VerificationSelector, Receipt: *published.Receipt, Expected: expected})
	code(t, err, CustodyFailed)
	if err := os.WriteFile(receiptPath, receiptBytes, 0600); err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-2] = 'y'
	if err := os.WriteFile(filepath.Join(dir, published.Receipt.Generation, "artifact.json"), tampered, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = c.FromSelector(SelectorRequest{Selector: published.Receipt.VerificationSelector, Receipt: *published.Receipt, Expected: expected})
	code(t, err, CustodyFailed)
}

func TestContentAddressedIngress(t *testing.T) {
	dir := t.TempDir()
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw := artifact(3 << 20)
	id := digest(raw)
	hexID := strings.TrimPrefix(id, "sha256:")
	p := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: hexID, Bytes: raw, ArtifactSchemaID: testSchema})
	if p.Failure != nil {
		t.Fatal(p.Failure)
	}
	c := config(7 << 20)
	c.ArtifactStore = root
	got, err := c.FromContent(ContentRequest{ID: id, Expected: Expected{SchemaID: testSchema, ByteLength: uint64(len(raw)), Generation: "store-v1"}})
	if err != nil || !bytes.Equal(got.Bytes, raw) || got.Evidence.SafeSelector != id {
		t.Fatalf("ASSERT_CONTENT_VALID result=FAIL err=%v", err)
	}
	_, err = c.FromContent(ContentRequest{ID: "SHA256:" + hexID, Expected: Expected{SchemaID: testSchema, ByteLength: uint64(len(raw))}})
	code(t, err, SelectorInvalid)
	wrong := "sha256:" + strings.Repeat("0", 64)
	if wrong == id {
		t.Fatal("fixture")
	}
	p = publication.NewPublisher().Publish(publication.Request{Root: root, Selector: strings.TrimPrefix(wrong, "sha256:"), Bytes: raw, ArtifactSchemaID: testSchema})
	if p.Failure != nil {
		t.Fatal(p.Failure)
	}
	_, err = c.FromContent(ContentRequest{ID: wrong, Expected: Expected{SchemaID: testSchema, ByteLength: uint64(len(raw))}})
	code(t, err, DigestMismatch)
}

func TestPrivatePathDisabledUnsafeAndNonRegular(t *testing.T) {
	c := config(7 << 20)
	_, err := c.FromPrivatePath(PrivatePathRequest{Selector: "artifact.json"})
	code(t, err, PathDisabled)
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c.PrivatePaths = true
	c.PrivateRoot = root
	for _, name := range []string{"../escape", "/absolute", "a/../../b", "a\\b"} {
		t.Run(name, func(t *testing.T) {
			_, e := c.FromPrivatePath(PrivatePathRequest{Selector: name})
			code(t, e, PathUnsafe)
		})
	}
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = c.FromPrivatePath(PrivatePathRequest{Selector: "directory"})
	code(t, err, NotRegular)
	if err := os.WriteFile(filepath.Join(dir, "target"), artifact(100), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	_, err = c.FromPrivatePath(PrivatePathRequest{Selector: "link"})
	code(t, err, PathUnsafe)
	if runtime.GOOS != "windows" {
		fifo := filepath.Join(dir, "fifo")
		if err := makeFIFO(fifo); err != nil {
			t.Fatal(err)
		}
		_, err = c.FromPrivatePath(PrivatePathRequest{Selector: "fifo"})
		code(t, err, NotRegular)
	}
}

func TestPrivatePathLargeSuccessNoLeakAndHardLinkRejected(t *testing.T) {
	dir := t.TempDir()
	privateNeedle := filepath.Join(dir, "private-root-secret")
	if err := os.Mkdir(privateNeedle, 0700); err != nil {
		t.Fatal(err)
	}
	raw := artifact(7 << 20)
	file := filepath.Join(privateNeedle, "artifact.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(privateNeedle)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c := config(8 << 20)
	c.PrivatePaths = true
	c.PrivateRoot = root
	req := PrivatePathRequest{Selector: "artifact.json", Expected: Expected{SchemaID: testSchema, ByteLength: uint64(len(raw)), Generation: "private-v1"}, Digest: digest(raw)}
	got, err := c.FromPrivatePath(req)
	if err != nil || !bytes.Equal(got.Bytes, raw) || got.Evidence.PrivateDiagnosticsToken == "" {
		t.Fatalf("ASSERT_PRIVATE_LARGE result=FAIL err=%v", err)
	}
	encoded, _ := json.Marshal(got.Evidence)
	if bytes.Contains(encoded, []byte(privateNeedle)) {
		t.Fatal("ASSERT_PRIVATE_PATH_LEAK")
	}
	_, missingErr := c.FromPrivatePath(PrivatePathRequest{Selector: "missing.json", Expected: req.Expected, Digest: req.Digest})
	if strings.Contains(missingErr.Error(), privateNeedle) || strings.Contains(missingErr.Error(), file) {
		t.Fatal("ASSERT_PRIVATE_FAILURE_PATH_LEAK")
	}
	if runtime.GOOS != "windows" {
		if err := os.Link(file, filepath.Join(privateNeedle, "alias")); err != nil {
			t.Fatal(err)
		}
		_, err = c.FromPrivatePath(req)
		code(t, err, CustodyFailed)
	}
}

func TestPrivatePathReplacementRace(t *testing.T) {
	dir := t.TempDir()
	raw := artifact(2 << 20)
	name := filepath.Join(dir, "artifact.json")
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c := config(7 << 20)
	c.PrivatePaths = true
	c.PrivateRoot = root
	privateReadHook = func() {
		replacement := filepath.Join(dir, "replacement.json")
		if err := os.WriteFile(replacement, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, name); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { privateReadHook = nil }()
	_, err = c.FromPrivatePath(PrivatePathRequest{Selector: "artifact.json", Expected: Expected{SchemaID: testSchema, ByteLength: uint64(len(raw))}, Digest: digest(raw)})
	code(t, err, PathReplaced)
}

func TestExplicitLargeBound(t *testing.T) {
	for _, max := range []int64{0, InlineMaxBytes, HydrationCoreMaxBytes + 1} {
		c := Config{MaxBytes: max, Validate: validate}
		_, err := c.FromContent(ContentRequest{})
		code(t, err, TooLarge)
	}
}
