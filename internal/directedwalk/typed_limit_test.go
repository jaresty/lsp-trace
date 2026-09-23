package directedwalk_test

import (
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/directedwalk"
)

func TestTypedWalkCandidateLimitCountsUnselectedBeforeContentValidation(t *testing.T) {
	req := mixedRequest()
	req.Kinds = []directedwalk.Kind{directedwalk.Calls} // 3 other kinds must still count
	req.MaxCandidates = len(req.Candidates) - 1
	req.Candidates[0].ID = "" // invalid content must not displace the limit outcome
	got, err := directedwalk.Walk(req)
	if !errors.Is(err, directedwalk.ErrCandidateLimit) || !reflect.DeepEqual(got, directedwalk.Result{}) {
		t.Fatalf("ASSERT_TYPED_CANDIDATE_LIMIT_PRECEDENCE: result=%v err=%v", got, err)
	}
	t.Log("ASSERT_TYPED_CANDIDATE_LIMIT_PRECEDENCE: PASS")
}

func TestTypedWalkCandidateExactLimitRetainsProjection(t *testing.T) {
	req := mixedRequest()
	baseline, err := directedwalk.Walk(req)
	if err != nil {
		t.Fatal(err)
	}
	req.MaxCandidates = len(req.Candidates)
	got, err := directedwalk.Walk(req)
	if err != nil || !reflect.DeepEqual(got, baseline) {
		t.Fatalf("ASSERT_TYPED_CANDIDATE_EXACT_LIMIT: got=%v baseline=%v err=%v", got, baseline, err)
	}
	req.MaxCandidates = directedwalk.MaxCandidateLimit
	got, err = directedwalk.Walk(req)
	if err != nil || !reflect.DeepEqual(got, baseline) {
		t.Fatalf("ASSERT_TYPED_CANDIDATE_EXACT_LIMIT: ceiling got=%v baseline=%v err=%v", got, baseline, err)
	}
	t.Log("ASSERT_TYPED_CANDIDATE_EXACT_LIMIT: PASS")
}

func TestTypedWalkRejectsInvalidCandidateLimitPolicy(t *testing.T) {
	for _, limit := range []int{-1, 0, directedwalk.MaxCandidateLimit + 1} {
		req := mixedRequest()
		req.MaxCandidates = limit
		got, err := directedwalk.Walk(req)
		if !errors.Is(err, directedwalk.ErrInvalidCandidateLimit) || !reflect.DeepEqual(got, directedwalk.Result{}) {
			t.Fatalf("ASSERT_TYPED_CANDIDATE_LIMIT_POLICY: limit=%d result=%v err=%v", limit, got, err)
		}
	}
	t.Log("ASSERT_TYPED_CANDIDATE_LIMIT_POLICY: PASS")
}
