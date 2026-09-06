package schema

import "fmt"

// HostTrustGrant is privileged host configuration, never request or producer input.
// Hosts must independently verify the grant before provisioning it. This API pins
// approved bytes; it does not perform the cryptographic method named in a receipt.
type HostTrustGrant struct {
	Receipt        []byte
	Context        TrustAuthenticationContext
	GitAttestation *GitAttestationEvidence
}

// HostTrustRequest deliberately contains no verifier context or authority labels.
type HostTrustRequest struct {
	Receipt                       []byte
	ClaimedSourceSnapshotIdentity string
	GitAttestation                *GitAttestationEvidence
}

// HostTrustStore is provisioned once by the trusted host, outside request handling.
// Possession of constructor access is host privilege, not proof of independence.
type HostTrustStore struct {
	grants map[string]hostTrustBinding
}

type hostTrustBinding struct {
	snapshot string
	evidence *GitAttestationEvidence
}

func NewHostTrustStore(grants []HostTrustGrant) (*HostTrustStore, error) {
	s := &HostTrustStore{grants: make(map[string]hostTrustBinding, len(grants))}
	for i, g := range grants {
		if g.Context.ClaimantID == "" || g.Context.ProducerID == "" {
			return nil, fmt.Errorf("host trust grant %d: claimant and producer identities required", i)
		}
		result, err := AdmitTrust(TrustAdmissionRequest{Receipt: g.Receipt, Context: g.Context, ClaimedSourceSnapshotIdentity: g.Context.SourceSnapshotIdentity, GitAttestation: g.GitAttestation})
		if err != nil || result.Status != AuthenticationAuthenticated {
			return nil, fmt.Errorf("host trust grant %d: invalid provisioning: status=%s error=%v", i, result.Status, err)
		}
		key := string(g.Receipt)
		if _, exists := s.grants[key]; exists {
			return nil, fmt.Errorf("host trust grant %d: duplicate receipt", i)
		}
		s.grants[key] = hostTrustBinding{snapshot: g.Context.SourceSnapshotIdentity, evidence: result.GitAttestation}
	}
	return s, nil
}

func (s *HostTrustStore) Admit(request HostTrustRequest) (TrustAdmissionResult, error) {
	if len(request.Receipt) == 0 {
		return TrustAdmissionResult{Status: AuthenticationMissingTrust}, nil
	}
	reject := func(reason string) (TrustAdmissionResult, error) {
		return TrustAdmissionResult{Status: AuthenticationRejected}, fmt.Errorf("host authentication rejected: %s", reason)
	}
	if s == nil {
		return reject("no host authority provisioned")
	}
	binding, ok := s.grants[string(request.Receipt)]
	if !ok {
		return reject("receipt bytes not host-provisioned")
	}
	if request.ClaimedSourceSnapshotIdentity != binding.snapshot {
		return reject("source snapshot identity mismatch")
	}
	if (request.GitAttestation == nil) != (binding.evidence == nil) {
		return reject("Git attestation presence mismatch")
	}
	if binding.evidence != nil && *request.GitAttestation != *binding.evidence {
		return reject("Git attestation not host-provisioned")
	}
	result := TrustAdmissionResult{Status: AuthenticationAuthenticated}
	if binding.evidence != nil {
		evidence := *binding.evidence
		result.GitAttestation = &evidence
	}
	return result, nil
}
