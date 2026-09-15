package vcssymbolsidecar

import "testing"

func TestAttributePreservesClosedAccounting(t *testing.T) {
	changes := []ChangedLine{{Side: "NEW", Path: "a.go", Line: 4}, {Side: "NEW", Path: "a.go", Line: 8}, {Side: "NEW", Path: "a.go", Line: 12}}
	symbols := []Symbol{
		{Path: "a.go", Name: "outer", Kind: 12, Range: Range{Start: Position{Line: 0}, End: Position{Line: 10}}},
		{Path: "a.go", Name: "inner-a", Kind: 12, Range: Range{Start: Position{Line: 3}, End: Position{Line: 6}}},
		{Path: "a.go", Name: "inner-b", Kind: 12, Range: Range{Start: Position{Line: 3}, End: Position{Line: 6}}},
	}
	got, err := Attribute(changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if got.LineCount != 3 || got.AttributedLineCount != 1 || got.AmbiguousLineCount != 1 || got.UnmatchedLineCount != 1 {
		t.Fatalf("ASSERT_SYMBOL_ATTRIBUTION_CLOSED_ACCOUNTING: %+v", got)
	}
	if got.Lines[0].Outcome != "AMBIGUOUS" || got.Lines[1].SymbolName != "outer" || got.Lines[2].Outcome != "UNMATCHED" {
		t.Fatalf("ASSERT_SYMBOL_ATTRIBUTION_NARROWEST_OR_EXPLICIT: %+v", got.Lines)
	}
}

func TestAttributeHonorsLSPEndExclusiveRanges(t *testing.T) {
	changes := []ChangedLine{{Side: "NEW", Path: "a.go", Line: 4}}
	symbols := []Symbol{{Path: "a.go", Name: "A", Kind: 12, Range: Range{Start: Position{Line: 1}, End: Position{Line: 4, Character: 0}}}}
	got, err := Attribute(changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnmatchedLineCount != 1 || got.AttributedLineCount != 0 {
		t.Fatalf("ASSERT_SYMBOL_ATTRIBUTION_LSP_END_EXCLUSIVE: %+v", got)
	}
}

func TestAttributeUsesContainmentNotSyntheticSpanScalar(t *testing.T) {
	changes := []ChangedLine{{Side: "NEW", Path: "a.go", Line: 5}}
	symbols := []Symbol{
		{Path: "a.go", Name: "outer", Kind: 12, Range: Range{Start: Position{Line: 0}, End: Position{Line: 10}}},
		{Path: "a.go", Name: "inner", Kind: 12, Range: Range{Start: Position{Line: 5}, End: Position{Line: 5, Character: 20000000}}},
	}
	got, err := Attribute(changes, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if got.AttributedLineCount != 1 || got.Lines[0].SymbolName != "inner" {
		t.Fatalf("ASSERT_SYMBOL_ATTRIBUTION_RANGE_CONTAINMENT_NARROWEST: %+v", got)
	}
}

func TestValidateRejectsAuthorityOrAccountingMutation(t *testing.T) {
	valid := Result{SchemaVersion: SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", Attribution: "HISTORICAL_SYMBOL_RANGE", CrossRevisionIdentity: "NOT_EVALUATED", LineCount: 1, UnmatchedLineCount: 1, Lines: []LineAttribution{{Side: "NEW", Path: "a.go", Line: 1, Outcome: "UNMATCHED"}}}
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	valid.Authority = 1
	if err := Validate(valid); err == nil {
		t.Fatal("ASSERT_SYMBOL_ATTRIBUTION_AUTHORITY_ZERO")
	}
	valid.Authority = 0
	valid.LineCount = 2
	if err := Validate(valid); err == nil {
		t.Fatal("ASSERT_SYMBOL_ATTRIBUTION_ACCOUNTING_RECOMPUTED")
	}
}
