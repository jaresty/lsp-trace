package retainedpartialprogramc

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/programc"
)

func TestPrivateInstabilityPreEncodingBounds(t *testing.T) {
	p := consumerMathFixture(true)
	a, err := ComputeInstabilityPrivate(context.Background(), p, []uint64{42, 43})
	if err != nil {
		t.Fatal(err)
	}
	historical, _ := json.Marshal(a)
	raw, err := ValidatedInstabilityBytesPrivate(context.Background(), p, a)
	if err != nil || !bytes.Equal(raw, historical) {
		t.Fatal("historical canonical encoding changed", err)
	}
	raw[0] = 'x'
	if err := ValidateInstabilityPrivate(context.Background(), p, a, historical); err != nil {
		t.Fatal("returned bytes alias seal", err)
	}
	huge := strings.Repeat("x", 1<<20)
	cases := []struct {
		name   string
		mutate func(*InstabilityPrivate)
	}{
		{"seeds", func(a *InstabilityPrivate) { a.Seeds = make([]uint64, 7) }},
		{"runs", func(a *InstabilityPrivate) { a.Runs = make([]InstabilityRunPrivate, 7) }},
		{"pairs", func(a *InstabilityPrivate) { a.Pairs = make([]InstabilityPairPrivate, 16) }},
		{"communities", func(a *InstabilityPrivate) { a.Runs[0].Partition.Communities = make([]programc.Community, 10001) }},
		{"members", func(a *InstabilityPrivate) { a.Runs[0].Partition.Communities[0].Members = make([]string, 10001) }},
		{"member-string", func(a *InstabilityPrivate) { a.Runs[0].Partition.Communities[0].Members[0] = huge }},
		{"claim-string", func(a *InstabilityPrivate) { a.ClaimCeiling = huge }},
		{"algorithm-string", func(a *InstabilityPrivate) { a.Runs[0].Partition.Algorithm = huge }},
		{"pair-guard", func(a *InstabilityPrivate) { a.Pairs[0].Metrics.Guard = huge }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			bad := CloneInstabilityPrivate(a)
			tt.mutate(&bad)
			// Oversized fixtures are allocated outside the measured validation window.
			// A former json.Marshal-first path would allocate >1 MiB for huge strings.
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := ValidateInstabilityPrivate(context.Background(), p, bad, historical)
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatal("mutated assessment accepted")
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 128<<10 {
				t.Fatalf("allocated %d bytes before rejection", allocated)
			}
			if b, err := marshalInstabilityPrivateBounded(p, bad); err == nil || b != nil {
				t.Fatal("bounded serializer emitted rejected shape")
			}
		})
	}
	// Counts alone are not enough: aggregate conservative encoded size also caps
	// otherwise small individual fields. Use a valid-shaped 10,000-node state.
	large := consumerMathFixture(true)
	large.state.math.NodeIdentities = make([]string, 10000)
	bad := CloneInstabilityPrivate(a)
	memberLabel := strings.Repeat("z", 1024)
	for i := range bad.Runs {
		bad.Runs[i].Partition.Communities = []programc.Community{{Members: make([]string, 10000)}}
		for j := range bad.Runs[i].Partition.Communities[0].Members {
			bad.Runs[i].Partition.Communities[0].Members[j] = memberLabel
		}
	}
	if b, err := marshalInstabilityPrivateBounded(large, bad); err == nil || b != nil {
		t.Fatal("aggregate encoded bound ignored")
	}
}
