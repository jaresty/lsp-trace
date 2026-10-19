package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func providerProbeV6Observation(policy sessionruntime.ProviderNotificationCeilingPolicy, notifications, controls uint64, lifecycle sessionruntime.ProviderTransactionLifecycle) sessionruntime.ProviderTransactionObservation {
	return sessionruntime.ProviderTransactionObservation{
		SelectedResponseObserved: notifications+controls > 0, SelectedResponseReadCompleted: notifications+controls > 0,
		ProviderMessagesProcessed:     notifications + controls,
		NotificationMessagesProcessed: notifications, ControlMessagesProcessed: controls,
		NotificationCeilingPolicy: policy, ConfiguredControlCeiling: 64, ConfiguredMaxBytes: 4096,
		TerminalPoisonObserved: lifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned,
		FailurePostEffect:      lifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned,
		RequestWriteCompleted:  true, TerminalLifecycle: lifecycle,
	}
}

func TestPrivateProviderProbeV6OptInAdmission(t *testing.T) {
	path := probePath(t)
	base := []string{"census", "--workspace", ".", "--server", "gopls", "--publication-root", probePath(t), "--machine"}
	for name, args := range map[string][]string{
		"without-path": append(append([]string{}, base...), "--private-provider-probe-byte-time-notifications"),
		"without-v6":   append(append([]string{}, base...), "--private-provider-probe-path", path, "--private-provider-probe-version", "v5", "--private-provider-probe-byte-time-notifications"),
		"duplicate":    append(append([]string{}, base...), "--private-provider-probe-path", path, "--private-provider-probe-version", "v6", "--private-provider-probe-byte-time-notifications", "--private-provider-probe-byte-time-notifications"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCensusCLIOptions(args); err == nil {
				t.Fatal("ASSERT_PROVIDER_PROBE_V6_OPT_IN_ADMISSION accepted invalid opt-in")
			}
		})
	}
	valid := append(append([]string{}, base...), "--private-provider-probe-path", path, "--private-provider-probe-version", "v6", "--private-provider-probe-byte-time-notifications")
	got, err := parseCensusCLIOptions(valid)
	if err != nil || !got.PrivateProviderProbeByteTimeNotifications || got.PrivateProviderProbeVersion != "v6" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_OPT_IN_ADMISSION got=%+v err=%v", got, err)
	}
	if strings.Contains(censusUsage, "private-provider-probe-byte-time-notifications") {
		t.Fatal("ASSERT_PROVIDER_PROBE_V6_OPT_IN_ADMISSION private flag disclosed")
	}
}

