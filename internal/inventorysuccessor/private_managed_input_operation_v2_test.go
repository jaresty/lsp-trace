package inventorysuccessor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"strings"
	"sync"
	"testing"
	"time"
)

// controlledManagedOwnerV2 uses ONLY the existing kernel fixture and initializer.
// Its generous logical ceilings are synthetic test authorization, NOT a grant.
func controlledManagedOwnerV2(t *testing.T) (*PrivateManagedInputOwnerV2, harnessV2) {
	t.Helper()
	x := harnessTestV2(t, func(a *authorizationV2) {
		a.Ceiling = countersFromV2([14]uint64{1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12, 1e12})
		a.MaxTranche = a.Ceiling
		a.MaxWindows = 100
		a.MaxWindowNS = uint64(15 * time.Second)
		a.Expires = 4102444800000000000
	})
	raw := sealTestV2(t, &x.j)
	owner, err := OpenPrivateManagedInputOwnerV2(context.Background(), x.k.root, raw, x.h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if err != nil {
		t.Fatal(err)
	}
	return owner, x
}

type managedNeverStartV2 struct{}

func (managedNeverStartV2) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	panic("controlled no-start fixture was unexpectedly invoked")
}

func managedBindingTestV2(x harnessV2) PrivateManagedInputBindingV2 {
	return PrivateManagedInputBindingV2{Manager: new(sessionruntime.Manager), Root: x.k.root, Session: sessionruntime.SessionLeaseRequest{SessionID: "controlled", Generation: 1, Workspace: "/controlled"}, URI: "file:///controlled/query.go", Occurrence: "occ", Revision: "declared", Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Kinds: [3]string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, KindCount: 2, Mode: "ACQUIRE", Deadline: time.Now().Add(10 * time.Second)}
}
func TestPrivateManagedInputUncertainV2(t *testing.T) {
	owner, x := controlledManagedOwnerV2(t)
	b := managedBindingTestV2(x)
	owner.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, err := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
		if err != nil {
			return receipt, err
		}
		return receipt, errors.New("controlled committed-plus-error")
	}
	permit, err := owner.Reserve(context.Background(), hashLabelV2("uncertain-managed"), b)
	if err == nil || permit.state != nil {
		t.Fatal("ASSERT_UNCERTAIN_NO_OPERATION_PERMISSION")
	}
	if ctx, _, err := permit.Consume(context.Background(), b); err == nil || ctx != nil {
		t.Fatal("ASSERT_UNCERTAIN_ZERO_DOWNSTREAM_PERMISSION")
	}
	h, err := owner.k.reopen(context.Background(), x.j, owner.limits)
	if err != nil || h.Disposition != "PENDING" || h.Windows != 1 || h.Charged.Calls == 0 {
		t.Fatalf("ASSERT_UNCERTAIN_STICKY_ACCOUNTING: %+v %v", h, err)
	}
	if _, err = owner.Reserve(context.Background(), hashLabelV2("retry"), b); err == nil {
		t.Fatal("ASSERT_UNCERTAIN_OWNER_STICKY")
	}
	if _, err = OpenPrivateManagedInputOwnerV2(context.Background(), x.k.root, sealTestV2(t, &x.j), h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384}); err == nil {
		t.Fatal("ASSERT_UNCERTAIN_REOPEN_NOT_READY")
	}
	t.Log("ASSERT_UNCERTAIN_NO_OPERATION_PERMISSION: PASS")
	t.Log("ASSERT_UNCERTAIN_STICKY_ACCOUNTING: PASS")
}
func TestPrivateManagedInputFiniteBoundsV2(t *testing.T) {
	for _, mode := range []string{"zero-owner", "expired", "zero-deadline", "extended-window", "unknown-mode", "unknown-kind", "overflow-kinds", "exhausted", "cancelled", "unknown-wire"} {
		t.Run(mode, func(t *testing.T) {
			owner, x := controlledManagedOwnerV2(t)
			b := managedBindingTestV2(x)
			ctx := context.Background()
			switch mode {
			case "zero-owner":
				owner = &PrivateManagedInputOwnerV2{}
			case "expired":
				b.Deadline = time.Now().Add(-time.Second)
			case "zero-deadline":
				b.Deadline = time.Time{}
			case "extended-window":
				b.Deadline = time.Now().Add(time.Minute)
			case "unknown-mode":
				b.Mode = "unknown"
			case "unknown-kind":
				b.Kinds[0] = "unknown"
			case "unknown-wire":
				var err error
				b.Manager, err = sessionruntime.New(sessionruntime.Config{Starter: managedNeverStartV2{}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 4, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Wire: lspwire.Limits{MaxBodyBytes: 1 << 30, MaxHeaderBytes: 64 << 10}})
				if err != nil {
					t.Fatal(err)
				}
			case "overflow-kinds":
				b.KindCount = 255
			case "exhausted":
				low := harnessTestV2(t, nil)
				owner = &PrivateManagedInputOwnerV2{k: low.k, job: low.j, head: low.h.Identity, limits: low.l}
			case "cancelled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			if p, err := owner.Reserve(ctx, hashLabelV2(mode), b); err == nil || p.state != nil {
				t.Fatal("ASSERT_FINITE_OR_UNAFFORDABLE_REFUSAL")
			}
			h, err := x.k.reopen(context.Background(), x.j, x.l)
			if err != nil || h.Windows != 0 {
				t.Fatal("ASSERT_REFUSAL_NO_NEW_CHARGE")
			}
			t.Log("ASSERT_FINITE_OR_UNAFFORDABLE_REFUSAL: PASS")
		})
	}
}
func TestPrivateManagedInputOneShotV2(t *testing.T) {
	owner, x := controlledManagedOwnerV2(t)
	b := managedBindingTestV2(x)
	p, err := owner.Reserve(context.Background(), hashLabelV2("one-shot"), b)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel, err := p.consumeFixed(context.Background(), b)
			if err == nil {
				defer cancel()
				d, _ := ctx.Deadline()
				results <- d.Equal(b.Deadline)
			} else {
				results <- false
			}
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for win := range results {
		if win {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("ASSERT_KERNEL_PERMISSION_ONE_SHOT_DEADLINE")
	}
	h, err := x.k.reopen(context.Background(), x.j, x.l)
	if err != nil || h.Windows != 1 {
		t.Fatal("ASSERT_KERNEL_SINGLE_RESERVATION")
	}
	t.Log("ASSERT_KERNEL_PERMISSION_ONE_SHOT_DEADLINE: PASS")
}

// ControlledManagedInputOwnerForTest is linked ONLY into this package's external
// test variant. It cannot be imported from the production inventorysuccessor
// library. No production setter/callback or alternate authority path exists.
func ControlledManagedInputOwnerForTest(t *testing.T, uncertain bool) (*PrivateManagedInputOwnerV2, *publication.Root) {
	owner, x := controlledManagedOwnerV2(t)
	if uncertain {
		owner.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
			receipt, err := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
			if err != nil {
				return receipt, err
			}
			return receipt, errors.New("controlled committed-plus-error")
		}
	}
	return owner, x.k.root
}

