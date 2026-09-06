package integratedconformance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

func TestOperationalHostBindingAndFreeze(t *testing.T) {
	request := operationalRequest(t)
	grant := operationalHostGrant(t, request, "")
	snapshot := grant.Grant.Context.SourceSnapshotIdentity
	git := schema.GitAttestationEvidence{EvidenceType: schema.GitCommitAttestation, SourceSnapshotIdentity: snapshot, CommitIdentity: "approved-test-commit"}
	grant.Grant.GitAttestation = &git
	original := append([]byte(nil), grant.Grant.Receipt...)
	originalGit := git
	store, err := custodyevidence.NewHostTrustStore([]custodyevidence.HostTrustGrant{grant})
	if err != nil {
		t.Fatal(err)
	}
	grant.IdentityPolicy = "changed"
	grant.Grant.Receipt[0] = '!'
	git.CommitIdentity = "changed"
	delete(grant.Grant.Context.ProvisionedReceiptIDs, grant.Grant.Context.ReceiptID)
	a, err := store.Admit(source.ObservedIdentityPolicyV1, snapshot, original, &originalGit)
	if err != nil || a.Status != schema.AuthenticationAuthenticated {
		t.Fatal("ASSERT_OPERATIONAL_HOST_FREEZE: caller mutation changed approved host binding")
	}
	a.Receipt[0] = '!'
	a.GitAttestation.CommitIdentity = "mutated-result"
	again, err := store.Admit(source.ObservedIdentityPolicyV1, snapshot, original, &originalGit)
	if err != nil || !bytes.Equal(again.Receipt, original) || again.GitAttestation.CommitIdentity != originalGit.CommitIdentity {
		t.Fatal("ASSERT_OPERATIONAL_HOST_FREEZE: result mutation escaped")
	}
	for _, pair := range [][2]string{{source.IdentityPolicyV1, snapshot}, {"", snapshot}, {source.ObservedIdentityPolicyV1, "wrong-snapshot"}} {
		result, err := store.Admit(pair[0], pair[1], original, &originalGit)
		if err == nil || result.Status != schema.AuthenticationRejected {
			t.Fatal("ASSERT_OPERATIONAL_HOST_EXACT_BINDING: wrong acquired policy or snapshot accepted")
		}
	}
	if result, err := store.Admit(source.ObservedIdentityPolicyV1, snapshot, original, nil); err == nil || result.Status != schema.AuthenticationRejected {
		t.Fatal("ASSERT_OPERATIONAL_HOST_EXACT_GIT: approved evidence omission accepted")
	}
}

func TestOperationalApprovedGitParity(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	grant := operationalHostGrant(t, request, "")
	grant.Grant.GitAttestation = &schema.GitAttestationEvidence{EvidenceType: schema.GitCommitAttestation, SourceSnapshotIdentity: grant.Grant.Context.SourceSnapshotIdentity, CommitIdentity: "approved-test-commit"}
	config := operationalHostConfig(t, grant)
	op := operationalMode(request)
	op["require_authenticated"] = true
	op["trust_receipt"] = grant.Grant.Receipt
	op["git_attestation"] = grant.Grant.GitAttestation
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			resetOperationalPublication(t, request)
			a := operationalSuccess(t, h.run(mode, request, config, nil))
			if a.Operational.Admission.Status != schema.AuthenticationAuthenticated {
				t.Fatal("ASSERT_OPERATIONAL_APPROVED_GIT: approved optional evidence lost")
			}
		})
	}
}

func TestOperationalReceiptPublicationFailure(t *testing.T) {
	h := newOperationalHarness(t)
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			request := operationalRequest(t)
			if err := os.Mkdir(filepath.Join(request["root"].(string), "receipt.json"), 0700); err != nil {
				t.Fatal(err)
			}
			got := h.run(mode, request, "", nil)
			if !got.Failed {
				t.Fatal("ASSERT_OPERATIONAL_RECEIPT_FAILURE: receipt failure reported success")
			}
			retained := operationalFailureEvidence(t, got)
			if len(retained.InputEvidence.Inputs) != 4 {
				t.Fatal("ASSERT_OPERATIONAL_RECEIPT_FAILURE: original bytes/receipts lost")
			}
			if _, err := os.Stat(filepath.Join(request["root"].(string), "artifact.json")); err != nil {
				t.Fatal("ASSERT_OPERATIONAL_RECEIPT_FAILURE: published first artifact should remain without false pair completion")
			}
		})
	}
}
