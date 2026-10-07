package adr0007locationv1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProspectiveCaseSchemasAndCensus(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "experiment", "location-intersection-prospective-v1")
	sb, e := os.ReadFile(filepath.Join(root, "schemas", "Case.schema.json"))
	if e != nil {
		t.Fatal(e)
	}
	var s map[string]any
	if json.Unmarshal(sb, &s) != nil || s["$schema"] != "https://json-schema.org/draft/2020-12/schema" || s["additionalProperties"] != false {
		t.Fatal("schema is not closed Draft2020-12")
	}
	paths, e := filepath.Glob(filepath.Join(root, "cases", "*", "EXPECTED.json"))
	if e != nil || len(paths) != 24 {
		t.Fatalf("case census=%d %v", len(paths), e)
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		var v struct {
			Schema, Case, ExpectedOutcome string
			Accounting                    struct{ Input, Eligible, Ranked, Work int }
		}
		if json.Unmarshal(b, &v) != nil || v.Schema != "lsp-trace.adr0007.location.case.v1" || v.Case == "" || v.ExpectedOutcome == "" || v.Accounting.Input < 0 || v.Accounting.Eligible < 0 || v.Accounting.Ranked < 0 || v.Accounting.Work < 0 {
			t.Fatalf("invalid fixture %s", p)
		}
	}
}
