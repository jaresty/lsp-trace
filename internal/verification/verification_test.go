package verification

import (
	"strings"
	"testing"
)

func TestSelectorReceiptAndVerificationSemantics(t *testing.T) {
	artifact := []byte("exact artifact bytes\n")

	t.Run("strict selector", func(t *testing.T) {
		sel, err := DecodeSelector([]byte("{\"generation\":\"bundle.generation-1\"}\n"))
		if err != nil || sel.Generation != "bundle.generation-1" {
			t.Fatalf("ASSERT_SELECTOR_VALID: selector=%#v err=%v", sel, err)
		}
		for _, input := range []string{
			`{"generation":""}`,
			`{"generation":"../escape"}`,
			`{"generation":"g","extra":true}`,
			`{"generation":"g"}{"generation":"second"}`,
		} {
			if _, err := DecodeSelector([]byte(input)); err == nil || !strings.Contains(err.Error(), "malformed generation selector") {
				t.Fatalf("ASSERT_SELECTOR_STRICT: input=%q err=%v", input, err)
			}
		}
	})

	t.Run("deterministic narrow receipt", func(t *testing.T) {
		first, err := ReceiptBytes(artifact, DirectoryDurabilityChecked)
		if err != nil {
			t.Fatalf("ASSERT_RECEIPT_BUILD: %v", err)
		}
		second, err := ReceiptBytes(artifact, DirectoryDurabilityChecked)
		if err != nil || string(first) != string(second) {
			t.Fatalf("ASSERT_RECEIPT_DETERMINISTIC: first=%q second=%q err=%v", first, second, err)
		}
		for _, field := range []string{
			`"receipt_version":"lsp-trace.byte-custody-receipt.v1"`,
			`"digest_algorithm":"SHA-256"`,
			`"digest_scope":"EXACT_SERIALIZED_OUTPUT_BYTES"`,
			`"integrity_claim":"INTEGRITY_AND_CUSTODY_ONLY"`,
			`"authenticity_claim":false`,
			`"directory_durability":"CHECKED"`,
		} {
			if !strings.Contains(string(first), field) {
				t.Fatalf("ASSERT_RECEIPT_NARROW_METADATA: missing=%s receipt=%s", field, first)
			}
		}
		if err := VerifyReceipt(artifact, first); err != nil {
			t.Fatalf("ASSERT_RECEIPT_VALID: %v", err)
		}
	})
}

func TestVerifyReceiptRejectsMalformedMetadataAndDigest(t *testing.T) {
	artifact := []byte("exact artifact bytes\n")
	receipt, err := ReceiptBytes(artifact, DirectoryDurabilityUnavailable)
	if err != nil {
		t.Fatalf("ASSERT_RECEIPT_FIXTURE: %v", err)
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"trailing JSON", append(append([]byte(nil), receipt...), []byte(`{"second":true}`)...), "malformed: trailing JSON content"},
		{"unknown field", []byte(`{"receipt_version":"lsp-trace.byte-custody-receipt.v1","exact_serialized_bytes_digest":"sha256:00","digest_algorithm":"SHA-256","digest_scope":"EXACT_SERIALIZED_OUTPUT_BYTES","integrity_claim":"INTEGRITY_AND_CUSTODY_ONLY","authenticity_claim":false,"directory_durability":"CHECKED","extra":true}`), "malformed:"},
		{"metadata authority", []byte(strings.Replace(string(receipt), "INTEGRITY_AND_CUSTODY_ONLY", "AUTHENTIC", 1)), "receipt metadata mismatch"},
		{"exact bytes", receipt, "exact-byte integrity mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputArtifact := artifact
			if tc.name == "exact bytes" {
				inputArtifact = []byte("tampered\n")
			}
			err := VerifyReceipt(inputArtifact, tc.data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ASSERT_RECEIPT_REJECT_%s: err=%v want=%q", tc.name, err, tc.want)
			}
		})
	}
}
