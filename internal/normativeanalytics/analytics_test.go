package normativeanalytics

import (
	"errors"
	"reflect"
	"testing"
)

type fixedExecutor struct {
	units int64
	err   error
}

func (e fixedExecutor) Execute(Operation) (int64, error) { return e.units, e.err }
func admitted() error                                    { return nil }

func TestContractIdentityAndEquivalentSurfaces(t *testing.T) {
	if Family != "normative-analytics" || Version != "v2" || SchemaVersion != "lsp-trace.normative-analytics.v2" || Scope != "NORMATIVE_PROGRAM_B_ONLY" {
		t.Fatal("ASSERT_NORMATIVE_V2_IDENTITY")
	}
	want := []Descriptor{{Analysis, "normative-analysis", "lsp_trace_v2_normative_analysis"}, {Metrics, "normative-metrics", "lsp_trace_v2_normative_metrics"}, {Ranking, "normative-ranking", "lsp_trace_v2_normative_ranking"}}
	if !reflect.DeepEqual(Descriptors(), want) {
		t.Fatal("ASSERT_EQUIVALENT_ANALYTICS_SURFACES")
	}
	got := Descriptors()
	got[0].Command = "mutated"
	if reflect.DeepEqual(got, Descriptors()) {
		t.Fatal("ASSERT_DESCRIPTOR_COPY")
	}
}
func TestAdmissionBlocksNormativeEvidence(t *testing.T) {
	for _, r := range []Request{{Operation: Analysis, BuildRevision: "526f658", Limit: 1, Executor: fixedExecutor{units: 1}}, {Operation: Metrics, BuildRevision: "other", Limit: 1, Executor: fixedExecutor{units: 1}}} {
		if got, err := Evaluate(r); !errors.Is(err, ErrProgramBNotAdmitted) || got != (Result{}) {
			t.Fatalf("ASSERT_PROGRAM_B_GATE: %#v %v", got, err)
		}
	}
}
func TestDeterministicExecutorOwnedResourceAccounting(t *testing.T) {
	r := Request{Operation: Ranking, BuildRevision: "526f658", Limit: 7, Executor: fixedExecutor{units: 7}}
	first, err := evaluate(r, admitted)
	second, err2 := evaluate(r, admitted)
	if err != nil || err2 != nil || first != second || first.Accounting != (Accounting{7, 7}) || first.Status != Complete {
		t.Fatalf("ASSERT_DETERMINISTIC_ACCOUNTING: %#v %#v %v %v", first, second, err, err2)
	}
	r.Executor = fixedExecutor{units: 8}
	limited, err := evaluate(r, admitted)
	if err != nil || limited.Status != Limit || limited.Evidence != "" || limited.Accounting != (Accounting{8, 7}) {
		t.Fatalf("ASSERT_TYPED_LIMIT: %#v %v", limited, err)
	}
}
func TestInvalidRequestsFailClosed(t *testing.T) {
	for _, r := range []Request{{Operation: Analysis, BuildRevision: "526f658", Limit: 0, Executor: fixedExecutor{}}, {Operation: Analysis, BuildRevision: "526f658", Limit: 1}, {Operation: Analysis, BuildRevision: "526f658", Limit: 1, Executor: fixedExecutor{units: -1}}} {
		if got, err := evaluate(r, admitted); !errors.Is(err, ErrInvalidRequest) || got != (Result{}) {
			t.Fatalf("ASSERT_INVALID_FAIL_CLOSED: %#v %v", got, err)
		}
	}
}
