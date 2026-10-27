//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func bundleFixture(t *testing.T, equal bool) (*scopeBundleOwner, featureScope) {
	t.Helper()
	p, b, pp, bp := retentionRoots(t)
	if err := errors.Join(p.Close(), b.Close()); err != nil {
		t.Fatal(err)
	}
	jp := filepath.Join(filepath.Dir(pp), "journal")
	if err := os.Mkdir(jp, 0700); err != nil {
		t.Fatal(err)
	}
	scope := retentionFixture(t, equal)
	owner, err := newScopeBundleOwner(scope, pp, bp, jp)
	if err != nil {
		t.Fatal(err)
	}
	return owner, scope
}
func bundleHeadPath(o *scopeBundleOwner) string {
	key := scopeBundleKey(*o.binding)
	return filepath.Join(o.paths[2], "feature-scope-bundle-head-private-v1-"+key[7:]+".json")
}
func runBundle(t *testing.T, o *scopeBundleOwner) scopeBundleResult {
	t.Helper()
	var cap *scopeBundlePending
	var out scopeBundleResult
	total := len(o.plan.state.objects) + 1
	for i := 0; i < total*2; i++ {
		oldCalls, oldPairs := 0, 0
		ops := o.ops
		o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
			oldCalls++
			return ops.cas(ctx, r, key, old, new)
		}
		o.ops.pair = func(ctx context.Context, p scopeRetentionPlan, n int, a, b *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
			oldPairs++
			return ops.pair(ctx, p, n, a, b)
		}
		var err error
		out, err = o.advance(context.Background(), cap)
		o.ops = ops
		if err != nil || oldCalls != 1 || oldPairs > 1 {
			t.Fatalf("one CAS/attempt: %s cas=%d pairs=%d err=%v", out.outcome, oldCalls, oldPairs, err)
		}
		if i%2 == 0 {
			if out.outcome != "CLAIMED" || out.pending == nil || oldPairs != 1 {
				t.Fatalf("claim: %+v", out)
			}
		} else if out.pending != nil || oldPairs != 0 {
			t.Fatal("finalize executed pair")
		}
		cap = out.pending
	}
	if out.outcome != "RETAINED_INPUT_BUNDLE" || !out.casCommitted || out.authority != 0 || out.accepted || out.completeness != "UNKNOWN" || out.identity != "UNRESOLVED" {
		t.Fatalf("qualified final: %+v", out)
	}
	return out
}
func TestScopeBundleCompleteProductionAndReopen(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(map[bool]string{false: "distinct", true: "dedup"}[equal], func(t *testing.T) {
			o, scope := bundleFixture(t, equal)
			runBundle(t, o)
			if equal && len(o.plan.state.objects) != 4 {
				t.Fatal("dedup storage count")
			}
			fresh, err := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
			if err != nil {
				t.Fatal(err)
			}
			fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
				t.Fatal("reopen CAS")
				return nil, nil
			}
			fresh.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				t.Fatal("reopen pair")
				return scopeRetentionAttempt{}, nil
			}
			out, err := fresh.advance(context.Background(), nil)
			if err != nil || out.outcome != "RETAINED_INPUT_BUNDLE" {
				t.Fatalf("resume: %s %v", out.outcome, err)
			}
			raw := out.HeadBytes()
			raw[0] = '!'
			if out.HeadBytes()[0] == '!' {
				t.Fatal("head alias")
			}
			for _, path := range o.paths[:2] {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != len(o.plan.state.objects)+1 {
					t.Fatalf("copies/meta storage: %d %v", len(entries), err)
				}
			}
		})
	}
}
func TestScopeBundleCASReceiptMismatch(t *testing.T) {
	for _, kind := range []string{"digest", "length", "selector", "namespace", "verification", "crash"} {
		t.Run(kind, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			normal := o.ops.cas
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				got, err := normal(ctx, r, key, old, new)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "digest":
					got.Digest = Digest([]byte("wrong"))
				case "length":
					got.ByteLength++
				case "selector":
					got.FinalSelector = "other"
				case "namespace":
					got.NamespaceAtomic = false
				case "verification":
					got.VerificationStatus = "UNKNOWN"
				case "crash":
					got.CrashDurability = "UNKNOWN"
				}
				return got, nil
			}
			pairs := 0
			o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				pairs++
				return scopeRetentionAttempt{}, errors.New("must not execute")
			}
			out, _ := o.advance(context.Background(), nil)
			if out.outcome != "INDETERMINATE_EXECUTION" || !out.casCommitted || len(out.head) == 0 || pairs != 0 || out.pending != nil {
				t.Fatal("mismatched CAS receipt executed pair", kind, pairs, out.outcome)
			}
		})
	}
}
func TestScopeBundleClaimOutcomes(t *testing.T) {
	for _, kind := range []string{"unknown", "conflict", "not-committed", "known-error", "cancel-commit"} {
		t.Run(kind, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			normal := o.ops.cas
			sentinel := errors.New("CAS fault")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				switch kind {
				case "unknown":
					return nil, sentinel
				case "conflict":
					return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
				case "not-committed":
					return &publication.CompareAndReplaceReceipt{Outcome: publication.CASOutcomeNotCommitted}, nil
				}
				got, err := normal(ctx, r, key, old, new)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "cancel-commit" {
					cancel()
					return got, context.Canceled
				}
				return got, sentinel
			}
			pairs := 0
			o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				pairs++
				return scopeRetentionAttempt{}, nil
			}
			out, _ := o.advance(ctx, nil)
			want := "INDETERMINATE_EXECUTION"
			if kind == "conflict" {
				want = "CONFLICT"
			}
			if out.outcome != want || pairs != 0 || out.pending != nil {
				t.Fatalf("claim guard: %s pairs=%d", out.outcome, pairs)
			}
			if kind == "known-error" || kind == "cancel-commit" {
				if !out.casCommitted || len(out.head) == 0 {
					t.Fatal("known CAS commit erased")
				}
			}
		})
	}
}
func TestScopeBundleInterruptedClaimAndPendingCustody(t *testing.T) {
	o, scope := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _ := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
	for _, cap := range []*scopeBundlePending{nil, claim.pending, &scopeBundlePending{}} {
		calls := 0
		fresh.ops.getReceipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
			calls++
			return nil, errors.New("must not scan")
		}
		fresh.ops.put = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
			calls++
			return nil, errors.New("must not put")
		}
		fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
			calls++
			return nil, errors.New("must not CAS")
		}
		out, _ := fresh.advance(context.Background(), cap)
		if out.outcome != "INDETERMINATE_EXECUTION" || calls != 0 {
			t.Fatal("recreated owner recovered abandoned claim", out.outcome, calls)
		}
	}
	forged := *claim.pending
	out, _ := o.advance(context.Background(), &forged)
	if out.outcome != "INDETERMINATE_EXECUTION" {
		t.Fatal("copied capability qualified")
	}
	done, err := o.advance(context.Background(), claim.pending)
	if err != nil || done.outcome != "READY" {
		t.Fatal("live pending", done.outcome, err)
	}
	if _, err = o.advance(context.Background(), claim.pending); err == nil {
		t.Fatal("pending replay")
	}
}
func TestScopeBundlePartialAndCloseFacts(t *testing.T) {
	for _, kind := range []string{"primary", "backup", "pair-close", "journal-put", "journal-unknown", "journal-get", "journal-close", "final-close"} {
		t.Run(kind, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			sentinel := errors.New("injected " + kind)
			normal := o.ops
			closeCalls := 0
			o.ops.close = func(r *publication.ContentRetentionRoot) error {
				closeCalls++
				err := normal.close(r)
				if kind == "pair-close" && closeCalls == 1 || kind == "journal-close" && closeCalls == 3 || kind == "final-close" && closeCalls == 6 {
					return errors.Join(err, sentinel)
				}
				return err
			}
			if kind == "primary" || kind == "backup" {
				o.ops.pair = func(ctx context.Context, p scopeRetentionPlan, i int, a, b *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
					ops := defaultScopeRetentionOps()
					real := ops.publish
					calls := 0
					ops.publish = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
						calls++
						if kind == "primary" && calls == 1 || kind == "backup" && calls == 2 {
							return nil, sentinel
						}
						return real(ctx, r, id, raw)
					}
					return retainScopeObjectWithOps(ctx, p, i, a, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling, ops)
				}
			}
			if kind == "journal-put" || kind == "journal-unknown" {
				o.ops.put = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
					if kind == "journal-unknown" {
						_, err := normal.put(ctx, r, id, raw)
						if err != nil {
							t.Fatal(err)
						}
					}
					return nil, sentinel
				}
			}
			if kind == "journal-get" {
				o.ops.getReceipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
					return nil, sentinel
				}
			}
			out, err := o.advance(context.Background(), nil)
			if !out.casCommitted || len(out.head) == 0 {
				t.Fatal("claim facts lost")
			}
			if kind != "final-close" && err == nil {
				t.Fatal("failure erased")
			}
			if kind != "primary" && out.attempt.primary.receipt == nil {
				t.Fatal("primary fact lost")
			}
			if kind == "journal-put" || kind == "journal-unknown" || kind == "journal-get" || kind == "journal-close" {
				if out.pending != nil || out.outcome != "INDETERMINATE_EXECUTION" || len(out.receipt) == 0 {
					t.Fatal("uncertain journal custody minted pending")
				}
				return
			}
			if out.pending == nil {
				t.Fatal("known receipt missing pending", err)
			}
			if kind == "pair-close" {
				var r scopeBundleReceipt
				if json.Unmarshal(out.receipt, &r) != nil || r.Success || !bytes.Contains([]byte(r.Failure), []byte(sentinel.Error())) {
					t.Fatal("close missing before receipt success")
				}
			}
			finalized, e := o.advance(context.Background(), out.pending)
			if kind == "final-close" {
				if !errors.Is(e, sentinel) || !finalized.casCommitted || len(finalized.head) == 0 || finalized.outcome != "INDETERMINATE_EXECUTION" {
					t.Fatal("post-CAS close erased known revision", finalized.outcome, e)
				}
				return
			}
			if e != nil || finalized.outcome != "FAILED" {
				t.Fatal("failed attempt finalization", finalized.outcome, e)
			}
		})
	}
}
func TestScopeBundleFinalizedCorruption(t *testing.T) {
	for _, kind := range []string{"head-empty", "head-malformed", "receipt-missing", "receipt-corrupt", "primary", "backup", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			o, scope := bundleFixture(t, true)
			final := runBundle(t, o)
			path := bundleHeadPath(o)
			raw := []byte("bad")
			switch kind {
			case "head-empty":
				raw = nil
			case "head-malformed":
			case "receipt-missing":
				path = filepath.Join(o.paths[2], scopeBundleReceiptSelector(final.receiptSelector))
				if err := os.Rename(path, path+".unlinked-fixture"); err != nil {
					t.Fatal(err)
				}
			case "receipt-corrupt":
				path = filepath.Join(o.paths[2], scopeBundleReceiptSelector(final.receiptSelector))
			case "primary":
				path = filepath.Join(o.paths[0], o.item(0).digest)
			case "backup":
				path = filepath.Join(o.paths[1], o.item(1).digest)
			case "metadata":
				path = filepath.Join(o.paths[1], o.item(len(o.plan.state.objects)).digest)
			}
			if kind != "receipt-missing" {
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			fresh, _ := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
			calls := 0
			fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
				calls++
				return nil, nil
			}
			out, err := fresh.advance(context.Background(), nil)
			if err == nil || out.outcome == "RETAINED_INPUT_BUNDLE" || calls != 0 {
				t.Fatal("corrupt bundle finalized", out.outcome, err, calls)
			}
		})
	}
}
func TestScopeBundleFinalizeFailureRetainsAttempt(t *testing.T) {
	o, _ := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("reopen failed")
	o.ops.getReceipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
		return nil, sentinel
	}
	out, err := o.advance(context.Background(), claim.pending)
	if !errors.Is(err, sentinel) || out.attempt.primary.receipt == nil || out.attempt.backup.receipt == nil || len(out.ReceiptBytes()) == 0 || len(out.HeadBytes()) == 0 {
		t.Fatal("finalize lost known copies/receipt/claim")
	}
	if out.pending != nil {
		t.Fatal("consumed capability returned")
	}
}
func TestScopeBundleWrongRootAndOrphan(t *testing.T) {
	o, scope := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// An exact orphan receipt does not authorize a recreated owner to finalize.
	fresh, _ := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
	out, err := fresh.advance(context.Background(), nil)
	if err != nil || out.outcome != "INDETERMINATE_EXECUTION" {
		t.Fatal("orphan discovered", out.outcome, err)
	}
	// Substitute a new admitted directory at the old path after claim. No root
	// identity re-admission may silently consume the old owner's capability.
	old := o.paths[1] + "-fixture-old"
	if err := os.Rename(o.paths[1], old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(o.paths[1], 0700); err != nil {
		t.Fatal(err)
	}
	out, err = o.advance(context.Background(), claim.pending)
	if err == nil || out.pending != nil {
		t.Fatal("wrong-root pending admitted")
	}
}

func TestScopeBundlePrefixFailureRetainsLinkedReceipts(t *testing.T) {
	o, scope := bundleFixture(t, true)
	runBundle(t, o)
	if err := os.WriteFile(filepath.Join(o.paths[0], o.item(0).digest), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, _ := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
	out, err := fresh.advance(context.Background(), nil)
	if err == nil || len(out.LinkedReceiptBytes()) != len(o.plan.state.objects)+1 || len(out.HeadBytes()) == 0 {
		t.Fatal("prefix failure lost known receipts/head")
	}
	linked := out.LinkedReceiptBytes()
	linked[0][0] = '!'
	if out.LinkedReceiptBytes()[0][0] == '!' {
		t.Fatal("linked receipt alias")
	}
}

func TestScopeBundleAcquisitionClosesEveryReturnedRoot(t *testing.T) {
	for failureAt := 1; failureAt <= 3; failureAt++ {
		t.Run(string(rune('0'+failureAt)), func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			normal := o.ops
			opened, closed, cas := 0, 0, 0
			acquireFault := errors.New("root acquisition fault")
			closeFault := errors.New("root close fault")
			o.ops.open = func(ctx context.Context, path string) (*publication.ContentRetentionRoot, error) {
				r, err := normal.open(ctx, path)
				opened++
				if opened == failureAt {
					return r, errors.Join(err, acquireFault)
				}
				return r, err
			}
			o.ops.close = func(r *publication.ContentRetentionRoot) error {
				closed++
				return errors.Join(normal.close(r), closeFault)
			}
			o.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
				cas++
				return nil, nil
			}
			out, err := o.advance(context.Background(), nil)
			if !errors.Is(err, acquireFault) || !errors.Is(err, closeFault) || opened != closed || cas != 0 || out.casCommitted {
				t.Fatal("root acquisition/close cause lost", opened, closed, cas, err)
			}
		})
	}
}
func TestScopeBundleCancellationRetainsAttemptReceipt(t *testing.T) {
	o, _ := bundleFixture(t, false)
	normal := o.ops.pair
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.ops.pair = func(ctx context.Context, p scopeRetentionPlan, i int, a, b *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
		out, err := normal(ctx, p, i, a, b)
		cancel()
		return out, errors.Join(err, ctx.Err())
	}
	out, err := o.advance(ctx, nil)
	if !errors.Is(err, context.Canceled) || out.pending == nil || !out.casCommitted || out.attempt.primary.receipt == nil || out.attempt.backup.receipt == nil {
		t.Fatal("cancel erased committed copies/receipt", out.outcome, err)
	}
	var r scopeBundleReceipt
	if json.Unmarshal(out.receipt, &r) != nil || r.Success || r.Failure == "" {
		t.Fatal("cancel not recorded")
	}
	done, err := o.advance(context.Background(), out.pending)
	if err != nil || done.outcome != "FAILED" {
		t.Fatal("cancel failed receipt finalize", done.outcome, err)
	}
}
func TestScopeBundleNamespaceAndDeterministicReconstruction(t *testing.T) {
	o, scope := bundleFixture(t, true)
	final := runBundle(t, o)
	if final.receiptPublication == nil || final.receiptPublication.FinalSelector != scopeBundleReceiptSelector(final.receiptSelector) {
		t.Fatal("exact namespaced receipt selector")
	}
	// Retain a new exact hash-valid receipt with the wrong claim, then point the
	// fixture head at it. Hash-valid wire is not a valid reconstructed chain.
	var r scopeBundleReceipt
	if err := json.Unmarshal(final.receipt, &r); err != nil {
		t.Fatal(err)
	}
	r.Claim = Digest([]byte("wrong claim"))
	raw, _ := json.Marshal(r)
	id := Digest(raw)
	journal, err := publication.OpenContentRetentionRoot(context.Background(), o.paths[2])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = journal.PublishScopeBundleReceipt(context.Background(), id, raw); err != nil {
		t.Fatal(err)
	}
	if err = journal.Close(); err != nil {
		t.Fatal(err)
	}
	h, err := parseScopeBundleHead(final.head)
	if err != nil {
		t.Fatal(err)
	}
	h.Receipt = id
	if err = os.WriteFile(bundleHeadPath(o), scopeBundleEncode(h), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, _ := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
	cas := 0
	fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		cas++
		return nil, nil
	}
	out, err := fresh.advance(context.Background(), nil)
	if err == nil || cas != 0 || out.outcome == "RETAINED_INPUT_BUNDLE" {
		t.Fatal("hash-valid wrong claim resumed", out.outcome, err)
	}
}
func TestScopeBundleCumulativeReadOnlyBudget(t *testing.T) {
	o, _ := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil || claim.pending == nil {
		t.Fatal(err)
	}
	reads, cas := 0, 0
	o.ops.getReceipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
		reads++
		return nil, ErrInvalid
	}
	o.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		cas++
		return nil, nil
	}
	for i := 1; i < scopeBundleCalls; i++ {
		out, err := o.advance(context.Background(), nil)
		if err != nil || out.outcome != "INDETERMINATE_EXECUTION" {
			t.Fatal("bounded read-only observation", err)
		}
	}
	if o.charged.work != o.budget.work || reads != 0 || cas != 0 {
		t.Fatal("cumulative read-only work uncharged", o.charged, o.budget, reads, cas)
	}
	if _, err = o.advance(context.Background(), nil); err == nil {
		t.Fatal("read-only exhaustion")
	}
	for _, work := range []bool{false, true} {
		fresh, _ := bundleFixture(t, false)
		if work {
			fresh.charged.work = fresh.budget.work - fresh.budget.work/scopeBundleCalls + 1
		} else {
			fresh.charged.bytes = fresh.budget.bytes
		}
		opens := 0
		fresh.ops.open = func(context.Context, string) (*publication.ContentRetentionRoot, error) { opens++; return nil, nil }
		if _, err = fresh.advance(context.Background(), nil); err == nil || opens != 0 {
			t.Fatal("shortage not rejected before acquisition")
		}
	}
}

