package custodyevidence

import (
	"fmt"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

// HostTrustGrant is privileged startup configuration. IdentityPolicy is an
// additional pin; the underlying schema store binds receipt bytes and snapshot,
// but does not know which source identity policy produced that snapshot.
// The host must independently approve the grant before constructing the store.
// This allowlist is not cryptographic verification of receipt metadata labels.
type HostTrustGrant struct {
	IdentityPolicy string                `json:"identity_policy"`
	Grant          schema.HostTrustGrant `json:"grant"`
}

type HostTrustStore struct {
	store    *schema.HostTrustStore
	policies map[string]string
}

func NewHostTrustStore(grants []HostTrustGrant) (*HostTrustStore, error) {
	native := make([]schema.HostTrustGrant, 0, len(grants))
	policies := map[string]string{}
	for _, grant := range grants {
		if grant.IdentityPolicy != source.ObservedIdentityPolicyV1 {
			return nil, fmt.Errorf("host custody grant requires exact observed identity policy")
		}
		policies[string(grant.Grant.Receipt)] = grant.IdentityPolicy
		native = append(native, grant.Grant)
	}
	store, err := schema.NewHostTrustStore(native)
	if err != nil {
		return nil, err
	}
	return &HostTrustStore{store: store, policies: policies}, nil
}

// Admit binds the acquired policy AND snapshot. No verifier context or grant can
// enter through this operation. Results retain supplied rejection evidence too.
func (s *HostTrustStore) Admit(policy, snapshot string, receipt []byte, git *schema.GitAttestationEvidence) (Admission, error) {
	result := Admission{Policy: policy, SnapshotID: snapshot, Status: schema.AuthenticationRejected, Receipt: append([]byte(nil), receipt...)}
	if git != nil {
		copy := *git
		result.GitAttestation = &copy
	}
	if policy != source.ObservedIdentityPolicyV1 {
		return result, fmt.Errorf("acquired identity policy mismatch")
	}
	if len(receipt) == 0 {
		result.Status = schema.AuthenticationMissingTrust
		return result, nil
	}
	if s == nil || s.policies[string(receipt)] != policy {
		return result, fmt.Errorf("receipt not host-provisioned for acquired policy")
	}
	admitted, err := s.store.Admit(schema.HostTrustRequest{Receipt: receipt, ClaimedSourceSnapshotIdentity: snapshot, GitAttestation: git})
	result.Status = admitted.Status
	return result, err
}
