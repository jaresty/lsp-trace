package schema

import (
	"encoding/json"
	"fmt"
)

// TrustAuthenticationContext is verifier-controlled input used to authenticate
// a presented trust-provisioning receipt.
type TrustAuthenticationContext struct {
	ReceiptID              string
	TrustPolicyID          string
	AnchorType             string
	AnchorIdentity         string
	SourceSnapshotIdentity string
	ClaimantID             string
	ProducerID             string
	ProvisionedReceiptIDs  map[string]struct{}
}

type AuthenticationStatus string

const (
	AuthenticationMissingTrust  AuthenticationStatus = "MISSING_TRUST"
	AuthenticationRejected      AuthenticationStatus = "REJECTED"
	AuthenticationAuthenticated AuthenticationStatus = "AUTHENTICATED"
	GitCommitAttestation                             = "GIT_COMMIT_ATTESTATION"
)

type GitAttestationEvidence struct {
	EvidenceType           string
	SourceSnapshotIdentity string
	CommitIdentity         string
}

type TrustAdmissionRequest struct {
	Receipt                       []byte
	Context                       TrustAuthenticationContext
	ClaimedSourceSnapshotIdentity string
	GitAttestation                *GitAttestationEvidence
}

type TrustAdmissionResult struct {
	Status         AuthenticationStatus
	GitAttestation *GitAttestationEvidence
}

func AdmitTrust(request TrustAdmissionRequest) (TrustAdmissionResult, error) {
	if len(request.Receipt) == 0 {
		return TrustAdmissionResult{Status: AuthenticationMissingTrust}, nil
	}
	reject := func(format string, args ...any) (TrustAdmissionResult, error) {
		return TrustAdmissionResult{Status: AuthenticationRejected}, fmt.Errorf("authentication rejected: "+format, args...)
	}
	if _, err := ValidateFor(request.Receipt, FamilyTrustProvisioningReceipt, "v1"); err != nil {
		return reject("invalid trust provisioning receipt: %v", err)
	}
	var receipt trustProvisioningReceiptV1
	if err := json.Unmarshal(request.Receipt, &receipt); err != nil {
		return reject("invalid trust provisioning receipt: %v", err)
	}
	context := request.Context
	if _, ok := context.ProvisionedReceiptIDs[receipt.ReceiptID]; !ok {
		return reject("receipt is not verifier-provisioned")
	}
	if receipt.ProvisioningChannel != "VERIFIER_TRUST_STORE" && receipt.ProvisioningChannel != "INDEPENDENT_TRUST_POLICY" {
		return reject("provisioning channel is not independent")
	}
	if receipt.ProvisioningAuthority == context.ClaimantID || receipt.ProvisioningAuthority == context.ProducerID {
		return reject("provisioning authority is claimant-controlled")
	}
	if receipt.ReceiptID != context.ReceiptID {
		return reject("receipt identity mismatch")
	}
	if receipt.TrustPolicyID != context.TrustPolicyID {
		return reject("trust policy mismatch")
	}
	if receipt.AnchorType != context.AnchorType || receipt.AnchorIdentity != context.AnchorIdentity {
		return reject("trust anchor mismatch")
	}
	if receipt.SourceSnapshotIdentity == "" || receipt.SourceSnapshotIdentity != request.ClaimedSourceSnapshotIdentity || receipt.SourceSnapshotIdentity != context.SourceSnapshotIdentity {
		return reject("source snapshot identity mismatch")
	}
	if receipt.VerificationResult != "VERIFIED" {
		return reject("verification result is not VERIFIED")
	}

	result := TrustAdmissionResult{Status: AuthenticationAuthenticated}
	if request.GitAttestation != nil {
		evidence := *request.GitAttestation
		if evidence.EvidenceType != GitCommitAttestation || evidence.CommitIdentity == "" || evidence.SourceSnapshotIdentity != receipt.SourceSnapshotIdentity {
			return reject("invalid Git attestation evidence")
		}
		result.GitAttestation = &evidence
	}
	return result, nil
}

type trustProvisioningReceiptV1 struct {
	ReceiptID              string `json:"receipt_id"`
	TrustPolicyID          string `json:"trust_policy_id"`
	AnchorType             string `json:"anchor_type"`
	AnchorIdentity         string `json:"anchor_identity"`
	SourceSnapshotIdentity string `json:"source_snapshot_identity"`
	ProvisioningAuthority  string `json:"provisioning_authority"`
	ProvisioningChannel    string `json:"provisioning_channel"`
	VerificationResult     string `json:"verification_result"`
}

// ValidateTrustAuthentication validates a receipt against independently
// provisioned verifier-side authentication context. Structural receipt
// admission always precedes authentication semantics.
func ValidateTrustAuthentication(data []byte, context TrustAuthenticationContext) error {
	if _, err := ValidateFor(data, FamilyTrustProvisioningReceipt, "v1"); err != nil {
		return fmt.Errorf("trust provisioning receipt: %w", err)
	}
	var receipt trustProvisioningReceiptV1
	if err := json.Unmarshal(data, &receipt); err != nil {
		return fmt.Errorf("trust provisioning receipt: %w", err)
	}
	if _, ok := context.ProvisionedReceiptIDs[receipt.ReceiptID]; !ok {
		return fmt.Errorf("authentication rejected: receipt is not verifier-provisioned")
	}
	if receipt.ReceiptID != context.ReceiptID {
		return fmt.Errorf("authentication rejected: receipt identity mismatch")
	}
	if receipt.TrustPolicyID != context.TrustPolicyID {
		return fmt.Errorf("authentication rejected: trust policy mismatch")
	}
	if receipt.AnchorType != context.AnchorType || receipt.AnchorIdentity != context.AnchorIdentity {
		return fmt.Errorf("authentication rejected: trust anchor mismatch")
	}
	if receipt.VerificationResult != "VERIFIED" {
		return fmt.Errorf("authentication rejected: verification result is not VERIFIED")
	}
	if receipt.ProvisioningAuthority == context.ClaimantID || receipt.ProvisioningAuthority == context.ProducerID {
		return fmt.Errorf("authentication rejected: provisioning authority is claimant-controlled")
	}
	return nil
}
