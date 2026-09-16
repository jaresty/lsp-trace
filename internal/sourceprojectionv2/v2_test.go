package sourceprojectionv2

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/sourceprojection"
)

type retainedBinding struct {
	Custody  string `json:"custody"`
	Artifact string `json:"artifact"`
}

func TestAssembleBoundedUsesCallerCustodyAndPreservesProjection(t *testing.T) {
	const assertion = "neutral assembler uses caller custody and preserves resolved projection"
	candidate := sourceprojection.Candidate{
		UnitID: "unit", CitationID: "citation", Role: "ENDPOINT", GraphSubjectID: "subject",
		LogicalSourceID: "file:///target.go", Range: testRange(1, 2, 3, 4), EvidenceRange: testRange(5, 6, 7, 8),
		ItemRange: testRange(9, 10, 11, 12), SelectionRange: testRange(13, 14, 15, 16),
		DisplayProvenance: "SERVER_REPORTED_DOCUMENT_SYMBOL", PositionEncoding: "utf-16", PrivacyClassification: "SOURCE",
	}
	projection := sourceprojection.Result{
		Status: "COMPLETE",
		Units: []sourceprojection.Unit{{
			UnitID: "unit", Role: "ENDPOINT", GraphSubjectID: "subject", LogicalSourceID: "file:///target.go",
			Range: candidate.Range, PositionEncoding: "utf-16", SourceDigest: "sha256:source", SourceByteLength: 17,
			SelectionDisposition: "SELECTED", BodyDisposition: "RETURNED", PrivacyClassification: "SOURCE", Body: "body",
		}},
		Citations:      []sourceprojection.Citation{{CitationID: "citation", UnitID: "unit", Role: "ENDPOINT", SubjectID: "subject"}},
		EmittedSpans:   []sourceprojection.Span{{LogicalSourceID: "file:///target.go", Range: candidate.Range, SourceDigest: "sha256:source", ByteLength: 17, UnitIDs: []string{"unit"}, Body: "body"}},
		Accounting:     sourceprojection.Accounting{Candidates: 1, Selected: 1, LogicalSelectedBytes: 4, UniqueEmittedBytes: 17, Evaluated: 1, Terminal: 1},
		Omissions:      []sourceprojection.Omission{},
		PrivacySummary: sourceprojection.PrivacySummary{PolicyID: "policy", BodyRequested: true, BodyReturned: 1},
	}
	binding := retainedBinding{Custody: "RETAINED", Artifact: "sha256:artifact"}
	got, err := AssembleBounded(Input{
		TargetURI: "file:///target.go", SelectedURIs: []string{"file:///target.go"},
		Documents:  []DocumentSource{{URI: "file:///target.go", DocumentVersion: 3, PositionEncoding: "utf-16", SourceDigest: "sha256:source", SourceByteLength: 17}},
		Candidates: candidateSlice(candidate), Projection: projection, DocumentsObserved: 1, TotalAcquiredBytes: 17,
		RequestPolicyID: "sha256:" + strings.Repeat("f", 64),
	}, "RETAINED", binding, 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	if got.CustodyMode != "RETAINED" || got.CustodyBinding != binding || got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 {
		t.Fatalf("%s: custody/neutrality=%+v", assertion, got)
	}
	if len(got.Units) != 1 || got.Units[0].EvidenceRange != candidate.EvidenceRange || got.Units[0].DisplayRange != candidate.Range || got.Units[0].ItemRange == nil || *got.Units[0].ItemRange != candidate.ItemRange || got.Units[0].SelectionRange == nil || *got.Units[0].SelectionRange != candidate.SelectionRange {
		t.Fatalf("%s: ranges=%+v", assertion, got.Units)
	}
	if !reflect.DeepEqual(got.EmittedSpans, projection.EmittedSpans) || !reflect.DeepEqual(got.Omissions, projection.Omissions) || got.Accounting != projection.Accounting || got.PrivacySummary != projection.PrivacySummary {
		t.Fatalf("%s: projection=%+v", assertion, got)
	}
}

func TestAssembleBoundedUsesExactResponseBudget(t *testing.T) {
	const assertion = "neutral assembler enforces exact marshaled response budget"
	input := Input{
		TargetURI: "file:///target.go", SelectedURIs: []string{"file:///target.go"},
		Documents:  []DocumentSource{{URI: "file:///target.go", PositionEncoding: "utf-16", SourceDigest: "sha256:source", SourceByteLength: 1}},
		Candidates: []sourceprojection.Candidate{}, Projection: sourceprojection.Result{Status: "SUCCESSFUL_EMPTY", Units: []sourceprojection.Unit{}, Citations: []sourceprojection.Citation{}, EmittedSpans: []sourceprojection.Span{}, Omissions: []sourceprojection.Omission{}},
		DocumentsObserved: 1, TotalAcquiredBytes: 1, RequestPolicyID: "policy",
	}
	binding := retainedBinding{Custody: "RETAINED", Artifact: "artifact"}
	got, err := AssembleBounded(input, "RETAINED", binding, 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AssembleBounded(input, "RETAINED", binding, len(raw)); err != nil {
		t.Fatalf("%s: exact limit rejected: %v", assertion, err)
	}
	if limited, err := AssembleBounded(input, "RETAINED", binding, len(raw)-1); err == nil || limited.SchemaVersion != "" {
		t.Fatalf("%s: result=%+v err=%v", assertion, limited, err)
	}
}

func candidateSlice(candidate sourceprojection.Candidate) []sourceprojection.Candidate {
	return []sourceprojection.Candidate{candidate}
}

func testRange(sl, sc, el, ec uint32) sourceprojection.Range {
	return sourceprojection.Range{Start: sourceprojection.Position{Line: sl, Character: sc}, End: sourceprojection.Position{Line: el, Character: ec}}
}
