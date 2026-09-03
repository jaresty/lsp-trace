package schema

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestValidationCoreRetrievalReturnsIndependentExactBytes(t *testing.T) {
	for _, tc := range []struct{ family, version string }{
		{FamilyGraph, "v1"}, {FamilyGraph, "v2"}, {FamilyGraph, "v3"},
		{FamilyInspect, "v1"}, {FamilyFilter, "v1"},
	} {
		first, err := BytesFor(tc.family, tc.version)
		if err != nil {
			t.Fatalf("ASSERT_CORE_EXACT_RETRIEVAL %s/%s: %v", tc.family, tc.version, err)
		}
		original := append([]byte(nil), first...)
		first[0] ^= 0xff
		second, err := BytesFor(tc.family, tc.version)
		if err != nil || !bytes.Equal(second, original) {
			t.Fatalf("ASSERT_CORE_RETRIEVAL_COPY %s/%s: err=%v equal=%v", tc.family, tc.version, err, bytes.Equal(second, original))
		}
	}
}

func TestValidationCoreSeparatesStructureFromFamilySemantics(t *testing.T) {
	mutated := strings.Replace(validFilterV1, `"pair_universe_count":0`, `"pair_universe_count":99`, 1)
	structural, err := ValidateStructure([]byte(mutated), FamilyFilter, "v1")
	if err != nil || structural.Family != FamilyFilter || structural.Version != "lsp-trace.filter.v1" {
		t.Fatalf("ASSERT_CORE_STRUCTURE_ACCEPTS_SEMANTIC_MUTATION: result=%+v err=%v", structural, err)
	}
	if err := ValidateSemantics([]byte(mutated), structural); err == nil || !strings.Contains(err.Error(), "semantic validation lsp-trace.filter.v1") {
		t.Fatalf("ASSERT_CORE_FILTER_SEMANTIC_DISPATCH: %v", err)
	}

	invalid := []byte(`{"filter_schema_version":"lsp-trace.filter.v1"}`)
	if _, err := ValidateStructure(invalid, FamilyFilter, "v1"); err == nil || !strings.Contains(err.Error(), "schema validation lsp-trace.filter.v1") || strings.Contains(err.Error(), "semantic validation") {
		t.Fatalf("ASSERT_CORE_STRUCTURE_PRECEDES_SEMANTICS: %v", err)
	}
}

func TestValidationCoreRejectsForgedStructuralResult(t *testing.T) {
	err := ValidateSemantics([]byte(`{}`), StructuralResult{Family: FamilyGraph, Version: "lsp-trace.graph.v1"})
	if err == nil || !strings.Contains(err.Error(), "structural validation required") {
		t.Fatalf("ASSERT_CORE_STRUCTURAL_RESULT_UNFORGEABLE: %v", err)
	}
}

func TestValidationCoreHistoricalGraphSemanticsRemainNoOp(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		data := minimalHistoricalGraph(version)
		structural, err := ValidateStructure(data, FamilyGraph, version)
		if err != nil {
			t.Fatalf("ASSERT_CORE_HISTORICAL_STRUCTURE %s: %v", version, err)
		}
		if err := ValidateSemantics(data, structural); err != nil {
			t.Fatalf("ASSERT_CORE_HISTORICAL_SEMANTICS_NOOP %s: %v", version, err)
		}
	}
}

func TestValidationCoreConcurrentDeterministicDiagnostics(t *testing.T) {
	const workers = 32
	results := make(chan string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ValidateFor([]byte(`{"filter_schema_version":"lsp-trace.filter.v1"}`), FamilyFilter, "v1")
			results <- fmt.Sprint(err)
		}()
	}
	wg.Wait()
	close(results)
	var first string
	for got := range results {
		if first == "" {
			first = got
		}
		if got != first || !strings.Contains(got, "schema validation lsp-trace.filter.v1") {
			t.Fatalf("ASSERT_CORE_CONCURRENT_DETERMINISM: first=%q got=%q", first, got)
		}
	}
}

func minimalHistoricalGraph(version string) []byte {
	full := "lsp-trace.graph." + version
	capabilityQuality := ""
	if version == "v2" {
		capabilityQuality = `,"capability_quality":{}`
	}
	return []byte(fmt.Sprintf(`{"schema_version":%q,"invocation":{},"capabilities":{}%s,"targets":[],"nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{}}`, full, capabilityQuality))
}
