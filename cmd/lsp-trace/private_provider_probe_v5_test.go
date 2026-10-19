package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func providerProbeV5Observation(poisoned bool) sessionruntime.ProviderTransactionObservation {
	lifecycle := sessionruntime.ProviderTransactionLifecycleReady
	if poisoned {
		lifecycle = sessionruntime.ProviderTransactionLifecyclePoisoned
	}
	return sessionruntime.ProviderTransactionObservation{RequestWriteCompleted: true, FailurePostEffect: poisoned, TerminalPoisonObserved: poisoned, TerminalLifecycle: lifecycle}
}

func TestPrivateProviderProbeV5SelectionPresenceAndStability(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v5")
	if err != nil {
		t.Fatal(err)
	}
	absent := p.artifactV5()
	if absent.Transaction != nil || absent.Status != "INCOMPLETE" {
		t.Fatal("ASSERT_PROVIDER_PROBE_V5_ABSENT_NOT_ALL_FALSE")
	}
	absentRaw, _ := json.Marshal(absent)
	if _, err := parsePrivateProviderProbeV5(append(absentRaw, '\n')); err != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_ABSENT_CANONICAL: %v", err)
	}
	allFalse := sessionruntime.ProviderTransactionObservation{TerminalLifecycle: sessionruntime.ProviderTransactionLifecycleReady}
	p.observeV5(allFalse, true)
	if got := p.artifactV5(); got.Transaction == nil || got.Status != "COMPLETE" {
		t.Fatal("ASSERT_PROVIDER_PROBE_V5_PRESENT_ALL_FALSE")
	}
	p.transactionV5 = nil
	observedWithoutCompletedRead := providerProbeV5Observation(false)
	observedWithoutCompletedRead.SelectedResponseObserved = true
	p.observeV5(observedWithoutCompletedRead, true)
	if got := p.artifactV5(); got.Transaction == nil || !got.Transaction.SelectedResponseObserved || got.Transaction.SelectedResponseReadCompleted {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_RESPONSE_OBSERVED_WITHOUT_COMPLETED_READ artifact=%+v", got)
	}
	p.transactionV5 = nil
	accepted := providerProbeV5Observation(false)
	accepted.SelectedResponseObserved, accepted.SelectedResponseReadCompleted, accepted.ProviderMessagesProcessed = true, true, 2
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, 64, 4096, 2, 100)
	p.observeV5(accepted, true)
	poisoned := providerProbeV5Observation(true)
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), false, sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, 64, 4096, 1, 10)
	p.observeV5(poisoned, true)
	later := poisoned
	later.ProviderMessagesProcessed = 9
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), false, sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, 64, 4096, 9, 90)
	p.observeV5(later, true)
	a := p.artifactV5()
	if a.Work != 3 || a.Transaction == nil || !a.Transaction.TerminalPoisonObserved || a.Transaction.ProviderMessagesProcessed != 0 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_FIRST_POISON_ELSE_FIRST artifact=%+v", a)
	}
}

func TestPrivateProviderProbeV5CanonicalParserPrivacyAndV1V4Bytes(t *testing.T) {
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v5")
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), false, sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, 64, 4096, 0, 1)
	p.observeV5(providerProbeV5Observation(true), true)
	a := p.artifactV5()
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	if got, err := parsePrivateProviderProbeV5(raw); err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_CANONICAL err=%v got=%+v", err, got)
	}
	observedWithoutCompletedRead := a
	transaction := *a.Transaction
	observedWithoutCompletedRead.Transaction = &transaction
	observedWithoutCompletedRead.Transaction.SelectedResponseObserved = true
	observedWithoutCompletedRead.Transaction.SelectedResponseReadCompleted = false
	observedWithoutCompletedReadRaw, _ := json.Marshal(observedWithoutCompletedRead)
	if _, err := parsePrivateProviderProbeV5(append(observedWithoutCompletedReadRaw, '\n')); err != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_RESPONSE_OBSERVED_WITHOUT_COMPLETED_READ_CANONICAL: %v", err)
	}
	for _, mutate := range []func(*privateProviderProbeArtifactV5){
		func(v *privateProviderProbeArtifactV5) { v.Transaction.TerminalLifecycle = "UNKNOWN" },
		func(v *privateProviderProbeArtifactV5) { v.Transaction.TerminalPoisonObserved = false },
		func(v *privateProviderProbeArtifactV5) { v.Transaction.SelectedResponseReadCompleted = true },
		func(v *privateProviderProbeArtifactV5) { v.Transaction.RequestWriteCompleted = false },
	} {
		candidate := a
		transaction := *a.Transaction
		candidate.Transaction = &transaction
		mutate(&candidate)
		encoded, _ := json.Marshal(candidate)
		if _, err := parsePrivateProviderProbeV5(append(encoded, '\n')); err == nil {
			t.Fatal("ASSERT_PROVIDER_PROBE_V5_FAIL_CLOSED")
		}
	}
	forbidden := []string{"error", "method", "payload", "path", "uri", "_id", "generation", "hash", "stderr", "source", "params", "result"}
	typ := reflect.TypeOf(privateProviderProbeTransactionV5{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name + " " + typ.Field(i).Tag.Get("json"))
		for _, word := range forbidden {
			if strings.Contains(name, word) {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V5_PRIVACY field=%s forbidden=%s", name, word)
			}
		}
	}
	v4, _ := newPrivateProviderProbeVersion(probePath(t), "v4")
	v4.observeV3("DOCUMENT_SYMBOL", "", "", false, sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, 64, 4096, 1, 1)
	before, _ := json.Marshal(v4.artifactV4())
	after, _ := json.Marshal(v4.artifactV4())
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_PROVIDER_PROBE_V1_V4_BYTES_UNCHANGED")
	}
}
