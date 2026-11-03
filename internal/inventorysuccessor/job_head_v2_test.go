package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
)

type harnessV2 struct {
	k *kernelV2
	j jobV2
	a authorizationV2
	h resourceHeadV2
	l verificationLimitsV2
}

func harnessTestV2(t *testing.T, change func(*authorizationV2)) harnessV2 {
	t.Helper()
	p := t.TempDir()
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := publication.OpenRoot(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	j, a, d := fixtureV2(t)
	if change != nil {
		change(&a)
		sealTestV2(t, &a)
		d.Authorization = a.Identity
		d.Reference = a.Decision
	}
	k := &kernelV2{root: r}
	l := verificationLimitsV2{Records: 400, Bytes: 4 << 20, Work: 400}
	h, err := k.initialize(context.Background(), j, a, d, 10, l)
	if err != nil {
		t.Fatal(err)
	}
	return harnessV2{k, j, a, h, l}
}
func renewalTestV2(t *testing.T, x harnessV2, h resourceHeadV2) (authorizationV2, decisionV2) {
	t.Helper()
	a := x.a
	a.Revision++
	a.Previous = x.a.Identity
	a.BoundHead = h.Identity
	a.Charged = h.Charged
	a.Ceiling.Calls = 12
	a.MaxWindows = 6
	a.Expires = 120
	sealTestV2(t, &a)
	return a, decisionV2{x.j.Identity, a.Identity, a.Decision}
}
func claimTestV2(t *testing.T, x harnessV2, h resourceHeadV2, attempt string) resourceHeadV2 {
	t.Helper()
	r, e := x.k.reserve(context.Background(), x.j, h.Identity, hashLabelV2(attempt), countersV2{Calls: 3}, 30, 11, x.l)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func finishTestV2(t *testing.T, x harnessV2, h resourceHeadV2, attempt string) resourceHeadV2 {
	t.Helper()
	r, e := x.k.observe(context.Background(), x.j, h.Identity, hashLabelV2(attempt), "OBSERVED", 12, x.l)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func sameHeadTestV2(t *testing.T, x harnessV2, h resourceHeadV2) {
	t.Helper()
	got, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || got.Identity != h.Identity {
		t.Fatalf("refusal changed head: %v", e)
	}
}

func TestJobV2RenewalCarry(t *testing.T) {
	ctx := context.Background()
	x := harnessTestV2(t, nil)
	h := claimTestV2(t, x, x.h, "renew-attempt")
	h = finishTestV2(t, x, h, "renew-attempt")
	a, d := renewalTestV2(t, x, h)
	for name, mutate := range map[string]func(*authorizationV2){
		"stale binding": func(a *authorizationV2) { a.BoundHead = x.h.Identity }, "reset snapshot": func(a *authorizationV2) { a.Charged = countersV2{} }, "missing predecessor": func(a *authorizationV2) { a.Previous = hashLabelV2("missing") }, "foreign job": func(a *authorizationV2) { a.Job = hashLabelV2("foreign") }, "revision gap": func(a *authorizationV2) { a.Revision = 3 }, "shrink ceiling": func(a *authorizationV2) { a.Ceiling.Calls = 7 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := a
			mutate(&bad)
			sealTestV2(t, &bad)
			decision := decisionV2{x.j.Identity, bad.Identity, bad.Decision}
			if _, e := x.k.renew(ctx, x.j, h.Identity, bad, decision, 13, x.l); e == nil {
				t.Fatal("invalid linked renewal admitted")
			}
			sameHeadTestV2(t, x, h)
		})
	}
	if _, e := x.k.renew(ctx, x.j, h.Identity, a, decisionV2{}, 13, x.l); e == nil {
		t.Fatal("JSON authorization bypassed parent decision")
	}
	if _, e := x.k.renew(ctx, x.j, x.h.Identity, a, d, 13, x.l); e == nil {
		t.Fatal("stale expected head admitted")
	}
	next, e := x.k.renew(ctx, x.j, h.Identity, a, d, 13, x.l)
	if e != nil {
		t.Fatal(e)
	}
	if next.Charged.Calls != 3 || next.Windows != 1 || next.Job.Identity != x.j.Identity {
		t.Fatal("renewal reset spent prefix")
	}
	if _, e = x.k.renew(ctx, x.j, next.Identity, a, d, 14, x.l); e == nil {
		t.Fatal("authorization revision replayed")
	}
	next, e = x.k.reserve(ctx, x.j, next.Identity, hashLabelV2("next-window"), countersV2{Calls: 4}, 35, 14, x.l)
	if e != nil || next.Charged.Calls != 7 {
		t.Fatalf("renewal carry: %v %+v", e, next.Charged)
	}
	if _, e = x.k.reopen(ctx, x.j, x.l); e != nil {
		t.Fatal(e)
	}
}

func TestJobV2StickyAndSpent(t *testing.T) {
	ctx := context.Background()
	for _, outcome := range []string{"PENDING", "FAILED", "INDETERMINATE", "OBSERVED"} {
		t.Run(outcome, func(t *testing.T) {
			x := harnessTestV2(t, nil)
			h := claimTestV2(t, x, x.h, "original")
			var e error
			if outcome != "PENDING" {
				h, e = x.k.observe(ctx, x.j, h.Identity, hashLabelV2("original"), outcome, 12, x.l)
				if e != nil {
					t.Fatal(e)
				}
			}
			if _, e = x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("original"), countersV2{Calls: 1}, 30, 13, x.l); e == nil {
				t.Fatal("spent attempt replay admitted")
			}
			if outcome != "OBSERVED" {
				if _, e = x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("another"), countersV2{Calls: 1}, 30, 13, x.l); e == nil {
					t.Fatal("sticky claim allowed another window")
				}
				a, d := renewalTestV2(t, x, h)
				if _, e = x.k.renew(ctx, x.j, h.Identity, a, d, 13, x.l); e == nil {
					t.Fatal("renewal cleared ambiguous/failure claim")
				}
			}
			if h.Charged.Calls != 3 || h.Windows != 1 {
				t.Fatal("observation refunded or double-charged prepaid child")
			}
			if _, e = x.k.observe(ctx, x.j, h.Identity, hashLabelV2("foreign receipt"), "OBSERVED", 13, x.l); e == nil {
				t.Fatal("unknown receipt restored authority")
			}
			sameHeadTestV2(t, x, h)
		})
	}
}