func TestPrivateManagedInputChargeProfileV2(t *testing.T) {
	_, x := controlledManagedOwnerV2(t)
	b := managedBindingTestV2(x)
	now := time.Unix(1000000000, 0)
	b.Deadline = now.Add(10 * time.Second)
	c, err := privateManagedInputChargeV2(b, now)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range c.vector() {
		if n == 0 {
			t.Fatalf("ASSERT_ACQUIRE_ALL_CURRENCIES_FINITE_NONZERO: %d", i)
		}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("CURRENCY_PROFILE_10S=" + string(raw))
}

// Emit fixed canonical TEST specimens for a cross-package test's existing-root
// fixture. No exported test authority or production initialization API is added.
func TestPrivateManagedInputControlledSpecimensV2(t *testing.T) {
	_, x := controlledManagedOwnerV2(t)
	for name, raw := range map[string][]byte{"job": sealTestV2(t, &x.j), "authorization": sealTestV2(t, &x.a), "head": sealTestV2(t, &x.h)} {
		t.Logf("CONTROLLED_SPECIMEN_%s=%s", name, base64.StdEncoding.EncodeToString(raw))
	}
}

func TestPrivateManagedGroupingMEDAggregatePreflight(t *testing.T) {
	source := make([]byte, 1<<20)
	v := sessionruntime.PrivateGroupingOwnedView{Source: source, Supply: sessionruntime.DocumentSupply{Content: source}}
	var err error
	allocations := testing.AllocsPerRun(1, func() { _, err = privateGroupingQueryBytes(v) })
	if err == nil || allocations != 0 {
		t.Fatalf("ASSERT_MED2_AGGREGATE_BEFORE_DERIVED_COPY_OR_MARSHAL: err=%v allocations=%g", err, allocations)
	}
	t.Log("ASSERT_MED2_AGGREGATE_BEFORE_DERIVED_COPY_OR_MARSHAL: PASS")
}

func TestPrivateManagedGroupingMEDArithmeticAndAggregateBoundary(t *testing.T) {
	cap := sessionruntime.PrivateGroupingQueryMaxBytes
	if cap != uint64(chainMaxBytes) {
		t.Fatal("ASSERT_MED2_QUERY_CAP_MATCH")
	}
	for _, tc := range []struct {
		total, n, mult, extra uint64
		want                  bool
	}{{cap - 1, 1, 1, 0, true}, {cap, 1, 1, 0, false}, {0, ^uint64(0), 6, 2, false}, {^uint64(0), 0, 0, 0, false}, {0, 0, 0, ^uint64(0), false}} {
		result, ok := sessionruntime.PrivateGroupingCheckedEncodedSize(tc.total, tc.n, tc.mult, tc.extra)
		if ok != tc.want || (ok && result > cap) {
			t.Fatalf("ASSERT_MED2_CHECKED_ENCODING_ARITHMETIC: %+v result=%d ok=%v", tc, result, ok)
		}
	}
	buffer := make([]byte, int(cap))
	low, high := 0, len(buffer)
	for low < high {
		mid := low + (high-low+1)/2
		_, ok := sessionruntime.PrivateGroupingQueryEncodedBound(sessionruntime.PrivateGroupingOwnedView{Source: buffer[:mid]})
		if ok {
			low = mid
		} else {
			high = mid - 1
		}
	}
	v := sessionruntime.PrivateGroupingOwnedView{Source: buffer[:low]}
	bound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(v)
	if !ok {
		t.Fatal("ASSERT_MED2_AGGREGATE_BOUNDARY_ADMITTED")
	}
	raw, err := privateGroupingQueryBytes(v)
	if err != nil || uint64(len(raw)) > bound {
		t.Fatalf("ASSERT_MED2_AGGREGATE_BOUNDARY_CONTROL: len=%d bound=%d err=%v", len(raw), bound, err)
	}
	v.Source = buffer[:low+1]
	if _, err := privateGroupingQueryBytes(v); err == nil {
		t.Fatal("ASSERT_MED2_AGGREGATE_BOUNDARY_PLUS_ONE_REFUSED")
	}
	for _, bad := range []sessionruntime.PrivateGroupingOwnedView{{Methods: make([]sessionruntime.OwnedMethodPair, 3)}, {Unavailable: []string{"a", "b", "c"}}, {Metadata: sessionruntime.SessionMetadata{ProviderName: string([]byte{255})}}, {Metadata: sessionruntime.SessionMetadata{ServerCommand: strings.Repeat("x", 4097)}}} {
		if _, ok := sessionruntime.PrivateGroupingQueryEncodedBound(bad); ok {
			t.Fatal("ASSERT_MED2_QUERY_METADATA_COUNT_UTF8_REFUSED")
		}
	}
	escaped := strings.Repeat("<\x00", 2048)
	worst := sessionruntime.PrivateGroupingOwnedView{Revision: escaped, Metadata: sessionruntime.SessionMetadata{ServerCommand: escaped}, Target: adr0011querytarget.Candidate{SymbolName: escaped}}
	worstBound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(worst)
	worstRaw, worstErr := privateGroupingQueryBytes(worst)
	if !ok || worstErr != nil || uint64(len(worstRaw)) > worstBound {
		t.Fatalf("ASSERT_MED2_JSON_ESCAPING_UPPER_BOUND: bytes=%d bound=%d err=%v", len(worstRaw), worstBound, worstErr)
	}
	t.Log("ASSERT_MED2_CHECKED_ENCODING_ARITHMETIC: PASS")
}
