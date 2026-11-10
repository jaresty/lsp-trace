//go:build darwin || linux

package inventorysuccessor

import (
	"encoding/json"
	"lsp-trace/sessionruntime"
	"math"
	"testing"
	"time"
)

func TestHOSTTimingPolicyV2(t *testing.T) {
	p := PrivateSourceTimingV2{Mode: PrivateSourceHOST60V2, CaptureMS: 15000, PreparationMS: 15000, HOSTMS: 60000, PublicationCleanupMS: 5000}
	if d, e := p.Total(); e != nil || d != 95*time.Second {
		t.Fatal("ASSERT_HOST_SELECTED_POLICY", d, e)
	}
	bad := []PrivateSourceTimingV2{p, p, p, p, p, p, p, p}
	bad[0].CaptureMS = 0
	bad[1].CaptureMS = 15001
	bad[2].PreparationMS = 0
	bad[3].PreparationMS = 15001
	bad[4].HOSTMS = 5000
	bad[5].PublicationCleanupMS = 0
	bad[6].PublicationCleanupMS = math.MaxInt64
	bad[7].Mode = ""
	for _, v := range bad {
		if _, e := v.Total(); e == nil || v.Rules() != "" {
			t.Fatal("ASSERT_HOST_INVALID_POLICY", v)
		}
	}
	f, e := p.Bind(time.Now())
	if e != nil || !f.Valid() {
		t.Fatal(e)
	}
	f.TotalDeadline = f.TotalDeadline.Add(time.Nanosecond)
	if f.Valid() {
		t.Fatal("ASSERT_HOST_IMMUTABLE_FRAME")
	}
	t.Log("ASSERT_HOST_SELECTED_POLICY PASS; 95s selected example, not measured fit or a proven 90s total")
}
func TestHOSTTimingQuoteV2(t *testing.T) {
	p := PrivateSourceTimingV2{Mode: PrivateSourceHOST60V2, CaptureMS: 15000, PreparationMS: 15000, HOSTMS: 60000, PublicationCleanupMS: 5000}
	w := sessionruntime.PrivateProviderWindowBounds{RequestBytes: 512, ResponseBytes: 512, WireBytes: 2048, Frames: 2}
	m, pv, sum, tranche, window, e := PrivateSourceHOSTQuotesV2(p, w, PrivateManagedInputVerificationV2{400, 4 << 20, 400})
	if e != nil || m[8] != uint64(96*time.Second) || pv[8] != uint64(95*time.Second) || sum[8] != uint64(191*time.Second) || window != uint64(95*time.Second) {
		t.Fatal("ASSERT_HOST_FUNDED_LIFETIME", e, m, pv, sum, window)
	}
	for i := range m {
		if sum[i] != m[i]+pv[i] || tranche[i] != max(m[i], pv[i]) {
			t.Fatal("ASSERT_HOST_FOURTEEN_QUOTES", i)
		}
	}
	if pv[5] != 0 || pv[6] != 0 || pv[10] != 0 || pv[11] != 0 || pv[12] != 0 {
		t.Fatal("ASSERT_HOST_NOT_SOURCE_PROVIDER_CREDIT")
	}
	t.Log("ASSERT_HOST_FOURTEEN_QUOTES PASS; HOST account UNKNOWN_NOT_FREE")
}
func TestHOSTTimingJSONCannotMintV2(t *testing.T) {
	raw := []byte("{\"Profile\":{\"Mode\":\"HOST60_V2\"},\"sourceConfirmed\":true,\"sourceTiming\":{}}")
	var o PrivateProducerOriginals
	var in PrivateManagedGroupingInput
	if json.Unmarshal(raw, &o) != nil || json.Unmarshal(raw, &in) != nil {
		t.Fatal("fixture JSON")
	}
	if _, ok := o.HOSTTimingV2(); ok || o.HOSTRetainedContextV2() != nil || in.sourceTiming != nil || in.sourceConfirmed {
		t.Fatal("ASSERT_HOST_JSON_NOT_CUSTODY")
	}
	t.Log("ASSERT_HOST_JSON_NOT_CUSTODY PASS")
}
