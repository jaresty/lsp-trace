//go:build darwin || linux

package inventorysuccessor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fundProviderLaunchFixtureV2(t *testing.T, a *authorizationV2, source PrivateManagedInputVerificationV2) {
	t.Helper()
	v, e := FixedProviderLaunchQuoteV2(PrivateManagedInputVerificationV2{400, 4 << 20, 400}, source, 15*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	added := countersFromV2(v)
	sum, e := addCountersV2(a.Ceiling, added)
	if e != nil {
		t.Fatal(e)
	}
	old := a.MaxTranche.vector()
	for i, x := range v {
		if x > old[i] {
			old[i] = x
		}
	}
	a.MaxTranche = countersFromV2(old)
	a.Ceiling = sum
	a.MaxWindows = 2
}

// Test-only exported factory: external integration tests can obtain genuine
// tiny-provider custody, but no production binary exports a fixture/provider pin.
func ControlledProviderLaunchForTest(t *testing.T, base string, source *PrivateManagedInputOwnerV2) (PendingProviderLaunchV2, string, *PrivateManagedInputOwnerV2) {
	t.Helper()
	sv := PrivateManagedInputVerificationV2{source.limits.Records, source.limits.Bytes, source.limits.Work}
	o, p, n := providerControlFixtureV2(t, func(a *authorizationV2) { fundProviderLaunchFixtureV2(t, a, sv) })
	r, e := o.verifyProviderControlV2(context.Background(), hashLabelV2("verify"), time.Now().Add(5*time.Second), p, n)
	if e != nil {
		t.Fatal(e)
	}
	q, e := BeginProviderLaunchV2(context.Background(), r, hashLabelV2("launch"), time.Now().Add(14*time.Second), base, sv)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { q.Close() })
	return q, p.Path, o
}
func providerLaunchFixtureV2(t *testing.T) (PendingProviderLaunchV2, string, *PrivateManagedInputOwnerV2, *PrivateManagedInputOwnerV2) {
	t.Helper()
	source, _ := ControlledRPrepCanonicalOwnerForTest(t)
	base := providerProtectedScratchV2(t)
	if e := os.Mkdir(filepath.Join(base, "control"), 0700); e != nil {
		t.Fatal(e)
	}
	q, p, o := ControlledProviderLaunchForTest(t, base, source)
	return q, p, o, source
}
func TestProviderLaunchLifecycleV2(t *testing.T) {
	for _, mode := range []string{"positive", "replace", "write-drift", "mtime-restored", "path-drift", "stale-head", "close-failure", "cancel-at-close", "stat-eintr", "repeat", "expiry", "expired-at-close", "unknown-metadata"} {
		t.Run(mode, func(t *testing.T) {
			q, path, owner, source := providerLaunchFixtureV2(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if e := q.PublishStartReadyV2([]byte("ready\n")); e != nil {
				t.Fatal(e)
			}
			if mode == "unknown-metadata" {
				if _, e := q.ReadStartMetadataV2("private-history"); e == nil {
					t.Fatal("ASSERT_METADATA_CLOSED_SET")
				}
				return
			}
			if e := q.ConsumeStartV2([]byte("{\"start\":true}\n")); e != nil {
				t.Fatal(e)
			}
			if mode == "repeat" {
				if e := q.ConsumeStartV2([]byte("{\"start\":true}\n")); e == nil {
					t.Fatal("ASSERT_START_ONE_SHOT")
				}
				return
			}
			if _, e := q.SourceAuthorizationExpiryV2(ctx, source); e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "replace":
				if e := os.Rename(path, path+"-old"); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(path, []byte("tiny controlled provider; never executed\n"), 0700); e != nil {
					t.Fatal(e)
				}
			case "write-drift":
				if e := os.WriteFile(path, []byte("different generation\n"), 0700); e != nil {
					t.Fatal(e)
				}
			case "mtime-restored":
				b, re := os.ReadFile(path)
				if re != nil {
					t.Fatal(re)
				}
				b[0] ^= 1
				if re = os.WriteFile(path, b, 0700); re != nil {
					t.Fatal(re)
				}
				old := (managedAdmissionInfoV1{name: "provider", stat: &q.state.receipt.state.identity}).ModTime()
				if re = os.Chtimes(path, old, old); re != nil {
					t.Fatal(re)
				}
			case "path-drift":
				path += "-foreign"
			case "stale-head":
				owner.head = hashLabelV2("stale")
			case "close-failure", "cancel-at-close", "stat-eintr":
				q.state.io.fixture = &providerFaultFixtureV2{mode: mode, path: path, cancel: cancel}
			case "expiry":
				q.state.deadline = time.Now().Add(-time.Second)
			case "expired-at-close":
				q.state.io.fixture = &providerLaunchExpiryFaultV2{providerFaultFixtureV2: providerFaultFixtureV2{path: path}, state: q.state}
			}
			e := q.FinishProviderLaunchV2(ctx, path)
			if mode == "positive" {
				if e != nil {
					t.Fatal("ASSERT_PAID_FRESHNESS_SUCCESS", e)
				}
			} else if e == nil {
				t.Fatal("ASSERT_PAID_FRESHNESS_REFUSAL", mode)
			}
			h, he := owner.k.reopen(context.Background(), owner.job, owner.limits)
			if he != nil {
				t.Fatal(he)
			}
			want := "PENDING"
			if mode == "positive" {
				want = "OBSERVED"
			}
			if h.Disposition != want || h.Windows != 2 || h.Charged.ReadHashBytes == 0 {
				t.Fatalf("ASSERT_SEPARATE_CONTROL_TRUTH: mode=%s %+v", mode, h)
			}
			t.Log("ASSERT_PAID_FRESHNESS PASS", mode, "control-windows", h.Windows)
		})
	}
}

