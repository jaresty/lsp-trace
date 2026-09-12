package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
	publicschema "lsp-trace/internal/schema"
)

func TestExactBytesAuthorityAssignsAndVerifiesCanonicalConstituent(t *testing.T) {
	raw := []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)
	a := ExactBytesAuthority{AdmitGraphProvenanceV5: func(got []byte) (string, error) {
		if !bytes.Equal(got, raw) {
			t.Fatal("ASSERT_AUTHORITY_EXACT_BYTES")
		}
		return "native-v5-id", nil
	}}
	c, err := a.Constituent(raw)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])
	if c.ImmutableSelector != "graph-provenance-v5/sha256/"+hexsum || c.SHA256 != "sha256:"+hexsum || c.ByteLength != len(raw) || c.SchemaID != NativeV5SchemaID {
		t.Fatalf("ASSERT_CANONICAL_CONSTITUENT: %+v", c)
	}
	if err := a.VerifyConstituent(c, raw); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.ImmutableSelector = "caller/chosen"
	if err := a.VerifyConstituent(bad, raw); err == nil {
		t.Fatal("ASSERT_AUTHORITY_REJECTS_CALLER_SELECTOR")
	}
	bad = c
	bad.NativeV5Identity = "caller-forged-id"
	if err := a.VerifyConstituent(bad, raw); err == nil {
		t.Fatal("ASSERT_AUTHORITY_REJECTS_CALLER_NATIVE_IDENTITY")
	}
	if _, err := (ExactBytesAuthority{AdmitGraphProvenanceV5: func([]byte) (string, error) { return "", nil }}).Constituent(raw); err == nil {
		t.Fatal("ASSERT_AUTHORITY_REJECTS_EMPTY_DERIVED_IDENTITY")
	}
}

func TestRedactAppliesExactPolicyAndDistinctIdentity(t *testing.T) {
	ts, cs, f, s := fixture(64)
	m, err := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Redact(m)
	if err != nil {
		t.Fatal(err)
	}
	if r.Disclosure != "REDACTED" || r.LogicalDigest == m.LogicalDigest || r.ImmutableSelector == m.ImmutableSelector {
		t.Fatal("ASSERT_REDACTED_DISTINCT_IDENTITY")
	}
	if r.Targets[0].CanonicalSeedV2 != RedactedValue+":00000000" || r.FileLedger.Entries[0].Identity != RedactedValue+":00000000" || r.SymbolLedger.Entries[0].Identity != RedactedValue+":00000000" || r.Constituents[0].ImmutableSelector != RedactedValue+":00000000" || r.Constituents[0].NativeV5Identity != RedactedValue+":00000000" {
		t.Fatal("ASSERT_EXACT_REDACTION_FIELDS")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeSingleCaptureCustody || len(r.CrossCaptureCalls) != 0 || r.LeidenAdmissible {
		t.Fatal("ASSERT_REDACTION_PRESERVES_CEILING")
	}
	if r.CensusPolicy != m.CensusPolicy || r.DuplicatePolicy != m.DuplicatePolicy || r.Constituents[0].SHA256 != m.Constituents[0].SHA256 || r.Batches[0] != m.Batches[0] {
		t.Fatal("ASSERT_REDACTION_NO_EXTRA_FIELDS")
	}
}

func TestCaptureSetRemainsOutsidePublicSchemaRegistry(t *testing.T) {
	if _, ok := publicschema.RegisteredFamilies()["capture-set"]; ok {
		t.Fatal("ASSERT_CAPTURE_SET_NOT_PUBLICLY_REGISTERED")
	}
}

func TestValidatePublicationSelectorRejectsMalformedBeforeVerification(t *testing.T) {
	valid := CaptureSetSelectorPrefix + strings.Repeat("a", 64) + "/manifest.json"
	if err := ValidatePublicationSelector(valid); err != nil {
		t.Fatalf("ASSERT_CANONICAL_CAPTURE_SET_SELECTOR: %v", err)
	}
	for _, selector := range []string{
		"../" + valid,
		CaptureSetSelectorPrefix + strings.Repeat("A", 64) + "/manifest.json",
		CaptureSetSelectorPrefix + strings.Repeat("a", 63) + "/manifest.json",
		CaptureSetSelectorPrefix + strings.Repeat("z", 64) + "/manifest.json",
	} {
		if err := ValidatePublicationSelector(selector); err == nil {
			t.Fatalf("ASSERT_MALFORMED_CAPTURE_SET_SELECTOR_REJECTED: %q", selector)
		}
	}
}

func TestPublishPrivateRaceSafeAndVerifiable(t *testing.T) {
	ts, cs, f, s := fixture(64)
	m, _ := Prepare(ts, cs, f, s, "census.v1", "retain-exact.v1")
	root, err := publication.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	pub := NewPublisher(root)
	var wg sync.WaitGroup
	results := make(chan PublicationResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- pub.Publish(m) }()
	}
	wg.Wait()
	close(results)
	var successes, exists int
	var receipt PublicationReceipt
	for result := range results {
		if result.Err == nil {
			successes++
			receipt = *result.Receipt
		} else if result.Code == publication.CodeTargetExists {
			exists++
		}
	}
	if successes != 1 || exists != 1 {
		t.Fatalf("ASSERT_RACE_SAFE_CREATE: success=%d exists=%d", successes, exists)
	}
	if receipt.Disclosure != "PRIVATE" || receipt.Selector != CaptureSetPublicationSelector(m) {
		t.Fatalf("ASSERT_PRIVATE_RECEIPT: %+v", receipt)
	}
	got, err := pub.Verify(receipt.Selector)
	if err != nil {
		t.Fatal(err)
	}
	if got.LogicalDigest != m.LogicalDigest || got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.NativeSingleCaptureCustody || len(got.CrossCaptureCalls) != 0 || got.LeidenAdmissible {
		t.Fatal("ASSERT_PUBLICATION_VERIFY_CEILING")
	}
}
