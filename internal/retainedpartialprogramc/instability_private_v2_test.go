package retainedpartialprogramc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/programc"
)

// Package-private numerical fixture only. This does not claim producer
// qualification; external pilot tests exercise the real sealed packet path.
func mathFixturePrivateV2(n int, empty bool) ProjectionPrivateV2 {
	m := programc.CallsMathPrivate{PairWeights: map[programc.Pair]float64{}}
	for i := 0; i < n; i++ {
		m.NodeIdentities = append(m.NodeIdentities, fmt.Sprintf("n%03d", i))
	}
	if !empty {
		m.Occurrences = []programc.Occurrence{{Identity: "unit-occurrence", RelationID: "unit-relation", Provider: "owned", From: 0, To: 1, Weight: 1}}
		m.PairWeights[programc.Pair{From: 0, To: 1}] = 1
	}
	e := ProjectionEvidencePrivateV2{Source: SourceBindingPrivateV2{Profile: ProfilePrivateV2, ProfileDigest: ProfileDigestPrivateV2(), BatchOrdinal: 2, PacketSHA256: "unit-packet", QualificationReportSHA256: "unit-report", OriginalNativeV3SHA256: "unit-original", DerivedCanonicalV5SHA256: "unit-derived"}}
	st := &projectionStatePrivateV2{math: m, evidence: e, pin: instabilityDigest(e)}
	st.self = st
	return ProjectionPrivateV2{st}
}
func assertAccountingPrivateV2(t *testing.T, r InstabilityReportPrivateV2) {
	t.Helper()
	if len(r.Runs) != r.ExpectedRuns || len(r.Pairs) != r.ExpectedPairs || r.CompleteRuns+r.EmptyRuns+r.FailedRuns+r.NotAttemptedRuns != r.ExpectedRuns || r.CompletePairs+r.BlockedPairs+r.EmptyPairs+r.NotAttemptedPairs != r.ExpectedPairs {
		t.Fatal("ASSERT_V2_FAILURE_ACCOUNTING", r)
	}
}
func TestInstabilityPrivateV2EmptyUniverseAndSeeds(t *testing.T) {
	for _, n := range []int{0, 1, 4, 64} {
		for _, seeds := range [][]uint64{{42, 43}, {42, 43, 44, 45, 46, 47}} {
			p := mathFixturePrivateV2(n, true)
			a, r, err := InstabilityComputePrivateV2(context.Background(), p, seeds)
			if err != nil {
				t.Fatal(err)
			}
			assertAccountingPrivateV2(t, r)
			if r.Outcome != "EMPTY" || r.EmptyRuns != len(seeds) || r.EmptyPairs != len(seeds)*(len(seeds)-1)/2 || r.CompletePairs != 0 || r.BlockedPairs != 0 {
				t.Fatal("ASSERT_V2_EMPTY_NOT_STABLE", r)
			}
			for _, pair := range r.Pairs {
				if pair.Status != "NOT_APPLICABLE_EMPTY" || pair.Metrics != (programc.PureInstabilityMetrics{}) {
					t.Fatal("ASSERT_V2_EMPTY_NO_MATCHER", pair)
				}
			}
			if ValidateInstabilityPrivateV2(context.Background(), p, a, a.Bytes()) != nil {
				t.Fatal("ASSERT_V2_EMPTY_VALIDATE")
			}
		}
	}
	p := mathFixturePrivateV2(4, false)
	for _, seeds := range [][]uint64{nil, {1}, {1, 1}, {1, 2, 3, 4, 5, 6, 7}} {
		if a, _, err := InstabilityComputePrivateV2(context.Background(), p, seeds); err == nil || a.valid() {
			t.Fatal("ASSERT_V2_SEED_BOUNDS")
		}
	}
	if _, _, err := InstabilityComputePrivateV2(nil, p, []uint64{1, 2}); err == nil {
		t.Fatal("nil context")
	}
}
func TestInstabilityPrivateV2FailureAndCancelAccounting(t *testing.T) {
	seeds := []uint64{42, 43, 44, 45, 46, 47}
	for _, failAt := range []int{1, 3, 6} {
		for _, failCompute := range []bool{false, true} {
			t.Run(fmt.Sprint(failAt, failCompute), func(t *testing.T) {
				p := mathFixturePrivateV2(4, false)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				injected := &programc.Failure{Code: programc.CodeComputation, Message: "controlled V2 failure"}
				a, r, err := computeInstabilityPrivateV2(ctx, p, seeds, func(p ProjectionPrivateV2, s uint64) (OutcomePrivateV2, *programc.Failure) {
					calls++
					if calls == failAt && failCompute {
						return OutcomePrivateV2{}, injected
					}
					o, f := ComputeProjectionPrivateV2(p, s)
					if calls == failAt {
						cancel()
					}
					return o, f
				})
				assertAccountingPrivateV2(t, r)
				if a.valid() || len(a.Bytes()) != 0 || r.Outcome != "INCOMPLETE" || calls != failAt || r.NotAttemptedRuns != 6-failAt || r.NotAttemptedPairs != 15 {
					t.Fatal("ASSERT_V2_INTERRUPTION_RETAINS_ALL_ROWS", r, calls, err)
				}
				if failCompute {
					if err != injected || r.FailedRuns != 1 || r.CompleteRuns != failAt-1 {
						t.Fatal("ASSERT_V2_ORIGINAL_COMPUTE_ERROR", r, err)
					}
				} else if !errors.Is(err, context.Canceled) || r.CompleteRuns != failAt {
					t.Fatal("ASSERT_V2_CANCEL_COMPLETED_ROWS", r, err)
				}
				raw, _ := json.Marshal(r)
				if ValidateInstabilityPrivateV2(context.Background(), p, a, raw) == nil {
					t.Fatal("ASSERT_V2_DIAGNOSTIC_UNSEALED")
				}
			})
		}
	}
	p := mathFixturePrivateV2(4, false)
	ctx := &countedContext{Context: context.Background(), cancelAt: 15}
	a, r, err := InstabilityComputePrivateV2(ctx, p, seeds)
	assertAccountingPrivateV2(t, r)
	if !errors.Is(err, context.Canceled) || a.valid() || r.CompleteRuns != 6 || r.CompletePairs != 1 || r.NotAttemptedPairs != 14 {
		t.Fatal("ASSERT_V2_INTERRUPTED_PAIR_ACCOUNTING", r, err)
	}
}
func TestInstabilityPrivateV2ResourceAndNoValidationRerun(t *testing.T) {
	p := mathFixturePrivateV2(64, false)
	calls := 0
	a, r, err := computeInstabilityPrivateV2(context.Background(), p, []uint64{42, 43, 44, 45, 46, 47}, func(p ProjectionPrivateV2, s uint64) (OutcomePrivateV2, *programc.Failure) {
		calls++
		return ComputeProjectionPrivateV2(p, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	assertAccountingPrivateV2(t, r)
	if r.Outcome != "RESOURCE_INCOMPLETE" || r.BlockedPairs != 15 || r.CompletePairs != 0 || calls != 6 {
		t.Fatal("ASSERT_V2_BLOCKED_ALL_PAIRS_RETAINED", r, calls)
	}
	before := calls
	if err = ValidateInstabilityPrivateV2(context.Background(), p, a, a.Bytes()); err != nil || calls != before {
		t.Fatal("ASSERT_V2_VALIDATION_NO_COMPUTE_RERUN", err, calls)
	}
	for _, pair := range r.Pairs {
		if pair.Status != "BLOCKED_MATCHING_RESOURCE" {
			t.Fatal("ASSERT_V2_NO_FAVORABLE_WITNESS_SELECTION")
		}
	}
}
func TestInstabilityPrivateV2ExactProjectionSeal(t *testing.T) {
	p := mathFixturePrivateV2(4, false)
	a, _, err := InstabilityComputePrivateV2(context.Background(), p, []uint64{42, 43})
	if err != nil {
		t.Fatal(err)
	}
	// Same bytes/math/pins, different actual sealed projection state. A DTO match
	// alone cannot transfer the assessment capability to a foreign producer state.
	foreign := mathFixturePrivateV2(4, false)
	if ValidateInstabilityPrivateV2(context.Background(), foreign, a, a.Bytes()) == nil {
		t.Fatal("ASSERT_V2_EXACT_PROJECTION_SEAL")
	}
	t.Log("ASSERT_V2_EXACT_PROJECTION_SEAL: PASS")
}
func TestInstabilityPrivateV2RetainedMetricsAndPartitions(t *testing.T) {
	p := mathFixturePrivateV2(4, false)
	for _, unstable := range []bool{false, true} {
		calls := 0
		a, r, err := computeInstabilityPrivateV2(context.Background(), p, []uint64{42, 43}, func(p ProjectionPrivateV2, s uint64) (OutcomePrivateV2, *programc.Failure) {
			o, f := ComputeProjectionPrivateV2(p, s)
			if f != nil {
				return o, f
			}
			calls++
			o.Math.Communities = []programc.Community{{Members: []string{"n000", "n001"}}, {Members: []string{"n002", "n003"}}}
			if unstable && calls == 2 {
				o.Math.Communities = []programc.Community{{Members: []string{"n000", "n002"}}, {Members: []string{"n001", "n003"}}}
			}
			o.Math.LogicalDigest = programc.PartitionDigestMathPrivate(o.Math.Communities)
			return o, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "STABLE"
		if unstable {
			want = "UNSTABLE"
		}
		if r.Outcome != want || ValidateInstabilityPrivateV2(context.Background(), p, a, a.Bytes()) != nil {
			t.Fatal("ASSERT_V2_RETAINED_MATH", r, err)
		}
	}
}