func TestScopeBundleConcurrentOwnersHaveOnePairExecutor(t *testing.T) {
	first, scope := bundleFixture(t, false)
	second, err := newScopeBundleOwner(scope, first.paths[0], first.paths[1], first.paths[2])
	if err != nil {
		t.Fatal(err)
	}
	var pairs atomic.Int64
	for _, owner := range []*scopeBundleOwner{first, second} {
		normal := owner.ops.pair
		owner.ops.pair = func(ctx context.Context, p scopeRetentionPlan, i int, a, b *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
			pairs.Add(1)
			return normal(ctx, p, i, a, b)
		}
	}
	var wg sync.WaitGroup
	type observation struct {
		result scopeBundleResult
		err    error
	}
	results := make(chan observation, 2)
	for _, owner := range []*scopeBundleOwner{first, second} {
		wg.Add(1)
		go func(o *scopeBundleOwner) {
			defer wg.Done()
			out, err := o.advance(context.Background(), nil)
			results <- observation{out, err}
		}(owner)
	}
	wg.Wait()
	close(results)
	winners := 0
	for obs := range results {
		out := obs.result
		if out.pending != nil {
			if obs.err != nil {
				t.Fatal("executor did not retain a qualified attempt", obs.err)
			}
			winners++
		} else if out.outcome != "CONFLICT" && out.outcome != "INDETERMINATE_EXECUTION" {
			t.Fatal("losing owner admitted", out.outcome, obs.err)
		}
		if obs.err != nil {
			t.Logf("fail-closed concurrent store observation: %s: %v", out.outcome, obs.err)
		}
	}
	if winners != 1 || pairs.Load() != 1 {
		t.Fatal("duplicate executor", winners, pairs.Load())
	}
}
func TestScopeBundleFinalizeCASFailurePreservesKnownFacts(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "conflict", true: "known-commit-error"}[committed], func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			claim, err := o.advance(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			normal := o.ops.cas
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				if !committed {
					return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
				}
				got, err := normal(ctx, r, key, old, new)
				return got, errors.Join(err, context.Canceled)
			}
			out, _ := o.advance(context.Background(), claim.pending)
			if out.attempt.primary.receipt == nil || out.attempt.backup.receipt == nil || len(out.receipt) == 0 || len(out.head) == 0 || out.pending != nil || out.casCommitted != committed {
				t.Fatal("final CAS failure lost facts")
			}
			want := "CONFLICT"
			if committed {
				want = "INDETERMINATE_EXECUTION"
			}
			if out.outcome != want {
				t.Fatal(out.outcome)
			}
		})
	}
}

