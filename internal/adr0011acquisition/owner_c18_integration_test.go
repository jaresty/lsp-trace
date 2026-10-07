package adr0011acquisition

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/adr0011c18"
)

type c18AcquireObservation struct {
	coarse []adr0011c18.Point
	events []adr0011c18.Event
}

func c18AcquireRun(ctx context.Context, owner *Owner) (c18AcquireObservation, error) {
	var observed c18AcquireObservation
	ctx = adr0011c18.WithObserver(ctx, func(point adr0011c18.Point) { observed.coarse = append(observed.coarse, point) })
	ctx = adr0011c18.WithEventObserver(ctx, func(event adr0011c18.Event) { observed.events = append(observed.events, event) })
	_, err := owner.acquireManaged(ctx, Query{}, false)
	return observed, err
}

func c18RequireOneTerminal(t *testing.T, events []adr0011c18.Event, transaction string, outcome adr0011c18.Outcome) {
	t.Helper()
	want := []adr0011c18.Event{{Point: adr0011c18.PointTerminalCustody, Transaction: transaction, Outcome: outcome}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("C18_ACQUISITION_TERMINAL_CUSTODY: omission, duplication, identity substitution, or outcome mismatch\nwant=%v\n got=%v", want, events)
	}
}

func TestADR0011C18AcquireManagedSealsErrorCustodyExactlyOnce(t *testing.T) {
	observed, err := c18AcquireRun(context.Background(), NewDisabled(nil))
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("BLOCKED_NOT_RED: disabled real path returned %v", err)
	}
	if !reflect.DeepEqual(observed.coarse, []adr0011c18.Point{adr0011c18.PointAcquisitionEntered, adr0011c18.PointAcquisitionReturned}) {
		t.Fatalf("BLOCKED_NOT_RED: inert observer order changed: %v", observed.coarse)
	}
	c18RequireOneTerminal(t, observed.events, "disabled", adr0011c18.OutcomeError)
}

func TestADR0011C18AcquireManagedObserverPanicCannotReplaceTerminalResolution(t *testing.T) {
	coarseCalls := 0
	semanticCalls := 0
	ctx := adr0011c18.WithObserver(context.Background(), func(adr0011c18.Point) {
		coarseCalls++
		panic("coarse observer fault")
	})
	ctx = adr0011c18.WithEventObserver(ctx, func(adr0011c18.Event) {
		semanticCalls++
		panic("semantic observer fault")
	})
	adr0011c18.NotifyEvent(ctx, adr0011c18.Event{Point: adr0011c18.PointTerminalCustody, Transaction: "probe", Outcome: adr0011c18.OutcomePanic})
	_, err := NewDisabled(nil).acquireManaged(ctx, Query{}, false)
	if !errors.Is(err, ErrDisabled) || coarseCalls != 2 || semanticCalls != 1 {
		t.Fatalf("C18_ACQUISITION_PANIC_PATH: observer fault changed production result: err=%v coarse=%d semantic=%d", err, coarseCalls, semanticCalls)
	}
}

func TestADR0011C18TerminalValidatorRejectsWrongCardinalityIdentityAndOutcome(t *testing.T) {
	valid := adr0011c18.Event{Point: adr0011c18.PointTerminalCustody, Transaction: "tx", Outcome: adr0011c18.OutcomeSuccess}
	for _, tc := range []struct {
		name   string
		events []adr0011c18.Event
	}{
		{"missing", nil},
		{"duplicate", []adr0011c18.Event{valid, valid}},
		{"identity", []adr0011c18.Event{{Point: valid.Point, Transaction: "other", Outcome: valid.Outcome}}},
		{"outcome", []adr0011c18.Event{{Point: valid.Point, Transaction: valid.Transaction, Outcome: adr0011c18.OutcomeError}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if reflect.DeepEqual(tc.events, []adr0011c18.Event{valid}) {
				t.Fatalf("invalid terminal event accepted: %v", tc.events)
			}
		})
	}
}

func TestADR0011C18CoordinatorTerminalSealExactlyOnceWithoutObserver(t *testing.T) {
	_, receipt := adr0011c18.BeginAcquisition(context.Background(), "tx")
	if receipt == nil || !receipt.SealTerminal("tx", adr0011c18.OutcomeError) {
		t.Fatal("C18_TERMINAL_FIRST_SEAL: observerless terminal seal was not committed")
	}
	if receipt.SealTerminal("tx", adr0011c18.OutcomeError) {
		t.Fatal("C18_TERMINAL_DUPLICATE_SEAL: duplicate terminal seal was accepted")
	}
}

func TestADR0011C18AcquireManagedRejectsIncompatibleReceiptReuse(t *testing.T) {
	ctx, existing := adr0011c18.BeginAcquisition(context.Background(), "existing")
	if existing == nil {
		t.Fatal("C18_TEST_SETUP: could not activate existing receipt")
	}
	var events []adr0011c18.Event
	ctx = adr0011c18.WithEventObserver(ctx, func(event adr0011c18.Event) { events = append(events, event) })
	_, err := NewDisabled(nil).acquireManaged(ctx, Query{}, false)
	if !errors.Is(err, ErrAcquisition) || len(events) != 0 {
		t.Fatalf("C18_INCOMPATIBLE_RECEIPT_REUSE: err=%v events=%v", err, events)
	}
}

func TestADR0011C18AcquireManagedObserverDefaultOffPreservesError(t *testing.T) {
	_, plainErr := NewDisabled(nil).acquireManaged(context.Background(), Query{}, false)
	_, observedErr := c18AcquireRun(context.Background(), NewDisabled(nil))
	if !errors.Is(plainErr, ErrDisabled) || !errors.Is(observedErr, ErrDisabled) {
		t.Fatalf("C18_ACQUISITION_DEFAULT_OFF: observer changed disabled behavior: plain=%v observed=%v", plainErr, observedErr)
	}
}
