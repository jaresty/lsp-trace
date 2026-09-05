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
	a, err := NewObservationSemanticAdapter(p, identity, RevisionVerifierFunc(func(context.Context, StrictCollectorRequest, graph.SourceDocumentRecord) error { return nil }))
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
	for name, mutate := range map[string]func(json.RawMessage) json.RawMessage{
		"unknown-field": func(raw json.RawMessage) json.RawMessage {
			return append(append(json.RawMessage(nil), raw[:len(raw)-1]...), []byte(`,"unknown":true}`)...)
		},
		"trailing-value": func(raw json.RawMessage) json.RawMessage {
			return append(append(json.RawMessage(nil), raw...), []byte(` {}`)...)
		},
		"malformed": func(raw json.RawMessage) json.RawMessage { return raw[:len(raw)-1] },
	} {
		t.Run(name, func(t *testing.T) {
			a, request, receipt := semanticFixture(t)
			receipt.Response = mutate(receipt.Response)
			if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
				t.Fatalf("ASSERT_PROVIDER_SEMANTIC_ENVELOPE_STRICT: accepted %s", name)
			}
		})
	}
}

func TestSemanticBindingIdentityConsistent(t *testing.T) {
	for name, mutate := range map[string]func(*StrictCollectorRequest, *Receipt, *observationadapter.Envelope){
		"receipt-provider": func(_ *StrictCollectorRequest, r *Receipt, _ *observationadapter.Envelope) { r.ProviderID = "beta@1" },
		"provider":         func(_ *StrictCollectorRequest, _ *Receipt, e *observationadapter.Envelope) { e.Provider.Version = "2" },
		"protocol":         func(_ *StrictCollectorRequest, _ *Receipt, e *observationadapter.Envelope) { e.Protocol.Version = "2" },
		"adapter":          func(_ *StrictCollectorRequest, _ *Receipt, e *observationadapter.Envelope) { e.Adapter.Version = "2" },
		"request":          func(_ *StrictCollectorRequest, _ *Receipt, e *observationadapter.Envelope) { e.RequestID = "other" },
		"relation": func(_ *StrictCollectorRequest, _ *Receipt, e *observationadapter.Envelope) {
			e.Observations[0].Kind = relations.RelationRendersFrom
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, request, receipt := semanticFixture(t)
			var envelope observationadapter.Envelope
			_ = json.Unmarshal(receipt.Response, &envelope)
			mutate(&request, &receipt, &envelope)
			receipt.Response, _ = json.Marshal(envelope)
			_, err := a.Adapt(context.Background(), request, receipt)
			if err == nil {
				t.Fatalf("ASSERT_PROVIDER_SEMANTIC_IDENTITY_CONSISTENT: accepted %s mismatch", name)
			}
			if name == "protocol" && err.Error() != "observation protocol identity/version mismatch" {
				t.Fatalf("ASSERT_PROVIDER_SEMANTIC_IDENTITY_CONSISTENT: protocol mismatch escaped provider boundary: %v", err)
			}
		})
	}
}

func TestSemanticBindingCustodyConsistent(t *testing.T) {
	t.Run("original-uri", func(t *testing.T) {
		a, request, receipt := semanticFixture(t)
		var e observationadapter.Envelope
		_ = json.Unmarshal(receipt.Response, &e)
		e.Documents[0].OriginalURI = "file:///workspace/other.gts"
		receipt.Response, _ = json.Marshal(e)
		if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal("ASSERT_PROVIDER_SEMANTIC_ORIGINAL_DOCUMENT_CUSTODY: accepted original URI mismatch")
		}
	})
	t.Run("workspace-revision", func(t *testing.T) {
		a, request, receipt := semanticFixture(t)
		var e observationadapter.Envelope
		_ = json.Unmarshal(receipt.Response, &e)
		e.Documents[0].Revision.Value = strings.Repeat("d", 40)
		receipt.Response, _ = json.Marshal(e)
		if _, err := a.Adapt(context.Background(), request, receipt); err == nil {
			t.Fatal("ASSERT_PROVIDER_SEMANTIC_REVISION_CONSISTENT: accepted workspace revision mismatch")
		}
	})
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
	var got Result
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ProviderID != "alpha@1" || got.Terminal != "PARTIAL" || got.Complete || got.Truncated || got.Bounds.MaxNodes == 0 || len(got.Observations) != 1 || len(got.GraphV4.Relations) != 1 || got.GraphV4.Relations[0].Kind != "PASSES_CALLBACK" || got.GraphV4.Relations[0].RelationID == "" || got.LogicalDigest == "" {
		t.Fatalf("ASSERT_PROVIDER_SEMANTIC_COMPOSITION_PAYLOAD_EXACT: raw=%s decoded=%+v", raw, got)
	}
	if strings.Contains(string(raw), "ember") || strings.Contains(string(raw), "glimmer") {
		t.Fatalf("ASSERT_PROVIDER_NEUTRAL_NO_EMBER_PARSING: %s", raw)
	}
}
