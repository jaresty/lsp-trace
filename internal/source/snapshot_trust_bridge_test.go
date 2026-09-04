package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/schema"
)

const (
	assertBridgeCanonical = "ASSERT_SNAPSHOT_TRUST_CANONICAL_BYTES: bridge returns the exact canonical manifest bytes"
	assertBridgeIdentity  = "ASSERT_SNAPSHOT_TRUST_EXACT_IDENTITY: snapshot identity is SHA-256 of the exact canonical manifest bytes"
	assertBridgeMissing   = "ASSERT_SNAPSHOT_TRUST_MISSING_EXPLICIT: absent trust remains explicit MISSING_TRUST without error"
	assertBridgeAdmission = "ASSERT_SNAPSHOT_TRUST_ADMISSION_BOUND: independently provisioned trust authenticates only the computed snapshot identity and present wrong trust is REJECTED"
	assertBridgeNoPublish = "ASSERT_SNAPSHOT_TRUST_NO_PUBLICATION: bridge creates no publication artifacts"
	assertBridgeCompat    = "ASSERT_SNAPSHOT_TRUST_COMPONENT_COMPATIBILITY: existing graph V2 and V3 schema bytes remain unchanged"
)

func TestBindSnapshotTrust(t *testing.T) {
	for _, assertion := range []string{assertBridgeCanonical, assertBridgeIdentity, assertBridgeMissing, assertBridgeAdmission, assertBridgeNoPublish, assertBridgeCompat} {
		t.Log("ASSERTION: " + assertion)
	}

	receipts, decisions := baseReceipts(), baseDecisions()
	canonical, err := AssembleManifest(receipts, decisions)
	if err != nil {
		t.Fatal(err)
	}
	result, err := BindSnapshotTrust(SnapshotTrustRequest{Receipts: receipts, Decisions: decisions})
	if err != nil {
		t.Fatalf("%s: %v", assertBridgeMissing, err)
	}
	if !bytes.Equal(result.Manifest, canonical) {
		t.Errorf("%s: got %q want %q", assertBridgeCanonical, result.Manifest, canonical)
	}
	sum := sha256.Sum256(canonical)
	wantID := "sha256:" + hex.EncodeToString(sum[:])
	if result.SourceSnapshotID != wantID {
		t.Errorf("%s: got %q want %q", assertBridgeIdentity, result.SourceSnapshotID, wantID)
	}
	if result.TrustAdmission.Status != schema.AuthenticationMissingTrust {
		t.Errorf("%s: got %q", assertBridgeMissing, result.TrustAdmission.Status)
	}

	reverseReceipts(receipts)
	reverseDecisions(decisions)
	reordered, err := BindSnapshotTrust(SnapshotTrustRequest{Receipts: receipts, Decisions: decisions})
	if err != nil || reordered.SourceSnapshotID != wantID || !bytes.Equal(reordered.Manifest, canonical) {
		t.Errorf("%s: reordered result=%#v err=%v", assertBridgeCanonical, reordered, err)
	}
	mutatedReceipts := baseReceipts()
	mutatedReceipts[1].Digest = digestC
	mutated, err := BindSnapshotTrust(SnapshotTrustRequest{Receipts: mutatedReceipts, Decisions: baseDecisions()})
	if err != nil || mutated.SourceSnapshotID == wantID {
		t.Errorf("%s: manifest mutation did not change identity: result=%#v err=%v", assertBridgeIdentity, mutated, err)
	}

	trustRequest := bridgeTrustRequest(wantID)
	authenticated, err := BindSnapshotTrust(SnapshotTrustRequest{Receipts: baseReceipts(), Decisions: baseDecisions(), Trust: trustRequest})
	if err != nil || authenticated.TrustAdmission.Status != schema.AuthenticationAuthenticated {
		t.Errorf("%s: authenticated result=%#v err=%v", assertBridgeAdmission, authenticated, err)
	}
	wrongSnapshot := strings.Repeat("d", 64)
	wrongTrust := bridgeTrustRequest("sha256:" + wrongSnapshot)
	rejected, err := BindSnapshotTrust(SnapshotTrustRequest{Receipts: baseReceipts(), Decisions: baseDecisions(), Trust: wrongTrust})
	if err == nil || rejected.TrustAdmission.Status != schema.AuthenticationRejected {
		t.Errorf("%s: wrong snapshot result=%#v err=%v", assertBridgeAdmission, rejected, err)
	}

	for _, typ := range []reflect.Type{reflect.TypeOf(SnapshotTrustRequest{}), reflect.TypeOf(SnapshotTrustResult{})} {
		for i := 0; i < typ.NumField(); i++ {
			field := strings.ToLower(typ.Field(i).Name + " " + typ.Field(i).Tag.Get("json"))
			if strings.Contains(field, "path") || strings.Contains(field, "selector") || strings.Contains(field, "publish") || strings.Contains(field, "output") {
				t.Errorf("%s: publication-capable field %q", assertBridgeNoPublish, field)
			}
		}
	}
	for _, version := range []string{"v2", "v3"} {
		embedded, err := schema.Bytes(version)
		if err != nil {
			t.Fatal(err)
		}
		committed, err := os.ReadFile("../schema/schemas/lsp-trace.graph." + version + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embedded, committed) {
			t.Errorf("%s: %s bytes differ", assertBridgeCompat, version)
		}
	}
}

func bridgeTrustRequest(snapshotID string) schema.TrustAdmissionRequest {
	const receiptID = "trust-receipt:test-root"
	const policyID = "trust-policy:test"
	const anchorID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receipt := fmt.Sprintf(`{"trust_provisioning_receipt_schema_version":"lsp-trace.trust-provisioning-receipt.v1","receipt_id":%q,"trust_policy_id":%q,"anchor_type":"ED25519_PUBLIC_KEY","anchor_identity":%q,"source_snapshot_identity":%q,"provisioning_authority":"security-operations","provisioning_channel":"VERIFIER_TRUST_STORE","provisioning_event":"test-store:v1","verification_method":"ED25519_SIGNATURE","verification_result":"VERIFIED"}`, receiptID, policyID, anchorID, snapshotID)
	return schema.TrustAdmissionRequest{
		Receipt: []byte(receipt),
		Context: schema.TrustAuthenticationContext{
			ReceiptID: receiptID, TrustPolicyID: policyID, AnchorType: "ED25519_PUBLIC_KEY", AnchorIdentity: anchorID,
			SourceSnapshotIdentity: snapshotID, ClaimantID: "claimant", ProducerID: "producer",
			ProvisionedReceiptIDs: map[string]struct{}{receiptID: {}},
		},
		ClaimedSourceSnapshotIdentity: "sha256:" + strings.Repeat("f", 64),
	}
}
