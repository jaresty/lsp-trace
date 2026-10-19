package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func testProviderComposition(outcome sessionruntime.ProviderMessageOutcome) sessionruntime.ProviderMessageComposition {
	return sessionruntime.ProviderMessageComposition{ProgressNotifications: 1, AcceptedTerminalResponses: 1, Outcome: outcome}
}

func TestPrivateProviderProbeV3AdmissionAndRetention(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v3")
	if err != nil {
		t.Fatal(err)
	}
	first := testProviderComposition(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse)
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, first, true, 64, 4096, 2, 100)
	got := p.artifactV3()
	if got.Work != 1 || got.Composition == nil || got.Composition.Sequence != 1 || got.Composition.Selection != "FIRST_COMPLETED" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_FIRST: %+v", got)
	}
	exhausted := sessionruntime.ProviderMessageComposition{ProgressNotifications: 2, Outcome: sessionruntime.ProviderMessageOutcomeByteLimit}
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), false, exhausted, true, 64, 4096, 3, 4097)
	selected := p.artifactV3().Composition
	if selected == nil || selected.Sequence != 2 || selected.Selection != "FIRST_RESOURCE_EXHAUSTED" || selected.ProgressNotifications != 2 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_REPLACE: %+v", selected)
	}
	later := sessionruntime.ProviderMessageComposition{OtherNotifications: 9, Outcome: sessionruntime.ProviderMessageOutcomeMessageCeiling}
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.ResourceExhausted), false, later, true, 64, 4096, 64, 100)
	if !reflect.DeepEqual(selected, p.artifactV3().Composition) {
		t.Fatal("ASSERT_PROVIDER_PROBE_V3_EXHAUSTED_STABLE")
	}
}

func TestPrivateProviderProbeV3ConcurrentOneOperationOneWorkSequence(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v3")
	if err != nil {
		t.Fatal(err)
	}
	const operations = 64
	var wg sync.WaitGroup
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.observeV3("DOCUMENT_SYMBOL", "", "", false, testProviderComposition(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse), true, 64, 4096, 2, 100)
		}()
	}
	wg.Wait()
	got := p.artifactV3()
	if got.Work != operations || p.sequence != operations || got.Composition == nil || got.Composition.Sequence == 0 || got.Composition.Sequence > operations {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_ATOMIC work=%d sequence=%d composition=%+v", got.Work, p.sequence, got.Composition)
	}
}

func TestPrivateProviderProbeV3PrivacySurface(t *testing.T) {
	typ := reflect.TypeOf(privateProviderProbeCompositionV3{})
	for i := 0; i < typ.NumField(); i++ {
		kind := typ.Field(i).Type.Kind()
		if kind != reflect.Uint64 && kind != reflect.String {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V3_SCALAR_ONLY field=%s kind=%s", typ.Field(i).Name, kind)
		}
	}
	raw, _ := json.Marshal(privateProviderProbeCompositionV3{})
	for _, sentinel := range []string{"notification", "response", "payload", "method", "uri", "path", "content"} {
		if strings.Contains(strings.ToLower(string(raw)), `"`+sentinel+`"`) {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V3_RAW_FIELD %s", raw)
		}
	}
}

func providerProbeV3Artifact(c privateProviderProbeCompositionV3) privateProviderProbeArtifactV3 {
	counts := []privateProviderProbeCount{}
	if c.Failure != "" {
		counts = append(counts, privateProviderProbeCount{Cause: c.Failure, Count: 1})
	}
	return privateProviderProbeArtifactV3{
		Schema: privateProviderProbeSchemaV3, Status: "COMPLETE", CauseCounts: counts, Work: 1, Composition: &c,
	}
}

func providerProbeV3Composition(outcome sessionruntime.ProviderMessageOutcome, failure string) privateProviderProbeCompositionV3 {
	c := privateProviderProbeCompositionV3{
		Sequence: 1, Selection: "FIRST_COMPLETED", ManagerMessages: 1, ManagerBytes: 100,
		ConfiguredMaxMessages: 4, ConfiguredMaxBytes: 4096, Outcome: string(outcome), Failure: failure,
	}
	if failure == string(session.ResourceExhausted) {
		c.Selection = "FIRST_RESOURCE_EXHAUSTED"
	}
	switch outcome {
	case sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse:
		c.AcceptedTerminalResponses = 1
	case sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage:
		c.InvalidProtocolMessages = 1
		c.ManagerMessages = 0
	case sessionruntime.ProviderMessageOutcomeByteLimit:
		c.ManagerBytes = c.ConfiguredMaxBytes + 1
	case sessionruntime.ProviderMessageOutcomeMessageCeiling:
		c.ManagerMessages = c.ConfiguredMaxMessages
		c.OtherNotifications = c.ManagerMessages
	default:
		c.OtherNotifications = c.ManagerMessages
	}
	return c
}

