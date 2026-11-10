//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"testing"
	"time"
)

func TestFixedProviderEveryPaidCoordinateV2(t *testing.T) {
	for _, coordinate := range []int{0, 1, 2, 3, 4, 7, 8, 9, 13} {
		t.Run(string(rune('A'+coordinate)), func(t *testing.T) {
			o, p, n := providerControlFixtureV2(t, func(a *authorizationV2) {
				v := a.Ceiling.vector()
				v[coordinate] = 0
				a.Ceiling = countersFromV2(v)
				a.MaxTranche = a.Ceiling
			})
			before := o.head
			fault := &providerFaultFixtureV2{mode: "normal", path: p.Path}
			r, e := o.verifyProviderControlV2(context.Background(), hashLabelV2("underfund-coordinate"), time.Now().Add(time.Second), p, n, fault)
			if e == nil || r.state != nil || o.head != before || fault.stats != 0 || fault.reads != 0 || fault.closes != 0 {
				t.Fatalf("ASSERT_COORDINATE_BEFORE_IO: %d stats=%d reads=%d closes=%d", coordinate, fault.stats, fault.reads, fault.closes)
			}
			t.Log("ASSERT_COORDINATE_BEFORE_IO PASS", coordinate)
		})
	}
}
func TestProviderLaunchQuoteLimitsV2(t *testing.T) {
	v := PrivateManagedInputVerificationV2{400, 4 << 20, 400}
	q, e := FixedProviderLaunchQuoteV2(v, v, 180*time.Second)
	if e != nil || q[5] != 0 || q[6] != 0 || q[10] != 0 || q[11] != 0 || q[12] != 0 || q[8] != uint64(180*time.Second) {
		t.Fatal("ASSERT_LAUNCH_QUOTE_14", q, e)
	}
	if _, e = FixedProviderLaunchQuoteV2(v, v, 180*time.Second+1); e == nil {
		t.Fatal("ASSERT_RETENTION_BOUND")
	}
	if _, e = FixedProviderLaunchQuoteV2(v, PrivateManagedInputVerificationV2{^uint64(0), ^uint64(0), ^uint64(0)}, time.Second); e == nil {
		t.Fatal("ASSERT_LAUNCH_QUOTE_OVERFLOW")
	}
	t.Log("ASSERT_LAUNCH_QUOTE_14 PASS", q)
}
