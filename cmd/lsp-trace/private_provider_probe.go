package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const (
	privateProviderProbeSchema   = "lsp-trace.private-provider-probe.v1"
	privateProviderProbeSchemaV2 = "lsp-trace.private-provider-probe.v2"
	privateProviderProbeSchemaV3 = "lsp-trace.private-provider-probe.v3"
	privateProviderProbeSchemaV4 = "lsp-trace.private-provider-probe.v4"
	privateProviderProbeSchemaV5 = "lsp-trace.private-provider-probe.v5"
	privateProviderProbeSchemaV6 = "lsp-trace.private-provider-probe.v6"
	privateProviderProbeSchemaV7 = "lsp-trace.private-provider-probe.v7"
	privateProviderProbeSchemaV8 = "lsp-trace.private-provider-probe.v8"
)

type privateProviderProbeRecord struct {
	Sequence uint64 `json:"sequence"`
	Phase    string `json:"phase"`
	Class    string `json:"class"`
	Cause    string `json:"cause"`
}
type privateProviderProbeCount struct {
	Cause string `json:"cause"`
	Count uint64 `json:"count"`
}
type privateProviderProbeArtifact struct {
	Schema          string                       `json:"schema"`
	Status          string                       `json:"status"`
	FileDenominator uint64                       `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount  `json:"cause_counts"`
	Records         []privateProviderProbeRecord `json:"records"`
	RerunCount      uint64                       `json:"rerun_count"`
	Work            uint64                       `json:"work"`
}

type privateProviderProbeExhaustionV2 struct {
	ObservedMessages      int    `json:"observed_messages"`
	ObservedBytes         int64  `json:"observed_bytes"`
	ConfiguredMaxMessages int    `json:"configured_max_messages"`
	ConfiguredMaxBytes    int64  `json:"configured_max_bytes"`
	Boundary              string `json:"boundary"`
}
type privateProviderProbeRecordV2 struct {
	Sequence   uint64                            `json:"sequence"`
	Phase      string                            `json:"phase"`
	Class      string                            `json:"class"`
	Cause      string                            `json:"cause"`
	Exhaustion *privateProviderProbeExhaustionV2 `json:"exhaustion,omitempty"`
}
type privateProviderProbeArtifactV2 struct {
	Schema          string                         `json:"schema"`
	Status          string                         `json:"status"`
	FileDenominator uint64                         `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount    `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2 `json:"records"`
	RerunCount      uint64                         `json:"rerun_count"`
	Work            uint64                         `json:"work"`
}
type privateProviderProbeCompositionV3 struct {
	Sequence                      uint64 `json:"sequence"`
	Selection                     string `json:"selection"`
	ProgressNotifications         uint64 `json:"progress_notifications"`
	LogShowTelemetryNotifications uint64 `json:"log_show_telemetry_notifications"`
	OtherNotifications            uint64 `json:"other_notifications"`
	ServerRequests                uint64 `json:"server_requests"`
	UnmatchedLateInvalidResponses uint64 `json:"unmatched_late_invalid_responses"`
	AcceptedTerminalResponses     uint64 `json:"accepted_terminal_responses"`
	InvalidProtocolMessages       uint64 `json:"invalid_protocol_messages"`
	ManagerMessages               uint64 `json:"manager_messages"`
	ManagerBytes                  uint64 `json:"manager_bytes"`
	ConfiguredMaxMessages         uint64 `json:"configured_max_messages"`
	ConfiguredMaxBytes            uint64 `json:"configured_max_bytes"`
	Outcome                       string `json:"outcome"`
	Failure                       string `json:"failure"`
}
type privateProviderProbeArtifactV3 struct {
	Schema          string                             `json:"schema"`
	Status          string                             `json:"status"`
	FileDenominator uint64                             `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount        `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2     `json:"records"`
	RerunCount      uint64                             `json:"rerun_count"`
	Work            uint64                             `json:"work"`
	Composition     *privateProviderProbeCompositionV3 `json:"message_composition,omitempty"`
}
type privateProviderProbeCompositionV4 struct {
	Sequence                        uint64 `json:"sequence"`
	Selection                       string `json:"selection"`
	ProgressNotifications           uint64 `json:"progress_notifications"`
	LogShowTelemetryNotifications   uint64 `json:"log_show_telemetry_notifications"`
	PublishDiagnosticsNotifications uint64 `json:"publish_diagnostics_notifications"`
	OtherNotifications              uint64 `json:"other_notifications"`
	ServerRequests                  uint64 `json:"server_requests"`
	UnmatchedLateInvalidResponses   uint64 `json:"unmatched_late_invalid_responses"`
	AcceptedTerminalResponses       uint64 `json:"accepted_terminal_responses"`
	InvalidProtocolMessages         uint64 `json:"invalid_protocol_messages"`
	ManagerMessages                 uint64 `json:"manager_messages"`
	ManagerBytes                    uint64 `json:"manager_bytes"`
	ConfiguredMaxMessages           uint64 `json:"configured_max_messages"`
	ConfiguredMaxBytes              uint64 `json:"configured_max_bytes"`
	Outcome                         string `json:"outcome"`
	Failure                         string `json:"failure"`
}
type privateProviderProbeArtifactV4 struct {
	Schema          string                             `json:"schema"`
	Status          string                             `json:"status"`
	FileDenominator uint64                             `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount        `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2     `json:"records"`
	RerunCount      uint64                             `json:"rerun_count"`
	Work            uint64                             `json:"work"`
	Composition     *privateProviderProbeCompositionV4 `json:"message_composition,omitempty"`
}

// privateProviderProbeTransactionV5 reports Manager-side transaction facts.
// SelectedResponseReadCompleted never claims provider write completion.
type privateProviderProbeTransactionV5 struct {
	SelectedResponseObserved      bool                                        `json:"selected_response_observed"`
	RequestWriteCompleted         bool                                        `json:"request_write_completed"`
	ProviderMessagesProcessed     uint64                                      `json:"provider_messages_processed"`
	SelectedResponseReadCompleted bool                                        `json:"selected_response_read_completed"`
	FailurePostEffect             bool                                        `json:"failure_post_effect"`
	TerminalPoisonObserved        bool                                        `json:"terminal_poison_observed"`
	TerminalLifecycle             sessionruntime.ProviderTransactionLifecycle `json:"terminal_lifecycle"`
}
type privateProviderProbeArtifactV5 struct {
	Schema          string                             `json:"schema"`
	Status          string                             `json:"status"`
	FileDenominator uint64                             `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount        `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2     `json:"records"`
	RerunCount      uint64                             `json:"rerun_count"`
	Work            uint64                             `json:"work"`
	Composition     *privateProviderProbeCompositionV4 `json:"message_composition,omitempty"`
	Transaction     *privateProviderProbeTransactionV5 `json:"document_symbol_transaction,omitempty"`
}
type privateProviderProbeTransactionV6 struct {
	NotificationCeilingPolicy sessionruntime.ProviderNotificationCeilingPolicy `json:"notification_ceiling_policy"`
	ConfiguredControlCeiling  uint64                                           `json:"configured_control_ceiling"`
	ConfiguredBytes           uint64                                           `json:"configured_bytes"`
	ObservedNotificationCount uint64                                           `json:"observed_notification_count"`
	ObservedControlCount      uint64                                           `json:"observed_control_count"`
	TerminalResponseObserved  bool                                             `json:"terminal_response_observed"`
	ReadCompleted             bool                                             `json:"read_completed"`
	TerminalLifecycle         sessionruntime.ProviderTransactionLifecycle      `json:"terminal_lifecycle"`
}

type privateProviderProbeArtifactV6 struct {
	Schema          string                             `json:"schema"`
	Status          string                             `json:"status"`
	FileDenominator uint64                             `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount        `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2     `json:"records"`
	RerunCount      uint64                             `json:"rerun_count"`
	Work            uint64                             `json:"work"`
	Composition     *privateProviderProbeCompositionV4 `json:"message_composition,omitempty"`
	Transaction     *privateProviderProbeTransactionV6 `json:"document_symbol_transaction,omitempty"`
}

type privateProviderProbeTransactionV7 struct {
	CompletionSequence        uint64                                           `json:"completion_sequence"`
	NotificationCeilingPolicy sessionruntime.ProviderNotificationCeilingPolicy `json:"notification_ceiling_policy"`
	ConfiguredControlCeiling  uint64                                           `json:"configured_control_ceiling"`
	ConfiguredBytes           uint64                                           `json:"configured_bytes"`
	ObservedNotificationCount uint64                                           `json:"observed_notification_count"`
	ObservedControlCount      uint64                                           `json:"observed_control_count"`
	TerminalResponseObserved  bool                                             `json:"terminal_response_observed"`
	ReadCompleted             bool                                             `json:"read_completed"`
	TerminalLifecycle         sessionruntime.ProviderTransactionLifecycle      `json:"terminal_lifecycle"`
}

type privateProviderProbeWitnessV7 struct {
	Phase       string                            `json:"phase"`
	Class       string                            `json:"class"`
	Cause       string                            `json:"cause"`
	Composition privateProviderProbeCompositionV4 `json:"message_composition"`
	Transaction privateProviderProbeTransactionV7 `json:"provider_transaction"`
}

type privateProviderProbeArtifactV7 struct {
	Schema  string                         `json:"schema"`
	Status  string                         `json:"status"`
	Witness *privateProviderProbeWitnessV7 `json:"witness,omitempty"`
}

type privateProviderProbeArtifactV8 struct {
	Schema                            string                         `json:"schema"`
	Status                            string                         `json:"status"`
	FirstCompleted                    *privateProviderProbeWitnessV7 `json:"first_completed"`
	FirstNonPoisonFailure             *privateProviderProbeWitnessV7 `json:"first_non_poison_failure"`
	FirstPoison                       *privateProviderProbeWitnessV7 `json:"first_poison"`
	PreProviderRefusalCount           uint64                         `json:"pre_provider_refusal_count"`
	MissingDispatchedObservationCount uint64                         `json:"missing_dispatched_observation_count"`
}

type privateProviderProbe struct {
	mu                      sync.Mutex
	path, version           string
	byteTimeNotifications   bool
	sequence, files, work   uint64
	counts                  map[string]uint64
	records                 []privateProviderProbeRecord
	recordsV2               []privateProviderProbeRecordV2
	compositionV3           *privateProviderProbeCompositionV3
	compositionV4           *privateProviderProbeCompositionV4
	transactionV5           *privateProviderProbeTransactionV5
	transactionV6           *privateProviderProbeTransactionV6
	witnessV7               *privateProviderProbeWitnessV7
	firstCompletedV8        *privateProviderProbeWitnessV7
	firstNonPoisonFailureV8 *privateProviderProbeWitnessV7
	firstPoisonV8           *privateProviderProbeWitnessV7
	preProviderRefusalsV8   uint64
	missingDispatchedV8     uint64
	incomplete              bool
}

func newPrivateProviderProbe(path string) (*privateProviderProbe, error) {
	return newPrivateProviderProbeVersion(path, "v1")
}
func newPrivateProviderProbeVersion(path, version string) (*privateProviderProbe, error) {
	if version != "v1" && version != "v2" && version != "v3" && version != "v4" && version != "v5" && version != "v6" && version != "v7" && version != "v8" {
		return nil, errors.New("invalid private provider probe version")
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("invalid private provider probe path")
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return nil, errors.New("private provider probe parent unavailable")
	}
	if info, err = os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 {
			return nil, errors.New("private provider probe unavailable")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("private provider probe unavailable")
	}
	return &privateProviderProbe{path: path, version: version, counts: make(map[string]uint64)}, nil
}
func checkedProbeAdd(a *uint64, n uint64) bool {
	if math.MaxUint64-*a < n {
		return false
	}
	*a += n
	return true
}
func (p *privateProviderProbe) observe(phase, class, cause string, file bool) {
	if p == nil || cause == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) {
		p.incomplete = true
		return
	}
	if file && !checkedProbeAdd(&p.files, 1) {
		p.incomplete = true
		return
	}
	if math.MaxUint64-p.counts[cause] < 1 {
		p.incomplete = true
		return
	}
	p.counts[cause]++
	wantV1 := phase == "DOCUMENT_SYMBOL" && !hasProbePhase(p.records, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhase(p.records, "DOCUMENT_SYMBOL") && !hasProbePhase(p.records, phase)
	if wantV1 && len(p.records) < 2 {
		p.records = append(p.records, privateProviderProbeRecord{p.sequence, phase, class, cause})
	}
	wantV2 := phase == "DOCUMENT_SYMBOL" && !hasProbePhaseV2(p.recordsV2, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhaseV2(p.recordsV2, "DOCUMENT_SYMBOL") && !hasProbePhaseV2(p.recordsV2, phase)
	if p.version == "v2" && wantV2 && len(p.recordsV2) < 2 {
		p.recordsV2 = append(p.recordsV2, privateProviderProbeRecordV2{Sequence: p.sequence, Phase: phase, Class: class, Cause: cause})
	}
}
func hasProbePhase(records []privateProviderProbeRecord, phase string) bool {
	for _, r := range records {
		if r.Phase == phase {
			return true
		}
	}
	return false
}
func hasProbePhaseV2(records []privateProviderProbeRecordV2, phase string) bool {
	for _, r := range records {
		if r.Phase == phase {
			return true
		}
	}
	return false
}
func privateProviderProbeBoundary(observedMessages int, observedBytes int64, configuredMaxMessages int, configuredMaxBytes int64) (string, error) {
	if observedMessages < 0 || observedBytes < 0 {
		return "INVALID_OBSERVATION", nil
	}
	if configuredMaxMessages <= 0 || configuredMaxBytes <= 0 {
		return "UNKNOWN", nil
	}
	if observedBytes > configuredMaxBytes {
		return "BYTE_LIMIT_EXCEEDED", nil
	}
	if observedMessages >= configuredMaxMessages {
		return "MESSAGE_CEILING_OBSERVED", nil
	}
	return "NEITHER_BOUNDARY_OBSERVED", nil
}
func (p *privateProviderProbe) observeExhaustion(phase, class, cause string, configuredMaxMessages int, configuredMaxBytes int64, result sessionruntime.RoundTripResult) {
	if p == nil || cause == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) {
		p.incomplete = true
		return
	}
	if math.MaxUint64-p.counts[cause] < 1 {
		p.incomplete = true
		return
	}
	p.counts[cause]++
	wantV1 := phase == "DOCUMENT_SYMBOL" && !hasProbePhase(p.records, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhase(p.records, "DOCUMENT_SYMBOL") && !hasProbePhase(p.records, phase)
	if wantV1 && len(p.records) < 2 {
		p.records = append(p.records, privateProviderProbeRecord{p.sequence, phase, class, cause})
	}
	wantV2 := phase == "DOCUMENT_SYMBOL" && !hasProbePhaseV2(p.recordsV2, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhaseV2(p.recordsV2, "DOCUMENT_SYMBOL") && !hasProbePhaseV2(p.recordsV2, phase)
	if !wantV2 || len(p.recordsV2) >= 2 {
		return
	}
	boundary, err := privateProviderProbeBoundary(result.Messages, result.Bytes, configuredMaxMessages, configuredMaxBytes)
	if err != nil {
		p.incomplete = true
		return
	}
	if boundary == "INVALID_OBSERVATION" {
		p.incomplete = true
	}
	p.recordsV2 = append(p.recordsV2, privateProviderProbeRecordV2{
		Sequence: p.sequence, Phase: phase, Class: class, Cause: cause,
		Exhaustion: &privateProviderProbeExhaustionV2{result.Messages, result.Bytes, configuredMaxMessages, configuredMaxBytes, boundary},
	})
}
func (p *privateProviderProbe) observeSuccess(file bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) || file && !checkedProbeAdd(&p.files, 1) {
		p.incomplete = true
	}
}
func (p *privateProviderProbe) observeV3(phase, class, cause string, file bool, c sessionruntime.ProviderMessageComposition, present bool, maxMessages int, maxBytes int64, messages int, bytes int64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) || file && !checkedProbeAdd(&p.files, 1) {
		p.incomplete = true
		return
	}
	if cause != "" {
		if math.MaxUint64-p.counts[cause] < 1 {
			p.incomplete = true
			return
		}
		p.counts[cause]++
		if phase == "DOCUMENT_SYMBOL" && !hasProbePhase(p.records, phase) {
			p.records = append(p.records, privateProviderProbeRecord{p.sequence, phase, class, cause})
		}
		if phase == "DOCUMENT_SYMBOL" && !hasProbePhaseV2(p.recordsV2, phase) {
			r := privateProviderProbeRecordV2{Sequence: p.sequence, Phase: phase, Class: class, Cause: cause}
			if cause == string(session.ResourceExhausted) {
				b, _ := privateProviderProbeBoundary(messages, bytes, maxMessages, maxBytes)
				r.Exhaustion = &privateProviderProbeExhaustionV2{messages, bytes, maxMessages, maxBytes, b}
			}
			p.recordsV2 = append(p.recordsV2, r)
		}
	}
	if p.version != "v3" && p.version != "v4" && p.version != "v5" && p.version != "v6" || phase != "DOCUMENT_SYMBOL" || !present || messages < 0 || bytes < 0 || maxMessages <= 0 || maxBytes <= 0 {
		if (p.version == "v3" || p.version == "v4" || p.version == "v5" || p.version == "v6") && phase == "DOCUMENT_SYMBOL" {
			p.incomplete = true
		}
		return
	}
	exhausted := cause == string(session.ResourceExhausted)
	failed := cause != ""
	if p.version == "v3" && p.compositionV3 != nil && (p.compositionV3.Selection == "FIRST_RESOURCE_EXHAUSTED" || !exhausted) || (p.version == "v4" || p.version == "v5") && p.compositionV4 != nil && (p.compositionV4.Selection == "FIRST_RESOURCE_EXHAUSTED" || !exhausted) || p.version == "v6" && p.compositionV4 != nil && (p.compositionV4.Selection == "FIRST_FAILURE_OR_POISON" || !failed) {
		return
	}
	selection := "FIRST_COMPLETED"
	if p.version == "v6" && failed {
		selection = "FIRST_FAILURE_OR_POISON"
	} else if exhausted {
		selection = "FIRST_RESOURCE_EXHAUSTED"
	}
	if p.version == "v4" || p.version == "v5" || p.version == "v6" {
		p.compositionV4 = &privateProviderProbeCompositionV4{p.sequence, selection, c.ProgressNotifications, c.LogShowTelemetryNotifications, c.PublishDiagnosticsNotifications, c.OtherNotifications, c.ServerRequests, c.UnmatchedLateInvalidResponses, c.AcceptedTerminalResponses, c.InvalidProtocolMessages, uint64(messages), uint64(bytes), uint64(maxMessages), uint64(maxBytes), string(c.Outcome), cause}
		return
	}
	p.compositionV3 = &privateProviderProbeCompositionV3{Sequence: p.sequence, Selection: selection, ProgressNotifications: c.ProgressNotifications, LogShowTelemetryNotifications: c.LogShowTelemetryNotifications, OtherNotifications: c.OtherNotifications + c.PublishDiagnosticsNotifications, ServerRequests: c.ServerRequests, UnmatchedLateInvalidResponses: c.UnmatchedLateInvalidResponses, AcceptedTerminalResponses: c.AcceptedTerminalResponses, InvalidProtocolMessages: c.InvalidProtocolMessages, ManagerMessages: uint64(messages), ManagerBytes: uint64(bytes), ConfiguredMaxMessages: uint64(maxMessages), ConfiguredMaxBytes: uint64(maxBytes), Outcome: string(c.Outcome), Failure: cause}
}

func (p *privateProviderProbe) observeV5(observation sessionruntime.ProviderTransactionObservation, present bool) {
	if p == nil || p.version != "v5" || !present {
		if p != nil && p.version == "v5" {
			p.mu.Lock()
			p.incomplete = true
			p.mu.Unlock()
		}
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if observation.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady && observation.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned ||
		observation.TerminalPoisonObserved != (observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned) ||
		observation.SelectedResponseReadCompleted && !observation.SelectedResponseObserved ||
		observation.FailurePostEffect && !observation.RequestWriteCompleted {
		p.incomplete = true
		return
	}
	poisoned := observation.TerminalPoisonObserved
	if p.transactionV5 != nil && (p.transactionV5.TerminalPoisonObserved || !poisoned) {
		return
	}
	p.transactionV5 = &privateProviderProbeTransactionV5{
		SelectedResponseObserved:      observation.SelectedResponseObserved,
		RequestWriteCompleted:         observation.RequestWriteCompleted,
		ProviderMessagesProcessed:     observation.ProviderMessagesProcessed,
		SelectedResponseReadCompleted: observation.SelectedResponseReadCompleted,
		FailurePostEffect:             observation.FailurePostEffect,
		TerminalPoisonObserved:        observation.TerminalPoisonObserved,
		TerminalLifecycle:             observation.TerminalLifecycle,
	}
}

func (p *privateProviderProbe) observeV6(observation sessionruntime.ProviderTransactionObservation, present bool) {
	if p == nil || p.version != "v6" || !present {
		if p != nil && p.version == "v6" {
			p.mu.Lock()
			p.incomplete = true
			p.mu.Unlock()
		}
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	total, ok := checkedProbeSum(observation.NotificationMessagesProcessed, observation.ControlMessagesProcessed)
	if !ok || total != observation.ProviderMessagesProcessed ||
		(observation.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingConfiguredCount && observation.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingByteTimeBounded) ||
		observation.ConfiguredControlCeiling == 0 || observation.ConfiguredMaxBytes == 0 ||
		observation.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady && observation.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned ||
		observation.TerminalPoisonObserved != (observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned) ||
		observation.SelectedResponseReadCompleted && !observation.SelectedResponseObserved {
		p.incomplete = true
		return
	}
	failed := observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned
	if p.transactionV6 != nil && (p.transactionV6.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned || !failed) {
		return
	}
	p.transactionV6 = &privateProviderProbeTransactionV6{
		NotificationCeilingPolicy: observation.NotificationCeilingPolicy,
		ConfiguredControlCeiling:  observation.ConfiguredControlCeiling,
		ConfiguredBytes:           observation.ConfiguredMaxBytes,
		ObservedNotificationCount: observation.NotificationMessagesProcessed,
		ObservedControlCount:      observation.ControlMessagesProcessed,
		TerminalResponseObserved:  observation.SelectedResponseObserved,
		ReadCompleted:             observation.SelectedResponseReadCompleted,
		TerminalLifecycle:         observation.TerminalLifecycle,
	}
}

func coherentPrivateProviderProbeV7Observation(observation sessionruntime.ProviderTransactionObservation) bool {
	total, ok := checkedProbeSum(observation.NotificationMessagesProcessed, observation.ControlMessagesProcessed)
	return ok && total == observation.ProviderMessagesProcessed && observation.ConfiguredControlCeiling != 0 && observation.ConfiguredMaxBytes != 0 &&
		(observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecycleReady || observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned) &&
		observation.TerminalPoisonObserved == (observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned) &&
		(!observation.SelectedResponseReadCompleted || observation.SelectedResponseObserved)
}

func (p *privateProviderProbe) observeV7(phase, class, cause string, c sessionruntime.ProviderMessageComposition, compositionPresent bool, observation sessionruntime.ProviderTransactionObservation, transactionPresent bool, maxMessages int, maxBytes int64, messages int, bytes int64) {
	if p == nil || p.version != "v7" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) {
		p.incomplete = true
		return
	}
	if !compositionPresent || !transactionPresent || messages < 0 || bytes < 0 || maxMessages <= 0 || maxBytes <= 0 {
		p.incomplete = true
		return
	}
	if !coherentPrivateProviderProbeV7Observation(observation) {
		p.incomplete = true
		return
	}
	if observation.CompletionSequence == 0 {
		p.incomplete = true
		return
	}
	failed := cause != "" || observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned
	if p.witnessV7 != nil {
		retainedFailed := p.witnessV7.Composition.Selection == "FIRST_FAILURE_OR_POISON"
		if retainedFailed && !failed || retainedFailed == failed && p.witnessV7.Transaction.CompletionSequence <= observation.CompletionSequence {
			return
		}
	}
	selection := "FIRST_COMPLETED"
	if failed {
		selection = "FIRST_FAILURE_OR_POISON"
	}
	p.witnessV7 = &privateProviderProbeWitnessV7{
		Phase: phase, Class: class, Cause: cause,
		Composition: privateProviderProbeCompositionV4{p.sequence, selection, c.ProgressNotifications, c.LogShowTelemetryNotifications, c.PublishDiagnosticsNotifications, c.OtherNotifications, c.ServerRequests, c.UnmatchedLateInvalidResponses, c.AcceptedTerminalResponses, c.InvalidProtocolMessages, uint64(messages), uint64(bytes), uint64(maxMessages), uint64(maxBytes), string(c.Outcome), cause},
		Transaction: privateProviderProbeTransactionV7{observation.CompletionSequence, observation.NotificationCeilingPolicy, observation.ConfiguredControlCeiling, observation.ConfiguredMaxBytes, observation.NotificationMessagesProcessed, observation.ControlMessagesProcessed, observation.SelectedResponseObserved, observation.SelectedResponseReadCompleted, observation.TerminalLifecycle},
	}
}

func (p *privateProviderProbe) observeV8(phase, class, cause string, c sessionruntime.ProviderMessageComposition, compositionPresent bool, observation sessionruntime.ProviderTransactionObservation, transactionPresent bool, maxMessages int, maxBytes int64, messages int, bytes int64) {
	if p == nil || p.version != "v8" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !transactionPresent {
		if cause == string(session.LifecycleConflict) {
			if !checkedProbeAdd(&p.preProviderRefusalsV8, 1) {
				p.incomplete = true
			}
			return
		}
		if !checkedProbeAdd(&p.missingDispatchedV8, 1) {
			p.incomplete = true
		}
		p.incomplete = true
		return
	}
	if !compositionPresent || observation.CompletionSequence == 0 || messages < 0 || bytes < 0 || maxMessages <= 0 || maxBytes <= 0 || !coherentPrivateProviderProbeV7Observation(observation) {
		p.incomplete = true
		return
	}
	selection := "FIRST_COMPLETED"
	if cause != "" {
		selection = "FIRST_FAILURE_OR_POISON"
	}
	w := &privateProviderProbeWitnessV7{Phase: phase, Class: class, Cause: cause,
		Composition: privateProviderProbeCompositionV4{Sequence: observation.CompletionSequence, Selection: selection, ProgressNotifications: c.ProgressNotifications, LogShowTelemetryNotifications: c.LogShowTelemetryNotifications, PublishDiagnosticsNotifications: c.PublishDiagnosticsNotifications, OtherNotifications: c.OtherNotifications, ServerRequests: c.ServerRequests, UnmatchedLateInvalidResponses: c.UnmatchedLateInvalidResponses, AcceptedTerminalResponses: c.AcceptedTerminalResponses, InvalidProtocolMessages: c.InvalidProtocolMessages, ManagerMessages: uint64(messages), ManagerBytes: uint64(bytes), ConfiguredMaxMessages: uint64(maxMessages), ConfiguredMaxBytes: uint64(maxBytes), Outcome: string(c.Outcome), Failure: cause},
		Transaction: privateProviderProbeTransactionV7{observation.CompletionSequence, observation.NotificationCeilingPolicy, observation.ConfiguredControlCeiling, observation.ConfiguredMaxBytes, observation.NotificationMessagesProcessed, observation.ControlMessagesProcessed, observation.SelectedResponseObserved, observation.SelectedResponseReadCompleted, observation.TerminalLifecycle}}
	retain := func(slot **privateProviderProbeWitnessV7) {
		if *slot == nil || (*slot).Transaction.CompletionSequence > observation.CompletionSequence {
			*slot = w
		}
	}
	retain(&p.firstCompletedV8)
	if observation.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned {
		retain(&p.firstPoisonV8)
	} else if cause != "" {
		retain(&p.firstNonPoisonFailureV8)
	}
}

func checkedProbeSum(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, false
	}
	return a + b, true
}

func (p *privateProviderProbe) artifact() privateProviderProbeArtifact {
	p.mu.Lock()
	defer p.mu.Unlock()
	causes := make([]string, 0, len(p.counts))
	for c := range p.counts {
		causes = append(causes, c)
	}
	sort.Strings(causes)
	counts := make([]privateProviderProbeCount, 0, len(causes))
	for _, c := range causes {
		counts = append(counts, privateProviderProbeCount{c, p.counts[c]})
	}
	status := "COMPLETE"
	if p.incomplete {
		status = "INCOMPLETE"
	}
	return privateProviderProbeArtifact{privateProviderProbeSchema, status, p.files, counts, append([]privateProviderProbeRecord(nil), p.records...), 0, p.work}
}
func (p *privateProviderProbe) artifactV2() privateProviderProbeArtifactV2 {
	base := p.artifact()
	p.mu.Lock()
	records := append([]privateProviderProbeRecordV2(nil), p.recordsV2...)
	p.mu.Unlock()
	return privateProviderProbeArtifactV2{privateProviderProbeSchemaV2, base.Status, base.FileDenominator, base.CauseCounts, records, base.RerunCount, base.Work}
}
func (p *privateProviderProbe) artifactV3() privateProviderProbeArtifactV3 {
	p.mu.Lock()
	defer p.mu.Unlock()
	causes := make([]string, 0, len(p.counts))
	for cause := range p.counts {
		causes = append(causes, cause)
	}
	sort.Strings(causes)
	counts := make([]privateProviderProbeCount, 0, len(causes))
	for _, cause := range causes {
		counts = append(counts, privateProviderProbeCount{cause, p.counts[cause]})
	}
	status := "COMPLETE"
	if p.incomplete {
		status = "INCOMPLETE"
	}
	var c *privateProviderProbeCompositionV3
	if p.compositionV3 != nil {
		copy := *p.compositionV3
		c = &copy
	}
	return privateProviderProbeArtifactV3{privateProviderProbeSchemaV3, status, p.files, counts, append([]privateProviderProbeRecordV2(nil), p.recordsV2...), 0, p.work, c}
}
func (p *privateProviderProbe) artifactV4() privateProviderProbeArtifactV4 {
	p.mu.Lock()
	defer p.mu.Unlock()
	causes := make([]string, 0, len(p.counts))
	for cause := range p.counts {
		causes = append(causes, cause)
	}
	sort.Strings(causes)
	counts := make([]privateProviderProbeCount, 0, len(causes))
	for _, cause := range causes {
		counts = append(counts, privateProviderProbeCount{cause, p.counts[cause]})
	}
	status := "COMPLETE"
	if p.incomplete {
		status = "INCOMPLETE"
	}
	var c *privateProviderProbeCompositionV4
	if p.compositionV4 != nil {
		copy := *p.compositionV4
		c = &copy
	}
	return privateProviderProbeArtifactV4{privateProviderProbeSchemaV4, status, p.files, counts, append([]privateProviderProbeRecordV2(nil), p.recordsV2...), 0, p.work, c}
}
func (p *privateProviderProbe) artifactV5() privateProviderProbeArtifactV5 {
	v4 := p.artifactV4()
	p.mu.Lock()
	var transaction *privateProviderProbeTransactionV5
	if p.transactionV5 != nil {
		copy := *p.transactionV5
		transaction = &copy
	}
	p.mu.Unlock()
	status := v4.Status
	if transaction == nil {
		status = "INCOMPLETE"
	}
	return privateProviderProbeArtifactV5{privateProviderProbeSchemaV5, status, v4.FileDenominator, v4.CauseCounts, v4.Records, v4.RerunCount, v4.Work, v4.Composition, transaction}
}

func (p *privateProviderProbe) artifactV6() privateProviderProbeArtifactV6 {
	v4 := p.artifactV4()
	p.mu.Lock()
	var transaction *privateProviderProbeTransactionV6
	if p.transactionV6 != nil {
		copy := *p.transactionV6
		transaction = &copy
	}
	p.mu.Unlock()
	status := v4.Status
	if transaction == nil {
		status = "INCOMPLETE"
	}
	return privateProviderProbeArtifactV6{privateProviderProbeSchemaV6, status, v4.FileDenominator, v4.CauseCounts, v4.Records, v4.RerunCount, v4.Work, v4.Composition, transaction}
}

func (p *privateProviderProbe) artifactV7() privateProviderProbeArtifactV7 {
	p.mu.Lock()
	defer p.mu.Unlock()
	status := "COMPLETE"
	if p.incomplete || p.witnessV7 == nil {
		status = "INCOMPLETE"
	}
	var witness *privateProviderProbeWitnessV7
	if p.witnessV7 != nil {
		copy := *p.witnessV7
		witness = &copy
	}
	return privateProviderProbeArtifactV7{privateProviderProbeSchemaV7, status, witness}
}

func (p *privateProviderProbe) artifactV8() privateProviderProbeArtifactV8 {
	p.mu.Lock()
	defer p.mu.Unlock()
	copyWitness := func(w *privateProviderProbeWitnessV7) *privateProviderProbeWitnessV7 {
		if w == nil {
			return nil
		}
		c := *w
		return &c
	}
	status := "COMPLETE"
	if p.incomplete || p.firstCompletedV8 == nil || p.firstNonPoisonFailureV8 == nil || p.firstPoisonV8 == nil || p.missingDispatchedV8 != 0 {
		status = "INCOMPLETE"
	}
	return privateProviderProbeArtifactV8{privateProviderProbeSchemaV8, status, copyWitness(p.firstCompletedV8), copyWitness(p.firstNonPoisonFailureV8), copyWitness(p.firstPoisonV8), p.preProviderRefusalsV8, p.missingDispatchedV8}
}

func (p *privateProviderProbe) finalize() error {
	if p == nil {
		return nil
	}
	var value any = p.artifact()
	if p.version == "v2" {
		value = p.artifactV2()
	} else if p.version == "v3" {
		value = p.artifactV3()
	} else if p.version == "v4" {
		value = p.artifactV4()
	} else if p.version == "v5" {
		value = p.artifactV5()
	} else if p.version == "v6" {
		value = p.artifactV6()
	} else if p.version == "v7" {
		value = p.artifactV7()
	} else if p.version == "v8" {
		value = p.artifactV8()
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	info, err := os.Lstat(filepath.Dir(p.path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errors.New("private provider probe parent unavailable")
	}
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("private provider probe unavailable")
	}
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if n, e := f.Write(raw); e != nil || n != len(raw) {
		return errors.New("private provider probe unavailable")
	}
	return f.Sync()
}
func parsePrivateProviderProbe(raw []byte) (privateProviderProbeArtifact, error) {
	var a privateProviderProbeArtifact
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil {
		return a, errors.New("invalid private provider probe")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return a, errors.New("invalid private provider probe")
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchema || (a.Status != "COMPLETE" && a.Status != "INCOMPLETE") || len(a.Records) > 2 || a.RerunCount != 0 || a.FileDenominator > a.Work {
		return a, errors.New("invalid private provider probe")
	}
	previousCause := ""
	for _, count := range a.CauseCounts {
		if count.Cause == "" || count.Count == 0 || previousCause >= count.Cause {
			return a, errors.New("invalid private provider probe")
		}
		previousCause = count.Cause
	}
	var previousSequence uint64
	for i, record := range a.Records {
		if record.Sequence == 0 || record.Sequence <= previousSequence || record.Sequence > a.Work || record.Cause == "" {
			return a, errors.New("invalid private provider probe")
		}
		if i == 0 && record.Phase != "DOCUMENT_SYMBOL" || i == 1 && record.Phase != "DOCUMENT_SUPPLY" {
			return a, errors.New("invalid private provider probe")
		}
		validClass := record.Class == "TYPED_SESSION_RUNTIME_FAILURE" || record.Class == "JSON_RPC_CODE_BUCKET" || record.Class == "DECODE_FAILURE" || record.Class == "DOCUMENT_FAILURE" || record.Class == "ADAPTER_IDENTITY_MISMATCH"
		if !validClass {
			return a, errors.New("invalid private provider probe")
		}
		previousSequence = record.Sequence
	}
	return a, nil
}

func validPrivateProviderProbePhaseClass(phase, class string) bool {
	switch phase {
	case "DOCUMENT_SYMBOL":
		return class == "TYPED_SESSION_RUNTIME_FAILURE" || class == "JSON_RPC_CODE_BUCKET" || class == "DECODE_FAILURE"
	case "DOCUMENT_SUPPLY":
		return class == "DOCUMENT_FAILURE" || class == "ADAPTER_IDENTITY_MISMATCH"
	}
	return false
}

func validPrivateProviderProbeCause(class, cause string) bool {
	switch class {
	case "TYPED_SESSION_RUNTIME_FAILURE":
		switch session.Failure(cause) {
		case session.SessionNotFound, session.StaleGeneration, session.LifecycleConflict, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout:
			return true
		}
		return cause == "OTHER_TYPED_FAILURE"
	case "JSON_RPC_CODE_BUCKET":
		switch cause {
		case "PARSE_ERROR", "INVALID_REQUEST", "METHOD_NOT_FOUND", "INVALID_PARAMS", "INTERNAL_ERROR", "SERVER_ERROR", "OTHER":
			return true
		}
	case "DECODE_FAILURE":
		return cause == "DECODE_FAILURE"
	case "DOCUMENT_FAILURE":
		switch session.Failure(cause) {
		case session.RequestCancelled, session.RequestTimeout, session.StaleGeneration, session.LifecycleConflict, session.ResourceExhausted, session.SessionPoisoned, sessionruntime.DocumentSupplyUnavailable, sessionruntime.DocumentURIUnavailable, sessionruntime.DocumentOutsideWorkspace, sessionruntime.DocumentSourceUnavailable, sessionruntime.LanguageIDUnavailable:
			return true
		}
		return cause == "OTHER_TYPED_FAILURE"
	case "ADAPTER_IDENTITY_MISMATCH":
		return cause == "ADAPTER_IDENTITY_MISMATCH"
	}
	return false
}

func knownPrivateProviderProbeCause(cause string) bool {
	for _, class := range []string{"TYPED_SESSION_RUNTIME_FAILURE", "JSON_RPC_CODE_BUCKET", "DECODE_FAILURE", "DOCUMENT_FAILURE", "ADAPTER_IDENTITY_MISMATCH"} {
		if validPrivateProviderProbeCause(class, cause) {
			return true
		}
	}
	return false
}

const (
	privateProviderProbeCauseRoundTripOnly = iota
	privateProviderProbeCauseSupplyOnly
	privateProviderProbeCauseShared
)

func privateProviderProbeCauseSchedule(cause string) int {
	roundTrip := validPrivateProviderProbeCause("TYPED_SESSION_RUNTIME_FAILURE", cause) ||
		validPrivateProviderProbeCause("JSON_RPC_CODE_BUCKET", cause) ||
		validPrivateProviderProbeCause("DECODE_FAILURE", cause)
	supply := validPrivateProviderProbeCause("DOCUMENT_FAILURE", cause) ||
		validPrivateProviderProbeCause("ADAPTER_IDENTITY_MISMATCH", cause)
	if roundTrip && supply {
		return privateProviderProbeCauseShared
	}
	if supply {
		return privateProviderProbeCauseSupplyOnly
	}
	return privateProviderProbeCauseRoundTripOnly
}

func parsePrivateProviderProbeV2(raw []byte) (privateProviderProbeArtifactV2, error) {
	var a privateProviderProbeArtifactV2
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return a, errors.New("invalid private provider probe")
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil {
		return a, errors.New("invalid private provider probe")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return a, errors.New("invalid private provider probe")
	}
	if a.CauseCounts == nil || len(a.Records) == 0 && a.Records != nil {
		return a, errors.New("invalid private provider probe")
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV2 || (a.Status != "COMPLETE" && a.Status != "INCOMPLETE") || a.Status == "INCOMPLETE" && a.Work == 0 || len(a.Records) > 2 || a.RerunCount != 0 || a.FileDenominator > a.Work {
		return a, errors.New("invalid private provider probe")
	}
	previousCause := ""
	causeCounts := make(map[string]uint64, len(a.CauseCounts))
	var totalCount, roundTripOnlyCount, supplyOnlyCount uint64
	for _, count := range a.CauseCounts {
		if !knownPrivateProviderProbeCause(count.Cause) || count.Count == 0 || count.Count > a.Work || previousCause >= count.Cause || math.MaxUint64-totalCount < count.Count {
			return a, errors.New("invalid private provider probe")
		}
		totalCount += count.Count
		if totalCount > a.Work {
			return a, errors.New("invalid private provider probe")
		}
		switch privateProviderProbeCauseSchedule(count.Cause) {
		case privateProviderProbeCauseRoundTripOnly:
			if math.MaxUint64-roundTripOnlyCount < count.Count {
				return a, errors.New("invalid private provider probe")
			}
			roundTripOnlyCount += count.Count
		case privateProviderProbeCauseSupplyOnly:
			if math.MaxUint64-supplyOnlyCount < count.Count {
				return a, errors.New("invalid private provider probe")
			}
			supplyOnlyCount += count.Count
		}
		causeCounts[count.Cause] = count.Count
		previousCause = count.Cause
	}
	if roundTripOnlyCount > a.Work-a.FileDenominator || supplyOnlyCount > a.FileDenominator {
		return a, errors.New("invalid private provider probe")
	}
	retainedCounts := make(map[string]uint64, len(a.Records))
	var previousSequence, retainedSymbolCount, retainedSupplyCount uint64
	invalidExhaustionWitness := false
	for i, record := range a.Records {
		if record.Sequence == 0 || record.Sequence <= previousSequence || record.Sequence > a.Work || record.Cause == "" {
			return a, errors.New("invalid private provider probe")
		}
		if i == 0 && record.Phase != "DOCUMENT_SYMBOL" || i == 1 && record.Phase != "DOCUMENT_SUPPLY" {
			return a, errors.New("invalid private provider probe")
		}
		if !validPrivateProviderProbePhaseClass(record.Phase, record.Class) || !validPrivateProviderProbeCause(record.Class, record.Cause) {
			return a, errors.New("invalid private provider probe")
		}
		if record.Phase == "DOCUMENT_SYMBOL" {
			retainedSymbolCount++
		} else {
			retainedSupplyCount++
		}
		retainedCounts[record.Cause]++
		if retainedCounts[record.Cause] > causeCounts[record.Cause] {
			return a, errors.New("invalid private provider probe")
		}
		wantExhaustion := record.Class == "TYPED_SESSION_RUNTIME_FAILURE" && record.Cause == string(session.ResourceExhausted)
		if wantExhaustion != (record.Exhaustion != nil) {
			return a, errors.New("invalid private provider probe")
		}
		if record.Exhaustion != nil {
			e := record.Exhaustion
			boundary, err := privateProviderProbeBoundary(e.ObservedMessages, e.ObservedBytes, e.ConfiguredMaxMessages, e.ConfiguredMaxBytes)
			if err != nil || e.Boundary != boundary {
				return a, errors.New("invalid private provider probe")
			}
			if boundary == "INVALID_OBSERVATION" {
				invalidExhaustionWitness = true
			}
		}
		previousSequence = record.Sequence
	}
	if retainedSymbolCount > a.Work-a.FileDenominator || retainedSupplyCount > a.FileDenominator {
		return a, errors.New("invalid private provider probe")
	}
	if invalidExhaustionWitness && a.Status != "INCOMPLETE" || a.Work != math.MaxUint64 && a.Status == "INCOMPLETE" && !invalidExhaustionWitness {
		return a, errors.New("invalid private provider probe")
	}
	return a, nil
}

func parsePrivateProviderProbeV3(raw []byte) (privateProviderProbeArtifactV3, error) {
	var a privateProviderProbeArtifactV3
	bad := func() (privateProviderProbeArtifactV3, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV3 {
		return bad()
	}
	v2 := privateProviderProbeArtifactV2{privateProviderProbeSchemaV2, a.Status, a.FileDenominator, a.CauseCounts, a.Records, a.RerunCount, a.Work}
	v2raw, _ := json.Marshal(v2)
	if _, err := parsePrivateProviderProbeV2(append(v2raw, '\n')); err != nil {
		if a.Status != "INCOMPLETE" {
			return bad()
		}
		v2.Status = "COMPLETE"
		v2raw, _ = json.Marshal(v2)
		if _, err = parsePrivateProviderProbeV2(append(v2raw, '\n')); err != nil {
			return bad()
		}
	}
	if a.Composition == nil {
		if a.Status != "INCOMPLETE" {
			return bad()
		}
		return a, nil
	}
	c := a.Composition
	if c.Sequence == 0 || c.Sequence > a.Work || c.Selection != "FIRST_COMPLETED" && c.Selection != "FIRST_RESOURCE_EXHAUSTED" || c.ConfiguredMaxMessages == 0 || c.ConfiguredMaxBytes == 0 {
		return bad()
	}
	var dispatched uint64
	for _, n := range []uint64{c.ProgressNotifications, c.LogShowTelemetryNotifications, c.OtherNotifications, c.ServerRequests, c.UnmatchedLateInvalidResponses, c.AcceptedTerminalResponses} {
		if math.MaxUint64-dispatched < n {
			return bad()
		}
		dispatched += n
	}
	causeCount := uint64(0)
	for _, count := range a.CauseCounts {
		if count.Cause == c.Failure {
			causeCount = count.Count
			break
		}
	}
	resourceExhausted := c.Failure == string(session.ResourceExhausted)
	if (c.Selection == "FIRST_RESOURCE_EXHAUSTED") != resourceExhausted || c.Failure != "" && causeCount == 0 {
		return bad()
	}
	normalTerminal := func(failure string) bool {
		return dispatched == c.ManagerMessages && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && c.ManagerBytes <= c.ConfiguredMaxBytes && c.ManagerMessages < c.ConfiguredMaxMessages && c.Failure == failure
	}
	valid := false
	switch sessionruntime.ProviderMessageOutcome(c.Outcome) {
	case sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse:
		acceptedCause := c.Failure == "" || validPrivateProviderProbeCause("DECODE_FAILURE", c.Failure) || validPrivateProviderProbeCause("JSON_RPC_CODE_BUCKET", c.Failure)
		valid = dispatched == c.ManagerMessages && c.AcceptedTerminalResponses == 1 && c.InvalidProtocolMessages == 0 && c.ManagerBytes <= c.ConfiguredMaxBytes && c.ManagerMessages <= c.ConfiguredMaxMessages && acceptedCause
	case sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage:
		valid = dispatched == c.ManagerMessages && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 1 && c.Failure == string(session.SessionPoisoned)
	case sessionruntime.ProviderMessageOutcomeByteLimit:
		valid = c.ManagerMessages > 0 && dispatched == c.ManagerMessages-1 && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && c.ManagerBytes > c.ConfiguredMaxBytes && c.Failure == string(session.ResourceExhausted)
	case sessionruntime.ProviderMessageOutcomeMessageCeiling:
		valid = dispatched == c.ManagerMessages && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && c.ManagerBytes <= c.ConfiguredMaxBytes && c.ManagerMessages == c.ConfiguredMaxMessages && c.Failure == string(session.ResourceExhausted)
	case sessionruntime.ProviderMessageOutcomeCancelled:
		valid = normalTerminal(string(session.RequestCancelled))
	case sessionruntime.ProviderMessageOutcomeTimedOut:
		valid = normalTerminal(string(session.RequestTimeout))
	case sessionruntime.ProviderMessageOutcomeReadOrTransportFailure:
		valid = normalTerminal(string(session.SessionCrashed)) || normalTerminal("OTHER_TYPED_FAILURE")
	case sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure:
		otherCause := validPrivateProviderProbeCause("TYPED_SESSION_RUNTIME_FAILURE", c.Failure) &&
			c.Failure != string(session.RequestCancelled) && c.Failure != string(session.RequestTimeout) && c.Failure != string(session.SessionCrashed)
		valid = otherCause && normalTerminal(c.Failure)
	}
	if !valid {
		return bad()
	}
	return a, nil
}

func parsePrivateProviderProbeV4(raw []byte) (privateProviderProbeArtifactV4, error) {
	var a privateProviderProbeArtifactV4
	bad := func() (privateProviderProbeArtifactV4, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV4 {
		return bad()
	}
	var c3 *privateProviderProbeCompositionV3
	if a.Composition != nil {
		c := a.Composition
		if math.MaxUint64-c.OtherNotifications < c.PublishDiagnosticsNotifications {
			return bad()
		}
		c3 = &privateProviderProbeCompositionV3{c.Sequence, c.Selection, c.ProgressNotifications, c.LogShowTelemetryNotifications, c.OtherNotifications + c.PublishDiagnosticsNotifications, c.ServerRequests, c.UnmatchedLateInvalidResponses, c.AcceptedTerminalResponses, c.InvalidProtocolMessages, c.ManagerMessages, c.ManagerBytes, c.ConfiguredMaxMessages, c.ConfiguredMaxBytes, c.Outcome, c.Failure}
	}
	v3 := privateProviderProbeArtifactV3{privateProviderProbeSchemaV3, a.Status, a.FileDenominator, a.CauseCounts, a.Records, a.RerunCount, a.Work, c3}
	v3raw, _ := json.Marshal(v3)
	if _, err := parsePrivateProviderProbeV3(append(v3raw, '\n')); err != nil {
		return bad()
	}
	return a, nil
}

func parsePrivateProviderProbeV5(raw []byte) (privateProviderProbeArtifactV5, error) {
	var a privateProviderProbeArtifactV5
	bad := func() (privateProviderProbeArtifactV5, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV5 || a.Transaction == nil && a.Status != "INCOMPLETE" {
		return bad()
	}
	v4 := privateProviderProbeArtifactV4{privateProviderProbeSchemaV4, a.Status, a.FileDenominator, a.CauseCounts, a.Records, a.RerunCount, a.Work, a.Composition}
	v4raw, _ := json.Marshal(v4)
	if _, err := parsePrivateProviderProbeV4(append(v4raw, '\n')); err != nil {
		return bad()
	}
	if a.Transaction == nil {
		return a, nil
	}
	t := a.Transaction
	if t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady && t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned ||
		t.TerminalPoisonObserved != (t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned) ||
		t.SelectedResponseReadCompleted && !t.SelectedResponseObserved ||
		t.FailurePostEffect && !t.RequestWriteCompleted {
		return bad()
	}
	return a, nil
}

func parsePrivateProviderProbeV6(raw []byte) (privateProviderProbeArtifactV6, error) {
	var a privateProviderProbeArtifactV6
	bad := func() (privateProviderProbeArtifactV6, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV6 || a.Transaction == nil && a.Status != "INCOMPLETE" {
		return bad()
	}
	v2 := privateProviderProbeArtifactV2{privateProviderProbeSchemaV2, a.Status, a.FileDenominator, a.CauseCounts, a.Records, a.RerunCount, a.Work}
	v2raw, _ := json.Marshal(v2)
	if _, err := parsePrivateProviderProbeV2(append(v2raw, '\n')); err != nil {
		if a.Status != "INCOMPLETE" {
			return bad()
		}
		v2.Status = "COMPLETE"
		v2raw, _ = json.Marshal(v2)
		if _, err = parsePrivateProviderProbeV2(append(v2raw, '\n')); err != nil {
			return bad()
		}
	}
	if a.Transaction == nil {
		if a.Composition != nil && a.Status != "INCOMPLETE" {
			return bad()
		}
		return a, nil
	}
	t := a.Transaction
	c := a.Composition
	total, ok := checkedProbeSum(t.ObservedNotificationCount, t.ObservedControlCount)
	if !ok || c == nil || c.Sequence == 0 || c.Sequence > a.Work || total != c.ManagerMessages ||
		c.ConfiguredMaxMessages == 0 || c.ConfiguredMaxBytes == 0 ||
		t.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingConfiguredCount && t.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingByteTimeBounded ||
		t.ConfiguredControlCeiling == 0 || t.ConfiguredBytes == 0 ||
		t.ConfiguredControlCeiling != c.ConfiguredMaxMessages || t.ConfiguredBytes != c.ConfiguredMaxBytes ||
		t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady && t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned ||
		t.ReadCompleted && !t.TerminalResponseObserved {
		return bad()
	}
	notifications, ok := checkedProbeSum(c.ProgressNotifications, c.LogShowTelemetryNotifications)
	if ok {
		notifications, ok = checkedProbeSum(notifications, c.PublishDiagnosticsNotifications)
	}
	if ok {
		notifications, ok = checkedProbeSum(notifications, c.OtherNotifications)
	}
	controls, controlsOK := checkedProbeSum(c.ServerRequests, c.UnmatchedLateInvalidResponses)
	if controlsOK {
		controls, controlsOK = checkedProbeSum(controls, c.AcceptedTerminalResponses)
	}
	if !ok || !controlsOK || notifications != t.ObservedNotificationCount || controls != t.ObservedControlCount {
		return bad()
	}
	if t.NotificationCeilingPolicy == sessionruntime.ProviderNotificationCeilingConfiguredCount && t.ObservedNotificationCount > t.ConfiguredControlCeiling || t.ObservedControlCount > t.ConfiguredControlCeiling {
		return bad()
	}
	if c.Selection == "FIRST_COMPLETED" {
		if t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady || c.Outcome != string(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse) || c.Failure != "" || c.AcceptedTerminalResponses != 1 || c.InvalidProtocolMessages != 0 || c.ManagerBytes > c.ConfiguredMaxBytes || !t.TerminalResponseObserved || !t.ReadCompleted {
			return bad()
		}
	} else if c.Selection == "FIRST_FAILURE_OR_POISON" {
		if t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned || c.Failure == "" || c.Outcome == string(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse) || c.AcceptedTerminalResponses != 0 {
			return bad()
		}
		switch sessionruntime.ProviderMessageOutcome(c.Outcome) {
		case sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage:
			if c.InvalidProtocolMessages != 1 {
				return bad()
			}
		case sessionruntime.ProviderMessageOutcomeByteLimit:
			if c.ManagerBytes <= c.ConfiguredMaxBytes {
				return bad()
			}
		case sessionruntime.ProviderMessageOutcomeMessageCeiling, sessionruntime.ProviderMessageOutcomeCancelled, sessionruntime.ProviderMessageOutcomeTimedOut, sessionruntime.ProviderMessageOutcomeReadOrTransportFailure, sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure:
		default:
			return bad()
		}
	} else {
		return bad()
	}
	return a, nil
}

func parsePrivateProviderProbeV7(raw []byte) (privateProviderProbeArtifactV7, error) {
	var a privateProviderProbeArtifactV7
	bad := func() (privateProviderProbeArtifactV7, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV7 || a.Status != "COMPLETE" && a.Status != "INCOMPLETE" || a.Witness == nil && a.Status != "INCOMPLETE" {
		return bad()
	}
	if a.Witness == nil {
		return a, nil
	}
	w := a.Witness
	if w.Phase != "DOCUMENT_SYMBOL" && w.Phase != "PROVIDER_REQUEST" || w.Composition.Selection != "FIRST_COMPLETED" && w.Composition.Selection != "FIRST_FAILURE_OR_POISON" || w.Composition.Sequence == 0 || w.Composition.ConfiguredMaxMessages == 0 || w.Composition.ConfiguredMaxBytes == 0 {
		return bad()
	}
	if w.Cause == "" != (w.Class == "") || w.Cause != "" && (!validPrivateProviderProbeCause(w.Class, w.Cause) || w.Class == "DOCUMENT_FAILURE" || w.Class == "ADAPTER_IDENTITY_MISMATCH") {
		return bad()
	}
	t := w.Transaction
	c := w.Composition
	total, ok := checkedProbeSum(t.ObservedNotificationCount, t.ObservedControlCount)
	if !ok || t.CompletionSequence == 0 || total != c.ManagerMessages || t.ConfiguredControlCeiling != c.ConfiguredMaxMessages || t.ConfiguredBytes != c.ConfiguredMaxBytes || t.ConfiguredControlCeiling == 0 || t.ConfiguredBytes == 0 ||
		t.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingConfiguredCount && t.NotificationCeilingPolicy != sessionruntime.ProviderNotificationCeilingByteTimeBounded ||
		t.ReadCompleted && !t.TerminalResponseObserved || t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady && t.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned {
		return bad()
	}
	notifications, notificationsOK := checkedProbeSum(c.ProgressNotifications, c.LogShowTelemetryNotifications)
	if notificationsOK {
		notifications, notificationsOK = checkedProbeSum(notifications, c.PublishDiagnosticsNotifications)
	}
	if notificationsOK {
		notifications, notificationsOK = checkedProbeSum(notifications, c.OtherNotifications)
	}
	controls, controlsOK := checkedProbeSum(c.ServerRequests, c.UnmatchedLateInvalidResponses)
	if controlsOK {
		controls, controlsOK = checkedProbeSum(controls, c.AcceptedTerminalResponses)
	}
	if !notificationsOK || !controlsOK || notifications != t.ObservedNotificationCount || controls != t.ObservedControlCount ||
		t.NotificationCeilingPolicy == sessionruntime.ProviderNotificationCeilingConfiguredCount && t.ObservedNotificationCount > t.ConfiguredControlCeiling || t.ObservedControlCount > t.ConfiguredControlCeiling {
		return bad()
	}
	failed := w.Cause != "" || t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned
	if (c.Selection == "FIRST_FAILURE_OR_POISON") != failed {
		return bad()
	}
	normalTerminal := func(cause string) bool {
		return c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && c.ManagerBytes <= c.ConfiguredMaxBytes && c.ManagerMessages < c.ConfiguredMaxMessages && w.Cause == cause
	}
	valid := false
	switch sessionruntime.ProviderMessageOutcome(c.Outcome) {
	case sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse:
		acceptedCause := w.Cause == "" || validPrivateProviderProbeCause("DECODE_FAILURE", w.Cause) || validPrivateProviderProbeCause("JSON_RPC_CODE_BUCKET", w.Cause)
		valid = t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecycleReady && c.AcceptedTerminalResponses == 1 && c.InvalidProtocolMessages == 0 && c.ManagerBytes <= c.ConfiguredMaxBytes && c.ManagerMessages <= c.ConfiguredMaxMessages && t.TerminalResponseObserved && t.ReadCompleted && acceptedCause
	case sessionruntime.ProviderMessageOutcomeInvalidProtocolMessage:
		valid = t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 1 && w.Cause == string(session.SessionPoisoned)
	case sessionruntime.ProviderMessageOutcomeByteLimit:
		valid = t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && c.ManagerBytes > c.ConfiguredMaxBytes && w.Cause == string(session.ResourceExhausted)
	case sessionruntime.ProviderMessageOutcomeMessageCeiling:
		valid = t.TerminalLifecycle == sessionruntime.ProviderTransactionLifecyclePoisoned && c.AcceptedTerminalResponses == 0 && c.InvalidProtocolMessages == 0 && (t.ObservedNotificationCount > t.ConfiguredControlCeiling || t.ObservedControlCount > t.ConfiguredControlCeiling) && w.Cause == string(session.ResourceExhausted)
	case sessionruntime.ProviderMessageOutcomeCancelled:
		valid = normalTerminal(string(session.RequestCancelled))
	case sessionruntime.ProviderMessageOutcomeTimedOut:
		valid = normalTerminal(string(session.RequestTimeout))
	case sessionruntime.ProviderMessageOutcomeReadOrTransportFailure:
		valid = normalTerminal(string(session.SessionCrashed)) || normalTerminal("OTHER_TYPED_FAILURE")
	case sessionruntime.ProviderMessageOutcomeOtherRuntimeFailure:
		valid = validPrivateProviderProbeCause("TYPED_SESSION_RUNTIME_FAILURE", w.Cause) && normalTerminal(w.Cause)
	}
	if !valid {
		return bad()
	}
	return a, nil
}

func parsePrivateProviderProbeV8(raw []byte) (privateProviderProbeArtifactV8, error) {
	var a privateProviderProbeArtifactV8
	bad := func() (privateProviderProbeArtifactV8, error) { return a, errors.New("invalid private provider probe") }
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return bad()
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || d.Decode(&struct{}{}) != io.EOF {
		return bad()
	}
	canonical, err := json.Marshal(a)
	if err != nil || !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV8 || a.Status != "COMPLETE" && a.Status != "INCOMPLETE" {
		return bad()
	}
	forbidden := []string{`"path":`, `"uri":`, `"session":`, `"source":`, `"content":`, `"request":`, `"params":`, `"frame":`, `"raw":`, `"error":`}
	lower := strings.ToLower(string(line))
	for _, field := range forbidden {
		if strings.Contains(lower, field) {
			return bad()
		}
	}
	validateWitness := func(w *privateProviderProbeWitnessV7) bool {
		if w == nil {
			return true
		}
		candidate := privateProviderProbeArtifactV7{Schema: privateProviderProbeSchemaV7, Status: "COMPLETE", Witness: w}
		encoded, marshalErr := json.Marshal(candidate)
		if marshalErr != nil {
			return false
		}
		_, parseErr := parsePrivateProviderProbeV7(append(encoded, '\n'))
		return parseErr == nil
	}
	if !validateWitness(a.FirstCompleted) || !validateWitness(a.FirstNonPoisonFailure) || !validateWitness(a.FirstPoison) {
		return bad()
	}
	allPresent := a.FirstCompleted != nil && a.FirstNonPoisonFailure != nil && a.FirstPoison != nil
	if a.Status == "COMPLETE" && (!allPresent || a.MissingDispatchedObservationCount != 0) {
		return bad()
	}
	if a.FirstNonPoisonFailure != nil && (a.FirstNonPoisonFailure.Cause == "" || a.FirstNonPoisonFailure.Composition.Selection != "FIRST_FAILURE_OR_POISON" || a.FirstNonPoisonFailure.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady) {
		return bad()
	}
	if a.FirstPoison != nil && (a.FirstPoison.Cause == "" || a.FirstPoison.Composition.Selection != "FIRST_FAILURE_OR_POISON" || a.FirstPoison.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned) {
		return bad()
	}
	sequences := make(map[uint64]privateProviderProbeWitnessV7, 3)
	for _, witness := range []*privateProviderProbeWitnessV7{a.FirstCompleted, a.FirstNonPoisonFailure, a.FirstPoison} {
		if witness == nil {
			continue
		}
		sequence := witness.Transaction.CompletionSequence
		if retained, duplicate := sequences[sequence]; duplicate && retained != *witness {
			return bad()
		}
		sequences[sequence] = *witness
	}
	if a.FirstCompleted != nil {
		firstSequence := a.FirstCompleted.Transaction.CompletionSequence
		for _, classified := range []*privateProviderProbeWitnessV7{a.FirstNonPoisonFailure, a.FirstPoison} {
			if classified == nil {
				continue
			}
			classifiedSequence := classified.Transaction.CompletionSequence
			if firstSequence > classifiedSequence || firstSequence == classifiedSequence && *a.FirstCompleted != *classified {
				return bad()
			}
		}
	}
	return a, nil
}

func privateSessionFailureCause(f session.Failure) (string, string) {
	switch f {
	case session.SessionNotFound, session.StaleGeneration, session.LifecycleConflict, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout:
		return "TYPED_SESSION_RUNTIME_FAILURE", string(f)
	default:
		return "TYPED_SESSION_RUNTIME_FAILURE", "OTHER_TYPED_FAILURE"
	}
}
func privateDocumentFailureCause(f session.Failure) (string, string) {
	switch f {
	case session.RequestCancelled, session.RequestTimeout, session.StaleGeneration, session.LifecycleConflict, session.ResourceExhausted, session.SessionPoisoned, sessionruntime.DocumentSupplyUnavailable, sessionruntime.DocumentURIUnavailable, sessionruntime.DocumentOutsideWorkspace, sessionruntime.DocumentSourceUnavailable, sessionruntime.LanguageIDUnavailable:
		return "DOCUMENT_FAILURE", string(f)
	default:
		return "DOCUMENT_FAILURE", "OTHER_TYPED_FAILURE"
	}
}
func privateRPCErrorCause(e *lspwire.RPCError) (string, string) {
	c := "OTHER"
	if e.Code == -32700 {
		c = "PARSE_ERROR"
	} else if e.Code == -32600 {
		c = "INVALID_REQUEST"
	} else if e.Code == -32601 {
		c = "METHOD_NOT_FOUND"
	} else if e.Code == -32602 {
		c = "INVALID_PARAMS"
	} else if e.Code == -32603 {
		c = "INTERNAL_ERROR"
	} else if e.Code >= -32099 && e.Code <= -32000 {
		c = "SERVER_ERROR"
	}
	return "JSON_RPC_CODE_BUCKET", c
}
func privateProbePhase(method string) string {
	if method == "textDocument/documentSymbol" {
		return "DOCUMENT_SYMBOL"
	}
	return "PROVIDER_REQUEST"
}