func parseProviderProbeV3Artifact(a privateProviderProbeArtifactV3) error {
	raw, _ := json.Marshal(a)
	_, err := parsePrivateProviderProbeV3(append(raw, '\n'))
	return err
}

func TestPrivateProviderProbeV3OutcomeFailureMatrixAndAccounting(t *testing.T) {
	rows := []struct {
		name    string
		outcome sessionruntime.ProviderMessageOutcome
		failure string
	}{
		{name: "accepted success", outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse},
		{name: "accepted server error", outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse, failure: "SERVER_ERROR"},
		{name: "accepted decode failure", outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse, failure: "DECODE_FAILURE"},
		{name: "invalid protocol", outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage, failure: string(session.SessionPoisoned)},
		{name: "byte limit", outcome: sessionruntime.ProviderMessageOutcomeByteLimit, failure: string(session.ResourceExhausted)},
		{name: "message ceiling", outcome: sessionruntime.ProviderMessageOutcomeMessageCeiling, failure: string(session.ResourceExhausted)},
		{name: "cancelled", outcome: sessionruntime.ProviderMessageOutcomeCancelled, failure: string(session.RequestCancelled)},
		{name: "timed out", outcome: sessionruntime.ProviderMessageOutcomeTimedOut, failure: string(session.RequestTimeout)},
		{name: "read eof", outcome: sessionruntime.ProviderMessageOutcomeReadOrTransportFailure, failure: string(session.SessionCrashed)},
		{name: "transport normalized", outcome: sessionruntime.ProviderMessageOutcomeReadOrTransportFailure, failure: "OTHER_TYPED_FAILURE"},
		{name: "other typed", outcome: sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure, failure: string(session.SessionNotFound)},
		{name: "other normalized", outcome: sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure, failure: "OTHER_TYPED_FAILURE"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			c := providerProbeV3Composition(row.outcome, row.failure)
			if err := parseProviderProbeV3Artifact(providerProbeV3Artifact(c)); err != nil {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V3_OUTCOME_FAILURE_MATRIX: outcome=%s failure=%q err=%v", row.outcome, row.failure, err)
			}
		})
	}
}

