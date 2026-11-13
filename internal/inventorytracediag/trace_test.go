package inventorytracediag

import (
	"context"
	"runtime/trace"
	"testing"
)

func disabled(t *testing.T) {
	t.Helper()
	if trace.IsEnabled() {
		t.Fatal("ASSERT_TRACE_DISABLED_PRECONDITION")
	}
}
func TestTraceClosedCodes(t *testing.T) {
	disabled(t)
	for i := 0; i < 256; i++ {
		c := Code(i)
		want := i > 0 && i < len(codeNames)
		if Valid(c) != want {
			t.Fatalf("ASSERT_TRACE_CLOSED_CODES code=%d", i)
		}
	}
	seen := map[string]bool{}
	for _, s := range codeNames[1:] {
		if s == "" || seen[s] {
			t.Fatal("ASSERT_TRACE_CLOSED_CODES vocabulary")
		}
		seen[s] = true
		for _, r := range s {
			if !(r == '_' || r >= 'A' && r <= 'Z') {
				t.Fatal("ASSERT_TRACE_CLOSED_CODES payload")
			}
		}
	}
	t.Log("ASSERT_TRACE_CLOSED_CODES PASS")
}
func TestTraceCapabilityMasks(t *testing.T) {
	disabled(t)
	for q := 0; q < 8; q++ {
		for obs := 0; obs < 16; obs++ {
			req, got, missing := CapabilityMasks(q&1 != 0, q&2 != 0, q&4 != 0, obs&1 != 0, obs&2 != 0, obs&4 != 0, obs&8 != 0)
			want := Mask(1 | q<<1)
			if req != want || got != Mask(obs) || missing != want&^Mask(obs) {
				t.Fatalf("ASSERT_TRACE_CAPABILITY_MASKS q=%d obs=%d", q, obs)
			}
		}
	}
	for m := 0; m < 256; m++ {
		if ValidMask(Mask(m)) != (m < 16) {
			t.Fatal("ASSERT_TRACE_CAPABILITY_MASKS bound")
		}
	}
	t.Log("ASSERT_TRACE_CAPABILITY_MASKS PASS")
}
func TestTraceDisabledNoop(t *testing.T) {
	disabled(t)
	n := testing.AllocsPerRun(100, func() {
		for _, code := range []Code{Invalid, Execute, NativeProofWithoutCalls, 255} {
			Event(nil, code)
			Begin(nil, code).End()
		}
		Capabilities(nil, All, 0)
		Capabilities(nil, 255, 255)
		MethodState(context.Background(), true, true)
	})
	if n != 0 || Enabled() || trace.IsEnabled() {
		t.Fatalf("ASSERT_TRACE_DISABLED_NOOP allocations=%v", n)
	}
	t.Log("ASSERT_TRACE_DISABLED_NOOP PASS")
}
