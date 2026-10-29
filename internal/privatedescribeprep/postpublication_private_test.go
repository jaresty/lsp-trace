//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func postPublicationRoots(t *testing.T) ([2]*publication.ContentRetentionRoot, []*publication.ObservedRoot) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(home, ".postpublication-owned-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	var outs [2]*publication.ContentRetentionRoot
	var ins []*publication.ObservedRoot
	for i, name := range []string{"primary", "backup", "input"} {
		p := filepath.Join(base, name)
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
		if i < 2 {
			r, e := publication.OpenContentRetentionRoot(context.Background(), p)
			if e != nil {
				t.Fatal(e)
			}
			outs[i] = r
			t.Cleanup(func() {
				if e := r.Close(); e != nil {
					t.Error(e)
				}
			})
		} else {
			r, e := publication.OpenObservedRoot(context.Background(), p)
			if e != nil {
				t.Fatal(e)
			}
			ins = append(ins, r)
			t.Cleanup(func() {
				if e := r.Close(); e != nil {
					t.Error(e)
				}
			})
		}
	}
	return outs, ins
}
func publicationCallerPair(t *testing.T, h PostPreparation) ([]CallerSubmission, []CallerReview) {
	v := mechanicalSubmission(t, h, 0)
	first, e := h.Reconcile(context.Background(), []CallerSubmission{v}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return []CallerSubmission{v}, []CallerReview{mechanicalReview(v, first.Targets[0].SubmissionSHA256)}
}
func TestPostPublicationPrivateNoLedgerAdmission(t *testing.T) {
	// Structural API check: no mutable ledger argument or rehydration constructor.
	typ := reflect.TypeOf(RetainPostPreparationPrivate)
	if typ.NumIn() != 6 || typ.In(1) != reflect.TypeOf(PostPreparation{}) {
		t.Fatal("sealed handle boundary changed")
	}
	for i := 0; i < typ.NumIn(); i++ {
		if typ.In(i) == reflect.TypeOf(StageLedger{}) {
			t.Fatal("ledger admission exposed")
		}
	}
	h := mechanicalState(t, 1)
	raw, _ := h.PreparedBytes()
	var forged PostPreparation
	if e := json.Unmarshal(raw, &forged); e != nil {
		t.Fatal(e)
	}
	roots, ins := postPublicationRoots(t)
	if out, e := RetainPostPreparationPrivate(context.Background(), forged, nil, nil, roots, ins); e == nil || out.Copies[0].Outcome != "NOT_ATTEMPTED" {
		t.Fatal("DTO minted publication handle")
	}
}

func TestPostPublicationPrivateEnvelopeAndPair(t *testing.T) {
	roots, ins := postPublicationRoots(t)
	h := mechanicalState(t, 1)
	subs, reviews := publicationCallerPair(t, h)
	for pass := 0; pass < 2; pass++ {
		out, e := RetainPostPreparationPrivate(context.Background(), h, subs, reviews, roots, ins)
		if e != nil || out.Phase != "PAIR_VERIFIED" {
			t.Fatal(e, out.Phase)
		}
		var env postPublicationEnvelope
		if e = json.Unmarshal(out.EnvelopeBytes(), &env); e != nil {
			t.Fatal(e)
		}
		prepared, _ := h.PreparedBytes()
		ledger, e := h.Reconcile(context.Background(), subs, reviews)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(prepared, env.Preparation) || !bytes.Equal(mechanicalLedgerBytes(ledger), mechanicalLedgerBytes(env.Ledger)) || env.Absence != "PROHIBITED" || !env.PrivateCeilings.valid() || len(env.Ledger.Leads) != 1 {
			t.Fatal("envelope mismatch")
		}
		if pass == 0 {
			if out.Copies[0].Receipt == nil {
				t.Fatal("missing receipt")
			}
		} else if out.Copies[0].Outcome != "EXISTING_VERIFIED" {
			t.Fatal("existing pre-read")
		}
		raw := out.EnvelopeBytes()
		raw[0] = '!'
		if out.EnvelopeBytes()[0] == '!' {
			t.Fatal("envelope alias")
		}
	}
}
func TestPostPublicationPrivateZeroEffectsPreflight(t *testing.T) {
	for _, kind := range []string{"handle", "foreign-submission", "foreign-review", "submission-ceilings", "review-ceilings", "bad-quote", "bad-review", "envelope-estimate", "string", "topology", "cancel", "nil-context"} {
		t.Run(kind, func(t *testing.T) {
			roots, ins := postPublicationRoots(t)
			h := mechanicalState(t, 1)
			subs, reviews := publicationCallerPair(t, h)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "handle":
				h = PostPreparation{}
			case "foreign-submission":
				subs[0].Target = "foreign"
			case "foreign-review":
				reviews[0].Target = "foreign"
			case "submission-ceilings":
				subs[0].Accepted = true
			case "review-ceilings":
				reviews[0].Authority = 1
			case "bad-quote":
				subs[0].Claims[0].Citations[0].SourceQuote = "not retained"
			case "bad-review":
				reviews[0].SubmissionSHA256 = "stale"
			case "envelope-estimate":
				// Mechanical state perturbation, not constructor/producer proof.
				h.state.prepared = bytes.Repeat([]byte{' '}, 6<<20)
			case "string":
				subs[0].AuthorTag = strings.Repeat("x", mechanicalLimits().MaxStringBytes+1)
			case "topology":
				roots[1] = roots[0]
			case "cancel":
				cancel()
			case "nil-context":
				ctx = nil
			}
			calls := 0
			ops := defaultPostPublicationOps()
			ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
				calls++
				return nil, os.ErrNotExist
			}
			ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
				calls++
				return nil, errors.New("unexpected")
			}
			out, e := retainPostPreparationPrivate(ctx, h, subs, reviews, roots, ins, ops)
			if e == nil || calls != 0 || out.Copies[0].Outcome != "NOT_ATTEMPTED" {
				t.Fatal("preflight effect", e, calls)
			}
		})
	}
}
func TestPostPublicationPrivatePairFailures(t *testing.T) {
	for _, kind := range []string{"backup-fail", "corrupt", "mixed-cancel", "mixed-close", "late-exist", "late-exist-close", "commit-cancel", "receipt", "reopen-close", "reopen-cancel", "nil-receipt", "detached"} {
		t.Run(kind, func(t *testing.T) {
			roots, ins := postPublicationRoots(t)
			h := mechanicalState(t, 1)
			subs, reviews := publicationCallerPair(t, h)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops := defaultPostPublicationOps()
			reads, publishes := 0, 0
			var receipt *publication.BoundFileReceipt
			fault := errors.New("injected close error")
			var expected []byte
			ops.read = func(_ context.Context, _ *publication.ContentRetentionRoot, _ string, _ int64) ([]byte, error) {
				reads++
				if reads == 1 {
					switch kind {
					case "corrupt":
						return []byte("corrupt"), nil
					case "mixed-cancel":
						return nil, errors.Join(os.ErrNotExist, context.Canceled)
					case "mixed-close":
						return nil, errors.Join(os.ErrNotExist, fault)
					}
				}
				if reads <= 2 {
					return nil, os.ErrNotExist
				}
				if kind == "reopen-close" {
					return nil, fault
				}
				if kind == "reopen-cancel" {
					cancel()
				}
				return bytes.Clone(expected), nil
			}
			ops.publish = func(_ context.Context, _ *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				publishes++
				expected = bytes.Clone(raw)
				r := &publication.BoundFileReceipt{FinalSelector: id, Digest: id, ByteLength: uint64(len(raw)), Mechanism: publication.BoundFileMechanism, NamespaceAtomic: true, CrashDurability: publication.DirectorySyncComplete, DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete, VerificationStatus: "VERIFIED"}
				receipt = r
				switch kind {
				case "backup-fail":
					if publishes == 2 {
						return nil, fault
					}
				case "late-exist":
					return r, os.ErrExist
				case "late-exist-close":
					return r, errors.Join(os.ErrExist, fault)
				case "commit-cancel":
					cancel()
					return r, context.Canceled
				case "receipt":
					r.CloseStatus = "FAILED"
				case "nil-receipt":
					return nil, nil
				}
				return r, nil
			}
			out, e := retainPostPreparationPrivate(ctx, h, subs, reviews, roots, ins, ops)
			if kind == "detached" {
				if e != nil || out.Phase != "PAIR_VERIFIED" || reads != 6 || publishes != 2 {
					t.Fatal(e, out.Phase, reads, publishes)
				}
				receipt.Digest = "changed"
				if out.Copies[1].Receipt.Digest == "changed" {
					t.Fatal("receipt aliased")
				}
				return
			}
			if e == nil || out.Phase == "PAIR_VERIFIED" {
				t.Fatal("failure admitted", kind)
			}
			switch kind {
			case "corrupt", "mixed-cancel", "mixed-close":
				if publishes != 0 || reads != 1 {
					t.Fatal("absence misclassified")
				}
			case "backup-fail":
				if publishes != 2 || out.Copies[0].Outcome != "NAMESPACE_COMMITTED" {
					t.Fatal("partial outcome lost")
				}
			case "nil-receipt":
				if publishes != 1 || reads != 1 || out.Copies[0].Receipt != nil {
					t.Fatal("nil receipt progressed")
				}
			case "reopen-close", "reopen-cancel":
				if publishes != 2 || reads != 3 {
					t.Fatal("reopen retried")
				}
			default:
				if publishes != 1 || reads != 1 || out.Copies[0].Receipt == nil {
					t.Fatal("late collision retried/read back or receipt lost", publishes, reads)
				}
			}
		})
	}
}
func TestPostPublicationPrivateReceiptConsistency(t *testing.T) {
	cases := map[string]func(*publication.BoundFileReceipt){
		"namespace":    func(r *publication.BoundFileReceipt) { r.NamespaceAtomic = false },
		"selector":     func(r *publication.BoundFileReceipt) { r.FinalSelector = "wrong" },
		"digest":       func(r *publication.BoundFileReceipt) { r.Digest = "wrong" },
		"length":       func(r *publication.BoundFileReceipt) { r.ByteLength++ },
		"mechanism":    func(r *publication.BoundFileReceipt) { r.Mechanism = "wrong" },
		"directory":    func(r *publication.BoundFileReceipt) { r.DirectorySyncStatus = "wrong" },
		"durability":   func(r *publication.BoundFileReceipt) { r.CrashDurability = "wrong" },
		"close":        func(r *publication.BoundFileReceipt) { r.CloseStatus = "wrong" },
		"verification": func(r *publication.BoundFileReceipt) { r.VerificationStatus = "wrong" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			roots, ins := postPublicationRoots(t)
			h := mechanicalState(t, 1)
			subs, reviews := publicationCallerPair(t, h)
			reads, publishes := 0, 0
			ops := defaultPostPublicationOps()
			ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
				reads++
				return nil, os.ErrNotExist
			}
			ops.publish = func(_ context.Context, _ *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				publishes++
				r := &publication.BoundFileReceipt{FinalSelector: id, Digest: id, ByteLength: uint64(len(raw)), Mechanism: publication.BoundFileMechanism, NamespaceAtomic: true, CrashDurability: publication.DirectorySyncComplete, DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete, VerificationStatus: "VERIFIED"}
				change(r)
				return r, nil
			}
			out, e := retainPostPreparationPrivate(context.Background(), h, subs, reviews, roots, ins, ops)
			if e == nil || reads != 1 || publishes != 1 || out.Copies[1].Outcome != "NOT_ATTEMPTED" || out.Copies[0].Receipt == nil {
				t.Fatal("inconsistent receipt progressed", e, reads, publishes)
			}
		})
	}
}

func TestPostPublicationPrivateEncodingBoundary(t *testing.T) {
	// Pure fixtures, deliberately NOT producer/seal proofs. Measure fixed JSON
	// overhead then vary one ASCII string to reach equality and exactly one over.
	prepared := []byte(`{"fixture":"not producer proof"}`)
	ledger := StageLedger{PrivateCeilings: privateCeilings()}
	base, e := encodePostPublicationPrivate(prepared, ledger, "fixture")
	if e != nil {
		t.Fatal(e)
	}
	ledger.NominationCensus = strings.Repeat("x", postPublicationLimit-len(base))
	raw, e := encodePostPublicationPrivate(prepared, ledger, "fixture")
	if e != nil || len(raw) != postPublicationLimit {
		t.Fatal("envelope equality", e, len(raw))
	}
	ledger.NominationCensus += "x"
	if _, e = encodePostPublicationPrivate(prepared, ledger, "fixture"); e == nil {
		t.Fatal("envelope over admitted")
	}
}
