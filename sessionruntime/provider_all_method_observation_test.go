package sessionruntime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

func TestProviderObservationsDocumentSuccessThenNonDocumentPoison(t *testing.T) {
	m, started, _ := roundTripManager(t, "document-success-then-malformed")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	request := func(method string) RoundTripResult {
		return m.RoundTrip(context.Background(), RoundTripRequest{
			SessionID: started.SessionID, Generation: started.Generation, Method: method,
			Params:   json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0}}`),
			Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096,
			CaptureProviderMessageCompositionAllMethods: true, CaptureProviderTransactionObservationAllMethods: true,
		})
	}
	completed := request("textDocument/documentSymbol")
	completedObservation, completedObservationPresent := completed.ProviderTransactionObservation()
	if completed.Failure != "" || !completedObservationPresent || completedObservation.CompletionSequence == 0 {
		t.Fatalf("ASSERT_PROVIDER_V7_SEQUENCE_DOCUMENT_SUCCESS result=%+v observation=%+v present=%v", completed, completedObservation, completedObservationPresent)
	}
	poisoned := request("textDocument/prepareCallHierarchy")
	composition, compositionPresent := poisoned.ProviderMessageComposition()
	transaction, transactionPresent := poisoned.ProviderTransactionObservation()
	if poisoned.Failure != session.SessionPoisoned || !compositionPresent || composition.Outcome != ProviderMessageOutcomeInvalidProtocolMessage || composition.InvalidProtocolMessages != 1 || !transactionPresent || transaction.TerminalLifecycle != ProviderTransactionLifecyclePoisoned || transaction.CompletionSequence <= completedObservation.CompletionSequence {
		t.Fatalf("ASSERT_PROVIDER_V7_SEQUENCE_POISON failure=%q composition=%+v composition_present=%v transaction=%+v transaction_present=%v", poisoned.Failure, composition, compositionPresent, transaction, transactionPresent)
	}
	subsequent := request("callHierarchy/outgoingCalls")
	if subsequent.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_PROVIDER_V7_SEQUENCE_LIFECYCLE_CONFLICT failure=%q", subsequent.Failure)
	}
}

func TestProviderObservationsOptInForEveryRoundTripMethod(t *testing.T) {
	m, started, _ := roundTripManager(t, "references-empty")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	result := m.RoundTrip(context.Background(), RoundTripRequest{
		SessionID:   started.SessionID,
		Generation:  started.Generation,
		Method:      "textDocument/prepareCallHierarchy",
		Params:      json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0}}`),
		Deadline:    time.Now().Add(time.Second),
		MaxMessages: 1,
		MaxBytes:    4096,
		CaptureProviderMessageCompositionAllMethods:     true,
		CaptureProviderTransactionObservationAllMethods: true,
	})
	composition, compositionPresent := result.ProviderMessageComposition()
	transaction, transactionPresent := result.ProviderTransactionObservation()
	if result.Failure != "" || !compositionPresent || !transactionPresent {
		t.Fatalf("ASSERT_PROVIDER_OBSERVATIONS_ALL_METHODS failure=%q composition_present=%v transaction_present=%v", result.Failure, compositionPresent, transactionPresent)
	}
	if composition.AcceptedTerminalResponses != 1 || transaction.TerminalLifecycle != ProviderTransactionLifecycleReady {
		t.Fatalf("ASSERT_PROVIDER_OBSERVATIONS_ALL_METHODS composition=%+v transaction=%+v", composition, transaction)
	}
}
