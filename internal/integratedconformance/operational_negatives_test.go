package integratedconformance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

func assertOperationalUnpublished(t *testing.T, request map[string]any) {
	t.Helper()
	for _, name := range []string{"artifact.json", "receipt.json"} {
		if _, err := os.Stat(filepath.Join(request["root"].(string), name)); !os.IsNotExist(err) {
			t.Fatalf("ASSERT_OPERATIONAL_NO_PUBLICATION: %s exists or cannot be checked: %v", name, err)
		}
	}
}

func TestOperationalTrustNegativeParity(t *testing.T) {
	h := newOperationalHarness(t)
	for _, scenario := range []string{"no-grant", "no-receipt", "changed-bytes", "source-id", "collection-id", "wrong-snapshot", "receipt-replay", "git-injection"} {
		t.Run(scenario, func(t *testing.T) {
			request := operationalRequest(t)
			field := ""
			switch scenario {
			case "source-id":
				field = "source"
			case "collection-id":
				field = "collection"
			case "wrong-snapshot":
				field = "wrong"
			}
			grant := operationalHostGrant(t, request, field)
			config := operationalHostConfig(t, grant)
			op := operationalMode(request)
			op["require_authenticated"] = true
			op["trust_receipt"] = grant.Grant.Receipt
			switch scenario {
			case "no-grant":
				config = ""
			case "no-receipt":
				delete(op, "trust_receipt")
			case "changed-bytes":
				if err := os.WriteFile(filepath.Join(op["source_root"].(string), "input.go"), []byte("new actual source bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "receipt-replay":
				op["trust_receipt"] = bytes.ReplaceAll(grant.Grant.Receipt, []byte("fixture-independent-review"), []byte("another-review"))
			case "git-injection":
				op["git_attestation"] = &schema.GitAttestationEvidence{EvidenceType: schema.GitCommitAttestation, SourceSnapshotIdentity: grant.Grant.Context.SourceSnapshotIdentity, CommitIdentity: "unapproved"}
			}
			var prior []byte
			for _, mode := range []string{"direct", "cli", "mcp"} {
				t.Run(mode, func(t *testing.T) {
					resetOperationalPublication(t, request)
					got := h.run(mode, request, config, nil)
					if !got.Failed {
						t.Fatalf("ASSERT_OPERATIONAL_REJECT_%s_%s: unauthorized request succeeded", scenario, mode)
					}
					e := operationalFailureEvidence(t, got)
					if e.PublicationPermitted || e.Admission.Status == schema.AuthenticationAuthenticated {
						t.Fatalf("ASSERT_OPERATIONAL_REJECT_%s_%s: failure lost rejection", scenario, mode)
					}
					encoded, _ := json.Marshal(e)
					if _, err := custodyevidence.ValidateFor(encoded, schema.FamilyOperationalCustody, "v1"); err != nil {
						t.Fatalf("failure evidence not semantically valid: %v", err)
					}
					if prior == nil {
						prior = encoded
					} else if !bytes.Equal(prior, encoded) {
						t.Fatal("ASSERT_OPERATIONAL_FAILURE_PARITY: differing retained rejection evidence")
					}
					assertOperationalUnpublished(t, request)
				})
			}
		})
	}
}

func TestOperationalPartialParity(t *testing.T) {
	h := newOperationalHarness(t)
	for _, all := range []bool{false, true} {
		name := "mixed"
		if all {
			name = "all-failed"
		}
		t.Run(name, func(t *testing.T) {
			request := operationalRequest(t)
			op := operationalMode(request)
			root := op["source_root"].(string)
			names := []string{"input.go"}
			if all {
				names = []string{"input.go", "config.json", "types.d.ts", "mapping.json"}
			}
			for _, name := range names {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
			// Test host deliberately approves this exact partial snapshot. That approval
			// still cannot satisfy authenticated-required execution's required reads.
			grant := operationalHostGrant(t, request, "")
			config := operationalHostConfig(t, grant)
			var prior []byte
			for _, mode := range []string{"direct", "cli", "mcp"} {
				t.Run(mode, func(t *testing.T) {
					resetOperationalPublication(t, request)
					op["require_authenticated"] = false
					delete(op, "trust_receipt")
					got := h.run(mode, request, "", nil)
					a := operationalSuccess(t, got)
					e := a.Operational
					if e.AcquisitionStatus != "PARTIAL" || e.Admission.Status != schema.AuthenticationMissingTrust || !e.PublicationPermitted || len(e.InputEvidence.Inputs) != 4 || len(e.Outputs) != 4 {
						t.Fatal("ASSERT_OPERATIONAL_PARTIAL: missing reads silently lost or strengthened")
					}
					failed := 0
					for _, in := range e.InputEvidence.Inputs {
						if in.Receipt.Status == source.Unreadable {
							failed++
							if in.Content != nil || in.Receipt.ContentIdentity != nil {
								t.Fatal("ASSERT_OPERATIONAL_PARTIAL: fabricated failed-read content")
							}
						}
					}
					if failed != len(names) {
						t.Fatal("ASSERT_OPERATIONAL_PARTIAL: failed observations omitted")
					}
					published, err := os.ReadFile(a.Artifact)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := custodyevidence.ValidateFor(published, schema.FamilyOperationalCustody, "v1"); err != nil {
						t.Fatal(err)
					}
					if prior == nil {
						prior = published
					} else if !bytes.Equal(prior, published) {
						t.Fatal("ASSERT_OPERATIONAL_PARTIAL_PARITY: different published partial bytes")
					}
					resetOperationalPublication(t, request)
					op["require_authenticated"] = true
					op["trust_receipt"] = grant.Grant.Receipt
					denied := h.run(mode, request, config, nil)
					if !denied.Failed {
						t.Fatal("ASSERT_OPERATIONAL_REQUIRED_READ: host approval turned failed required acquisition into success")
					}
					retained := operationalFailureEvidence(t, denied)
					if retained.Admission.Status != schema.AuthenticationAuthenticated || retained.PublicationPermitted || retained.AcquisitionStatus != "PARTIAL" {
						t.Fatal("ASSERT_OPERATIONAL_REQUIRED_READ: trust and read permission were conflated")
					}
					assertOperationalUnpublished(t, request)
				})
			}
		})
	}
}

func TestOperationalInputNegativeParity(t *testing.T) {
	h := newOperationalHarness(t)
	for _, scenario := range []string{"grant", "context", "config-path", "nested-config", "mixed-source", "empty-source", "uri", "escape", "duplicate-class", "unknown-class"} {
		t.Run(scenario, func(t *testing.T) {
			request := operationalRequest(t)
			op := operationalMode(request)
			switch scenario {
			case "grant":
				op["grants"] = []any{map[string]any{"identity_policy": source.ObservedIdentityPolicyV1}}
			case "context":
				op["context"] = map[string]any{"ProvisionedReceiptIDs": map[string]any{"invented": map[string]any{}}}
			case "config-path":
				request["custody_trust_config"] = "/untrusted/config.json"
			case "nested-config":
				op["custody_trust_config"] = "/untrusted/config.json"
			case "mixed-source":
				request["source"] = "caller bytes"
			case "empty-source":
				request["source"] = ""
			case "uri":
				op["inputs"] = []any{map[string]any{"path": "file:///outside", "class": "SOURCE"}}
			case "escape":
				op["inputs"] = []any{map[string]any{"path": "../outside", "class": "SOURCE"}}
			case "duplicate-class":
				op["inputs"] = []any{map[string]any{"path": "input.go", "class": "SOURCE"}, map[string]any{"path": "input.go", "class": "CONFIGURATION"}}
			case "unknown-class":
				op["inputs"] = []any{map[string]any{"path": "input.go", "class": "MAGIC"}}
			}
			for _, mode := range []string{"direct", "cli", "mcp"} {
				t.Run(mode, func(t *testing.T) {
					resetOperationalPublication(t, request)
					got := h.run(mode, request, "", nil)
					if !got.Failed {
						t.Fatalf("ASSERT_OPERATIONAL_INPUT_%s_%s: rejected input accepted", scenario, mode)
					}
					assertOperationalUnpublished(t, request)
				})
			}
		})
	}
}

func TestOperationalPolicyStartupParity(t *testing.T) {
	h := newOperationalHarness(t)
	for _, policy := range []string{"", source.IdentityPolicyV1} {
		t.Run("policy-"+policy, func(t *testing.T) {
			request := operationalRequest(t)
			grant := operationalHostGrant(t, request, "")
			grant.IdentityPolicy = policy
			config := operationalHostConfig(t, grant)
			for _, mode := range []string{"direct", "cli", "mcp"} {
				t.Run(mode, func(t *testing.T) {
					resetOperationalPublication(t, request)
					got := h.run(mode, request, config, nil)
					if !got.Failed || !strings.Contains(got.Output, "exact observed identity policy") {
						t.Fatalf("ASSERT_OPERATIONAL_STARTUP_POLICY_%s: wrong host policy not rejected at startup: %s", mode, got.Output)
					}
					assertOperationalUnpublished(t, request)
				})
			}
		})
	}
}

func TestOperationalOptionalGitAndScope(t *testing.T) {
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	op := operationalMode(request)
	grant := operationalHostGrant(t, request, "")
	op["revision"] = &source.RevisionAttestation{System: "git", Revision: "metadata-only"}
	op["git_attestation"] = &schema.GitAttestationEvidence{EvidenceType: schema.GitCommitAttestation, SourceSnapshotIdentity: grant.Grant.Context.SourceSnapshotIdentity, CommitIdentity: "claim-only"}
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			resetOperationalPublication(t, request)
			a := operationalSuccess(t, h.run(mode, request, "", nil))
			if a.Operational.Admission.Status != schema.AuthenticationMissingTrust {
				t.Fatal("ASSERT_OPERATIONAL_GIT_LABEL: optional labels supplied authority")
			}
		})
	}
	// os.Root rejects the attempted symlink escape; retain failure, never secret bytes.
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(op["source_root"].(string), "escape")); err != nil {
		t.Fatal(err)
	}
	delete(op, "git_attestation")
	op["inputs"] = []any{map[string]any{"path": "escape", "class": "SOURCE"}}
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run("symlink-"+mode, func(t *testing.T) {
			resetOperationalPublication(t, request)
			a := operationalSuccess(t, h.run(mode, request, "", nil))
			if a.Operational.AcquisitionStatus != "PARTIAL" || a.Operational.InputEvidence.Inputs[0].Content != nil {
				t.Fatal("ASSERT_OPERATIONAL_SCOPE: escaped read yielded bytes")
			}
		})
	}
}
