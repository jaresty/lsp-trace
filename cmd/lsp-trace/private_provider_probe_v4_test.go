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

func providerProbeV4Artifact(c privateProviderProbeCompositionV4) privateProviderProbeArtifactV4 {
	counts := []privateProviderProbeCount{}
	if c.Failure != "" {
		counts = append(counts, privateProviderProbeCount{Cause: c.Failure, Count: 1})
	}
	return privateProviderProbeArtifactV4{Schema: privateProviderProbeSchemaV4, Status: "COMPLETE", CauseCounts: counts, Work: 1, Composition: &c}
}

func validProviderProbeV4Composition() privateProviderProbeCompositionV4 {
	return privateProviderProbeCompositionV4{Sequence: 1, Selection: "FIRST_COMPLETED", PublishDiagnosticsNotifications: 1, OtherNotifications: 1, AcceptedTerminalResponses: 1, ManagerMessages: 3, ManagerBytes: 100, ConfiguredMaxMessages: 64, ConfiguredMaxBytes: 4096, Outcome: string(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse)}
}

func TestPrivateProviderProbeV4CanonicalGoldenAndInvalidCases(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v4")
	if err != nil {
		t.Fatal(err)
	}
	c := sessionruntime.ProviderMessageComposition{PublishDiagnosticsNotifications: 1, OtherNotifications: 1, AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, c, true, 64, 4096, 3, 100)
	a := p.artifactV4()
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	const golden = "{\"schema\":\"lsp-trace.private-provider-probe.v4\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[],\"records\":null,\"rerun_count\":0,\"work\":1,\"message_composition\":{\"sequence\":1,\"selection\":\"FIRST_COMPLETED\",\"progress_notifications\":0,\"log_show_telemetry_notifications\":0,\"publish_diagnostics_notifications\":1,\"other_notifications\":1,\"server_requests\":0,\"unmatched_late_invalid_responses\":0,\"accepted_terminal_responses\":1,\"invalid_protocol_messages\":0,\"manager_messages\":3,\"manager_bytes\":100,\"configured_max_messages\":64,\"configured_max_bytes\":4096,\"outcome\":\"ACCEPTED_TERMINAL_RESPONSE\",\"failure\":\"\"}}\n"
	if string(raw) != golden {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V4_GOLDEN got=%q want=%q", raw, golden)
	}
	if got, err := parsePrivateProviderProbeV4(raw); err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V4_CANONICAL got=%+v err=%v", got, err)
	}
	for _, bad := range [][]byte{nil, {}, raw[:len(raw)-1], append(append([]byte{}, raw...), '\n')} {
		if _, err := parsePrivateProviderProbeV4(bad); err == nil {
			t.Fatal("ASSERT_PROVIDER_PROBE_V4_INVALID_FRAMING")
		}
	}
	v := a
	cc := *a.Composition
	v.Composition = &cc
	v.Composition.OtherNotifications++
	candidate, _ := json.Marshal(v)
	if _, err := parsePrivateProviderProbeV4(append(candidate, '\n')); err == nil {
		t.Fatal("ASSERT_PROVIDER_PROBE_V4_CATEGORY_SUM")
	}
}

func TestPrivateProviderProbeV4OutcomeFailureAccountingAndConcurrentCustody(t *testing.T) {
	base := validProviderProbeV4Composition()
	if raw, _ := json.Marshal(providerProbeV4Artifact(base)); func() bool { _, err := parsePrivateProviderProbeV4(append(raw, '\n')); return err != nil }() {
		t.Fatal("ASSERT_PROVIDER_PROBE_V4_OUTCOME_ACCOUNTING")
	}
	p, err := newPrivateProviderProbeVersion(probePath(t), "v4")
	if err != nil {
		t.Fatal(err)
	}
	const n = 32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			failure := ""
			outcome := sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse
			if i == 7 {
				failure = string(session.ResourceExhausted)
				outcome = sessionruntime.ProviderMessageOutcomeByteLimit
			}
			p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", failure, false, sessionruntime.ProviderMessageComposition{OtherNotifications: 1, Outcome: outcome}, true, 64, 4096, 2, func() int64 {
				if failure != "" {
					return 4097
				}
				return 100
			}())
		}(i)
	}
	wg.Wait()
	got := p.artifactV4()
	if got.Work != n || p.sequence != n || got.Composition == nil || got.Composition.Selection != "FIRST_RESOURCE_EXHAUSTED" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V4_CONCURRENT_CUSTODY %+v", got)
	}
}

func TestPrivateProviderProbeV4PrivacyAndV3BytesUnchanged(t *testing.T) {
	typ := reflect.TypeOf(privateProviderProbeCompositionV4{})
	for i := 0; i < typ.NumField(); i++ {
		k := typ.Field(i).Type.Kind()
		if k != reflect.Uint64 && k != reflect.String {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V4_SCALAR_ONLY %s", typ.Field(i).Name)
		}
	}
	raw, _ := json.Marshal(privateProviderProbeCompositionV4{})
	for _, sentinel := range []string{"payload", "method", "uri", "path", "content", "hash", "id"} {
		if strings.Contains(strings.ToLower(string(raw)), `"`+sentinel+`"`) {
			t.Fatalf("ASSERT_PROVIDER_PROBE_V4_RAW_FIELD %s", raw)
		}
	}
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v3")
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, sessionruntime.ProviderMessageComposition{PublishDiagnosticsNotifications: 1, AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, 64, 4096, 2, 100)
	v3raw, _ := json.Marshal(p.artifactV3())
	const want = "{\"schema\":\"lsp-trace.private-provider-probe.v3\",\"status\":\"COMPLETE\",\"file_denominator\":0,\"cause_counts\":[],\"records\":null,\"rerun_count\":0,\"work\":1,\"message_composition\":{\"sequence\":1,\"selection\":\"FIRST_COMPLETED\",\"progress_notifications\":0,\"log_show_telemetry_notifications\":0,\"other_notifications\":1,\"server_requests\":0,\"unmatched_late_invalid_responses\":0,\"accepted_terminal_responses\":1,\"invalid_protocol_messages\":0,\"manager_messages\":2,\"manager_bytes\":100,\"configured_max_messages\":64,\"configured_max_bytes\":4096,\"outcome\":\"ACCEPTED_TERMINAL_RESPONSE\",\"failure\":\"\"}}"
	if string(v3raw) != want {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V3_UNCHANGED_BYTES got=%q", v3raw)
	}
}
