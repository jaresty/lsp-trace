//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/describeworker"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func providerControlFixtureV2(t *testing.T, change ...func(*authorizationV2)) (*PrivateManagedInputOwnerV2, describeworker.FilePin, uint64) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	home, e = filepath.EvalSymlinks(home)
	if e != nil {
		t.Fatal(e)
	}
	// Use the real ancestor/owner/ACL admission before creating any fixture.
	// Never chmod a shared temp parent or fall back to an inadmissible path.
	var scratch [4096]byte
	parent, _, e := openProtectedLocalDirectoryV1(home, &scratch, nil)
	if e != nil {
		t.Fatal("protected provider fixture HOME unavailable", e)
	}
	if e = parent.Close(); e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".fixed-provider-control-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(dir); e != nil {
			t.Error(e)
		}
	})
	p := filepath.Join(dir, "provider")
	b := []byte("tiny controlled provider; never executed\n")
	if e := os.WriteFile(p, b, 0700); e != nil {
		t.Fatal(e)
	}
	j, a, _ := fixtureV2(t)
	j.ResourceRules = FixedProviderControlRulesV2()
	jb := sealTestV2(t, &j)
	a.Job = j.Identity
	a.Expires = time.Now().Add(time.Minute).UnixNano()
	a.MaxWindowNS = uint64(15 * time.Second)
	a.MaxWindows = 1
	// Genuine finite fixture credit on all fourteen coordinates, never real issuance.
	quote, qe := providerControlChargeV2(verificationLimitsV2{400, 4 << 20, 400}, uint64(len(b)), p, 15*time.Second)
	if qe != nil {
		t.Fatal(qe)
	}
	a.Ceiling = quote
	a.MaxTranche = quote
	for _, f := range change {
		f(&a)
	}
	ab := sealTestV2(t, &a)
	root := readyTestRoot(t)
	l := PrivateManagedInputVerificationV2{400, 4 << 20, 400}
	r, e := InitializePrivateReadyV2(context.Background(), root, jb, ab, PrivateReadyDecisionAssertionV2{j.Identity, a.Identity, a.Decision}, l)
	if e != nil {
		t.Fatal(e)
	}
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, r.CanonicalJob(), r.ConfirmedHead(), l)
	if e != nil {
		t.Fatal(e)
	}
	return o, describeworker.FilePin{Path: p, SHA256: privateGroupingDigest(b)}, uint64(len(b))
}
func TestFixedProviderControlConnectedV2(t *testing.T) {
	o, p, n := providerControlFixtureV2(t)
	r, e := o.verifyProviderControlV2(context.Background(), hashLabelV2("control-attempt"), time.Now().Add(5*time.Second), p, n)
	if e != nil || r.state == nil {
		t.Fatalf("ASSERT_CONTROL_FUNDED_COMPLETION: %v", e)
	}
	h, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil || h.Disposition != "OBSERVED" || h.Windows != 1 || h.Charged.ReadHashBytes < n || h.Charged.ProviderRequests != 0 {
		t.Fatalf("ASSERT_CONTROL_SEPARATE_OBSERVED: %+v %v", h, e)
	}
	if r.CheckCurrent(context.Background()) != nil {
		t.Fatal("ASSERT_CONTROL_CURRENT")
	}
	if _, e = o.verifyProviderControlV2(context.Background(), hashLabelV2("control-attempt"), time.Now().Add(time.Second), p, n); e == nil {
		t.Fatal("ASSERT_CONTROL_REPLAY")
	}
	// Counterfactual uses only this fixture's directory, never global /tmp.
	// The same real provider opener must reject an otherwise identical chain
	// when one test-owned ancestor becomes group/world writable.
	if e = os.Chmod(filepath.Dir(p.Path), 0777); e != nil {
		t.Fatal(e)
	}
	ioOwner := &providerControlIOV2{ctx: context.Background(), left: 129 * 8}
	f, openErr := ioOwner.open(p.Path)
	if f != nil {
		_ = f.Close()
	}
	if openErr == nil || f != nil || ioOwner.closeCredits != 0 {
		t.Fatal("ASSERT_CONTROL_WRITABLE_ANCESTOR_REFUSED", openErr, ioOwner.closeCredits)
	}
	t.Log("ASSERT_CONTROL_FUNDED_COMPLETION PASS; ASSERT_CONTROL_SEPARATE_OBSERVED PASS; ASSERT_CONTROL_WRITABLE_ANCESTOR_REFUSED PASS")
}

