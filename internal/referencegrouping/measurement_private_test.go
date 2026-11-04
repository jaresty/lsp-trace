package referencegrouping

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestGroupingMeasurementPrivateExactCounterPort(t *testing.T) {
	code := "CONTROLLED_DIAGNOSTIC"
	rows := []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code, Numerator: 1, Denominator: 2}, {CaseID: "b", MetricCode: code, Numerator: 1, Denominator: 4}}
	m, err := SumExactCountsPrivate(code, rows)
	if err != nil || m != (qualificationMetric{Code: code, Numerator: 1, Denominator: 3}) {
		t.Fatal("ASSERT_ACTUAL_COUNTER_PORT", m, err)
	}
	// Alias identity, not a similarly-shaped invented report type.
	if reflect.TypeOf(rows[0]) != reflect.TypeOf(qualificationObservation{}) {
		t.Fatal("ASSERT_EXISTING_COUNTER_TYPE")
	}
	for _, tc := range []struct {
		name string
		rows []ExactCountObservationPrivate
	}{
		{"missing", nil}, {"zero", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code}}},
		{"negative", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code, Numerator: -1, Denominator: 1}}},
		{"greater", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code, Numerator: 2, Denominator: 1}}},
		{"overflow", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code, Numerator: math.MaxInt, Denominator: math.MaxInt}}},
		{"sum-bound", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: code, Denominator: 1 << 30}, {CaseID: "b", MetricCode: code, Denominator: 1}}},
		{"duplicate", []ExactCountObservationPrivate{rows[0], rows[0]}},
		{"foreign-code", []ExactCountObservationPrivate{{CaseID: "a", MetricCode: "FOREIGN", Denominator: 1}}},
		{"case-bound", []ExactCountObservationPrivate{{CaseID: strings.Repeat("x", 4097), MetricCode: code, Denominator: 1}}},
		{"record-bound", make([]ExactCountObservationPrivate, 4097)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SumExactCountsPrivate(code, tc.rows); err == nil {
				t.Fatal("ASSERT_INCOMPLETE_COUNTS_NOT_PERFECT")
			}
		})
	}
	if len(issuedQualificationApprovals()) != 0 {
		t.Fatal("ASSERT_NO_ISSUED_PRODUCTION_QUALIFICATIONS")
	}
	t.Log("ASSERT_ACTUAL_COUNTER_PORT: PASS")
}
