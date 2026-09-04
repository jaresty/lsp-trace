package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
	"lsp-trace/internal/relations"
)

func semanticFixture(t *testing.T) (*ObservationSemanticAdapter, StrictCollectorRequest, Receipt) {
	t.Helper()
	p := mustProvisionAdmission(t, admissionDeclaration("alpha@1", "PASSES_CALLBACK"))
	identity := observationadapter.Identity{Name: "semantic", Version: "1"}
	a, err := NewObservationSemanticAdapter(p, identity)
	if err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("b", 40)
	blob := strings.Repeat("c", 40)
	envelope := observationadapter.Envelope{
		Provider: observationadapter.Identity{Name: "alpha", Version: "1"}, Protocol: observationadapter.Identity{Name: observationadapter.ProtocolName, Version: observationadapter.ProtocolVersion}, Adapter: identity,
		Authority: observationadapter.AuthorityProviderReported, RequestID: "managed:7:file:///workspace/a.gts", Coverage: relations.Coverage{Status: relations.CoveragePartial, Denominator: []string{"file:///workspace/a.gts"}, Covered: []string{"file:///workspace/a.gts"}, CoveredCount: 1},
		Documents:    []graph.SourceDocumentRecord{{DocumentID: "source", OriginalURI: "file:///workspace/a.gts", ContentSHA256: strings.Repeat("a", 64), Revision: graph.RevisionIdentity{Kind: "git", Value: revision, Blob: blob, Custody: graph.CustodyProviderProved}, Coordinates: graph.CoordinatesOriginal}},
		Observations: []observationadapter.ReportedObservation{{Kind: relations.RelationPassesCallback, From: relations.Endpoint{NodeID: "from", Role: relations.RoleCallableReference}, To: relations.Endpoint{NodeID: "to", Role: relations.RoleCallbackParameter}, OriginalAnchor: observationadapter.Anchor{DocumentID: "source", URI: "file:///workspace/a.gts", Revision: revision, Blob: blob}, Supports: []relations.Claim{relations.ClaimSourceDependencyRelation}, DoesNotSupport: append([]relations.Claim(nil), relations.ProhibitedClaims...)}},
	}
	raw, _ := json.Marshal(envelope)
	request := StrictCollectorRequest{SchemaVersion: CollectorRequestSchema, ProviderID: "alpha@1", AdapterID: "semantic@1", Session: ManagedSessionCustody{SessionID: "managed", Generation: 7}, Seed: SeedCustody{URI: "file:///workspace/a.gts"}, Relations: []string{"PASSES_CALLBACK"}, Documents: DocumentCustody{OriginalURI: "file:///workspace/a.gts", WorkspaceRevision: json.RawMessage(`{"kind":"git","value":"` + revision + `","custody":"CALLER_ASSERTED"}`)}, Limits: CollectorLimits{MaxNodes: 20, MaxBytes: 2048, TimeoutMS: 1000}}
	return a, request, Receipt{ProviderID: "alpha@1", Response: raw, Messages: 1, Reaped: true}
}

func TestSemanticBindingEnvelopeStrict(t *testing.T) {
	a, request, receipt := semanticFixture(t)
	bad := receipt
	bad.Response = append(append(json.RawMessage(nil), receipt.Response[:len(receipt.Response)-1]...), []byte(`,"unknown":true}`)...)
	if _, err := a.Adapt(context.Background(), request, bad); err == nil {
		t.Fatal("ASSERT_PROVIDER_SEMANTIC_ENVELOPE_STRICT: accepted unknown field")
	}
}

func TestSemanticBindingIdentityConsistent(t *testing.T) {
	a, request, receipt := semanticFixture(t)
	receipt.ProviderID = "beta@1"
	if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
		t.Fatal("ASSERT_PROVIDER_SEMANTIC_IDENTITY_CONSISTENT: accepted receipt provider mismatch")
	}
}

func TestSemanticBindingCustodyConsistent(t *testing.T) {
	a, request, receipt := semanticFixture(t)
	var e observationadapter.Envelope
	_ = json.Unmarshal(receipt.Response, &e)
	e.Documents[0].OriginalURI = "file:///workspace/other.gts"
	receipt.Response, _ = json.Marshal(e)
	if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
		t.Fatal("ASSERT_PROVIDER_SEMANTIC_CUSTODY_CONSISTENT: accepted original URI mismatch")
	}
}

func TestSemanticBindingDelegatesObservationAdapter(t *testing.T) {
	a, request, receipt := semanticFixture(t)
	var e observationadapter.Envelope
	_ = json.Unmarshal(receipt.Response, &e)
	e.Observations[0].Kind = relations.RelationCalls
	receipt.Response, _ = json.Marshal(e)
	if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
		t.Fatal("ASSERT_PROVIDER_SEMANTIC_DELEGATES_OBSERVATION_ADAPTER: accepted forbidden CALLS observation")
	}
}

func TestSemanticBindingDelegatesAndProjectsExactReceipt(t *testing.T) {
	a, request, receipt := semanticFixture(t)
	raw, err := a.Adapt(context.Background(), request, receipt)
	if err != nil {
		t.Fatalf("ASSERT_PROVIDER_SEMANTIC_DELEGATES_OBSERVATION_ADAPTER: %v", err)
	}
	var got struct {
		ProviderID, Terminal string
		Complete, Truncated  bool
		Bounds               json.RawMessage
		Relations            []struct{ RelationID, Kind string }
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ProviderID != "alpha@1" || got.Terminal != "PARTIAL" || got.Complete || got.Truncated || len(got.Bounds) == 0 || len(got.Relations) != 1 || got.Relations[0].Kind != "PASSES_CALLBACK" || got.Relations[0].RelationID == "" {
		t.Fatalf("ASSERT_PROVIDER_SEMANTIC_COMPOSITION_PAYLOAD_EXACT: raw=%s decoded=%+v", raw, got)
	}
	if strings.Contains(string(raw), "ember") || strings.Contains(string(raw), "glimmer") {
		t.Fatalf("ASSERT_PROVIDER_NEUTRAL_NO_EMBER_PARSING: %s", raw)
	}
}
