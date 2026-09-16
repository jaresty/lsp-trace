package vcssymbolsidecar

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

type metricProjection struct {
	SchemaVersion           string         `json:"schema_version"`
	CrossRevisionIdentity   string         `json:"cross_revision_identity"`
	LineCount               int            `json:"line_count"`
	AttributedLineCount     int            `json:"attributed_line_count"`
	AmbiguousLineCount      int            `json:"ambiguous_line_count"`
	UnmatchedLineCount      int            `json:"unmatched_line_count"`
	HistoricalSymbolMetrics []symbolMetric `json:"historical_symbol_metrics"`
	CurrentSymbolMetrics    []symbolMetric `json:"current_symbol_metrics"`
}

type symbolMetric struct {
	Path                    string `json:"path"`
	Name                    string `json:"name"`
	Kind                    int    `json:"kind"`
	Range                   Range  `json:"range"`
	ChangedLineCount        int    `json:"changed_line_count"`
	SymbolSpanLineCount     int    `json:"symbol_span_line_count"`
	ChurnDensityBasisPoints int    `json:"churn_density_basis_points"`
}

func projectMetrics(t *testing.T, changes []ChangedLine, symbols []Symbol) (metricProjection, error) {
	t.Helper()
	got, err := Attribute(changes, symbols)
	if err != nil {
		return metricProjection{}, err
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var projected metricProjection
	if err := json.Unmarshal(raw, &projected); err != nil {
		t.Fatal(err)
	}
	return projected, nil
}

func TestAttributeV3PerRevisionDensityBoundaries(t *testing.T) {
	changes := []ChangedLine{
		{Side: "NEW", Path: "a.go", Line: 3},
		{Side: "NEW", Path: "a.go", Line: 3},
		{Side: "OLD", Path: "a.go", Line: 8},
		{Side: "NEW", Path: "a.go", Line: 11},
	}
	symbols := []Symbol{
		{Side: "NEW", Path: "a.go", Name: "single", Kind: 12, Range: Range{Start: Position{Line: 3, Character: 2}, End: Position{Line: 3, Character: 9}}},
		{Side: "OLD", Path: "a.go", Name: "same-name", Kind: 12, Range: Range{Start: Position{Line: 7}, End: Position{Line: 10}}},
		{Side: "NEW", Path: "a.go", Name: "same-name", Kind: 12, Range: Range{Start: Position{Line: 10}, End: Position{Line: 13, Character: 2}}},
	}
	got, err := projectMetrics(t, changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != "lsp-trace.vcs-symbol-churn-sidecar.v3" {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_SCHEMA_VERSION: got=%q", got.SchemaVersion)
	}
	if got.CrossRevisionIdentity != "NOT_EVALUATED" || len(got.HistoricalSymbolMetrics) != 1 || len(got.CurrentSymbolMetrics) != 2 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_REVISION_SEPARATION: identity=%q historical=%+v current=%+v", got.CrossRevisionIdentity, got.HistoricalSymbolMetrics, got.CurrentSymbolMetrics)
	}
	if got.CurrentSymbolMetrics[0].Name != "single" || got.CurrentSymbolMetrics[0].ChangedLineCount != 1 || got.CurrentSymbolMetrics[0].SymbolSpanLineCount != 1 || got.CurrentSymbolMetrics[0].ChurnDensityBasisPoints != 10000 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_SINGLE_LINE_AND_DUPLICATE_DEDUP: %+v", got.CurrentSymbolMetrics[0])
	}
	if got.HistoricalSymbolMetrics[0].ChangedLineCount != 1 || got.HistoricalSymbolMetrics[0].SymbolSpanLineCount != 3 || got.HistoricalSymbolMetrics[0].ChurnDensityBasisPoints != 3333 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_END_CHARACTER_ZERO_SPAN: %+v", got.HistoricalSymbolMetrics[0])
	}
	if got.CurrentSymbolMetrics[1].ChangedLineCount != 1 || got.CurrentSymbolMetrics[1].SymbolSpanLineCount != 4 || got.CurrentSymbolMetrics[1].ChurnDensityBasisPoints != 2500 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_MULTILINE_END_CHARACTER_NONZERO_SPAN: %+v", got.CurrentSymbolMetrics[1])
	}
	if got.LineCount != 4 || got.AttributedLineCount != 4 || got.AmbiguousLineCount != 0 || got.UnmatchedLineCount != 0 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_DUPLICATE_LEDGER_ACCOUNTING_PRESERVED: %+v", got)
	}
}

func TestAttributeV3PreservesAmbiguousAndUnmatchedOutsideMetrics(t *testing.T) {
	changes := []ChangedLine{{Side: "NEW", Path: "a.go", Line: 4}, {Side: "NEW", Path: "a.go", Line: 20}}
	symbols := []Symbol{
		{Side: "NEW", Path: "a.go", Name: "equal-a", Kind: 12, Range: Range{Start: Position{Line: 3}, End: Position{Line: 6}}},
		{Side: "NEW", Path: "a.go", Name: "equal-b", Kind: 12, Range: Range{Start: Position{Line: 3}, End: Position{Line: 6}}},
	}
	got, err := projectMetrics(t, changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if got.AttributedLineCount != 0 || got.AmbiguousLineCount != 1 || got.UnmatchedLineCount != 1 || len(got.CurrentSymbolMetrics) != 0 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_AMBIGUOUS_UNMATCHED_EXCLUDED: %+v", got)
	}
}

