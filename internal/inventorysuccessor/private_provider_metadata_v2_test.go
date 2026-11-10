//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderLaunchRawMetadataV2(t *testing.T) {
	for _, mode := range []string{"normal", "stat-eintr", "close-failure", "call-exhaustion"} {
		t.Run(mode, func(t *testing.T) {
			q, _, _, _ := providerLaunchFixtureV2(t)
			base := q.state.base
			path := filepath.Join(base, "control", "start.json")
			body := []byte(`{"operation":"START_CONFIRM"}`)
			if e := os.WriteFile(path, body, 0600); e != nil {
				t.Fatal(e)
			}
			fault := &providerFaultFixtureV2{mode: mode, path: path}
			q.state.io.fixture = fault
			raw, e := q.ReadStartMetadataV2("control/start.json")
			if mode == "normal" {
				if e != nil || string(raw) != string(body) || fault.reads != 2 {
					t.Fatalf("ASSERT_RAW_METADATA_SUCCESS %v %s %+v", e, raw, fault)
				}
			} else {
				if e == nil || raw != nil {
					t.Fatal("ASSERT_RAW_METADATA_REFUSAL", mode)
				}
				if mode == "stat-eintr" && (fault.stats != 1 || fault.reads != 0) {
					t.Fatal("ASSERT_RAW_METADATA_EINTR", fault)
				}
			}
			if q.state.io.closeCredits != 1 {
				t.Fatal("ASSERT_METADATA_CLEANUP_OWNERSHIP")
			}
			h, he := q.state.receipt.state.owner.k.reopen(context.Background(), q.state.receipt.state.owner.job, q.state.receipt.state.owner.limits)
			if he != nil || h.Disposition != "PENDING" || h.Windows != 2 {
				t.Fatal("ASSERT_METADATA_PENDING", he, h)
			}
			t.Log("ASSERT_RAW_METADATA_BOUNDARY PASS", mode)
		})
	}
}
func TestFixedProviderPrepaidCloseAfterExhaustionV2(t *testing.T) {
	_, pin, _ := providerControlFixtureV2(t)
	fault := &providerFaultFixtureV2{mode: "normal", path: pin.Path}
	p := &providerControlIOV2{ctx: context.Background(), left: 129*8 + 16, fixture: fault}
	f, e := p.open(pin.Path)
	if e != nil {
		t.Fatal(e)
	}
	p.left = 0 // all ordinary attempt slots unavailable; close was prepaid at open
	before := fault.closes
	if e = f.Close(); e != nil || fault.closes != before+1 || p.closeCredits != 0 {
		t.Fatal("ASSERT_PREPAID_CLOSE_EXHAUSTION", e, fault.closes, before, p.closeCredits)
	}
	t.Log("ASSERT_PREPAID_CLOSE_EXHAUSTION PASS")
}
func TestProviderLaunchCrossOwnerStaleHeadV2(t *testing.T) {
	o, pin, n := providerControlFixtureV2(t, func(a *authorizationV2) {
		fundProviderLaunchFixtureV2(t, a, PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	})
	ctx := context.Background()
	receipt, e := o.verifyProviderControlV2(ctx, hashLabelV2("first-owner-control"), time.Now().Add(2*time.Second), pin, n)
	if e != nil {
		t.Fatal(e)
	}
	other, e := OpenPrivateManagedInputOwnerV2(ctx, o.k.root, sealTestV2(t, &o.job), o.head, PrivateManagedInputVerificationV2{o.limits.Records, o.limits.Bytes, o.limits.Work})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = other.verifyProviderControlV2(ctx, hashLabelV2("second-owner-control"), time.Now().Add(time.Second), pin, n); e != nil {
		t.Fatal(e)
	}
	base := filepath.Join(t.TempDir(), "never-opened")
	if q, e := BeginProviderLaunchV2(ctx, receipt, hashLabelV2("stale-first-owner-launch"), time.Now().Add(time.Second), base, PrivateManagedInputVerificationV2{400, 4 << 20, 400}); e == nil || q.state != nil {
		t.Fatal("ASSERT_CROSS_OWNER_STALE_HEAD")
	}
	if _, e = os.Stat(base); !os.IsNotExist(e) {
		t.Fatal("ASSERT_STALE_BEFORE_METADATA", e)
	}
	t.Log("ASSERT_CROSS_OWNER_STALE_HEAD PASS")
}
