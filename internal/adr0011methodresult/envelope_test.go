package adr0011methodresult

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func TestADR0011ProvisionalEnvelopeIsNotARequestDenominator(t *testing.T) {
	const location = `{"uri":"file:///w/a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	const duplicate = `{"uri":"file:///w/b.go","uri":"file:///w/c.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`
	for _, tc := range []struct {
		name, method, raw string
		limit             int
		capabilityAbsent  bool
		stale             bool
		timeout           bool
		absentRaw         bool
		presentEmptyRaw   bool
		outcome           transport.Outcome
		known             bool
		form              EnvelopeForm
		count             int
		reason            EnvelopeUnknownReason
		parseCode         Code
		parseOrdinal      int
		parsedItems       int
	}{
		{name: "definition-null", method: transport.MethodDefinition, raw: `null`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeNull},
		{name: "references-empty", method: transport.MethodReferences, raw: `[]`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeArray},
		{name: "definition-empty", method: transport.MethodDefinition, raw: `[]`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeArray},
		{name: "definition-scalar", method: transport.MethodDefinition, raw: location, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeDefinitionScalar, count: 1, parsedItems: 1},
		{name: "definition-repeated", method: transport.MethodDefinition, raw: `[` + location + `,` + location + `]`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeArray, count: 2, parsedItems: 2},
		{name: "exact-element-limit", method: transport.MethodDefinition, raw: `[` + location + `,` + location + `]`, limit: 2, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeArray, count: 2, parsedItems: 2},
		{name: "invalid-definition-scalar-fields", method: transport.MethodDefinition, raw: `{"uri":"file:///w/a.go"}`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeDefinitionScalar, count: 1, parseCode: FailureMalformed, parseOrdinal: 0},
		{name: "malformed-second-of-three", method: transport.MethodReferences, raw: `[` + location + `,` + duplicate + `,` + location + `]`, limit: 3, outcome: transport.OutcomeTransportSuccess, known: true, form: EnvelopeArray, count: 3, parseCode: FailureMalformed, parseOrdinal: 1},
		{name: "truncated-array", method: transport.MethodDefinition, raw: `[` + location + `,`, limit: 3, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeMalformed, parseCode: FailureMalformed, parseOrdinal: -1},
		{name: "over-limit-array", method: transport.MethodDefinition, raw: `[` + location + `,` + location + `]`, limit: 1, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeOverLimit, parseCode: FailureResource, parseOrdinal: 1},
		{name: "capability-absent", method: transport.MethodReferences, raw: `[]`, limit: 3, capabilityAbsent: true, outcome: transport.OutcomeUnsupportedCapability, reason: EnvelopeNonSuccess, parseCode: FailureTransport, parseOrdinal: -1},
		{name: "stale-generation", method: transport.MethodDefinition, raw: `[]`, limit: 3, stale: true, outcome: transport.OutcomePreflightFailure, reason: EnvelopeNonSuccess, parseCode: FailureTransport, parseOrdinal: -1},
		{name: "timeout", method: transport.MethodDefinition, raw: `[]`, limit: 3, timeout: true, outcome: transport.OutcomeTimeout, reason: EnvelopeNonSuccess, parseCode: FailureTransport, parseOrdinal: -1},
		{name: "missing-success-result", method: transport.MethodDefinition, limit: 3, absentRaw: true, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeUnavailable, parseCode: FailureMalformed, parseOrdinal: -1},
		{name: "present-empty-success-result", method: transport.MethodDefinition, limit: 3, presentEmptyRaw: true, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeMalformed, parseCode: FailureMalformed, parseOrdinal: -1},
		{name: "references-scalar", method: transport.MethodReferences, raw: location, limit: 3, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeMalformed, parseCode: FailureMalformed, parseOrdinal: -1},
		{name: "invalid-element-limit", method: transport.MethodDefinition, raw: `[]`, limit: 0, outcome: transport.OutcomeTransportSuccess, reason: EnvelopeInvalidLimit, parseCode: FailureLimit, parseOrdinal: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := fixtureResponse(tc.raw)
			if tc.absentRaw {
				response = sessionruntime.RoundTripResult{Messages: 1, Bytes: 100}
			}
			if tc.presentEmptyRaw {
				response = sessionruntime.RoundTripResult{Result: json.RawMessage{}, Messages: 1, Bytes: 100}
			}
			if tc.timeout {
				response.Failure = session.RequestTimeout
			}
			metadata := sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}
			if tc.capabilityAbsent {
				metadata.ReferencesSupport = false
			}
			f := &fixtureRuntime{metadata: metadata, response: response}
			if tc.stale {
				f.metadataFailure = session.StaleGeneration
			}
			wire := transport.New(f).Execute(context.Background(), fixtureRequest(tc.method, false))
			if wire.Outcome() != tc.outcome {
				t.Fatalf("fixture setup: case=%s outcome=%s want=%s", tc.name, wire.Outcome(), tc.outcome)
			}
			beforeRaw, beforeOutcome := wire.Raw(), wire.Outcome()
			beforeParsed, beforeFailure := Parse(wire, tc.limit)
			got := ObserveEnvelope(wire, tc.limit)
			if got.Reason != tc.reason || !got.Known && (got.Count != 0 || got.Form != "" || got.Reason == "") ||
				!tc.known && got.Known {
				t.Fatalf("ASSERT_ADR0011_ENVELOPE_UNKNOWN_NOT_ZERO: case=%s got=%+v wantReason=%s", tc.name, got, tc.reason)
			}
			if got.Known != tc.known || got.Form != tc.form || got.Count != tc.count {
				t.Fatalf("ASSERT_ADR0011_ENVELOPE_RESULT_COUNT: case=%s got=%+v wantKnown=%v wantForm=%s wantCount=%d parserItems=%d", tc.name, got, tc.known, tc.form, tc.count, len(beforeParsed.Items))
			}
			if len(beforeParsed.Items) != tc.parsedItems ||
				(tc.parseCode == "") != (beforeFailure == nil) ||
				(beforeFailure != nil && (beforeFailure.Code != tc.parseCode || beforeFailure.Ordinal != tc.parseOrdinal)) {
				t.Fatalf("fixture parser: case=%s items=%d failure=%+v", tc.name, len(beforeParsed.Items), beforeFailure)
			}
			afterParsed, afterFailure := Parse(wire, tc.limit)
			if wire.Outcome() != beforeOutcome || !reflect.DeepEqual(wire.Raw(), beforeRaw) ||
				!reflect.DeepEqual(afterParsed, beforeParsed) || !reflect.DeepEqual(afterFailure, beforeFailure) {
				t.Fatalf("ASSERT_ADR0011_ENVELOPE_NONINTERFERENCE: case=%s before=%+v/%+v after=%+v/%+v", tc.name, beforeParsed, beforeFailure, afterParsed, afterFailure)
			}
			t.Logf("ASSERT_ADR0011_ENVELOPE_RESULT_COUNT: PASS case=%s", tc.name)
			t.Logf("ASSERT_ADR0011_ENVELOPE_UNKNOWN_NOT_ZERO: PASS case=%s", tc.name)
			t.Logf("ASSERT_ADR0011_ENVELOPE_NONINTERFERENCE: PASS case=%s", tc.name)
		})
	}
}