func TestAttributeV3PreservesIncomparableOverlapAmbiguity(t *testing.T) {
	got, err := projectMetrics(t,
		[]ChangedLine{{Side: "NEW", Path: "a.go", Line: 5}},
		[]Symbol{
			{Side: "NEW", Path: "a.go", Name: "left", Kind: 12, Range: Range{Start: Position{Line: 0}, End: Position{Line: 6}}},
			{Side: "NEW", Path: "a.go", Name: "right", Kind: 12, Range: Range{Start: Position{Line: 4}, End: Position{Line: 10}}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.AttributedLineCount != 0 || got.AmbiguousLineCount != 1 || len(got.CurrentSymbolMetrics) != 0 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_INCOMPARABLE_OVERLAP_AMBIGUOUS: %+v", got)
	}
}

func TestAttributeV3RejectsInvalidRangeAndDoesNotDivideZeroSpan(t *testing.T) {
	_, err := projectMetrics(t,
		[]ChangedLine{{Side: "NEW", Path: "a.go", Line: 3}},
		[]Symbol{{Side: "NEW", Path: "a.go", Name: "reversed", Kind: 12, Range: Range{Start: Position{Line: 3, Character: 9}, End: Position{Line: 3, Character: 2}}}},
	)
	if err == nil {
		t.Fatal("ASSERT_SYMBOL_CHURN_V3_INVALID_RANGE_FAILS_CLOSED: accepted reversed same-line range")
	}

	got, err := projectMetrics(t,
		[]ChangedLine{{Side: "NEW", Path: "a.go", Line: 3}},
		[]Symbol{{Side: "NEW", Path: "a.go", Name: "zero", Kind: 12, Range: Range{Start: Position{Line: 3}, End: Position{Line: 3}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnmatchedLineCount != 1 || len(got.CurrentSymbolMetrics) != 0 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_ZERO_SPAN_NOT_ELIGIBLE: %+v", got)
	}
}

func TestAttributeV3DeterministicUnderInputPermutations(t *testing.T) {
	changes := []ChangedLine{{Side: "NEW", Path: "b.go", Line: 5}, {Side: "OLD", Path: "a.go", Line: 1}, {Side: "NEW", Path: "a.go", Line: 9}}
	symbols := []Symbol{
		{Side: "NEW", Path: "b.go", Name: "B", Kind: 12, Range: Range{Start: Position{Line: 5}, End: Position{Line: 6}}},
		{Side: "NEW", Path: "a.go", Name: "Z", Kind: 12, Range: Range{Start: Position{Line: 9}, End: Position{Line: 10}}},
		{Side: "OLD", Path: "a.go", Name: "A", Kind: 12, Range: Range{Start: Position{Line: 1}, End: Position{Line: 2}}},
	}
	first, err := projectMetrics(t, changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	reversedChanges := []ChangedLine{changes[2], changes[1], changes[0]}
	reversedSymbols := []Symbol{symbols[2], symbols[1], symbols[0]}
	second, err := projectMetrics(t, reversedChanges, reversedSymbols)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.HistoricalSymbolMetrics, second.HistoricalSymbolMetrics) || !reflect.DeepEqual(first.CurrentSymbolMetrics, second.CurrentSymbolMetrics) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_PERMUTATION_DETERMINISM: first=%+v second=%+v", first, second)
	}
	if len(first.CurrentSymbolMetrics) != 2 || first.CurrentSymbolMetrics[0].Path != "a.go" || first.CurrentSymbolMetrics[1].Path != "b.go" {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_EXACT_IDENTITY_ORDER: %+v", first.CurrentSymbolMetrics)
	}
}

func TestAttributeV3MaximumSafeSpanArithmetic(t *testing.T) {
	if density, err := densityBasisPoints(math.MaxInt, math.MaxInt); err != nil || density != 10000 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_MAXIMUM_MULTIPLICATION_ARITHMETIC: density=%d err=%v", density, err)
	}
	got, err := projectMetrics(t,
		[]ChangedLine{{Side: "NEW", Path: "a.go", Line: math.MaxInt - 1}},
		[]Symbol{{Side: "NEW", Path: "a.go", Name: "huge", Kind: 12, Range: Range{Start: Position{}, End: Position{Line: math.MaxInt}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CurrentSymbolMetrics) != 1 || got.CurrentSymbolMetrics[0].SymbolSpanLineCount != math.MaxInt || got.CurrentSymbolMetrics[0].ChurnDensityBasisPoints != 0 {
		t.Fatalf("ASSERT_SYMBOL_CHURN_V3_MAXIMUM_SAFE_ARITHMETIC: %+v", got.CurrentSymbolMetrics)
	}
}
