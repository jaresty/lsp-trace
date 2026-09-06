package integratedconformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/custodyevidence"
	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

// Modeled independently provisioned host fixture: a separate pre-execution
// measurement is approved by the test host. No operation result provisions its
// own grant. This models startup authority; it is not cryptographic verification.
func operationalHostGrant(t *testing.T, request map[string]any, field string) custodyevidence.HostTrustGrant {
	t.Helper()
	raw, _ := json.Marshal(request["operational"])
	var op executionruntime.OperationalInput
	if err := json.Unmarshal(raw, &op); err != nil {
		t.Fatal(err)
	}
	recorder, err := source.NewInputRecorder(op.SourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer recorder.Close()
	ids := []string{}
	for _, file := range op.Inputs {
		_, id, _ := recorder.ReadInput(file.Path, file.Class)
		if id == "" {
			t.Fatal("host fixture: no recorded read")
		}
		contribution := "input:" + file.Path
		ids = append(ids, contribution)
		if err := recorder.BindContribution(contribution, id); err != nil {
			t.Fatal(err)
		}
	}
	evidence, err := recorder.Evidence(ids)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := source.BuildObservedIdentity(source.ObservedIdentityRequest{Evidence: evidence, Acquisition: source.AcquisitionContext{Adapter: custodyevidence.Adapter, WorkspaceURI: (&url.URL{Scheme: "file", Path: op.SourceRoot}).String(), InvocationID: "offline-1"}, Revision: op.Revision})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := identity.SnapshotID
	switch field {
	case "source":
		snapshot = identity.SourceID
	case "collection":
		snapshot = identity.CollectionID
	case "wrong":
		snapshot = "sha256:" + strings.Repeat("0", 64)
	}
	receiptID := "fixture-host-receipt"
	anchor := "sha256:" + strings.Repeat("a", 64)
	receipt, _ := json.Marshal(map[string]any{"trust_provisioning_receipt_schema_version": "lsp-trace.trust-provisioning-receipt.v1", "receipt_id": receiptID, "trust_policy_id": "fixture-host-policy:v1", "anchor_type": "ED25519_PUBLIC_KEY", "anchor_identity": anchor, "provisioning_authority": "fixture-security-operations", "provisioning_channel": "VERIFIER_TRUST_STORE", "provisioning_event": "fixture-independent-review", "verification_method": "ED25519_SIGNATURE", "verification_result": "VERIFIED", "source_snapshot_identity": snapshot})
	return custodyevidence.HostTrustGrant{IdentityPolicy: source.ObservedIdentityPolicyV1, Grant: schema.HostTrustGrant{Receipt: receipt, Context: schema.TrustAuthenticationContext{ReceiptID: receiptID, TrustPolicyID: "fixture-host-policy:v1", AnchorType: "ED25519_PUBLIC_KEY", AnchorIdentity: anchor, SourceSnapshotIdentity: snapshot, ClaimantID: "fixture-claimant", ProducerID: "fixture-producer", ProvisionedReceiptIDs: map[string]struct{}{receiptID: {}}}}}
}

func operationalHostConfig(t *testing.T, grants ...custodyevidence.HostTrustGrant) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "host.json")
	b, _ := json.Marshal(map[string]any{"grants": grants})
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func operationalMode(request map[string]any) map[string]any {
	return request["operational"].(map[string]any)
}
func resetOperationalPublication(t *testing.T, request map[string]any) {
	t.Helper()
	root := request["root"].(string)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
}
func operationalSuccess(t *testing.T, got operationalObservation) executionruntime.ProductionArtifact {
	t.Helper()
	if got.Failed {
		t.Fatalf("operational success required: %s", got.Output)
	}
	var artifact executionruntime.ProductionArtifact
	if err := json.Unmarshal(got.Artifact, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Operational == nil {
		t.Fatal("missing operational result")
	}
	return artifact
}
func operationalFailureEvidence(t *testing.T, got operationalObservation) *custodyevidence.Evidence {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(got.Output), &value); err != nil {
		t.Fatalf("operation did not retain failure evidence (startup/setup failure is not an assertion witness): %s", got.Output)
	}
	var found *custodyevidence.Evidence
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			const prefix = "operational_failure_evidence:"
			if strings.HasPrefix(x, prefix) {
				var a executionruntime.ProductionArtifact
				if json.Unmarshal([]byte(strings.TrimPrefix(x, prefix)), &a) == nil {
					found = a.Operational
				}
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		case map[string]any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	walk(value)
	if found == nil {
		t.Fatalf("operation did not retain failure evidence: %s", got.Output)
	}
	return found
}

func TestOperationalIndependentHostCommands(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	grant := operationalHostGrant(t, request, "")
	config := operationalHostConfig(t, grant)
	operationalMode(request)["require_authenticated"] = true
	operationalMode(request)["trust_receipt"] = grant.Grant.Receipt
	var direct []byte
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			resetOperationalPublication(t, request)
			got := h.run(mode, request, config, nil)
			if got.Failed {
				evidence := operationalFailureEvidence(t, got)
				t.Fatalf("ASSERT_OPERATIONAL_HOST_%s: independently provisioned host did not authenticate: status=%s", mode, evidence.Admission.Status)
			}
			artifact := operationalSuccess(t, got)
			e := artifact.Operational
			if e.Admission.Status != schema.AuthenticationAuthenticated || !e.PublicationPermitted || e.Identity.Policy != source.ObservedIdentityPolicyV1 || !bytes.Contains(e.Identity.Manifest, []byte("INCOMPLETE")) {
				t.Fatalf("ASSERT_OPERATIONAL_HOST_%s: missing exact policy/admission or lost incompleteness", mode)
			}
			if mode == "direct" {
				direct = append([]byte(nil), got.Artifact...)
			} else if !bytes.Equal(direct, got.Artifact) {
				t.Fatalf("ASSERT_OPERATIONAL_PARITY_%s: different canonical result bytes", mode)
			}
		})
	}
}

func TestOperationalHostPolicyEnvelope(t *testing.T) {
	request := operationalRequest(t)
	grant := operationalHostGrant(t, request, "")
	for _, policy := range []string{"", source.IdentityPolicyV1, "invented-policy"} {
		t.Run(fmt.Sprintf("policy-%s", policy), func(t *testing.T) {
			bad := grant
			bad.IdentityPolicy = policy
			if _, err := custodyevidence.NewHostTrustStore([]custodyevidence.HostTrustGrant{bad}); err == nil {
				t.Fatal("ASSERT_OPERATIONAL_HOST_POLICY: missing/wrong host identity policy accepted")
			}
		})
	}
}