func TestPrivateProviderProbeV3SelectionOutcomeFailureCrossProduct(t *testing.T) {
	outcomes := []sessionruntime.ProviderMessageOutcome{
		sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse,
		sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage,
		sessionruntime.ProviderMessageOutcomeByteLimit,
		sessionruntime.ProviderMessageOutcomeMessageCeiling,
		sessionruntime.ProviderMessageOutcomeCancelled,
		sessionruntime.ProviderMessageOutcomeTimedOut,
		sessionruntime.ProviderMessageOutcomeReadOrTransportFailure,
		sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure,
	}
	failures := []string{
		"", "PARSE_ERROR", "INVALID_REQUEST", "METHOD_NOT_FOUND", "INVALID_PARAMS", "INTERNAL_ERROR", "SERVER_ERROR", "OTHER", "DECODE_FAILURE",
		string(session.SessionNotFound), string(session.StaleGeneration), string(session.LifecycleConflict), string(session.SessionStopping), string(session.SessionPoisoned),
		string(session.ResourceExhausted), string(session.SessionCrashed), string(session.RequestCancelled), string(session.RequestTimeout), "OTHER_TYPED_FAILURE",
	}
	selections := []string{"FIRST_COMPLETED", "FIRST_RESOURCE_EXHAUSTED"}
	allowed := func(outcome sessionruntime.ProviderMessageOutcome, failure string) bool {
		switch outcome {
		case sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse:
			return failure == "" || failure == "PARSE_ERROR" || failure == "INVALID_REQUEST" || failure == "METHOD_NOT_FOUND" || failure == "INVALID_PARAMS" || failure == "INTERNAL_ERROR" || failure == "SERVER_ERROR" || failure == "OTHER" || failure == "DECODE_FAILURE"
		case sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage:
			return failure == string(session.SessionPoisoned)
		case sessionruntime.ProviderMessageOutcomeByteLimit, sessionruntime.ProviderMessageOutcomeMessageCeiling:
			return failure == string(session.ResourceExhausted)
		case sessionruntime.ProviderMessageOutcomeCancelled:
			return failure == string(session.RequestCancelled)
		case sessionruntime.ProviderMessageOutcomeTimedOut:
			return failure == string(session.RequestTimeout)
		case sessionruntime.ProviderMessageOutcomeReadOrTransportFailure:
			return failure == string(session.SessionCrashed) || failure == "OTHER_TYPED_FAILURE"
		case sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure:
			return failure == string(session.SessionNotFound) || failure == string(session.StaleGeneration) || failure == string(session.LifecycleConflict) || failure == string(session.SessionStopping) || failure == string(session.SessionPoisoned) || failure == string(session.ResourceExhausted) || failure == "OTHER_TYPED_FAILURE"
		}
		return false
	}
	for _, outcome := range outcomes {
		for _, failure := range failures {
			for _, selection := range selections {
				name := string(outcome) + "/" + failure + "/" + selection
				t.Run(name, func(t *testing.T) {
					c := providerProbeV3Composition(outcome, failure)
					c.Selection = selection
					err := parseProviderProbeV3Artifact(providerProbeV3Artifact(c))
					want := allowed(outcome, failure) && (selection == "FIRST_RESOURCE_EXHAUSTED") == (failure == string(session.ResourceExhausted))
					if (err == nil) != want {
						t.Fatalf("ASSERT_PROVIDER_PROBE_V3_EXHAUSTIVE_MATRIX: outcome=%s failure=%q selection=%s accepted=%v want=%v", outcome, failure, selection, err == nil, want)
					}
				})
			}
		}
	}
}

func TestPrivateProviderProbeV3OutcomeSpecificMutations(t *testing.T) {
	tests := []struct {
		name      string
		base      privateProviderProbeCompositionV3
		mutate    func(*privateProviderProbeArtifactV3)
		assertion string
	}{
		{name: "normal bucket mismatch", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeCancelled, string(session.RequestCancelled)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.OtherNotifications-- }, assertion: "ASSERT_PROVIDER_PROBE_V3_OUTCOME_ACCOUNTING"},
		{name: "byte crossing dispatched", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeByteLimit, string(session.ResourceExhausted)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.OtherNotifications = 1 }, assertion: "ASSERT_PROVIDER_PROBE_V3_OUTCOME_ACCOUNTING"},
		{name: "invalid after dispatched", base: func() privateProviderProbeCompositionV3 {
			c := providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage, string(session.SessionPoisoned))
			c.ManagerMessages = 2
			c.OtherNotifications = 2
			return c
		}(), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.InvalidProtocolMessages = 0 }, assertion: "ASSERT_PROVIDER_PROBE_V3_OUTCOME_ACCOUNTING"},
		{name: "invalid exactly one", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage, string(session.SessionPoisoned)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.InvalidProtocolMessages = 2 }, assertion: "ASSERT_PROVIDER_PROBE_V3_TERMINAL_LIMITS"},
		{name: "message ceiling equality", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeMessageCeiling, string(session.ResourceExhausted)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.ManagerMessages++ }, assertion: "ASSERT_PROVIDER_PROBE_V3_TERMINAL_LIMITS"},
		{name: "accepted bytes bound", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse, ""), mutate: func(a *privateProviderProbeArtifactV3) {
			a.Composition.ManagerBytes = a.Composition.ConfiguredMaxBytes + 1
		}, assertion: "ASSERT_PROVIDER_PROBE_V3_TERMINAL_LIMITS"},
		{name: "resource selection reverse", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeByteLimit, string(session.ResourceExhausted)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.Selection = "FIRST_COMPLETED" }, assertion: "ASSERT_PROVIDER_PROBE_V3_SELECTION_LINKAGE"},
		{name: "nonresource exhausted selection", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeCancelled, string(session.RequestCancelled)), mutate: func(a *privateProviderProbeArtifactV3) { a.Composition.Selection = "FIRST_RESOURCE_EXHAUSTED" }, assertion: "ASSERT_PROVIDER_PROBE_V3_SELECTION_LINKAGE"},
		{name: "selected cause absent", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeCancelled, string(session.RequestCancelled)), mutate: func(a *privateProviderProbeArtifactV3) { a.CauseCounts = nil }, assertion: "ASSERT_PROVIDER_PROBE_V3_SELECTION_LINKAGE"},
		{name: "wrong normalized cause", base: providerProbeV3Composition(sessionruntime.ProviderMessageOutcomeByteLimit, string(session.ResourceExhausted)), mutate: func(a *privateProviderProbeArtifactV3) {
			a.Composition.Failure = string(session.SessionPoisoned)
			a.CauseCounts[0].Cause = string(session.SessionPoisoned)
		}, assertion: "ASSERT_PROVIDER_PROBE_V3_OUTCOME_FAILURE_MATRIX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := providerProbeV3Artifact(tc.base)
			if err := parseProviderProbeV3Artifact(a); err != nil {
				t.Fatalf("%s_VALID_BASE: %v", tc.assertion, err)
			}
			tc.mutate(&a)
			if err := parseProviderProbeV3Artifact(a); err == nil {
				t.Fatalf("%s: mutation accepted", tc.assertion)
			}
		})
	}
}

