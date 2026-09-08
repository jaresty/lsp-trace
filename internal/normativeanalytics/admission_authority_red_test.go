package normativeanalytics

import (
	"errors"
	"testing"
)

func TestNormativeCallerBooleanCannotAdmit(t *testing.T) {
	result, err := Evaluate(Request{Operation: Analysis, BuildRevision: "526f658", Limit: 1, Executor: fixedExecutor{units: 1}})
	if !errors.Is(err, ErrProgramBNotAdmitted) || result != (Result{}) {
		t.Fatalf("ASSERT_NORMATIVE_CALLER_BOOLEAN_CANNOT_ADMIT: unverified request produced %#v err=%v", result, err)
	}
}
