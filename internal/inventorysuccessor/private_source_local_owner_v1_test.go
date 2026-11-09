package inventorysuccessor

import (
	"context"
	"lsp-trace/internal/describeworker"
	"strings"
	"testing"
)

func TestSourceLocalZeroCannotMint(t *testing.T) {
	p := PendingLocalSourceRV1{}
	if _, _, _, _, _, e := p.Values(); e == nil {
		t.Fatal("ASSERT_LOCAL_ZERO_VALUES")
	}
	if _, e := p.SourceObjects(); e == nil {
		t.Fatal("ASSERT_LOCAL_ZERO_OBJECTS")
	}
	if _, e := p.OriginalQueryTargetID(nil); e == nil {
		t.Fatal("ASSERT_LOCAL_ZERO_TARGET")
	}
	if _, ok := p.RetainedPayloadUpper(); ok {
		t.Fatal("ASSERT_LOCAL_ZERO_RETENTION")
	}
	if p.VerifyPins(context.Background()) == nil || p.Finalize(context.Background(), nil) == nil {
		t.Fatal("ASSERT_LOCAL_ZERO_LIFETIME")
	}
	if got, e := ExecuteLocalRPreparationV1(context.Background(), PrivateSourceRPreparationOperationV1{}, PrivateSourceRPreparationRequestV1{Local: &PrivateSourceLocalConfigV1{}}); e == nil || got.state != nil {
		t.Fatal("ASSERT_LOCAL_ZERO_OPERATION")
	}
	if got, e := ExecuteRPreparationV1(context.Background(), PrivateSourceRPreparationOperationV1{}, PrivateSourceRPreparationRequestV1{Local: &PrivateSourceLocalConfigV1{}}); e == nil || got.state != nil {
		t.Fatal("ASSERT_LOCAL_NOT_OBSERVED_PREPARATION")
	}
	unissued := PendingLocalSourceRV1{state: &privateSourceRPreparationStateV1{local: &PrivateSourceLocalConfigV1{}, metadata: &preparedSourceRStateV1{}, execution: context.Background()}}
	if _, _, _, _, _, e := unissued.Values(); e == nil || unissued.VerifyPins(context.Background()) == nil || unissued.Finalize(context.Background(), nil) == nil {
		t.Fatal("ASSERT_LOCAL_UNISSUED_CANNOT_MINT")
	}
}
func TestSourceLocalReadCallBound(t *testing.T) {
	for _, tc := range []struct{ length, want uint64 }{
		{1, 2}, {32767, 2}, {32768, 2}, {32769, 3}, {1 << 20, 33}, {(1 << 63) - 2, (1 << 48) + 1},
	} {
		got, err := localReadCallBoundV1(tc.length)
		if err != nil || got != tc.want {
			t.Fatal("ASSERT_LOCAL_READ_CALL_BOUND", tc.length, got, tc.want, err)
		}
	}
	for _, length := range []uint64{0, (1 << 63) - 1, ^uint64(0)} {
		if got, err := localReadCallBoundV1(length); err == nil || got != 0 {
			t.Fatal("ASSERT_LOCAL_READ_CALL_BOUND_INVALID", length, got, err)
		}
	}
}

func TestSourceLocalClosedQuoteBounds(t *testing.T) {
	pin := describeworker.FilePin{Path: "/private/tiny", SHA256: hashLabelV2("tiny")}
	c := PrivateSourceLocalConfigV1{Config: describeworker.Config{Worker: pin, Model: pin, Library: pin, SandboxExecutable: pin, SandboxProfile: pin, Grammar: pin, RuntimeIdentity: "test", AdapterIdentity: "test", ModelIdentity: "test", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1}}, Lengths: [6]uint64{1, 1, 1, 1, 1, 1}}
	for _, tc := range []struct{ length, reads uint64 }{{1, 2}, {32767, 2}, {32768, 2}, {32769, 3}, {1 << 20, 33}} {
		v := c
		for i := range v.Lengths {
			v.Lengths[i] = tc.length
		}
		got, err := sourceLocalConfigChargeV1(v)
		want := uint64(64) + 6*(2*uint64(strings.Count(pin.Path, "/"))+8+tc.reads)
		if err != nil || got.Calls != want {
			t.Fatal("ASSERT_LOCAL_QUOTED_DESCRIPTOR_CALLS", tc.length, got.Calls, want, err)
		}
	}
	base, e := sourceLocalConfigChargeV1(c)
	if e != nil {
		t.Fatal(e)
	}
	if base.ProviderRequests != 0 || base.ProviderMessages != 0 || base.RequestBytes != 0 || base.ResponseBytes != 0 || base.WireBytes != 0 || base.ActiveNS != 0 {
		t.Fatal("ASSERT_LOCAL_NO_NEW_PROVIDER_OR_TIME_GRANT", base)
	}
	for i := 0; i < 6; i++ {
		v := c
		v.Lengths[i]++
		got, e := sourceLocalConfigChargeV1(v)
		if e != nil || got.ReadHashBytes != base.ReadHashBytes+2 || got.LogicalBytes != base.LogicalBytes+4 || got.Work != base.Work+4 {
			t.Fatal("ASSERT_LOCAL_PIN_LENGTH_FUNDS_READ_HASH", i, got, e)
		}
	}
	for i := 0; i < 4; i++ {
		v := c
		caps := []*int{&v.Config.Limits.WorkBytes, &v.Config.Limits.TempBytes, &v.Config.Limits.StdoutBytes, &v.Config.Limits.StderrBytes}
		*caps[i]++
		got, e := sourceLocalConfigChargeV1(v)
		if e != nil || got.OutputBytes != base.OutputBytes+1 || got.LogicalBytes != base.LogicalBytes+4 {
			t.Fatal("ASSERT_LOCAL_CAPACITY_JOIN", i, got, e)
		}
	}
	for _, length := range []uint64{0, ^uint64(0), ^uint64(0) >> 1} {
		v := c
		v.Lengths[0] = length
		if _, e := sourceLocalConfigChargeV1(v); e == nil {
			t.Fatal("ASSERT_LOCAL_PIN_OVERFLOW_OR_ZERO", length)
		}
	}
}
