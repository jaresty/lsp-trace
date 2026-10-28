package retainedpartialprogramc

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/programc"
	"testing"
)

func TestPrivateInstabilityExactExecutionAndAdmission(t *testing.T) {
	p := consumerMathFixture(false)
	calls := 0
	compute := func(p Projection, s uint64) (Outcome, *programc.Failure) { calls++; return ComputeProjection(p, s) }
	for _, seeds := range [][]uint64{{1, 2, 3, 4, 5, 6, 7}, {42, 42}, {42}} {
		if _, err := computeInstabilityPrivate(context.Background(), p, seeds, compute); err == nil {
			t.Fatal("request accepted")
		}
	}
	if calls != 0 {
		t.Fatal("computed invalid request")
	}
	seeds := []uint64{47, 42, 46, 43, 45, 44}
	a, err := computeInstabilityPrivate(context.Background(), p, seeds, compute)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 6 || len(a.Pairs) != 15 || a.Seeds[0] != 47 || a.Runs[0].Seed != 47 {
		t.Fatal("execution accounting", calls, a)
	}
	raw, _ := json.Marshal(a)
	for _, mutate := range []func(*InstabilityPrivate){
		func(a *InstabilityPrivate) { a.Seeds[0], a.Seeds[1] = a.Seeds[1], a.Seeds[0] },
		func(a *InstabilityPrivate) { a.Runs[0].Partition.Communities[0].Members[0] = "substituted-node" },
		func(a *InstabilityPrivate) { a.Runs[0].Partition.LogicalDigest = "changed" },
		func(a *InstabilityPrivate) { a.Runs[0].Partition.Outcome = "EMPTY" },
		func(a *InstabilityPrivate) { a.Runs[0].Status = "FAILED" },
		func(a *InstabilityPrivate) { a.ProjectionDigest = "changed" },
		func(a *InstabilityPrivate) { a.NativeGraphSHA256 = "changed" },
		func(a *InstabilityPrivate) { a.Pairs[0].Status = "NOT_ATTEMPTED" },
		func(a *InstabilityPrivate) { a.CompletePairs-- },
		func(a *InstabilityPrivate) { a.Outcome = "EMPTY" },
	} {
		b := CloneInstabilityPrivate(a)
		mutate(&b)
		changed, _ := json.Marshal(b)
		if ValidateInstabilityPrivate(context.Background(), p, a, changed) == nil {
			t.Fatal("accepted mutated bytes")
		}
		if ValidateInstabilityPrivate(context.Background(), p, b, changed) == nil {
			t.Fatal("accepted mutated producer assessment")
		}
	}
	var dto InstabilityPrivate
	_ = json.Unmarshal(raw, &dto)
	if ValidateInstabilityPrivate(context.Background(), p, dto, raw) == nil {
		t.Fatal("decoded DTO minted ownership")
	}
	var wire map[string]any
	_ = json.Unmarshal(raw, &wire)
	wire["caller_thresholds"] = map[string]any{"VI": 999}
	changed, _ := json.Marshal(wire)
	if ValidateInstabilityPrivate(context.Background(), p, a, changed) == nil {
		t.Fatal("caller threshold injection")
	}
	seeds[0] = 1
	again, _ := json.Marshal(a)
	if string(again) != string(raw) {
		t.Fatal("request alias")
	}
}
