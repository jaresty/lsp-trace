package referencegrouping

import "errors"

// These aliases expose only the actual downstream integer-counter shapes. They
// confer no experimental meaning, approval, receipt, registration or authority.
// Diagnostic codes must not masquerade as approved qualification definitions.
type ExactCountObservationPrivate = qualificationObservation
type ExactCountMetricPrivate = qualificationMetric

// SumExactCountsPrivate uses the report interpreter's checked integer sums and
// rational reduction. Missing/zero denominators are incomplete, never perfect.
// The count/aggregate bounds are private diagnostic bounds, not owner criteria.
func SumExactCountsPrivate(code string, observations []ExactCountObservationPrivate) (ExactCountMetricPrivate, error) {
	bad := func() (ExactCountMetricPrivate, error) {
		return ExactCountMetricPrivate{}, errors.New("INCOMPLETE_PRIVATE_EXACT_COUNTS")
	}
	if code == "" || len(code) > 4096 || len(observations) == 0 || len(observations) > 4096 {
		return bad()
	}
	n, d := 0, 0
	// Validate all counts before map allocation or aggregation.
	for _, o := range observations {
		if o.MetricCode != code || o.CaseID == "" || len(o.CaseID) > 4096 || o.Numerator < 0 || o.Denominator <= 0 || o.Numerator > o.Denominator || o.Denominator > 1<<30 {
			return bad()
		}
	}
	seen := make(map[string]bool, len(observations))
	for _, o := range observations {
		if seen[o.CaseID] {
			return bad()
		}
		seen[o.CaseID] = true
		var ok bool
		n, ok = qualificationCheckedSum(n, o.Numerator)
		if !ok || n > 1<<30 {
			return bad()
		}
		d, ok = qualificationCheckedSum(d, o.Denominator)
		if !ok || d > 1<<30 {
			return bad()
		}
	}
	divisor := gcd(n, d)
	return qualificationMetric{Code: code, Numerator: n / divisor, Denominator: d / divisor}, nil
}