func TestFixedProviderControlRefusalsV2(t *testing.T) {
	for _, name := range []string{"wrong-pin", "wrong-length", "expired", "cancelled", "source-job", "underfunded", "symlink", "zero-receipt", "stale-owner", "receipt-expiry"} {
		t.Run(name, func(t *testing.T) {
			var changes []func(*authorizationV2)
			if name == "underfunded" {
				changes = append(changes, func(a *authorizationV2) { a.Ceiling.ReadHashBytes = 1; a.MaxTranche.ReadHashBytes = 1 })
			}
			o, p, n := providerControlFixtureV2(t, changes...)
			ctx := context.Background()
			deadline := time.Now().Add(time.Second)
			switch name {
			case "wrong-pin":
				p.SHA256 = hashLabelV2("wrong")
			case "wrong-length":
				n-- // funded smaller declaration; actual raw stat must reject after reserve
			case "expired":
				deadline = time.Now().Add(-time.Second)
			case "cancelled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "source-job":
				o.job.ResourceRules = hashLabelV2("source-job")
			case "symlink":
				link := p.Path + "-link"
				if e := os.Symlink(p.Path, link); e != nil {
					t.Fatal(e)
				}
				p.Path = link
			case "zero-receipt":
				if (PrivateProviderControlV2{}).CheckCurrent(ctx) == nil {
					t.Fatal("ASSERT_CONTROL_ZERO")
				}
				return
			}
			before := o.head
			r, e := o.verifyProviderControlV2(ctx, hashLabelV2(name), deadline, p, n)
			if name == "stale-owner" || name == "receipt-expiry" {
				if e != nil {
					t.Fatal(e)
				}
				if name == "stale-owner" {
					o.head = hashLabelV2("different-head")
				} else {
					r.state.deadline = time.Now().Add(-time.Second)
				}
				if r.CheckCurrent(ctx) == nil {
					t.Fatal("ASSERT_CONTROL_STALE_RECEIPT")
				}
				return
			}
			if e == nil || r.state != nil {
				t.Fatal("ASSERT_CONTROL_REFUSAL", name)
			}
			if name == "wrong-pin" || name == "wrong-length" || name == "symlink" {
				h, e := o.k.reopen(context.Background(), o.job, o.limits)
				if e != nil || h.Disposition != "PENDING" || h.Windows != 1 || h.Charged.ReadHashBytes == 0 {
					t.Fatal("ASSERT_CONTROL_FAILURE_RETAINS_DEBIT", e)
				}
			} else if o.head != before {
				t.Fatal("ASSERT_CONTROL_BEFORE_RESERVE", name)
			}
			t.Log("ASSERT_CONTROL_REFUSAL PASS", name)
		})
	}
}

func TestProviderLaunchPaidBeginV2(t *testing.T) {
	o, p, n := providerControlFixtureV2(t, func(a *authorizationV2) {
		fundProviderLaunchFixtureV2(t, a, PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	})
	r, e := o.verifyProviderControlV2(context.Background(), hashLabelV2("verify"), time.Now().Add(5*time.Second), p, n)
	if e != nil {
		t.Fatal(e)
	}
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(filepath.Join(base, "control"), 0700); e != nil {
		t.Fatal(e)
	}
	q, e := BeginProviderLaunchV2(context.Background(), r, hashLabelV2("launch"), time.Now().Add(time.Second), base, PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	if e != nil || q.state == nil {
		t.Fatalf("ASSERT_PAID_LAUNCH_BEGIN: %v", e)
	}
	defer q.Close()
	t.Log("ASSERT_PAID_LAUNCH_BEGIN PASS")
}

func TestFixedProviderControlQuoteV2(t *testing.T) {
	l := PrivateManagedInputVerificationV2{400, 4 << 20, 400}
	q, e := FixedProviderControlQuoteV2(l, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	if q[1] != fixedProviderLengthV2+1+providerNamespaceBytesV2+3*l.Bytes || q[4] != providerNamespaceBytesV2+3*l.Bytes || q[5] != 0 || q[6] != 0 || q[10] != 0 || q[11] != 0 || q[12] != 0 || q[8] != uint64(time.Second) {
		t.Fatalf("ASSERT_CONTROL_QUOTE_14: %v", q)
	}
	if _, e := FixedProviderControlQuoteV2(PrivateManagedInputVerificationV2{^uint64(0), ^uint64(0), ^uint64(0)}, time.Second); e == nil {
		t.Fatal("ASSERT_CONTROL_QUOTE_OVERFLOW")
	}
	if _, e := FixedProviderControlQuoteV2(l, 16*time.Second); e == nil {
		t.Fatal("ASSERT_CONTROL_QUOTE_WINDOW")
	}
	t.Logf("ASSERT_CONTROL_QUOTE_14 PASS %v", q)
}