func TestScopeBundleRejectsInconsistentFailureReceipt(t *testing.T) {
	o, _ := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"phase", "not-attempted-receipt", "committed-without-receipt", "indeterminate-committed-receipt"} {
		t.Run(kind, func(t *testing.T) {
			var r scopeBundleReceipt
			if err := json.Unmarshal(claim.receipt, &r); err != nil {
				t.Fatal(err)
			}
			r.Success = false
			r.Failure = "failure observation"
			switch kind {
			case "phase":
				r.Phase = "UNKNOWN_PHASE"
			case "not-attempted-receipt":
				r.Primary.Outcome = "NOT_ATTEMPTED"
			case "committed-without-receipt":
				r.Primary.Receipt = nil
			case "indeterminate-committed-receipt":
				r.Primary.Outcome = "INDETERMINATE"
			}
			raw, _ := json.Marshal(r)
			if _, err := o.parseReceipt(raw, *o.binding); err == nil {
				t.Fatal("inconsistent failure receipt admitted", kind)
			}
		})
	}
}

func TestScopeBundleCASUsesRepositoryOwnedDeadline(t *testing.T) {
	o, _ := bundleFixture(t, false)
	o.ops.cas = func(ctx context.Context, _ *publication.ContentRetentionRoot, _ string, _, _ []byte) (*publication.CompareAndReplaceReceipt, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("unbounded CAS lock wait")
		}
		return &publication.CompareAndReplaceReceipt{Outcome: publication.CASOutcomeNotCommitted}, nil
	}
	if out, err := o.advance(context.Background(), nil); err != nil || out.outcome != "INDETERMINATE_EXECUTION" {
		t.Fatal(out.outcome, err)
	}
}

