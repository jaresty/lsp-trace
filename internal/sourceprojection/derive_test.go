package sourceprojection

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/transientstructural"
)

func derivationResult() transientstructural.Result {
	r := graph.Range{Start: graph.Position{Line: 1, Character: 2}, End: graph.Position{Line: 1, Character: 5}}
	return transientstructural.Result{TargetID: "root", Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"}, Analysis: transientstructural.AnalysisResult{
		Nodes:       []transientstructural.NodeFact{{ID: "other", URI: "file:///w/b.go", Range: r}, {ID: "root", URI: "file:///w/a.go", Range: r}},
		Occurrences: []transientstructural.OccurrenceFact{{ID: "occ", CallerID: "root", CalleeID: "other", URI: "file:///w/a.go", Range: r}},
	}}
}

func TestDeriveCandidateIdentityIsEncodingSensitive(t *testing.T) {
	utf16 := derivationResult()
	utf8 := derivationResult()
	utf8.Qualification.PositionEncoding = "utf-8"
	one, err := DeriveCandidates(utf16, "TARGET", false)
	if err != nil {
		t.Fatal(err)
	}
	two, err := DeriveCandidates(utf8, "TARGET", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || len(two) != 1 || one[0].UnitID == two[0].UnitID || one[0].CitationID == two[0].CitationID {
		t.Fatalf("ASSERT_C01_IDENTITY_FIELD_SENSITIVITY: utf16=%+v utf8=%+v", one, two)
	}
}

func TestDeriveCandidatesPreservesDistinctEndpointAndRelationRanges(t *testing.T) {
	itemRange := graph.Range{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 6, Character: 1}}
	selectionRange := graph.Range{Start: graph.Position{Line: 2, Character: 7}, End: graph.Position{Line: 2, Character: 13}}
	occurrenceRange := graph.Range{Start: graph.Position{Line: 9, Character: 4}, End: graph.Position{Line: 9, Character: 10}}
	result := transientstructural.Result{
		TargetID:      "root",
		Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"},
		Analysis: transientstructural.AnalysisResult{
			Nodes: []transientstructural.NodeFact{{
				ID: "root", URI: "file:///w/a.go", Range: itemRange,
				ItemRange: itemRange, SelectionRange: selectionRange,
			}},
			Occurrences: []transientstructural.OccurrenceFact{{
				ID: "occ", CallerID: "root", CalleeID: "other", URI: "file:///w/a.go", Range: occurrenceRange,
			}},
		},
	}
	candidates, err := DeriveCandidates(result, "PROJECTED", true)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("ASSERT_PROJECTION_CANDIDATE_PRESERVES_RANGE_ROLES: candidates=%+v err=%v", candidates, err)
	}
	for _, candidate := range candidates {
		value := reflect.ValueOf(candidate)
		evidence := value.FieldByName("EvidenceRange")
		item := value.FieldByName("ItemRange")
		selection := value.FieldByName("SelectionRange")
		if !evidence.IsValid() || !item.IsValid() || !selection.IsValid() {
			t.Fatalf("ASSERT_PROJECTION_CANDIDATE_PRESERVES_RANGE_ROLES: role=%s evidence=%t item=%t selection=%t", candidate.Role, evidence.IsValid(), item.IsValid(), selection.IsValid())
		}
		if candidate.Role == "ENDPOINT" {
			if evidence.Interface().(Range) != projectionRange(selectionRange) || item.Interface().(Range) != projectionRange(itemRange) || selection.Interface().(Range) != projectionRange(selectionRange) {
				t.Fatalf("ASSERT_PROJECTION_ENDPOINT_EVIDENCE_ITEM_SELECTION_DISTINCT: %+v", candidate)
			}
		} else if evidence.Interface().(Range) != projectionRange(occurrenceRange) || item.Interface().(Range) != (Range{}) || selection.Interface().(Range) != (Range{}) {
			t.Fatalf("ASSERT_PROJECTION_RELATION_CALL_SITE_REMAINS_EVIDENCE: %+v", candidate)
		}
	}
}

