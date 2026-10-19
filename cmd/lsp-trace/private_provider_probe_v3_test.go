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
