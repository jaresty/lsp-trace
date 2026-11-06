package inventorysuccessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

// Test-variant only: the existing CAS seam is armed AFTER successful Reserve,
// so the fixed continuation reaches actual Observe, not an uncertain Reserve.
func ControlledMixedObserveUncertainForTest(o *PrivateManagedInputOwnerV2) {
	o.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, err := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
		if err != nil {
			return receipt, err
		}
		return receipt, errors.New("controlled Observe committed-plus-error")
	}
}
func ControlledMixedHeadForTest(t *testing.T, o *PrivateManagedInputOwnerV2) (string, [14]uint64) {
	t.Helper()
	h, err := o.k.reopen(context.Background(), o.job, o.limits)
	if err != nil {
		t.Fatal(err)
	}
	return h.Disposition, h.Charged.vector()
}
func ControlledMixedInsufficientForTest(t *testing.T) (*PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	x := harnessTestV2(t, func(a *authorizationV2) {
		a.Ceiling = countersFromV2([14]uint64{1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12})
		a.MaxTranche = a.Ceiling
		a.MaxTranche.RequestBytes = 1
		a.MaxWindows = 100
		a.MaxWindowNS = uint64(15 * time.Second)
		a.Expires = 4102444800000000000
	})
	o, err := OpenPrivateManagedInputOwnerV2(context.Background(), x.k.root, sealTestV2(t, &x.j), x.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if err != nil {
		t.Fatal(err)
	}
	return o, x.k.root
}

// Test-only preflight reads the current initialized authorization, not a DTO
// supplied to Reserve. Every coordinate except the one named shortage must fit.
func ControlledMixedFundingPreflightForTest(t *testing.T, o *PrivateManagedInputOwnerV2, q PrivateMixedInputRequestV2, shortage bool) string {
	t.Helper()
	h, err := o.k.reopen(context.Background(), o.job, o.limits)
	if err != nil || h.Identity != o.head || !readyV2(h) || h.Windows != 0 || h.Charged != (countersV2{}) {
		t.Fatal("ASSERT_MIXED_FRESH_HEAD", err)
	}
	b, err := budgetV2(o.limits)
	if err != nil {
		t.Fatal(err)
	}
	a, err := o.k.currentAuthorization(context.Background(), h, b)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if now.UnixNano() >= a.Expires || q.Start.Deadline.UnixNano() > a.Expires || a.Expires != 4102444800000000000 || a.MaxWindows != 100 || a.MaxWindowNS != uint64(15*time.Second) {
		t.Fatal("ASSERT_MIXED_FRESH_AUTHORIZATION")
	}
	plan, ok := privateMixedPlanV2(q)
	if !ok {
		t.Fatal("ASSERT_MIXED_FRESH_REQUEST")
	}
	envelope, err := q.Manager.PrivateProviderWindowEnvelope(q.Start, plan)
	if err != nil {
		t.Fatal(err)
	}
	charge, err := privateMixedChargeV2(now, q.Start.Deadline, envelope.Bounds())
	if err != nil {
		t.Fatal(err)
	}
	ceiling, tranche := a.Ceiling.vector(), a.MaxTranche.vector()
	for i, n := range charge.vector() {
		if ceiling[i] != 1e12 || (i != 10 || !shortage) && tranche[i] != 1e12 {
			t.Fatalf("ASSERT_MIXED_ONE_CURRENCY: coordinate=%d", i)
		}
		if n > ceiling[i] || (i != 10 || !shortage) && n > tranche[i] {
			t.Fatalf("ASSERT_MIXED_OTHER_CURRENCY_FITS: %d", i)
		}
	}
	if shortage {
		if a.MaxTranche.RequestBytes != 1 || charge.RequestBytes <= 1 || validTrancheV2(a, charge, now.UnixNano(), q.Start.Deadline.UnixNano()) == nil {
			t.Fatal("ASSERT_MIXED_ISOLATED_REQUEST_BYTES")
		}
		a.MaxTranche.RequestBytes = 1e12 // local counterfactual only, never published
	}
	if validTrancheV2(a, charge, now.UnixNano(), q.Start.Deadline.UnixNano()) != nil {
		t.Fatal("ASSERT_MIXED_RESTORED_COORDINATE_VALID")
	}
	t.Logf("ASSERT_MIXED_FRESH_AUTHORIZATION: PASS; current initialized head READY; shortage=%t; expiry=%d; all other coordinates unchanged; restored RequestBytes valid", shortage, a.Expires)
	return h.Identity
}
func ControlledMixedHeadIdentityForTest(t *testing.T, o *PrivateManagedInputOwnerV2) string {
	t.Helper()
	h, err := o.k.reopen(context.Background(), o.job, o.limits)
	if err != nil {
		t.Fatal(err)
	}
	return h.Identity
}

func TestPrivateInventoryMixedLeaseReplayV2Quote(t *testing.T) {
	now := time.Unix(1000000000, 0)
	w := sessionruntime.PrivateProviderWindowBounds{SourceReadBytes: 32770, RequestBytes: 2000000, ResponseBytes: 5000000, WireBytes: 7000000, Frames: 32}
	c, err := privateMixedChargeV2(now, now.Add(time.Second), w)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range c.vector() {
		if n == 0 || n > 1e12 {
			t.Fatalf("ASSERT_MIXED_FINITE_14_QUOTE: coordinate=%d value=%d", i, n)
		}
	}
	t.Logf("ASSERT_MIXED_FINITE_14_QUOTE: PASS; %v", c.vector())
	for _, deadline := range []time.Time{now, now.Add(16 * time.Second)} {
		if _, e := privateMixedChargeV2(now, deadline, w); e == nil {
			t.Fatal("ASSERT_MIXED_QUOTE_DEADLINE")
		}
	}
}
func TestPrivateInventoryMixedLeaseReplayV2PreEncodeBoundary(t *testing.T) {
	// All opaque predecessor bytes are counted before base64/encoding. Test both
	// the exact bound and the immediately exceeding value without giant allocs.
	for _, n := range []int{0, 1, 4096, 100000} {
		value := struct{ Body []byte }{make([]byte, n)}
		bound, ok := referencegrouping.PrivateNativeEncodedBoundV2(value, 1<<20)
		if !ok || bound == 0 {
			t.Fatal("ASSERT_MIXED_PREENCODE_BOUND")
		}
		if _, ok = referencegrouping.PrivateNativeEncodedBoundV2(value, bound); !ok {
			t.Fatal("ASSERT_MIXED_PREENCODE_EQUALITY")
		}
		if _, ok = referencegrouping.PrivateNativeEncodedBoundV2(value, bound-1); ok {
			t.Fatal("ASSERT_MIXED_PREENCODE_OVERFLOW")
		}
	}
	if _, ok := referencegrouping.PrivateNativeEncodedBoundV2(struct{ Body []byte }{make([]byte, 1<<20)}, 1<<20); ok {
		t.Fatal("ASSERT_MIXED_PREENCODE_1MIB")
	}
	t.Log("ASSERT_MIXED_PREENCODE_BOUND: PASS; equality admitted, +1 refused before encoding")
}