func TestPrivateProviderProbeV6CanonicalScalarPrivacyConservation(t *testing.T) {
	p, err := newPrivateProviderProbeVersion(probePath(t), "v6")
	if err != nil {
		t.Fatal(err)
	}
	composition := sessionruntime.ProviderMessageComposition{LogShowTelemetryNotifications: 65, AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, composition, true, 64, 4096, 66, 2048)
	p.observeV6(providerProbeV6Observation(sessionruntime.ProviderNotificationCeilingByteTimeBounded, 65, 1, sessionruntime.ProviderTransactionLifecycleReady), true)
	a := p.artifactV6()
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	parsed, err := parsePrivateProviderProbeV6(raw)
	if err != nil || !reflect.DeepEqual(parsed, a) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_CANONICAL_SCALAR_PRIVACY_CONSERVATION err=%v parsed=%+v raw=%s", err, parsed, raw)
	}
	if a.Transaction == nil || a.Transaction.ObservedNotificationCount != 65 || a.Transaction.ObservedControlCount != 1 || !a.Transaction.TerminalResponseObserved || !a.Transaction.ReadCompleted {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_CANONICAL_SCALAR_PRIVACY_CONSERVATION artifact=%+v", a)
	}
	forbidden := []string{"error", "method", "message", "payload", "path", "uri", "_id", "generation", "hash", "stderr", "source", "params", "result"}
	typ := reflect.TypeOf(privateProviderProbeTransactionV6{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name + " " + typ.Field(i).Tag.Get("json"))
		for _, word := range forbidden {
			if strings.Contains(name, word) {
				t.Fatalf("ASSERT_PROVIDER_PROBE_V6_CANONICAL_SCALAR_PRIVACY_CONSERVATION field=%s forbidden=%s", name, word)
			}
		}
	}
	for _, mutate := range []func(*privateProviderProbeArtifactV6){
		func(v *privateProviderProbeArtifactV6) { v.Transaction.NotificationCeilingPolicy = "UNKNOWN" },
		func(v *privateProviderProbeArtifactV6) { v.Transaction.ConfiguredControlCeiling = 0 },
		func(v *privateProviderProbeArtifactV6) { v.Transaction.ConfiguredBytes = 0 },
		func(v *privateProviderProbeArtifactV6) { v.Transaction.ObservedNotificationCount = math.MaxUint64 },
		func(v *privateProviderProbeArtifactV6) {
			v.Transaction.ReadCompleted = true
			v.Transaction.TerminalResponseObserved = false
		},
		func(v *privateProviderProbeArtifactV6) { v.Transaction.TerminalLifecycle = "UNKNOWN" },
	} {
		candidate := a
		transaction := *a.Transaction
		candidate.Transaction = &transaction
		mutate(&candidate)
		encoded, _ := json.Marshal(candidate)
		if _, err := parsePrivateProviderProbeV6(append(encoded, '\n')); err == nil {
			t.Fatal("ASSERT_PROVIDER_PROBE_V6_CANONICAL_SCALAR_PRIVACY_CONSERVATION accepted invalid candidate")
		}
	}
	absent, _ := newPrivateProviderProbeVersion(probePath(t), "v6")
	absentRaw, _ := json.Marshal(absent.artifactV6())
	if parsed, err := parsePrivateProviderProbeV6(append(absentRaw, '\n')); err != nil || parsed.Transaction != nil || parsed.Status != "INCOMPLETE" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_CANONICAL_SCALAR_PRIVACY_CONSERVATION absent err=%v parsed=%+v", err, parsed)
	}
}

func TestPrivateProviderProbeV6FirstFailureElseFirstCompleted(t *testing.T) {
	p, _ := newPrivateProviderProbeVersion(probePath(t), "v6")
	completed := providerProbeV6Observation(sessionruntime.ProviderNotificationCeilingConfiguredCount, 0, 1, sessionruntime.ProviderTransactionLifecycleReady)
	p.observeV3("DOCUMENT_SYMBOL", "", "", false, sessionruntime.ProviderMessageComposition{AcceptedTerminalResponses: 1, Outcome: sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse}, true, 64, 4096, 1, 10)
	p.observeV6(completed, true)
	failed := providerProbeV6Observation(sessionruntime.ProviderNotificationCeilingByteTimeBounded, 2, 0, sessionruntime.ProviderTransactionLifecyclePoisoned)
	failed.SelectedResponseObserved, failed.SelectedResponseReadCompleted = false, false
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), false, sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, 64, 4096, 2, 20)
	p.observeV6(failed, true)
	later := failed
	later.NotificationMessagesProcessed = 9
	p.observeV3("DOCUMENT_SYMBOL", "TYPED_SESSION_RUNTIME_FAILURE", string(session.SessionPoisoned), false, sessionruntime.ProviderMessageComposition{InvalidProtocolMessages: 1, Outcome: sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage}, true, 64, 4096, 9, 90)
	p.observeV6(later, true)
	a := p.artifactV6()
	if a.Work != 3 || a.Transaction == nil || a.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned || a.Transaction.ObservedNotificationCount != 2 || a.Composition == nil || a.Composition.Selection != "FIRST_FAILURE_OR_POISON" {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_FIRST_FAILURE_ELSE_FIRST_COMPLETED artifact=%+v", a)
	}
}
