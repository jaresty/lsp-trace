package retainedpartialprogramc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/programc"
)

// Consumer unit state is package-internal numerical fixture state, not a producer
// proof or a claim of source qualification. The external owned fixture separately
// exercises Project on an actual qualified/derived producer handle.
func consumerMathFixture(empty bool) Projection {
	math := programc.CallsMathPrivate{NodeIdentities: []string{"a", "b", "c", "d"}, PairWeights: map[programc.Pair]float64{}}
	if !empty {
		math.Occurrences = []programc.Occurrence{{Identity: "o", RelationID: "r", Provider: "fixture", From: 0, To: 1, Weight: 1}}
		math.PairWeights[programc.Pair{From: 0, To: 1}] = 1
	}
	state := &projectionState{math: math, source: SourceBinding{InputSHA256: "unit-only-input", NativeGraphSHA256: "unit-only-graph", raw: []byte("input"), native: []byte("native"), manifest: []byte(`{}`)}}
	state.self = state
	return Projection{state}
}
func assertAccounting(t *testing.T, a InstabilityPrivate) {
	t.Helper()
	if len(a.Runs) != a.ExpectedRuns || len(a.Pairs) != a.ExpectedPairs || a.CompleteRuns+a.EmptyRuns+a.FailedRuns+a.NotAttemptedRuns != a.ExpectedRuns || a.CompletePairs+a.BlockedPairs+a.EmptyPairs+a.NotAttemptedPairs != a.ExpectedPairs {
		t.Fatalf("accounting %+v", a)
	}
}
func TestPrivateInstabilityEmptyAndNil(t *testing.T) {
	p := consumerMathFixture(true)
	seeds := []uint64{42, 43, 44, 45, 46, 47}
	if _, err := ComputeInstabilityPrivate(nil, p, seeds); err == nil {
		t.Fatal("nil context")
	}
	a, err := ComputeInstabilityPrivate(context.Background(), p, seeds)
	if err != nil {
		t.Fatal(err)
	}
	assertAccounting(t, a)
	if a.Outcome != "EMPTY" || a.EmptyRuns != 6 || a.EmptyPairs != 15 || a.CompletePairs != 0 || a.BlockedPairs != 0 {
		t.Fatal(a)
	}
	for _, pair := range a.Pairs {
		if pair.Status != "NOT_APPLICABLE_EMPTY" || pair.Metrics != (programc.PureInstabilityMetrics{}) {
			t.Fatal(pair)
		}
	}
	raw, _ := json.Marshal(a)
	if err := ValidateInstabilityPrivate(context.Background(), p, a, raw); err != nil {
		t.Fatal(err)
	}
	mixed := CloneInstabilityPrivate(a)
	mixed.Runs[0].Status = "COMPLETE"
	mixed.Runs[0].Partition.Outcome = "COMPLETE"
	if _, err := assessPrivate(context.Background(), p, mixed.Seeds, mixed.Runs); err == nil {
		t.Fatal("mixed empty complete")
	}
	clone := CloneInstabilityPrivate(a)
	clone.Seeds[0]++
	clone.Runs[0].Partition.Communities[0].Members[0] = "changed"
	clone.Pairs[0].Status = "COMPLETE"
	clone.retained[0] = 'x'
	again, _ := json.Marshal(a)
	if string(raw) != string(again) {
		t.Fatal("clone alias")
	}
}
func TestPrivateInstabilityInterruptedRunAccounting(t *testing.T) {
	p := consumerMathFixture(false)
	seeds := []uint64{42, 43, 44, 45, 46, 47}
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			original := &programc.Failure{Code: programc.CodeComputation, Message: "owned injected failure"}
			a, err := computeInstabilityPrivate(ctx, p, seeds, func(p Projection, s uint64) (Outcome, *programc.Failure) {
				calls++
				if failure {
					return Outcome{}, original
				}
				o, f := ComputeProjection(p, s)
				cancel()
				return o, f
			})
			assertAccounting(t, a)
			if calls != 1 || a.Outcome != "INCOMPLETE" || a.NotAttemptedRuns != 5 || a.NotAttemptedPairs != 15 || len(a.retained) != 0 {
				t.Fatalf("%+v calls=%d err=%v", a, calls, err)
			}
			if failure {
				if err != original || a.FailedRuns != 1 || a.CompleteRuns != 0 {
					t.Fatal(a, err)
				}
			} else if !errors.Is(err, context.Canceled) || a.CompleteRuns != 1 {
				t.Fatal(a, err)
			}
			raw, _ := json.Marshal(a)
			if ValidateInstabilityPrivate(context.Background(), p, a, raw) == nil {
				t.Fatal("incomplete admitted")
			}
		})
	}
}

// Err calls are local to this context, with no timers, goroutines or global hooks.
type countedContext struct {
	context.Context
	calls, cancelAt int
}

func (c *countedContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}
func TestPrivateInstabilityInterruptedPairs(t *testing.T) {
	p := consumerMathFixture(false)
	seeds := []uint64{42, 43, 44, 45, 46, 47}
	// Two checks per seed, then before/after pair 1. Stop before pair 2.
	ctx := &countedContext{Context: context.Background(), cancelAt: 15}
	a, err := ComputeInstabilityPrivate(ctx, p, seeds)
	assertAccounting(t, a)
	if !errors.Is(err, context.Canceled) || a.Outcome != "INCOMPLETE" || a.CompleteRuns != 6 || a.CompletePairs != 1 || a.NotAttemptedPairs != 14 || len(a.retained) != 0 {
		t.Fatalf("%+v %v checks=%d", a, err, ctx.calls)
	}
}
func TestPrivateInstabilityConsumerStableAndUnstable(t *testing.T) {
	p := consumerMathFixture(false)
	stable, err := ComputeInstabilityPrivate(context.Background(), p, []uint64{42, 43})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(stable)
	if err := ValidateInstabilityPrivate(context.Background(), p, stable, raw); err != nil {
		t.Fatal(err)
	}
	if stable.Outcome != "STABLE" {
		t.Fatal(stable)
	}
	// Pair policy is tested with real pure-math partition fixtures; this internal
	// injector supplies retained partitions only, never a public admission path.
	count := 0
	unstable, err := computeInstabilityPrivate(context.Background(), p, []uint64{42, 43}, func(p Projection, s uint64) (Outcome, *programc.Failure) {
		o, f := ComputeProjection(p, s)
		if f != nil {
			return o, f
		}
		count++
		if count == 1 {
			o.Math.Communities = []programc.Community{{Members: []string{"a", "b"}}, {Members: []string{"c", "d"}}}
		} else {
			o.Math.Communities = []programc.Community{{Members: []string{"a", "c"}}, {Members: []string{"b", "d"}}}
		}
		o.Math.LogicalDigest = programc.PartitionDigestMathPrivate(o.Math.Communities)
		return o, nil
	})
	if err != nil || unstable.Outcome != "UNSTABLE" {
		t.Fatal(unstable, err)
	}
	raw, _ = json.Marshal(unstable)
	if err := ValidateInstabilityPrivate(context.Background(), p, unstable, raw); err != nil {
		t.Fatal(err)
	}
}