func TestPrivateProviderProbeV3CanonicalIncomplete(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v3")
	if err != nil {
		t.Fatal(err)
	}
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, sessionruntime.ProviderMessageComposition{}, false, 64, 4096, 0, 0)
	a := p.artifactV3()
	if a.Status != "INCOMPLETE" || a.Composition != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_INCOMPLETE_REACHABLE: %+v", a)
	}
	if err := parseProviderProbeV3Artifact(a); err != nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_INCOMPLETE_REACHABLE: %v", err)
	}
	a.Status = "COMPLETE"
	if err := parseProviderProbeV3Artifact(a); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V3_INCOMPLETE_REACHABLE: complete accepted without composition")
	}
}

func TestPrivateProviderProbeV3CanonicalParser(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v3")
	if err != nil {
		t.Fatal(err)
	}
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, testProviderComposition(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse), true, 64, 4096, 2, 100)
	a := p.artifactV3()
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	const golden = "{\"schema\":\"lsp-trace.private-provider-probe.v3\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[],\"records\":null,\"rerun_count\":0,\"work\":1,\"message_composition\":{\"sequence\":1,\"selection\":\"FIRST_COMPLETED\",\"progress_notifications\":1,\"log_show_telemetry_notifications\":0,\"other_notifications\":0,\"server_requests\":0,\"unmatched_late_invalid_responses\":0,\"accepted_terminal_responses\":1,\"invalid_protocol_messages\":0,\"manager_messages\":2,\"manager_bytes\":100,\"configured_max_messages\":64,\"configured_max_bytes\":4096,\"outcome\":\"ACCEPTED_TERMINAL_RESPONSE\",\"failure\":\"\"}}\n"
	if string(raw) != golden {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_GOLDEN\n got=%q\nwant=%q", raw, golden)
	}
	if parsed, err := parsePrivateProviderProbeV3(raw); err != nil || !reflect.DeepEqual(parsed, a) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_CANONICAL parsed=%+v err=%v", parsed, err)
	}
	mutations := []func(*privateProviderProbeArtifactV3){
		func(v *privateProviderProbeArtifactV3) { v.Composition.Sequence = v.Work + 1 },
		func(v *privateProviderProbeArtifactV3) { v.Composition.AcceptedTerminalResponses = 2 },
		func(v *privateProviderProbeArtifactV3) { v.Composition.Outcome = "UNKNOWN" },
		func(v *privateProviderProbeArtifactV3) { v.Composition.ManagerMessages = 0 },
		func(v *privateProviderProbeArtifactV3) {
			v.Composition.Outcome = string(sessionruntime.ProviderMessageOutcomeByteLimit)
			v.Composition.ManagerBytes = v.Composition.ConfiguredMaxBytes
		},
	}
	for i, mutate := range mutations {
		v := a
		c := *a.Composition
		v.Composition = &c
		mutate(&v)
		candidate, _ := json.Marshal(v)
		if _, err := parsePrivateProviderProbeV3(append(candidate, '\n')); err == nil {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V3_MUTATION_%d", i)
		}
	}
	if _, err := parsePrivateProviderProbeV3(raw[:len(raw)-1]); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V3_NEWLINE")
	}
}