func TestJobV2FiniteAdmission(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"per-window", "zero denied currency", "zero tranche", "deadline", "expired", "max windows", "foreign source", "foreign policy", "stale head"} {
		t.Run(name, func(t *testing.T) {
			x := harnessTestV2(t, func(a *authorizationV2) { a.MaxWindows = 1 })
			h := x.h
			j := x.j
			expected := h.Identity
			c := countersV2{Calls: 3}
			deadline, now := int64(30), int64(11)
			switch name {
			case "per-window":
				c.Calls = 5
			case "zero denied currency":
				c.OutputBytes = 1
			case "zero tranche":
				c = countersV2{}
			case "deadline":
				deadline = 50
			case "expired":
				now = 100
				deadline = 101
			case "max windows":
				h = claimTestV2(t, x, h, "first")
				h = finishTestV2(t, x, h, "first")
				expected = h.Identity
				now = 13
			case "foreign source":
				j.SourceAuthorization = hashLabelV2("foreign source")
				sealTestV2(t, &j)
			case "foreign policy":
				j.ResourceRules = hashLabelV2("foreign policy")
				sealTestV2(t, &j)
			case "stale head":
				expected = hashLabelV2("stale")
			}
			if _, e := x.k.reserve(ctx, j, expected, hashLabelV2("rejected"), c, deadline, now, x.l); e == nil {
				t.Fatalf("finite admission accepted: %s", name)
			}
			sameHeadTestV2(t, x, h)
		})
	}
	x := harnessTestV2(t, nil)
	h := claimTestV2(t, x, x.h, "expires")
	if _, e := x.k.observe(ctx, x.j, h.Identity, hashLabelV2("expires"), "OBSERVED", 30, x.l); e == nil {
		t.Fatal("expired window refreshed")
	}
	a, d := renewalTestV2(t, x, x.h)
	if _, e := x.k.renew(ctx, x.j, x.h.Identity, a, d, 100, x.l); e == nil {
		t.Fatal("expired job renewed")
	}
}

func TestJobV2BoundedVerification(t *testing.T) {
	x := harnessTestV2(t, nil)
	h := claimTestV2(t, x, x.h, "bounded")
	h = finishTestV2(t, x, h, "bounded")
	ctx := context.Background()
	limits := []verificationLimitsV2{{}, {1, 1 << 20, 100}, {100, headLimitV2 - 1, 100}, {100, 1 << 20, 1}, {4097, 1 << 20, 100}, {100, 65 << 20, 100}, {100, 1 << 20, 16385}}
	for i, l := range limits {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, e := x.k.reopen(ctx, x.j, l); !errors.Is(e, errVerificationStopV2) {
				t.Fatalf("wanted typed verification stop: %v", e)
			}
			if _, e := x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("no-permission"), countersV2{Calls: 1}, 30, 13, l); !errors.Is(e, errVerificationStopV2) {
				t.Fatalf("partial history gave permission: %v", e)
			}
		})
	}
	// Five reads verify this prefix only partially; the admission read may not
	// obtain a reset allowance after replay consumes its incoming budget.
	if _, e := x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("no-budget-reset"), countersV2{Calls: 1}, 30, 13, verificationLimitsV2{9, 1 << 20, 9}); !errors.Is(e, errVerificationStopV2) {
		t.Fatalf("shared verification budget: %v", e)
	}
	sameHeadTestV2(t, x, h)
}

