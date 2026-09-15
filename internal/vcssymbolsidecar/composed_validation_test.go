package vcssymbolsidecar

import (
	"strings"
	"testing"
)

func validComposedFixture() Result {
	return Result{
		SchemaVersion: SchemaVersion, GraphArtifactDigest: "sha256:" + strings.Repeat("a", 64),
		FromRevision: strings.Repeat("b", 40), ToRevision: strings.Repeat("c", 40),
		OldAcquisition: []FileOutcome{{Path: "a.go", Status: "EMPTY", Symbols: []Symbol{}}},
		NewAcquisition: []FileOutcome{{Path: "a.go", Status: "COMPLETE", Symbols: []Symbol{{Path: "a.go", Name: "A", Kind: 12}}}},
		Authority:      0, SourceGraphComplete: "UNKNOWN", Attribution: "HISTORICAL_SYMBOL_RANGE", CrossRevisionIdentity: "NOT_EVALUATED",
		LineCount: 1, AttributedLineCount: 1,
		Lines: []LineAttribution{{Side: "NEW", Path: "a.go", Line: 1, Outcome: "ATTRIBUTED", SymbolName: "A", SymbolKind: 12, SymbolRange: &Range{}}},
	}
}

func TestValidateComposedRejectsMalformedAcquisitionLedger(t *testing.T) {
	cases := map[string]func(*Result){
		"unknown-status":       func(r *Result) { r.OldAcquisition[0].Status = "UNKNOWN" },
		"error-on-success":     func(r *Result) { r.NewAcquisition[0].Error = "hidden" },
		"symbols-on-empty":     func(r *Result) { r.OldAcquisition[0].Symbols = []Symbol{{Path: "a.go", Name: "X"}} },
		"symbol-path-mismatch": func(r *Result) { r.NewAcquisition[0].Symbols[0].Path = "b.go" },
		"path-set-mismatch":    func(r *Result) { r.NewAcquisition[0].Path = "b.go" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validComposedFixture()
			mutate(&r)
			if err := ValidateComposed(r, 1); err == nil {
				t.Fatalf("ASSERT_SYMBOL_CHURN_REJECTS_MALFORMED_ACQUISITION_%s", name)
			}
		})
	}
}
