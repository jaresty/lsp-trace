package schema

import (
	"encoding/json"
	"fmt"
)

// TrustAuthenticationContext is verifier-controlled input used to authenticate
// a presented trust-provisioning receipt.
type TrustAuthenticationContext struct {
	ReceiptID             string
	TrustPolicyID         string
	AnchorType            string
	AnchorIdentity        string
	ClaimantID            string
	ProducerID            string
	ProvisionedReceiptIDs map[string]struct{}
}

type trustProvisioningReceiptV1 struct {
	ReceiptID             string `json:"receipt_id"`
	TrustPolicyID         string `json:"trust_policy_id"`
	AnchorType            string `json:"anchor_type"`
	AnchorIdentity        string `json:"anchor_identity"`
	ProvisioningAuthority string `json:"provisioning_authority"`
	VerificationResult    string `json:"verification_result"`
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