func TestScopeBundleRootAliasesAndWrongPlanPending(t *testing.T) {
	o, scope := bundleFixture(t, false)
	for _, paths := range [][3]string{{o.paths[0], o.paths[0], o.paths[2]}, {o.paths[0], o.paths[1], o.paths[0]}} {
		owner, err := newScopeBundleOwner(scope, paths[0], paths[1], paths[2])
		if err != nil {
			t.Fatal(err)
		}
		cas := 0
		owner.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
			cas++
			return nil, nil
		}
		if _, err = owner.advance(context.Background(), nil); err == nil || cas != 0 {
			t.Fatal("aliased root identities admitted")
		}
	}
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	otherScope := retentionFixture(t, true)
	wrong, err := newScopeBundleOwner(otherScope, o.paths[0], o.paths[1], o.paths[2])
	if err != nil {
		t.Fatal(err)
	}
	cas := 0
	wrong.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		cas++
		return nil, nil
	}
	if _, err = wrong.advance(context.Background(), claim.pending); err == nil || cas != 0 {
		t.Fatal("wrong-plan pending admitted")
	}
}

func TestScopeBundleFreshOwnerRejectsChangedCopyRoot(t *testing.T) {
	o, scope := bundleFixture(t, true)
	runBundle(t, o)
	if err := os.Rename(o.paths[1], o.paths[1]+"-old-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(o.paths[1], 0700); err != nil {
		t.Fatal(err)
	}
	fresh, err := newScopeBundleOwner(scope, o.paths[0], o.paths[1], o.paths[2])
	if err != nil {
		t.Fatal(err)
	}
	cas := 0
	fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		cas++
		return nil, nil
	}
	out, err := fresh.advance(context.Background(), nil)
	if err == nil || cas != 0 || out.outcome == "RETAINED_INPUT_BUNDLE" {
		t.Fatal("changed root minted an alternate head key", out.outcome, err, cas)
	}
}

