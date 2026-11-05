package sessionruntime

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPrivateProviderWindowUnavailableIsNotObservedEmpty(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
		s.child.capabilityUnavailable = true
		st := m.Start(ctx, start)
		op := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
		ready, ok := m.WaitReadiness(ctx, op.ID)
		if !ok || ready.State != ReadinessFailed || ready.Failure == "" {
			t.Fatal("ASSERT_WINDOW_CAPABILITY_UNAVAILABLE")
		}
		got, err := m.FinishPrivateProviderWindow(ctx)
		if err == nil || got.Completed {
			t.Fatal("ASSERT_WINDOW_UNAVAILABLE_NOT_SUCCESS")
		}
	})
	t.Run("observed-empty", func(t *testing.T) {
		plan := PrivateProviderWindowPlan{References: true}
		m, start, ctx, s := windowTestSetup(t, plan, "")
		s.child.emptyResults = true
		st := windowTestReady(t, m, start, ctx)
		supply := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"})
		if supply.Failure != "" {
			t.Fatal(supply.Failure)
		}
		for _, method := range []string{"textDocument/documentSymbol", "textDocument/references"} {
			got := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: method, Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
			if got.Failure != "" || string(got.Result) != "[]" {
				t.Fatal("ASSERT_WINDOW_OBSERVED_EMPTY_IS_AVAILABLE")
			}
		}
		if got, err := m.FinishPrivateProviderWindow(ctx); err != nil || !got.Completed {
			t.Fatal("ASSERT_WINDOW_EMPTY_COMPLETES")
		}
	})
}
func TestPrivateProviderWindowScheduleAndParameterAdmissionPreEffect(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	st := windowTestReady(t, m, start, ctx)
	before := m.Census()
	bad := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "workspace/arbitrary", Params: json.RawMessage(`{}`), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
	if bad.Failure == "" {
		t.Fatal("ASSERT_WINDOW_CLOSED_SCHEDULE")
	}
	if after := m.Census(); after.Requests != before.Requests || after.Workers != before.Workers {
		t.Fatal("ASSERT_WINDOW_PRE_PENDING_ADMISSION")
	}
	_, _ = m.FinishPrivateProviderWindow(ctx)
	_ = s
}