func TestJobV2PrefixCorruptionAndNoImport(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"missing auth", "missing predecessor", "torn reservation", "foreign reservation", "unknown head", "oversized head", "v1 head", "nonzero initial", "missing selected"} {
		t.Run(kind, func(t *testing.T) {
			x := harnessTestV2(t, nil)
			h := claimTestV2(t, x, x.h, "corruption")
			path := x.k.root.Path()
			selector := ""
			var raw []byte
			switch kind {
			case "missing auth":
				selector = h.AuthorizationSelector
			case "missing predecessor":
				selector = recordSelectorV2(headsNamespaceV2, h.Previous)
			case "torn reservation":
				selector = h.ReservationSelector
				raw = []byte("{\n")
			case "foreign reservation":
				selector = h.ReservationSelector
				var r reservationV2
				b, e := x.k.root.ReadSelector(selector, recordLimitV2)
				if e != nil {
					t.Fatal(e)
				}
				if parseV2(b, &r) != nil {
					t.Fatal("fixture")
				}
				r.Job = hashLabelV2("foreign")
				raw = sealTestV2(t, &r)
			case "unknown head":
				selector = currentSelectorV2(x.j)
				b, _ := x.k.root.ReadSelector(selector, headLimitV2)
				raw = bytes.Replace(b, []byte("\"RESERVE\""), []byte("\"RUN_MODEL\""), 1)
			case "oversized head":
				selector = currentSelectorV2(x.j)
				raw = bytes.Repeat([]byte("x"), headLimitV2+1)
			case "v1 head":
				selector = currentSelectorV2(x.j)
				raw = []byte("{\"schema_version\":\"lsp-trace.inventory-successor-head.private.v1\",\"generation\":1}\n")
			case "nonzero initial":
				bad := x.h
				bad.Charged.Calls = 1
				if _, e := sealV2(&bad); e == nil {
					t.Fatal("nonzero prefix import resealed")
				}
				return
			case "missing selected":
				selector = currentSelectorV2(x.j)
			}
			full := filepath.Join(path, selector)
			if raw == nil {
				if e := os.Remove(full); e != nil {
					t.Fatal(e)
				}
			} else if e := os.WriteFile(full, raw, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := x.k.reopen(ctx, x.j, x.l); e == nil {
				t.Fatal("corrupt prefix trusted")
			}
			if _, e := x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("no replay"), countersV2{Calls: 1}, 30, 13, x.l); e == nil {
				t.Fatal("corrupt prefix executed")
			}
			_, _, d := fixtureV2(t)
			if _, e := x.k.initialize(ctx, x.j, x.a, d, 13, x.l); e == nil {
				t.Fatal("missing/torn history initialized new ledger")
			}
		})
	}
}

func TestJobV2ConcurrentCAS(t *testing.T) {
	x := harnessTestV2(t, nil)
	r, e := publication.OpenRoot(x.k.root.Path())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	other := &kernelV2{root: r}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i, k := range []*kernelV2{x.k, other} {
		wg.Add(1)
		go func(i int, k *kernelV2) {
			defer wg.Done()
			<-start
			_, e := k.reserve(context.Background(), x.j, x.h.Identity, hashLabelV2(fmt.Sprint(i)), countersV2{Calls: 3}, 30, 11, x.l)
			errs <- e
		}(i, k)
	}
	close(start)
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent winners=%d, want 1", wins)
	}
	h, e := x.k.reopen(context.Background(), x.j, x.l)
	if e != nil || h.Charged.Calls != 3 || h.Windows != 1 {
		t.Fatalf("CAS double spend: %v %+v", e, h)
	}
}

