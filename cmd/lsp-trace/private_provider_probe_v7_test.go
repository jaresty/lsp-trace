package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func providerProbeV7Observation(lifecycle sessionruntime.ProviderTransactionLifecycle, notifications, controls uint64) sessionruntime.ProviderTransactionObservation {
	return sessionruntime.ProviderTransactionObservation{
		CompletionSequence:       1,
		SelectedResponseObserved: notifications+controls > 0, SelectedResponseReadCompleted: notifications+controls > 0,
		ProviderMessagesProcessed: notifications + controls, NotificationMessagesProcessed: notifications, ControlMessagesProcessed: controls,
		NotificationCeilingPolicy: sessionruntime.ProviderNotificationCeilingConfiguredCount, ConfiguredControlCeiling: 64, ConfiguredMaxBytes: 4096,
		TerminalPoisonObserved: lifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned, RequestWriteCompleted: true, TerminalLifecycle: lifecycle,
	}
}

func setProviderProbeV7CompletionSequence(t *testing.T, observation *sessionruntime.ProviderTransactionObservation, sequence uint64) {
	t.Helper()
	field := reflect.ValueOf(observation).Elem().FieldByName("CompletionSequence")
	if !field.IsValid() || !field.CanSet() {
		t.Fatal("ASSERT_PROVIDER_PROBE_V7_MANAGER_COMPLETION_SEQUENCE field unavailable")
	}
	field.SetUint(sequence)
}

func TestPrivateProviderProbeV7AtomicSelectionAndStability(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v7")
	if err != nil {
		t.Fatal(err)
	}
	completedComposition := sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}
	p.observeV7("DOCUMENT_SYMBOL", "", "", completedComposition, true, providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1), true, 64, 4096, 1, 10)
	poisonComposition := sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}
	p.observeV7("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), poisonComposition, true, providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 0, 1), true, 64, 4096, 1, 20)
	p.observeV7("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), poisonComposition, true, providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 9, 0), true, 64, 4096, 9, 90)
	a := p.artifactV7()
	if a.Status != "COMPLETE" || a.Witness == nil || a.Witness.Phase != "PROVIDER_REQUEST" || a.Witness.Class != "TYPED_SESSION_RUNTIME_FAILURE" || a.Witness.Cause != string(session.SessionPoisoned) || a.Witness.Composition.Selection != "FIRST_FAILURE_OR_POISON" || a.Witness.Composition.ManagerBytes != 20 || a.Witness.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned || a.Witness.Transaction.ObservedControlCount != 1 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V7_ATOMIC_SELECTION artifact=%+v", a)
	}
}

func TestPrivateProviderProbeV7JSONRPCErrorNonPoisonAndParserPrivacy(t *testing.T) {
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v7")
	composition := sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}
	p.observeV7("PROVIDER_REQUEST", "JSON_RPC_CODE_BUCKET", "METHOD_NOT_FOUND", composition, true, providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1), true, 64, 4096, 1, 10)
	a := p.artifactV7()
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	parsed, err := parsePrivateProviderProbeV7(raw)
	if err != nil || !reflect.DeepEqual(parsed, a) || parsed.Witness.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V7_RPC_NON_POISON err=%v artifact=%+v", err, parsed)
	}
	for _, forbidden := range []string{`"method":`, `"uri":`, `"path":`, `"params":`, `"source":`, `"content":`, `"hash":`, `"id":`, `"frame":`, `"raw":`, `"error":`, `"session":`} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V7_PRIVACY sentinel=%q raw=%s", forbidden, raw)
		}
	}
	var loose map[string]any
	_ = json.Unmarshal(raw, &loose)
	loose["extra"] = true
	bad, _ := json.Marshal(loose)
	if _, err = parsePrivateProviderProbeV7(append(bad, '\n')); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V7_STRICT unknown field accepted")
	}
	if _, err = parsePrivateProviderProbeV7(raw[:len(raw)-1]); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V7_STRICT missing newline accepted")
	}
}

