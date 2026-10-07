package sessionruntime

import (
	"context"
	"reflect"
	"testing"

	"lsp-trace/internal/adr0011c18"
	"lsp-trace/internal/session"
)

type c18RuntimeObservation struct {
	coarse []adr0011c18.Point
	events []adr0011c18.Event
}

func c18RuntimeRun(ctx context.Context, f *c16ActivationFixture) (c18RuntimeObservation, RoundTripResult, B4DefinitionLease) {
	var observed c18RuntimeObservation
	ctx = adr0011c18.WithObserver(ctx, func(point adr0011c18.Point) { observed.coarse = append(observed.coarse, point) })
	ctx = adr0011c18.WithEventObserver(ctx, func(event adr0011c18.Event) { observed.events = append(observed.events, event) })
	result, lease := f.m.RoundTripPrivateB4(ctx, f.req, f.owner)
	return observed, result, lease
}

func TestADR0011C18RuntimePathOrdersReachedStagesAndSealsBeforeHandoff(t *testing.T) {
	f := newC16ActivationFixture(t)
	observed, result, lease := c18RuntimeRun(context.Background(), f)
	if result.Failure != "" || lease == (B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED: real runtime fixture failed: failure=%q", result.Failure)
	}
	want := []adr0011c18.Event{
		{Point: adr0011c18.PointPreflight},
		{Point: adr0011c18.PointDeadline},
		{Point: adr0011c18.PointHeaderFrame},
		{Point: adr0011c18.PointCumulativeWire},
		{Point: adr0011c18.PointMessageCount},
		{Point: adr0011c18.PointDecodedMessage},
		{Point: adr0011c18.PointRawResult},
		{Point: adr0011c18.PointSourceBuffer},
		{Point: adr0011c18.PointRuntimePrefixSealed},
	}
	if !reflect.DeepEqual(observed.events, want) {
		t.Fatalf("C18_RUNTIME_STAGE_ORDER: exact reached-stage prefix mismatch\nwant=%v\n got=%v", want, observed.events)
	}
	if !reflect.DeepEqual(observed.coarse, []adr0011c18.Point{adr0011c18.PointRuntimeEntered, adr0011c18.PointRuntimeHandoff, adr0011c18.PointRuntimeReturned}) {
		t.Fatalf("C18_RUNTIME_HANDOFF_ORDER: %v", observed.coarse)
	}
}

func TestADR0011C18RuntimePathEarlierRefusalSuppressesLaterEffects(t *testing.T) {
	f := newC16ActivationFixture(t)
	f.req.Method = "textDocument/references"
	observed, result, lease := c18RuntimeRun(context.Background(), f)
	if result.Failure != session.ToolNotImplemented || lease != (B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED: preflight refusal fixture: failure=%q lease=%v", result.Failure, lease != (B4DefinitionLease{}))
	}
	if f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld {
		t.Fatalf("C18_RUNTIME_REFUSAL_EFFECT: refused preflight changed source custody: %d", f.owner.TargetSources[0].state.state.Load())
	}
	wantEvents := []adr0011c18.Event{{Point: adr0011c18.PointPreflight}}
	if !reflect.DeepEqual(observed.events, wantEvents) {
		t.Fatalf("C18_RUNTIME_REFUSAL_REACHED_ONLY: want=%v got=%v", wantEvents, observed.events)
	}
	if !reflect.DeepEqual(observed.coarse, []adr0011c18.Point{adr0011c18.PointRuntimeEntered}) {
		t.Fatalf("C18_RUNTIME_REFUSAL_COARSE: %v", observed.coarse)
	}
}

func c18MaterialResult(result RoundTripResult) any {
	return struct {
		Key                       any
		Result                    []byte
		ServerError               any
		Failure                   session.Failure
		Messages, RequestMessages int
		Bytes, RequestBytes       int64
		ThermalPhase              string
		Notifications, Responses  any
	}{result.Key, append([]byte(nil), result.Result...), result.ServerError, result.Failure, result.Messages, result.RequestMessages, result.Bytes, result.RequestBytes, result.ThermalPhase, result.Notifications, result.Responses}
}

func TestADR0011C18CoordinatorRejectsReviewerCounterexamples(t *testing.T) {
	t.Run("observer is not activation authority", func(t *testing.T) {
		_, receipt := adr0011c18.BeginRuntime(context.Background())
		if receipt == nil || !receipt.ReachRuntime(adr0011c18.PointPreflight) {
			t.Fatal("C18_OBSERVER_AUTHORITY: explicit private runtime activation depended on observer presence")
		}
	})

	t.Run("acquisition and runtime share one receipt", func(t *testing.T) {
		ctx, acquisition := adr0011c18.BeginAcquisition(context.Background(), "tx")
		_, runtime := adr0011c18.BeginRuntime(ctx)
		if acquisition == nil || runtime != acquisition || !runtime.ReachRuntime(adr0011c18.PointPreflight) {
			t.Fatal("C18_SPLIT_RECEIPT: runtime did not enter the acquisition-owned receipt")
		}
	})

	t.Run("historical stage is rejected", func(t *testing.T) {
		ctx := adr0011c18.WithEventObserver(context.Background(), func(adr0011c18.Event) {})
		_, receipt := adr0011c18.BeginRuntime(ctx)
		if !receipt.ReachRuntime(adr0011c18.PointPreflight) || !receipt.ReachRuntime(adr0011c18.PointDeadline) {
			t.Fatal("C18_TEST_SETUP: could not establish advanced runtime stage")
		}
		if receipt.ReachRuntime(adr0011c18.PointPreflight) {
			t.Fatal("C18_HISTORICAL_STAGE_ACCEPTED: earlier-stage reuse was accepted")
		}
	})

	t.Run("suffix requires runtime seal", func(t *testing.T) {
		ctx := adr0011c18.WithEventObserver(context.Background(), func(adr0011c18.Event) {})
		ctx, _ = adr0011c18.BeginAcquisition(ctx, "tx")
		_, receipt := adr0011c18.BeginRuntime(ctx)
		if receipt.ReachSuffix(adr0011c18.PointObjectEvent) {
			t.Fatal("C18_SUFFIX_WITHOUT_SEAL: suffix advanced before verified runtime completion")
		}
	})
}

func TestADR0011C18RuntimeObserverDefaultOffPreservesResultAndCustody(t *testing.T) {
	plain := newC16ActivationFixture(t)
	plainResult, plainLease := plain.m.RoundTripPrivateB4(context.Background(), plain.req, plain.owner)
	plainSource := plain.owner.TargetSources[0].state.state.Load()

	observed := newC16ActivationFixture(t)
	_, observedResult, observedLease := c18RuntimeRun(context.Background(), observed)
	observedSource := observed.owner.TargetSources[0].state.state.Load()
	if !reflect.DeepEqual(c18MaterialResult(plainResult), c18MaterialResult(observedResult)) || (plainLease == (B4DefinitionLease{})) != (observedLease == (B4DefinitionLease{})) || plainSource != observedSource {
		t.Fatalf("C18_RUNTIME_DEFAULT_OFF: result/lease/custody changed: plain=%+v observed=%+v source=%d/%d", c18MaterialResult(plainResult), c18MaterialResult(observedResult), plainSource, observedSource)
	}
}