func TestJobV2PublicationFaults(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"orphan", "torn publish", "committed error", "pre-cancel", "post-commit cancel"} {
		t.Run(kind, func(t *testing.T) {
			x := harnessTestV2(t, nil)
			callCtx := ctx
			switch kind {
			case "orphan":
				x.k.cas = func(context.Context, *publication.Root, string, publication.BoundFilePredecessor, []byte, func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
					return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
				}
			case "torn publish":
				x.k.publish = func(req publication.Request) publication.Result {
					if strings.HasPrefix(req.Selector, reservationsNamespaceV2) {
						req.Bytes = []byte("{\n")
					}
					return publication.NewPublisher().Publish(req)
				}
			case "committed error":
				x.k.cas = func(c context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
					receipt, e := publication.CompareAndReplaceBoundFile(c, r, s, p, b, v)
					if e != nil {
						return receipt, e
					}
					return receipt, errors.New("synthetic post-commit uncertainty")
				}
			case "pre-cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = c
			case "post-commit cancel":
				c, cancel := context.WithCancel(ctx)
				defer cancel()
				callCtx = c
				x.k.cas = func(c context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
					receipt, e := publication.CompareAndReplaceBoundFile(c, r, s, p, b, v)
					if e != nil {
						return receipt, e
					}
					cancel()
					return receipt, c.Err()
				}
			}
			returned, e := x.k.reserve(callCtx, x.j, x.h.Identity, hashLabelV2("fault"), countersV2{Calls: 3}, 30, 11, x.l)
			if e == nil || returned.Identity != "" {
				t.Fatal("fault returned permission")
			}
			x.k.cas = nil
			x.k.publish = nil
			h, re := x.k.reopen(ctx, x.j, x.l)
			if re != nil {
				t.Fatal(re)
			}
			if kind == "committed error" || kind == "post-commit cancel" {
				if !errors.Is(e, errUncertainV2) || h.Charged.Calls != 3 || h.Disposition != "PENDING" {
					t.Fatalf("uncertain charge lost: %v %+v", e, h)
				}
				if _, e = x.k.reserve(ctx, x.j, h.Identity, hashLabelV2("retry"), countersV2{Calls: 1}, 30, 12, x.l); e == nil {
					t.Fatal("uncertain claim replayed")
				}
			} else if h.Identity != x.h.Identity {
				t.Fatal("unselected orphan charged job")
			}
		})
	}
}

func TestJobV2PrivateRootAndIsolation(t *testing.T) {
	x := harnessTestV2(t, nil)
	claimTestV2(t, x, x.h, "private")
	if e := filepath.WalkDir(x.k.root.Path(), func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		want := os.FileMode(0600)
		if d.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			return fmt.Errorf("private mode %s: %o", path, info.Mode().Perm())
		}
		rel, e := filepath.Rel(x.k.root.Path(), path)
		if e != nil {
			return e
		}
		if rel != "." && rel != "inventory-successor" && !strings.HasPrefix(filepath.ToSlash(rel), "inventory-successor/") {
			return fmt.Errorf("non-resource effect: %s", rel)
		}
		if !d.IsDir() && d.Name() != ".lsp-trace-candidate-publication.lock" && !strings.HasSuffix(d.Name(), ".json") {
			return fmt.Errorf("non-resource file: %s", rel)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	y := harnessTestV2(t, nil)
	sameHeadTestV2(t, y, y.h)
	// There are no provider/model/semantic hooks to fake or invoke: every effect
	// is rooted publication, and independent roots retain independent counters.
	h, e := y.k.reopen(context.Background(), y.j, y.l)
	if e != nil || h.Charged != (countersV2{}) {
		t.Fatal("root-local state leaked")
	}
	if _, e = x.k.initialize(context.Background(), x.j, x.a, decisionV2{}, 10, x.l); e == nil {
		t.Fatal("initializer reused spent job")
	}
}

func TestJobV2ChargedCarry(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	k := &kernelV2{root: root}
	j, a, d := fixtureV2(t)
	l := verificationLimitsV2{Records: 100, Bytes: 1 << 20, Work: 100}
	h, err := k.initialize(ctx, j, a, d, 10, l)
	if err != nil {
		t.Fatal(err)
	}
	h, err = k.reserve(ctx, j, h.Identity, hashLabelV2("attempt-1"), countersV2{Calls: 3}, 30, 11, l)
	if err != nil {
		t.Fatal(err)
	}
	h, err = k.observe(ctx, j, h.Identity, hashLabelV2("attempt-1"), "OBSERVED", 12, l)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	root, err = publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	k = &kernelV2{root: root}
	h, err = k.reopen(ctx, j, l)
	if err != nil {
		t.Fatal(err)
	}
	h, err = k.reserve(ctx, j, h.Identity, hashLabelV2("attempt-2"), countersV2{Calls: 3}, 30, 13, l)
	if err != nil {
		t.Fatal(err)
	}
	if h.Charged.Calls != 6 || h.Windows != 2 {
		t.Fatalf("cumulative carry: got calls=%d windows=%d, want 6/2", h.Charged.Calls, h.Windows)
	}
	h, err = k.observe(ctx, j, h.Identity, hashLabelV2("attempt-2"), "OBSERVED", 14, l)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.reserve(ctx, j, h.Identity, hashLabelV2("attempt-3"), countersV2{Calls: 3}, 30, 15, l); err == nil {
		t.Fatal("whole-job ceiling: overspend admitted")
	}
}
