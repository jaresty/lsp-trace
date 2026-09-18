package liveprojection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/sessionruntime"
)

type displayRequester struct{ result json.RawMessage }

func (r displayRequester) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	return sessionruntime.RoundTripResult{Result: r.result}
}

func TestResolveDisplayRangesPreservesEvidenceRoles(t *testing.T) {
	selection := sourceprojection.Range{Start: sourceprojection.Position{Line: 10, Character: 5}, End: sourceprojection.Position{Line: 10, Character: 12}}
	full := sourceprojection.Range{Start: sourceprojection.Position{Line: 10}, End: sourceprojection.Position{Line: 24, Character: 1}}
	raw, _ := json.Marshal([]documentSymbol{{Name: "Project", Range: full, SelectionRange: selection}})
	candidate := sourceprojection.Candidate{UnitID: "u", Role: "ENDPOINT", LogicalSourceID: "file:///x.go", Range: selection, EvidenceRange: selection, ItemRange: selection, SelectionRange: selection}
	got, err := ResolveDisplayRanges(context.Background(), displayRequester{raw}, "s", 1, []sourceprojection.Candidate{candidate}, DisplayResolutionLimits{MaxWork: 1, MaxMessages: 1, MaxBytes: 4096, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Range != full {
		t.Fatalf("ASSERT_FULL_DEFINITION_RANGE got %#v", got[0].Range)
	}
	if got[0].EvidenceRange != selection || got[0].ItemRange != selection || got[0].SelectionRange != selection {
		t.Fatal("ASSERT_EXACT_RANGES_PRESERVED")
	}
}

func TestResolveEndpointUsesSmallestContainingSymbolWhenExactDefinitionIsUnavailable(t *testing.T) {
	selection := sourceprojection.Range{Start: sourceprojection.Position{Line: 15, Character: 3}, End: sourceprojection.Position{Line: 15, Character: 18}}
	outer := sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 40}}
	inner := sourceprojection.Range{Start: sourceprojection.Position{Line: 12}, End: sourceprojection.Position{Line: 20}}
	raw, _ := json.Marshal([]documentSymbol{{Name: "T", Range: outer, SelectionRange: outer, Children: []documentSymbol{{Name: "Execute", Range: inner, SelectionRange: inner}}}})
	candidate := sourceprojection.Candidate{UnitID: "u", Role: "ENDPOINT", LogicalSourceID: "file:///x.go", Range: selection, EvidenceRange: selection, ItemRange: selection, SelectionRange: selection, DisplayProvenance: "CALL_HIERARCHY_ITEM"}
	got, err := ResolveDisplayRanges(context.Background(), displayRequester{raw}, "s", 1, []sourceprojection.Candidate{candidate}, DisplayResolutionLimits{MaxWork: 1, MaxMessages: 1, MaxBytes: 4096, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Range != inner || got[0].DisplayProvenance != "SERVER_REPORTED_DOCUMENT_SYMBOL" || got[0].EvidenceRange != selection {
		t.Fatalf("ASSERT_ENDPOINT_CONTAINING_DEFINITION: %+v", got[0])
	}
}

func TestResolveEndpointPreservesCallHierarchyItemWhenDocumentSymbolIsUnavailable(t *testing.T) {
	selection := sourceprojection.Range{Start: sourceprojection.Position{Line: 50, Character: 3}, End: sourceprojection.Position{Line: 50, Character: 18}}
	raw, _ := json.Marshal([]documentSymbol{{Name: "Other", Range: sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 10}}, SelectionRange: sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 1, Character: 5}}}})
	candidate := sourceprojection.Candidate{UnitID: "u", Role: "ENDPOINT", LogicalSourceID: "file:///x.go", Range: selection, EvidenceRange: selection, ItemRange: selection, SelectionRange: selection, DisplayProvenance: "CALL_HIERARCHY_ITEM"}
	got, err := ResolveDisplayRanges(context.Background(), displayRequester{raw}, "s", 1, []sourceprojection.Candidate{candidate}, DisplayResolutionLimits{MaxWork: 1, MaxMessages: 1, MaxBytes: 4096, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Range != selection || got[0].DisplayProvenance != "CALL_HIERARCHY_ITEM" || got[0].EvidenceRange != selection {
		t.Fatalf("ASSERT_ENDPOINT_CALL_HIERARCHY_FALLBACK: %+v", got[0])
	}
}

func TestResolveRelationUsesSmallestContainingSymbol(t *testing.T) {
	call := sourceprojection.Range{Start: sourceprojection.Position{Line: 15, Character: 3}, End: sourceprojection.Position{Line: 15, Character: 10}}
	outer := sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 40}}
	inner := sourceprojection.Range{Start: sourceprojection.Position{Line: 12}, End: sourceprojection.Position{Line: 20}}
	raw, _ := json.Marshal([]documentSymbol{{Name: "T", Range: outer, SelectionRange: outer, Children: []documentSymbol{{Name: "Caller", Range: inner, SelectionRange: inner}}}})
	candidate := sourceprojection.Candidate{UnitID: "r", Role: "RELATION", LogicalSourceID: "file:///x.go", Range: call, EvidenceRange: call}
	got, err := ResolveDisplayRanges(context.Background(), displayRequester{raw}, "s", 1, []sourceprojection.Candidate{candidate}, DisplayResolutionLimits{MaxWork: 1, MaxMessages: 1, MaxBytes: 4096, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Range != inner {
		t.Fatalf("ASSERT_RELATION_CALLER_DEFINITION got %#v", got[0].Range)
	}
	if got[0].EvidenceRange != call {
		t.Fatal("ASSERT_RELATION_EVIDENCE_PRESERVED")
	}
}
