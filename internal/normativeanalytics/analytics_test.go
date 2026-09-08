package normativeanalytics

import (
	"errors"
	"lsp-trace/internal/qualificationmatrix"
	"reflect"
	"testing"
	"time"
)

type fixedExecutor struct {
	units int64
	err   error
}

func (e fixedExecutor) Execute(Operation) (int64, error) { return e.units, e.err }

func admission(t *testing.T, revision string, operation Operation) qualificationmatrix.ProgramBAdmission {
	t.Helper()
	axes := []qualificationmatrix.Axis{}
	for _, x := range []struct{ n, v string }{{"relation_family", "CALLS"}, {"custody_adapter", "GIT"}, {"state", "COMPLETE"}, {"projection_class", "DIRECTED"}, {"transport", "CLI"}, {"publication_mode", "INLINE"}, {"provider_class", "TYPESCRIPT_LANGUAGE_SERVER"}, {"provider_version", "5.7.3"}, {"language", "TYPESCRIPT"}, {"framework", "NONE"}} {
		axes = append(axes, qualificationmatrix.Axis{Name: x.n, Members: []string{x.v}})
	}
	p := qualificationmatrix.Profile{SchemaVersion: qualificationmatrix.SchemaVersion, ProfileID: "analytics-profile", Version: "2", Authority: "release-council", CustodyReceiptID: "receipt", CustodyAuthenticationState: "AUTHENTICATED", Axes: axes, Products: []qualificationmatrix.Product{{ID: "analytics-provider-5.7.3", Version: "2", Axes: []string{"relation_family", "custody_adapter", "state", "projection_class", "transport", "publication_mode", "provider_class", "provider_version", "language", "framework"}}}}
	cells, err := qualificationmatrix.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	p.FoundationalCellIDs = []string{cells[0].ID}
	result := qualificationmatrix.Result{CellID: cells[0].ID, Status: qualificationmatrix.StatusPass, RealServerEvidence: true, EvidenceProviderClass: "TYPESCRIPT_LANGUAGE_SERVER", EvidenceProviderVersion: "5.7.3"}
	got, err := qualificationmatrix.VerifyProgramBAdmission(p, qualificationmatrix.AdmissionRequest{Results: []qualificationmatrix.Result{result}, RequestedOperations: []string{string(operation)}}, qualificationmatrix.AdmissionBinding{BuildRevision: revision, Operation: string(operation), Scope: Scope}, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

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
	for _, r := range []Request{{Operation: Analysis, BuildRevision: "526f658", Limit: 1, Executor: fixedExecutor{units: 1}}, {Operation: Metrics, BuildRevision: "other", Admission: admission(t, "526f658", Metrics), Limit: 1, Executor: fixedExecutor{units: 1}}, {Operation: Analysis, BuildRevision: "526f658", Admission: admission(t, "526f658", Metrics), Limit: 1, Executor: fixedExecutor{units: 1}}} {
		if got, err := Evaluate(r); !errors.Is(err, ErrProgramBNotAdmitted) || got != (Result{}) {
			t.Fatalf("ASSERT_PROGRAM_B_GATE: %#v %v", got, err)
		}
	}
}
func TestDeterministicExecutorOwnedResourceAccounting(t *testing.T) {
	a := admission(t, "526f658", Ranking)
	r := Request{Operation: Ranking, BuildRevision: "526f658", Admission: a, Limit: 7, Executor: fixedExecutor{units: 7}}
	first, err := Evaluate(r)
	second, err2 := Evaluate(r)
	if err != nil || err2 != nil || first != second || first.Accounting != (Accounting{7, 7}) || first.Status != Complete {
		t.Fatalf("ASSERT_DETERMINISTIC_ACCOUNTING: %#v %#v %v %v", first, second, err, err2)
	}
	r.Executor = fixedExecutor{units: 8}
	limited, err := Evaluate(r)
	if err != nil || limited.Status != Limit || limited.Evidence != "" || limited.Accounting != (Accounting{8, 7}) {
		t.Fatalf("ASSERT_TYPED_LIMIT: %#v %v", limited, err)
	}
}
func TestInvalidRequestsFailClosed(t *testing.T) {
	a := admission(t, "526f658", Analysis)
	for _, r := range []Request{{Operation: Analysis, BuildRevision: "526f658", Admission: a, Limit: 0, Executor: fixedExecutor{}}, {Operation: Analysis, BuildRevision: "526f658", Admission: a, Limit: 1}, {Operation: Analysis, BuildRevision: "526f658", Admission: a, Limit: 1, Executor: fixedExecutor{units: -1}}} {
		if got, err := Evaluate(r); !errors.Is(err, ErrInvalidRequest) || got != (Result{}) {
			t.Fatalf("ASSERT_INVALID_FAIL_CLOSED: %#v %v", got, err)
		}
	}
}