type providerLaunchExpiryFaultV2 struct {
	providerFaultFixtureV2
	state *privateProviderLaunchStateV2
}

func (f *providerLaunchExpiryFaultV2) providerClose(file *os.File) error {
	e := file.Close()
	if file.Name() == f.path {
		f.state.deadline = time.Now().Add(-time.Second)
	}
	return e
}
func TestProviderLaunchUnderfundBeforeMetadataV2(t *testing.T) {
	o, p, n := providerControlFixtureV2(t, func(a *authorizationV2) { a.MaxWindows = 2 })
	r, e := o.verifyProviderControlV2(context.Background(), hashLabelV2("verify"), time.Now().Add(time.Second), p, n)
	if e != nil {
		t.Fatal(e)
	}
	before := o.head
	// Missing root is deliberately not inspected: funding must fail first.
	q, e := BeginProviderLaunchV2(context.Background(), r, hashLabelV2("launch"), time.Now().Add(time.Second), filepath.Join(filepath.Dir(p.Path), "not-created"), PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	if e == nil || q.state != nil || o.head != before {
		t.Fatal("ASSERT_LAUNCH_UNDERFUND_NO_METADATA")
	}
	t.Log("ASSERT_LAUNCH_UNDERFUND_NO_METADATA PASS")
}

func TestProviderLaunchWritableAncestorV2(t *testing.T) {
	ctx := context.Background()
	limits := PrivateManagedInputVerificationV2{400, 4 << 20, 400}
	o, pin, n := providerControlFixtureV2(t, func(a *authorizationV2) {
		fundProviderLaunchFixtureV2(t, a, limits)
	})
	receipt, e := o.verifyProviderControlV2(ctx, hashLabelV2("verify"), time.Now().Add(5*time.Second), pin, n)
	if e != nil {
		t.Fatal(e)
	}
	before, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || before.Disposition != "OBSERVED" || before.Windows != 1 {
		t.Fatal("ASSERT_LAUNCH_ANCESTOR_CONTROL_OBSERVED", e)
	}
	ancestor := providerProtectedScratchV2(t)
	base := filepath.Join(ancestor, "metadata")
	if e = os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(filepath.Join(base, "control"), 0700); e != nil {
		t.Fatal(e)
	}
	// The provider pin and metadata leaf remain private. Only this test-owned
	// metadata ancestor becomes writable, isolating launch ancestry admission.
	if e = os.Chmod(ancestor, 0777); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.Chmod(ancestor, 0700); e != nil {
			t.Error(e)
		}
	})
	attempt := hashLabelV2("launch-writable-ancestor")
	// Match Begin's charge clock to the persisted Unix-nanosecond timestamps
	// used below to reconstruct the exact debit, without changing the deadline.
	deadline := time.Now().Add(time.Second).Round(0)
	pending, e := BeginProviderLaunchV2(ctx, receipt, attempt, deadline, base, limits)
	defer pending.Close()
	if e == nil || pending.state != nil {
		t.Fatal("ASSERT_LAUNCH_WRITABLE_ANCESTOR_REFUSED", e)
	}
	after, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || after.Identity != o.head || after.Previous != before.Identity || after.Disposition != "PENDING" || after.Event != "RESERVE" || after.Windows != before.Windows+1 || after.Attempt != attempt {
		t.Fatal("ASSERT_LAUNCH_ANCESTOR_CONFIRMED_RESERVATION", e)
	}
	budget, e := budgetV2(o.limits)
	if e != nil {
		t.Fatal(e)
	}
	var reservation reservationV2
	if e = o.k.read(ctx, after.ReservationSelector, &reservation, budget); e != nil {
		t.Fatal(e)
	}
	charge, e := providerLaunchChargeV2(o.limits, limits, time.Duration(reservation.Deadline-reservation.ClaimedAt))
	if e != nil {
		t.Fatal(e)
	}
	want, e := addCountersV2(before.Charged, charge)
	if e != nil || reservation.Attempt != attempt || reservation.PreviousHead != before.Identity || reservation.Disposition != "PENDING" || reservation.Tranche != charge || reservation.Charged != want || after.Charged != want {
		t.Fatal("ASSERT_LAUNCH_ANCESTOR_RETAINS_EXACT_DEBIT", e)
	}
	t.Log("ASSERT_LAUNCH_WRITABLE_ANCESTOR_REFUSED PASS; ASSERT_LAUNCH_ANCESTOR_CONFIRMED_RESERVATION PASS; ASSERT_LAUNCH_ANCESTOR_RETAINS_EXACT_DEBIT PASS")
}

