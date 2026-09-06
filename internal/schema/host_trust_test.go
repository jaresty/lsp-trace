package schema

import (
	"strings"
	"testing"
)

func hostGrant() HostTrustGrant {
	c := validTrustAuthenticationContext()
	c.SourceSnapshotIdentity = "sha256:" + strings.Repeat("c", 64)
	r := strings.Replace(validTrustProvisioningReceiptV1, "\n}", ",\n\"source_snapshot_identity\":\""+c.SourceSnapshotIdentity+"\"\n}", 1)
	return HostTrustGrant{Receipt: []byte(r), Context: c}
}

func TestHostTrustPositive(t *testing.T) {
	g := hostGrant()
	s, err := NewHostTrustStore([]HostTrustGrant{g})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Admit(HostTrustRequest{Receipt: g.Receipt, ClaimedSourceSnapshotIdentity: g.Context.SourceSnapshotIdentity})
	if err != nil || r.Status != AuthenticationAuthenticated {
		t.Fatalf("host provisioned grant must authenticate: %v %v", r, err)
	}
}

func TestHostTrustMissing(t *testing.T) {
	var s *HostTrustStore
	r, err := s.Admit(HostTrustRequest{})
	if err != nil || r.Status != AuthenticationMissingTrust {
		t.Fatalf("absent receipt must remain missing: %v %v", r, err)
	}
}

func TestHostTrustRequestSubstitution(t *testing.T) {
	g := hostGrant()
	s, err := NewHostTrustStore([]HostTrustGrant{g})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"security-operations", "unapproved-authority"},
		{"security-operations", "claimant-service"},
		{"security-operations", "artifact-producer"},
		{"trust-policy:production-v4", "trust-policy:old"},
		{g.Context.AnchorIdentity, "sha256:" + strings.Repeat("b", 64)},
		{"trust-store:v4", "trust-store:v3"},
		{"ED25519_SIGNATURE", "OTHER_METHOD"},
	} {
		t.Run(pair[1], func(t *testing.T) {
			request := HostTrustRequest{Receipt: []byte(strings.Replace(string(g.Receipt), pair[0], pair[1], 1)), ClaimedSourceSnapshotIdentity: g.Context.SourceSnapshotIdentity}
			r, err := s.Admit(request)
			if err == nil || r.Status != AuthenticationRejected {
				t.Fatalf("substituted receipt must reject: %v %v", r, err)
			}
		})
	}
	t.Run("snapshot", func(t *testing.T) {
		r, err := s.Admit(HostTrustRequest{Receipt: g.Receipt, ClaimedSourceSnapshotIdentity: "other-snapshot"})
		if err == nil || r.Status != AuthenticationRejected {
			t.Fatalf("snapshot replay must reject: %v %v", r, err)
		}
	})
	t.Run("unprovisioned", func(t *testing.T) {
		var empty *HostTrustStore
		r, err := empty.Admit(HostTrustRequest{Receipt: g.Receipt, ClaimedSourceSnapshotIdentity: g.Context.SourceSnapshotIdentity})
		if err == nil || r.Status != AuthenticationRejected {
			t.Fatalf("unprovisioned receipt must reject: %v %v", r, err)
		}
	})
}

func TestHostTrustProvisioningRejects(t *testing.T) {
	for _, authority := range []string{"claimant-service", "artifact-producer"} {
		t.Run(authority, func(t *testing.T) {
			g := hostGrant()
			g.Receipt = []byte(strings.Replace(string(g.Receipt), "security-operations", authority, 1))
			if _, err := NewHostTrustStore([]HostTrustGrant{g}); err == nil {
				t.Fatal("controlled authority must reject")
			}
		})
	}
	t.Run("unknown identities", func(t *testing.T) {
		g := hostGrant()
		g.Context.ClaimantID = ""
		if _, err := NewHostTrustStore([]HostTrustGrant{g}); err == nil {
			t.Fatal("missing principal identity must reject")
		}
	})
}

func TestHostTrustFrozenEvidence(t *testing.T) {
	g := hostGrant()
	g.GitAttestation = &GitAttestationEvidence{EvidenceType: GitCommitAttestation, SourceSnapshotIdentity: g.Context.SourceSnapshotIdentity, CommitIdentity: "commit-approved"}
	original := append([]byte(nil), g.Receipt...)
	evidence := *g.GitAttestation
	s, err := NewHostTrustStore([]HostTrustGrant{g})
	if err != nil {
		t.Fatal(err)
	}
	g.Receipt[0] = 'x'
	delete(g.Context.ProvisionedReceiptIDs, g.Context.ReceiptID)
	g.GitAttestation.CommitIdentity = "commit-substituted"
	req := HostTrustRequest{Receipt: original, ClaimedSourceSnapshotIdentity: g.Context.SourceSnapshotIdentity, GitAttestation: &evidence}
	r, err := s.Admit(req)
	if err != nil || r.Status != AuthenticationAuthenticated {
		t.Fatalf("host configuration must be frozen: %v %v", r, err)
	}
	r.GitAttestation.CommitIdentity = "mutated-result"
	r, err = s.Admit(req)
	if err != nil || r.GitAttestation.CommitIdentity != "commit-approved" {
		t.Fatal("result must not mutate provisioned evidence")
	}
	req.GitAttestation = g.GitAttestation
	r, err = s.Admit(req)
	if err == nil || r.Status != AuthenticationRejected {
		t.Fatal("unapproved evidence must reject")
	}
	req.GitAttestation = nil
	r, err = s.Admit(req)
	if err == nil || r.Status != AuthenticationRejected {
		t.Fatal("omitted provisioned evidence must reject")
	}
}
