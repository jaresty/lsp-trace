package schema

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const validTrustProvisioningReceiptV1 = `{
  "trust_provisioning_receipt_schema_version":"lsp-trace.trust-provisioning-receipt.v1",
  "receipt_id":"trust-receipt:production-root-2026-09",
  "trust_policy_id":"trust-policy:production-v4",
  "anchor_type":"ED25519_PUBLIC_KEY",
  "anchor_identity":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "provisioning_authority":"security-operations",
  "provisioning_channel":"VERIFIER_TRUST_STORE",
  "provisioning_event":"trust-store:v4",
  "verification_method":"ED25519_SIGNATURE",
  "verification_result":"VERIFIED"
}`

func validTrustAuthenticationContext() TrustAuthenticationContext {
	return TrustAuthenticationContext{
		ReceiptID:             "trust-receipt:production-root-2026-09",
		TrustPolicyID:         "trust-policy:production-v4",
		AnchorType:            "ED25519_PUBLIC_KEY",
		AnchorIdentity:        "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ClaimantID:            "claimant-service",
		ProducerID:            "artifact-producer",
		ProvisionedReceiptIDs: map[string]struct{}{"trust-receipt:production-root-2026-09": {}},
	}
}

func TestTrustProvisioningReceiptV1Contract(t *testing.T) {
	const assertion = "ASSERT_TRUST_RECEIPT_V1_CONTRACT: V1 accepts exactly the required typed provisioning fields"
	t.Log("ASSERTION: " + assertion)
	if detected, err := ValidateFor([]byte(validTrustProvisioningReceiptV1), "trust-provisioning-receipt", "v1"); err != nil || detected != "lsp-trace.trust-provisioning-receipt.v1" {
		t.Fatalf("%s: valid receipt detected=%q err=%v", assertion, detected, err)
	}
	for _, field := range []string{"receipt_id", "trust_policy_id", "anchor_type", "anchor_identity", "provisioning_authority", "provisioning_channel", "verification_method", "verification_result"} {
		mutated := removeJSONLine(validTrustProvisioningReceiptV1, field)
		if _, err := ValidateFor([]byte(mutated), "trust-provisioning-receipt", "v1"); err == nil {
			t.Errorf("%s: missing %s accepted", assertion, field)
		}
	}
	withoutProvisioningVersion := removeJSONLine(validTrustProvisioningReceiptV1, "provisioning_event")
	if _, err := ValidateFor([]byte(withoutProvisioningVersion), "trust-provisioning-receipt", "v1"); err == nil {
		t.Errorf("%s: missing provisioning event/version accepted", assertion)
	}
	withTrustStoreVersion := strings.Replace(validTrustProvisioningReceiptV1, `"provisioning_event":"trust-store:v4"`, `"trust_store_version":"v4"`, 1)
	if _, err := ValidateFor([]byte(withTrustStoreVersion), "trust-provisioning-receipt", "v1"); err != nil {
		t.Errorf("%s: trust-store version alternative rejected: %v", assertion, err)
	}
	withUnknown := strings.Replace(validTrustProvisioningReceiptV1, "\n}", ",\n  \"claimant_anchor\":true\n}", 1)
	if _, err := ValidateFor([]byte(withUnknown), "trust-provisioning-receipt", "v1"); err == nil {
		t.Errorf("%s: unknown field accepted", assertion)
	}
}

func TestTrustAuthenticationRequiresIndependentProvisioning(t *testing.T) {
	const assertion = "ASSERT_TRUST_AUTH_INDEPENDENT_PROVISIONING: AUTHENTICATED requires a matching verified receipt provisioned independently of claimant, producer, and bundle"
	t.Log("ASSERTION: " + assertion)
	if err := ValidateTrustAuthentication([]byte(validTrustProvisioningReceiptV1), validTrustAuthenticationContext()); err != nil {
		t.Fatalf("%s: valid independent receipt rejected: %v", assertion, err)
	}
	cases := []struct {
		name    string
		receipt string
		context TrustAuthenticationContext
	}{
		{name: "unprovisioned receipt", receipt: validTrustProvisioningReceiptV1, context: func() TrustAuthenticationContext {
			c := validTrustAuthenticationContext()
			c.ProvisionedReceiptIDs = map[string]struct{}{}
			return c
		}()},
		{name: "failed verification", receipt: strings.Replace(validTrustProvisioningReceiptV1, "\"VERIFIED\"", "\"FAILED\"", 1), context: validTrustAuthenticationContext()},
		{name: "policy mismatch", receipt: strings.Replace(validTrustProvisioningReceiptV1, "trust-policy:production-v4", "trust-policy:claimant", 1), context: validTrustAuthenticationContext()},
		{name: "anchor mismatch", receipt: strings.Replace(validTrustProvisioningReceiptV1, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1), context: validTrustAuthenticationContext()},
		{name: "claimant authority", receipt: strings.Replace(validTrustProvisioningReceiptV1, "security-operations", "claimant-service", 1), context: validTrustAuthenticationContext()},
		{name: "producer authority", receipt: strings.Replace(validTrustProvisioningReceiptV1, "security-operations", "artifact-producer", 1), context: validTrustAuthenticationContext()},
		{name: "presented bundle channel", receipt: strings.Replace(validTrustProvisioningReceiptV1, "VERIFIER_TRUST_STORE", "PRESENTED_BUNDLE", 1), context: validTrustAuthenticationContext()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateTrustAuthentication([]byte(tc.receipt), tc.context); err == nil {
				t.Fatalf("%s: %s accepted", assertion, tc.name)
			}
		})
	}
}

func TestTrustReceiptAdditionPreservesCommittedGraphSchemaBytes(t *testing.T) {
	const assertion = "ASSERT_TRUST_RECEIPT_GRAPH_COMPATIBILITY: committed V2 and V3 canonical schema bytes remain unchanged"
	t.Log("ASSERTION: " + assertion)
	for _, version := range []string{"v2", "v3"} {
		got, err := Bytes(version)
		if err != nil {
			t.Fatalf("%s: %s: %v", assertion, version, err)
		}
		full := "lsp-trace.graph." + version
		want, err := os.ReadFile("schemas/" + full + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: %s differs", assertion, version)
		}
	}
}

func removeJSONLine(document, field string) string {
	lines := strings.Split(document, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.Contains(line, `"`+field+`"`) {
			continue
		}
		out = append(out, line)
	}
	return strings.Replace(strings.Join(out, "\n"), ",\n}", "\n}", 1)
}
