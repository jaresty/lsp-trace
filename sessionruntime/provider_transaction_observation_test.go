package sessionruntime

import (
	"context"
	"testing"

	"lsp-trace/internal/session"
)

func TestProviderTransactionObservationAcceptedResponse(t *testing.T) {
	m, started, _ := roundTripManager(t, "success")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := documentSymbolFrameRequest(started)
	req.CaptureProviderTransactionObservation = true
	got := m.RoundTrip(context.Background(), req)
	observation, ok := got.ProviderTransactionObservation()
	if got.Failure != "" || !ok || !observation.SelectedResponseObserved || !observation.RequestWriteCompleted || observation.ProviderMessagesProcessed != 3 || !observation.ProviderResponseWriteCompleted || observation.FailurePostEffect || observation.TerminalPoisonObserved || observation.TerminalLifecycle != ProviderTransactionLifecycleReady {
		t.Fatalf("ASSERT_PROVIDER_TRANSACTION_ACCEPTED observation=%+v present=%t failure=%s", observation, ok, got.Failure)
	}
}

func TestProviderTransactionObservationPostEffectPoisonAndRefusalWithoutRedispatch(t *testing.T) {
	m, started, child := roundTripManager(t, "malformed")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := documentSymbolFrameRequest(started)
	req.CaptureProviderTransactionObservation = true
	got := m.RoundTrip(context.Background(), req)
	observation, ok := got.ProviderTransactionObservation()
	if got.Failure != session.SessionPoisoned || !ok || observation.SelectedResponseObserved || !observation.RequestWriteCompleted || observation.ProviderMessagesProcessed != 0 || observation.ProviderResponseWriteCompleted || !observation.FailurePostEffect || !observation.TerminalPoisonObserved || observation.TerminalLifecycle != ProviderTransactionLifecyclePoisoned {
		t.Fatalf("ASSERT_PROVIDER_TRANSACTION_POST_EFFECT_POISON observation=%+v present=%t failure=%s", observation, ok, got.Failure)
	}
	before, _ := child.snapshot()
	refused := m.RoundTrip(context.Background(), req)
	after, _ := child.snapshot()
	if refused.Failure != session.LifecycleConflict || len(after) != len(before) {
		t.Fatalf("ASSERT_PROVIDER_TRANSACTION_POISON_REFUSES_REDISPATCH failure=%s before=%d after=%d", refused.Failure, len(before), len(after))
	}
	if _, present := refused.ProviderTransactionObservation(); present {
		t.Fatal("ASSERT_PROVIDER_TRANSACTION_REFUSAL_TUPLE_ABSENT")
	}
}
