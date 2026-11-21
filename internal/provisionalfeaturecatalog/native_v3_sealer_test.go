package provisionalfeaturecatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A package-private transition fixture, not a native admission proof. It tests
// the one shared decision algorithm without acquisition or opaque DTO forgery.
func sealerFixture(t *testing.T) NativeCatalogV3 {
	t.Helper()
	b, e := NewCallerBudgetV3(time.Now().Add(30*time.Second), 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	c, e := BuildNativeV3(context.Background(), b, "interpreter", nil, ReviewModeSameThreadV3)
	if e != nil {
		t.Fatal(e)
	}
	w := c.state.wire
	w.Evidence = []SelectedEvidenceV3{{ID: "e1"}, {ID: "e2"}}
	w.WithoutMembership = 2
	c, e = sealNativeV3(context.Background(), b, w, "interpreter", false)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func sealerDecision(t *testing.T, c NativeCatalogV3) []byte {
	t.Helper()
	raw, e := json.Marshal(NativeDecisionV3{Predecessor: c.Revision(), Actor: "interpreter", Reason: "data-only transition", Affected: []string{}, Groups: []NativeGroupV3{{ID: "group", Label: "group", Reason: "data-only", Members: []string{"e2", "e1"}}}})
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestPrivateSealerParity(t *testing.T) {
	ctx := context.Background()
	a, b := sealerFixture(t), sealerFixture(t)
	raw := sealerDecision(t, a)
	public, e := ApplyNativeDecisionV3(ctx, a, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	private, e := applyNativeDecisionWithSeal(ctx, b, raw, false, func(ctx context.Context, budget NativeBudgetV3, w NativeCatalogWireV3, actor string, review bool) (NativeCatalogV3, error) {
		calls++
		if b.state.mu.TryLock() {
			b.state.mu.Unlock()
			t.Fatal("sealer callback escaped transition mutex")
		}
		return sealNativeV3(ctx, budget, w, actor, review)
	})
	if e != nil || calls != 1 {
		t.Fatal("private sealer call", calls, e)
	}
	x, e := public.Bytes(ctx)
	if e != nil {
		t.Fatal(e)
	}
	y, e := private.Bytes(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(x, y) || public.Revision() != private.Revision() {
		t.Fatal("public/private sealer parity")
	}
	// Review through the same algorithm preserves the exact full-format bytes too.
	decision := NativeDecisionV3{Predecessor: public.Revision(), Actor: "interpreter", Reason: "review", Affected: []string{"group"}, Groups: []NativeGroupV3{}}
	raw, e = json.Marshal(decision)
	if e != nil {
		t.Fatal(e)
	}
	public, e = ApplyNativeDecisionV3(ctx, public, raw, true)
	if e != nil {
		t.Fatal(e)
	}
	private, e = applyNativeDecisionWithSeal(ctx, private, raw, true, sealNativeV3)
	if e != nil {
		t.Fatal(e)
	}
	x, e = public.Bytes(ctx)
	if e != nil {
		t.Fatal(e)
	}
	y, e = private.Bytes(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(x, y) {
		t.Fatal("review sealer parity")
	}
}
func TestPrivateSealerGuards(t *testing.T) {
	for _, kind := range []string{"predecessor", "actor", "review-base", "membership", "affected"} {
		t.Run(kind, func(t *testing.T) {
			c := sealerFixture(t)
			if kind == "affected" {
				var err error
				c, err = ApplyNativeDecisionV3(context.Background(), c, sealerDecision(t, c), false)
				if err != nil {
					t.Fatal(err)
				}
			}
			raw := sealerDecision(t, c)
			var d NativeDecisionV3
			if json.Unmarshal(raw, &d) != nil {
				t.Fatal("decision")
			}
			review := false
			switch kind {
			case "predecessor":
				d.Predecessor = "wrong"
			case "actor":
				d.Actor = "wrong"
			case "review-base":
				review = true
			case "membership":
				d.Groups[0].Members = []string{"unknown"}
			case "affected":
				review = true
				d.Groups = []NativeGroupV3{} // deleting the predecessor group must name it
				d.Affected = []string{}
			}
			raw, _ = json.Marshal(d)
			calls := 0
			_, e := applyNativeDecisionWithSeal(context.Background(), c, raw, review, func(context.Context, NativeBudgetV3, NativeCatalogWireV3, string, bool) (NativeCatalogV3, error) {
				calls++
				return NativeCatalogV3{}, nil
			})
			if e != ErrNativeCatalogV3 || calls != 0 || c.state.successor {
				t.Fatalf("guard before sealer: err=%v calls=%d consumed=%v", e, calls, c.state.successor)
			}
			// Match the public error, not merely any failure.
			_, e = ApplyNativeDecisionV3(context.Background(), c, raw, review)
			if e != ErrNativeCatalogV3 {
				t.Fatal("public error parity", e)
			}
		})
	}
}
func TestPrivateSealerFailureDoesNotConsume(t *testing.T) {
	c := sealerFixture(t)
	raw := sealerDecision(t, c)
	want := errors.New("bounded sealer refusal")
	calls := 0
	seal := func(context.Context, NativeBudgetV3, NativeCatalogWireV3, string, bool) (NativeCatalogV3, error) {
		calls++
		return NativeCatalogV3{}, want
	}
	if _, e := applyNativeDecisionWithSeal(context.Background(), c, raw, false, seal); e != want || calls != 1 || c.state.successor {
		t.Fatalf("failed seal consumed successor: %v %d %v", e, calls, c.state.successor)
	}
	if _, e := ApplyNativeDecisionV3(context.Background(), c, raw, false); e != nil {
		t.Fatal("retry after failed seal", e)
	}
	if _, e := applyNativeDecisionWithSeal(context.Background(), c, raw, false, seal); e != ErrNativeCatalogV3 || calls != 1 {
		t.Fatal("spent successor reached callback", e, calls)
	}
}
func TestPrivateSealerConcurrentSingleSuccess(t *testing.T) {
	c := sealerFixture(t)
	raw := sealerDecision(t, c)
	var calls, success atomic.Int32
	var wg sync.WaitGroup
	seal := func(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, a string, r bool) (NativeCatalogV3, error) {
		calls.Add(1)
		return sealNativeV3(ctx, b, w, a, r)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := applyNativeDecisionWithSeal(context.Background(), c, raw, false, seal)
			if e == nil {
				success.Add(1)
			} else if e != ErrNativeCatalogV3 {
				t.Errorf("unexpected error %v", e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || success.Load() != 1 {
		t.Fatalf("single successor: calls=%d success=%d", calls.Load(), success.Load())
	}
}