func TestScopeBundleCanonicalWireAndBudget(t *testing.T) {
	o, _ := bundleFixture(t, false)
	claim, err := o.advance(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := claim.HeadBytes()
	for _, bad := range [][]byte{nil, {}, append(bytes.Clone(raw), ' '), append(bytes.Clone(raw), []byte(`{}`)...), bytes.Replace(raw, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1), bytes.Replace(raw, []byte(`"authority":0`), []byte(`"unknown":0,"authority":0`), 1), bytes.Replace(raw, []byte(`"generation":1`), []byte(`"generation":18446744073709551616`), 1), bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":10`), 1)} {
		if _, err := parseScopeBundleHead(bad); err == nil {
			t.Fatalf("noncanonical head admitted: %s", bad)
		}
	}
	for _, bad := range [][]byte{append(claim.ReceiptBytes(), ' '), bytes.Replace(claim.ReceiptBytes(), []byte(`"success":true`), []byte(`"success":true,"success":true`), 1)} {
		if _, err := o.parseReceipt(bad, *o.binding); err == nil {
			t.Fatal("receipt tamper admitted")
		}
	}
	b := scopeRetentionBudget{17, 23}
	if got, err := scopeBundleReserve(scopeRetentionBudget{}, b, b); err != nil || got != b {
		t.Fatal("budget equality")
	}
	for _, delta := range []scopeRetentionBudget{{18, 23}, {17, 24}, {math.MaxUint64, 1}} {
		if _, err := scopeBundleReserve(scopeRetentionBudget{1, 0}, delta, b); err == nil {
			t.Fatal("budget plus-one/overflow")
		}
	}
	if _, err := scopeBundleReserve(scopeRetentionBudget{math.MaxUint64, 0}, scopeRetentionBudget{1, 0}, scopeRetentionBudget{math.MaxUint64, math.MaxUint64}); err == nil {
		t.Fatal("overflow")
	}
	calls := 0
	o.ops.open = func(context.Context, string) (*publication.ContentRetentionRoot, error) {
		calls++
		return nil, errors.New("must not open")
	}
	o.calls = scopeBundleCalls
	if _, err := o.advance(context.Background(), nil); err == nil || calls != 0 {
		t.Fatal("sequence exhaustion")
	}
}

func TestScopeBundleMetadataPair(t *testing.T) {
	p, b, _, _ := retentionRoots(t)
	plan := pairPlan(t)
	out, err := retainScopeMetadata(context.Background(), plan, p, b)
	if err != nil || out.verified.state == nil {
		t.Fatalf("metadata pair: %v %v", out, err)
	}
	for _, r := range []interface {
		Read(context.Context, string, int64) ([]byte, error)
	}{p, b} {
		got, err := r.Read(context.Background(), Digest(plan.metadataBytes()), int64(len(plan.metadataBytes())))
		if err != nil || !bytes.Equal(got, plan.metadataBytes()) {
			t.Fatalf("metadata exact nine refs: %v", err)
		}
	}
	if len(plan.references()) != 9 {
		t.Fatal("nine references lost")
	}
}
