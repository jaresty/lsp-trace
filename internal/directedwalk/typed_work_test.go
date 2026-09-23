package directedwalk_test

import (
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/directedwalk"
)

func TestTypedWalkWorkLimitFailsClosedAndExactBoundary(t *testing.T) {
	req := mixedRequest()
	baseline, err := directedwalk.Walk(req)
	if err != nil || baseline.WorkUsed <= 1 || baseline.WorkUsed > req.MaxWork {
		t.Fatalf("ASSERT_TYPED_WORK_ACCOUNTING: used=%d max=%d err=%v", baseline.WorkUsed, req.MaxWork, err)
	}
	req.MaxWork = 1
	short, err := directedwalk.Walk(req)
	if !errors.Is(err, directedwalk.ErrWorkLimit) || !reflect.DeepEqual(short, directedwalk.Result{}) {
		t.Fatalf("ASSERT_TYPED_WORK_EARLY: result=%v err=%v", short, err)
	}
	req.MaxWork = baseline.WorkUsed - 1
	short, err = directedwalk.Walk(req)
	if !errors.Is(err, directedwalk.ErrWorkLimit) || !reflect.DeepEqual(short, directedwalk.Result{}) {
		t.Fatalf("ASSERT_TYPED_WORK_LATE: used=%d result=%v err=%v", baseline.WorkUsed, short, err)
	}
	req.MaxWork = baseline.WorkUsed
	atLimit, err := directedwalk.Walk(req)
	if err != nil || !reflect.DeepEqual(atLimit, baseline) {
		t.Fatalf("ASSERT_TYPED_WORK_EXACT: baseline=%v atLimit=%v err=%v", baseline, atLimit, err)
	}
	t.Log("ASSERT_TYPED_WORK_ACCOUNTING: PASS")
	t.Log("ASSERT_TYPED_WORK_EARLY: PASS")
	t.Log("ASSERT_TYPED_WORK_LATE: PASS")
	t.Log("ASSERT_TYPED_WORK_EXACT: PASS")
}

func TestTypedWalkWorkChargesEverySyntheticPhase(t *testing.T) {
	got, err := directedwalk.Walk(mixedRequest())
	// Includes two final-copy units for each of the two two-item adjacency sorts.
	if err != nil || got.WorkUsed != 166 {
		t.Fatalf("ASSERT_TYPED_WORK_PHASES: used=%d want=166 err=%v", got.WorkUsed, err)
	}
	t.Log("ASSERT_TYPED_WORK_PHASES: PASS")
}

func TestTypedWalkWorkUsedStableAcrossRepeatedIdenticalInputs(t *testing.T) {
	req := mixedRequest()
	first, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		next, err := directedwalk.Walk(req)
		if err != nil || !reflect.DeepEqual(first, next) {
			t.Fatalf("ASSERT_TYPED_WORK_REPLAY: first=%v next=%v err=%v iteration=%d", first, next, err, i)
		}
	}
	t.Log("ASSERT_TYPED_WORK_REPLAY: PASS")
}

func TestTypedWalkWorkPolicyRejectsMissingOrExcessiveLimit(t *testing.T) {
	for _, max := range []int{-1, 0, directedwalk.MaxWorkLimit + 1} {
		req := mixedRequest()
		req.MaxWork = max
		got, err := directedwalk.Walk(req)
		if !errors.Is(err, directedwalk.ErrInvalidWorkLimit) || !reflect.DeepEqual(got, directedwalk.Result{}) {
			t.Fatalf("ASSERT_TYPED_WORK_POLICY: max=%d result=%v err=%v", max, got, err)
		}
	}
	t.Log("ASSERT_TYPED_WORK_POLICY: PASS")
}

func TestTypedWalkUnselectedCandidatesConsumeValidationWork(t *testing.T) {
	req := mixedRequest()
	req.Kinds = []directedwalk.Kind{directedwalk.Calls}
	all, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Candidates = []directedwalk.Candidate{req.Candidates[0], req.Candidates[4]}
	onlyCalls, err := directedwalk.Walk(req)
	if err != nil || all.WorkUsed <= onlyCalls.WorkUsed || all.Unselected != 3 {
		t.Fatalf("ASSERT_TYPED_WORK_UNSELECTED: all=%v onlyCalls=%v err=%v", all, onlyCalls, err)
	}
	t.Log("ASSERT_TYPED_WORK_UNSELECTED: PASS")
}