func TestDeriveWorkspaceCandidatesMatchesCanonicalWorkspaceAdmission(t *testing.T) {
	workspace := t.TempDir()
	external := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	externalPath := filepath.Join(external, "context.go")
	if err := os.WriteFile(externalPath, []byte("package context\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileURI := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	r := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}
	result := transientstructural.Result{
		TargetID:      "root",
		Qualification: transientstructural.Qualification{PositionEncoding: "utf-16"},
		Analysis: transientstructural.AnalysisResult{
			Nodes: []transientstructural.NodeFact{
				{ID: "root", URI: fileURI(filepath.Join(workspace, "a.go")), Range: r},
				{ID: "peer", URI: fileURI(filepath.Join(workspace, "b.go")), Range: r},
				{ID: "external", URI: fileURI(externalPath), Range: r},
			},
			Occurrences: []transientstructural.OccurrenceFact{
				{ID: "local", CallerID: "root", CalleeID: "peer", URI: fileURI(filepath.Join(workspace, "a.go")), Range: r},
				{ID: "touches-external", CallerID: "root", CalleeID: "external", URI: fileURI(filepath.Join(workspace, "a.go")), Range: r},
			},
		},
	}

	candidates, err := DeriveWorkspaceCandidates(result, "PROJECTED", true, workspace)
	if err != nil {
		t.Fatalf("ASSERT_WORKSPACE_CANDIDATES_CANONICAL_ADMISSION: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("ASSERT_WORKSPACE_CANDIDATES_EXCLUDE_EXTERNAL_ENDPOINTS: %+v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.GraphSubjectID == "external" || candidate.LogicalSourceID == fileURI(externalPath) {
			t.Fatalf("ASSERT_WORKSPACE_CANDIDATES_EXCLUDE_EXTERNAL_ENDPOINTS: %+v", candidate)
		}
	}

	outsideOccurrence := result
	outsideOccurrence.Analysis.Occurrences = append([]transientstructural.OccurrenceFact(nil), result.Analysis.Occurrences...)
	outsideOccurrence.Analysis.Occurrences[0].URI = fileURI(externalPath)
	if _, err := DeriveWorkspaceCandidates(outsideOccurrence, "PROJECTED", true, workspace); err == nil {
		t.Fatal("ASSERT_WORKSPACE_CANDIDATES_OUTSIDE_CALLSITE_FAILS_CLOSED: got nil error")
	}

	externalTarget := result
	externalTarget.TargetID = "external"
	if _, err := DeriveWorkspaceCandidates(externalTarget, "TARGET", false, workspace); err == nil {
		t.Fatal("ASSERT_WORKSPACE_CANDIDATES_TARGET_MUST_BE_ADMITTED: got nil error")
	}

	for name, raw := range map[string]string{
		"malformed":  "https://example.invalid/a.go",
		"unresolved": fileURI(filepath.Join(workspace, "missing.go")),
	} {
		t.Run(name, func(t *testing.T) {
			invalid := result
			invalid.Analysis.Nodes = append([]transientstructural.NodeFact(nil), result.Analysis.Nodes...)
			invalid.Analysis.Nodes[0].URI = raw
			if _, err := DeriveWorkspaceCandidates(invalid, "PROJECTED", true, workspace); err == nil {
				t.Fatal("ASSERT_WORKSPACE_CANDIDATES_INVALID_URI_FAILS_CLOSED: got nil error")
			}
		})
	}
}

func TestDeriveCandidatesFromAdmittedTransientFacts(t *testing.T) {
	target, err := DeriveCandidates(derivationResult(), "TARGET", true)
	if err != nil || len(target) != 1 || target[0].Role != "ENDPOINT" || target[0].GraphSubjectID != "root" {
		t.Fatalf("ASSERT_PROJECTION_DERIVE_TARGET: candidates=%+v err=%v", target, err)
	}
	projected, err := DeriveCandidates(derivationResult(), "PROJECTED", true)
	if err != nil || len(projected) != 3 {
		t.Fatalf("ASSERT_PROJECTION_DERIVE_PROJECTED: candidates=%+v err=%v", projected, err)
	}
	for _, candidate := range projected {
		if !strings.HasPrefix(candidate.UnitID, "sha256:") || !strings.HasPrefix(candidate.CitationID, "sha256:") {
			t.Fatalf("ASSERT_PROJECTION_DERIVE_IDENTITIES: %+v", candidate)
		}
		if candidate.Role == "RELATION" && candidate.RelationProvenance != "SERVER_REPORTED" {
			t.Fatalf("ASSERT_PROJECTION_DERIVE_SERVER_ONLY: %+v", candidate)
		}
	}
	withoutRelations, err := DeriveCandidates(derivationResult(), "PROJECTED", false)
	if err != nil || len(withoutRelations) != 2 {
		t.Fatalf("ASSERT_PROJECTION_DERIVE_RELATION_OPT_IN: candidates=%+v err=%v", withoutRelations, err)
	}
}
