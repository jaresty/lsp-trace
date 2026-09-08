package normativeanalytics

import (
	"errors"
	"reflect"
	"testing"
)

func TestContractIdentityAndEquivalentSurfaces(t *testing.T) {
	if Family != "normative-analytics" || Version != "v2" || SchemaVersion != "lsp-trace.normative-analytics.v2" || Scope != "NORMATIVE_PROGRAM_B_ONLY" {
		t.Fatalf("ASSERT_NORMATIVE_V2_IDENTITY family=%q version=%q schema=%q scope=%q", Family, Version, SchemaVersion, Scope)
	}
	want := []Descriptor{
		{Operation: Analysis, Command: "normative-analysis", Protocol: "lsp_trace_v2_normative_analysis"},
		{Operation: Metrics, Command: "normative-metrics", Protocol: "lsp_trace_v2_normative_metrics"},
		{Operation: Ranking, Command: "normative-ranking", Protocol: "lsp_trace_v2_normative_ranking"},
	}
	if got := Descriptors(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_EQUIVALENT_ANALYTICS_SURFACES got=%#v want=%#v", got, want)
	}
	got := Descriptors()
	got[0].Command = "mutated"
	if reflect.DeepEqual(got, Descriptors()) {
		t.Fatal("ASSERT_DESCRIPTOR_COPY")
	}
}

func TestAdmissionBlocksNormativeEvidence(t *testing.T) {
	result, err := Evaluate(Request{Operation: Analysis, Admission: Admission{ProgramBAdmitted: false}, Units: 1, Limit: 1})
	if !errors.Is(err, ErrProgramBNotAdmitted) {
		t.Fatalf("ASSERT_PROGRAM_B_GATE err=%v", err)
	}
	if result != (Result{}) {
		t.Fatalf("ASSERT_BLOCKED_RESULT_EMPTY result=%#v", result)
	}
}

func TestDeterministicResourceAccounting(t *testing.T) {
	request := Request{Operation: Ranking, Admission: Admission{ProgramBAdmitted: true}, Units: 7, Limit: 7}
	first, err := Evaluate(request)
	if err != nil {
		t.Fatal("ASSERT_ACCOUNTING_BOUNDARY", err)
	}
	second, err := Evaluate(request)
	if err != nil || first != second {
		t.Fatalf("ASSERT_DETERMINISTIC_ACCOUNTING first=%#v second=%#v err=%v", first, second, err)
	}
	if first.Status != Complete || first.Accounting != (Accounting{Units: 7, Limit: 7}) {
		t.Fatalf("ASSERT_EXACT_ACCOUNTING result=%#v", first)
	}

	limited, err := Evaluate(Request{Operation: Ranking, Admission: Admission{ProgramBAdmitted: true}, Units: 8, Limit: 7})
	if err != nil {
		t.Fatal("ASSERT_TYPED_LIMIT", err)
	}
	if limited.Status != Limit || limited.Evidence != "" || limited.Accounting != (Accounting{Units: 8, Limit: 7}) {
		t.Fatalf("ASSERT_TYPED_LIMIT result=%#v", limited)
	}
}

func TestInvalidRequestsFailClosed(t *testing.T) {
	for _, request := range []Request{
		{Operation: "unknown", Admission: Admission{ProgramBAdmitted: true}, Units: 1, Limit: 1},
		{Operation: Analysis, Admission: Admission{ProgramBAdmitted: true}, Units: 1, Limit: 0},
		{Operation: Metrics, Admission: Admission{ProgramBAdmitted: true}, Units: -1, Limit: 1},
	} {
		if result, err := Evaluate(request); !errors.Is(err, ErrInvalidRequest) || result != (Result{}) {
			t.Fatalf("ASSERT_INVALID_FAIL_CLOSED request=%#v result=%#v err=%v", request, result, err)
		}
	}
}