func TestPrivateProviderProbeV7ParserMutationClosure(t *testing.T) {
	base := privateProviderProbeArtifactV7{
		Schema: privateProviderProbeSchemaV7, Status: "COMPLETE",
		Witness: &privateProviderProbeWitnessV7{
			Phase:       "DOCUMENT_SYMBOL",
			Composition: privateProviderProbeCompositionV4{Sequence: 1, Selection: "FIRST_COMPLETED", AcceptedTerminalResponses: 1, ManagerMessages: 1, ManagerBytes: 10, ConfiguredMaxMessages: 64, ConfiguredMaxBytes: 4096, Outcome: string(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse)},
			Transaction: privateProviderProbeTransactionV7{CompletionSequence: 1, NotificationCeilingPolicy: sessionruntime.ProviderNotificationCeilingConfiguredCount, ConfiguredControlCeiling: 64, ConfiguredBytes: 4096, ObservedControlCount: 1, TerminalResponseObserved: true, ReadCompleted: true, TerminalLifecycle: sessionruntime.ProviderTransactionLifecycleReady},
		},
	}
	parse := func(t *testing.T, candidate privateProviderProbeArtifactV7, wantValid bool) {
		t.Helper()
		raw, _ := json.Marshal(candidate)
		_, err := parsePrivateProviderProbeV7(append(raw, '\n'))
		if (err == nil) != wantValid {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V7_PARSER_CLOSURE want_valid=%v err=%v artifact=%s", wantValid, err, raw)
		}
	}
	parse(t, base, true)
	mutations := []func(*privateProviderProbeArtifactV7){
		func(v *privateProviderProbeArtifactV7) { v.Witness.Transaction.NotificationCeilingPolicy = "UNKNOWN" },
		func(v *privateProviderProbeArtifactV7) { v.Witness.Composition.OtherNotifications = 1 },
		func(v *privateProviderProbeArtifactV7) {
			v.Witness.Transaction.ObservedNotificationCount = math.MaxUint64
		},
		func(v *privateProviderProbeArtifactV7) {
			v.Witness.Transaction.ObservedControlCount = 65
			v.Witness.Composition.ManagerMessages = 65
			v.Witness.Composition.AcceptedTerminalResponses = 65
		},
		func(v *privateProviderProbeArtifactV7) { v.Witness.Composition.ManagerBytes = 4097 },
		func(v *privateProviderProbeArtifactV7) { v.Witness.Composition.Outcome = "UNKNOWN" },
		func(v *privateProviderProbeArtifactV7) { v.Witness.Composition.AcceptedTerminalResponses = 0 },
		func(v *privateProviderProbeArtifactV7) { v.Witness.Composition.InvalidProtocolMessages = 1 },
		func(v *privateProviderProbeArtifactV7) {
			v.Witness.Cause = string(session.SessionPoisoned)
			v.Witness.Class = "TYPED_SESSION_RUNTIME_FAILURE"
		},
	}
	for i, mutate := range mutations {
		candidate := base
		witness := *base.Witness
		candidate.Witness = &witness
		mutate(&candidate)
		t.Run(string(rune('a'+i)), func(t *testing.T) { parse(t, candidate, false) })
	}
	parse(t, privateProviderProbeArtifactV7{Schema: privateProviderProbeSchemaV7, Status: "COMPLETE", Witness: &privateProviderProbeWitnessV7{}}, false)
}

func TestPrivateProviderProbeV7ManagerEarliestFailureWinsReverseDelivery(t *testing.T) {
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v7")
	composition := sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}
	later := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 0, 1)
	earlier := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecyclePoisoned, 0, 1)
	setProviderProbeV7CompletionSequence(t, &later, 2)
	setProviderProbeV7CompletionSequence(t, &earlier, 1)
	p.observeV7("PROVIDER_REQUEST", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), composition, true, later, true, 64, 4096, 1, 20)
	p.observeV7("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), composition, true, earlier, true, 64, 4096, 1, 10)
	if got := p.artifactV7(); got.Witness == nil || got.Witness.Phase != "DOCUMENT_SYMBOL" || got.Witness.Composition.ManagerBytes != 10 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V7_MANAGER_EARLIEST_REVERSE_DELIVERY artifact=%+v", got)
	}
}

func TestPrivateProviderProbeV7AbsentUnknownAndConcurrentAtomic(t *testing.T) {
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v7")
	if a := p.artifactV7(); a.Status != "INCOMPLETE" || a.Witness != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V7_ABSENT %+v", a)
	}
	if _, err := newPrivateProviderProbeVersion(probePath(t), "v8"); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V7_UNKNOWN accepted")
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(sequence uint64) {
			defer wg.Done()
			observation := providerProbeV7Observation(sessionruntime.ProviderTransactionLifecycleReady, 0, 1)
			observation.CompletionSequence = sequence
			p.observeV7("DOCUMENT_SYMBOL", "", "", sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, observation, true, 64, 4096, 1, int64(sequence*10))
		}(uint64(i + 1))
	}
	wg.Wait()
	a := p.artifactV7()
	if a.Status != "COMPLETE" || a.Witness == nil || a.Witness.Transaction.CompletionSequence != 1 || a.Witness.Composition.ManagerBytes != 10 || a.Witness.Composition.ManagerMessages != a.Witness.Transaction.ObservedControlCount {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V7_CONCURRENT %+v", a)
	}
}