// Test-only stdio fixture bridge: cmd/main cannot import another package's
// _test.go helpers. No production fixture API, provider override or DTO credit.
func TestProviderLaunchFinalCloseCLIFixtureV2(t *testing.T) {
	if os.Getenv("LSP_TRACE_FINAL_CLOSE_FIXTURE") != "1" {
		t.Skip("only the bounded CLI regression launches this fixture")
	}
	source, _ := ControlledRPrepCanonicalOwnerForTest(t)
	sv := PrivateManagedInputVerificationV2{source.limits.Records, source.limits.Bytes, source.limits.Work}
	owner, pin, n := providerControlFixtureV2(t, func(a *authorizationV2) { fundProviderLaunchFixtureV2(t, a, sv) })
	base := filepath.Join(providerProtectedScratchV2(t), "host-assisted-full-definition-r-XoBFapmT")
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(base, "control"), 0700); e != nil {
		t.Fatal(e)
	}
	m := struct {
		Base, ControlRoot, ControlHead, SourceRoot, SourceHead, Receipt string
		ControlJob, SourceJob                                           []byte
		ControlVerification, SourceVerification                         PrivateManagedInputVerificationV2
		ControlDeadline, LaunchDeadline, Expiry                         time.Time
		FinalControlHead, Disposition                                   string
		Windows, SourceWindows                                          uint64
		Charged                                                         [14]uint64
	}{Base: base, ControlRoot: owner.k.root.Path(), ControlHead: owner.head, SourceRoot: source.k.root.Path(), SourceHead: source.head, ControlJob: sealTestV2(t, &owner.job), SourceJob: sealTestV2(t, &source.job), ControlVerification: PrivateManagedInputVerificationV2{owner.limits.Records, owner.limits.Bytes, owner.limits.Work}, SourceVerification: sv, ControlDeadline: time.Now().Add(5 * time.Second), LaunchDeadline: time.Now().Add(14 * time.Second)}
	receipt, e := owner.verifyProviderControlV2(context.Background(), hashLabelV2("final-close-pin"), m.ControlDeadline, pin, n)
	if e != nil {
		t.Fatal(e)
	}
	m.Receipt = receipt.String()
	pending, e := BeginProviderLaunchV2(context.Background(), receipt, hashLabelV2("final-close-launch"), m.LaunchDeadline, base, sv)
	if e != nil {
		t.Fatal(e)
	}
	defer pending.Close()
	emit := func(stage string) {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(stage + " " + base64.StdEncoding.EncodeToString(b))
	}
	emit("FINAL_CLOSE_READY")
	reader := bufio.NewReaderSize(os.Stdin, 65536)
	type input struct {
		Ready, Parent, Start []byte
		Started              time.Time
	}
	read := make(chan []byte, 1)
	go func() {
		b, err := reader.ReadSlice('\n')
		if err != nil {
			read <- nil
			return
		}
		read <- append([]byte(nil), b...)
	}()
	var request input
	select {
	case b := <-read:
		if len(b) == 0 || json.Unmarshal(b, &request) != nil {
			t.Fatal("fixture request")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("bounded fixture request expired")
	}
	if e = pending.PublishStartReadyV2(request.Ready); e != nil {
		t.Fatal(e)
	}
	for name, b := range map[string][]byte{"parent-ready.json": request.Parent, "start.json": request.Start} {
		if e = os.WriteFile(filepath.Join(base, "control", name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	for name, want := range map[string][]byte{"control/parent-ready.json": request.Parent, "control/start.json": request.Start} {
		got, re := pending.ReadStartMetadataV2(name)
		if re != nil || !bytes.Equal(got, want) {
			t.Fatal("fixture exact metadata", name, re)
		}
	}
	if e = pending.ConsumeStartV2(request.Start); e != nil {
		t.Fatal(e)
	}
	m.Expiry, e = pending.SourceAuthorizationExpiryV2(context.Background(), source)
	if e != nil {
		t.Fatal(e)
	}
	deadline := request.Started.Add(15 * time.Second)
	if m.Expiry.Before(deadline) {
		deadline = m.Expiry
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if e = pending.FinishProviderLaunchV2(ctx, pin.Path); e != nil {
		t.Fatal(e)
	}
	h, e := owner.k.reopen(context.Background(), owner.job, owner.limits)
	if e != nil {
		t.Fatal(e)
	}
	sh, e := source.k.reopen(context.Background(), source.job, source.limits)
	if e != nil {
		t.Fatal(e)
	}
	m.FinalControlHead, m.Disposition, m.Windows, m.SourceWindows, m.Charged = h.Identity, h.Disposition, h.Windows, sh.Windows, h.Charged.vector()
	if m.Disposition != "OBSERVED" || m.Windows != 2 || m.SourceWindows != 0 {
		t.Fatal("fixture ledger truth")
	}
	if e = owner.k.root.Close(); e != nil {
		t.Fatal(e)
	}
	if e = source.k.root.Close(); e != nil {
		t.Fatal(e)
	}
	emit("FINAL_CLOSE_OBSERVED")
	released := make(chan struct{})
	go func() { _, _ = reader.ReadByte(); close(released) }()
	select {
	case <-released:
	case <-time.After(10 * time.Second):
		t.Fatal("bounded fixture release expired")
	}
}
